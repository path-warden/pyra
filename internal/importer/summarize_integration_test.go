package importer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/changelog"
)

func writeSampleInputs(t *testing.T, dir string) {
	t.Helper()
	docs := map[string]string{
		"intro.md": `# Introduction

Penguins are flightless birds that live in cold environments. They are excellent swimmers and feed primarily on fish. Penguins have adapted to harsh polar climates.
`,
		"api.md": `# Photosynthesis

Photosynthesis is the process by which green plants convert sunlight into energy. Carbon dioxide and water are absorbed and oxygen is released. This process powers nearly all life on Earth.
`,
	}
	for name, body := range docs {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// TestImport_DefaultExtractive verifies the default extractive (LSA) path.
func TestImport_DefaultExtractive(t *testing.T) {
	src := t.TempDir()
	writeSampleInputs(t, src)
	out := filepath.Join(t.TempDir(), "bundle")

	res, err := Import(ImportOptions{
		InputPath:        src,
		OutDir:           out,
		StableTimestamps: true,
		// Defaults: extractive/lsa/english
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Documents) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(res.Documents))
	}

	// Verify summary callout is present and tagged.
	introBytes, err := os.ReadFile(filepath.Join(out, "intro.md"))
	if err != nil {
		t.Fatalf("read intro.md: %v", err)
	}
	intro := string(introBytes)
	if !strings.Contains(intro, "> [!summary]") {
		t.Errorf("intro.md missing summary callout")
	}

	// Verify changelog records the summarization config.
	cl, err := changelog.ReadChangelog(out)
	if err != nil {
		t.Fatalf("ReadChangelog: %v", err)
	}
	// Defaults were used (empty), so changelog should reflect empty (back-compat).
	if cl.SummarizeMode != "" && cl.SummarizeMode != "extractive" {
		t.Errorf("unexpected SummarizeMode: %q", cl.SummarizeMode)
	}
}

// TestImport_ExplicitAlgorithmRecordsConfig verifies that explicit flag
// values are persisted to the changelog.
func TestImport_ExplicitAlgorithmRecordsConfig(t *testing.T) {
	src := t.TempDir()
	writeSampleInputs(t, src)
	out := filepath.Join(t.TempDir(), "bundle")

	_, err := Import(ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "lexrank",
		Language:           "english",
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	cl, err := changelog.ReadChangelog(out)
	if err != nil {
		t.Fatalf("ReadChangelog: %v", err)
	}
	if cl.SummarizeMode != "extractive" {
		t.Errorf("SummarizeMode = %q", cl.SummarizeMode)
	}
	if cl.SummarizeAlgorithm != "lexrank" {
		t.Errorf("SummarizeAlgorithm = %q", cl.SummarizeAlgorithm)
	}
	if cl.Language != "english" {
		t.Errorf("Language = %q", cl.Language)
	}
}

// writeUserConfig writes an llm.config under a fake HOME directory and points
// HOME at it for the duration of the test.
func writeUserConfig(t *testing.T, contents string) {
	t.Helper()
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "pyra")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "llm.config"), []byte(contents), 0644); err != nil {
		t.Fatalf("write llm.config: %v", err)
	}
	t.Setenv("HOME", home)
}

// TestImport_LLMMode_WithMockedAPI runs the LLM path against a mocked OpenAI-
// compatible API. The llm.config is placed in a temporary HOME so it survives
// bundle directory creation/cleanup.
func TestImport_LLMMode_WithMockedAPI(t *testing.T) {
	src := t.TempDir()
	writeSampleInputs(t, src)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "Mocked summary from LLM."}},
			},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	writeUserConfig(t, "api_endpoint: "+srv.URL+"\napi_token: tok\nmodel: gpt-4o-mini\n")

	out := filepath.Join(t.TempDir(), "bundle")
	_, err := Import(ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "llm",
		SummarizeAlgorithm: "lsa",
		Language:           "english",
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	intro, err := os.ReadFile(filepath.Join(out, "intro.md"))
	if err != nil {
		t.Fatalf("read intro.md: %v", err)
	}
	if !strings.Contains(string(intro), "Mocked summary from LLM") {
		t.Errorf("expected LLM summary in intro, got: %s", string(intro))
	}
	if calls == 0 {
		t.Errorf("expected LLM provider to be called")
	}
}

// TestImport_EdmundsonWithExplicitConfig verifies that the --edmundson-config
// path is plumbed through and bonus/stigma/null words bias selection.
func TestImport_EdmundsonWithExplicitConfig(t *testing.T) {
	src := t.TempDir()
	// Two sentences. Without cues they'd score similarly; with "wonderful"
	// as a bonus word, sentence 2 should be preferred.
	body := "# Topic\n\nThis is the first sentence about cats. The second sentence is wonderful and explains everything.\n"
	if err := os.WriteFile(filepath.Join(src, "topic.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfgPath := filepath.Join(t.TempDir(), "edmundson.yaml")
	if err := os.WriteFile(cfgPath, []byte("bonus_words: [wonderful]\n"), 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	// Set HOME to an empty dir so no ambient ~/.config/pyra interferes.
	t.Setenv("HOME", t.TempDir())

	out := filepath.Join(t.TempDir(), "bundle")
	_, err := Import(ImportOptions{
		InputPath:           src,
		OutDir:              out,
		StableTimestamps:    true,
		SummarizeMode:       "extractive",
		SummarizeAlgorithm:  "edmundson",
		Language:            "english",
		EdmundsonConfigPath: cfgPath,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(out, "topic.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "> [!summary]") {
		t.Fatalf("expected summary callout")
	}
	if !strings.Contains(string(got), "wonderful") {
		t.Errorf("expected bonus-word sentence selected, got:\n%s", string(got))
	}
}

// TestImport_EdmundsonAutoDiscoversBundleConfig verifies that an
// edmundson.config inside the bundle output directory is auto-discovered.
// Because the bundle dir gets wiped on Import, we place the config in a fake
// HOME instead, exercising the second search location.
func TestImport_EdmundsonAutoDiscoversHomeConfig(t *testing.T) {
	src := t.TempDir()
	body := "# Topic\n\nFirst plain sentence. Another wonderful sentence with a cue word.\n"
	if err := os.WriteFile(filepath.Join(src, "topic.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "pyra")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "edmundson.config"), []byte("bonus_words: [wonderful]\n"), 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	t.Setenv("HOME", home)

	out := filepath.Join(t.TempDir(), "bundle")
	_, err := Import(ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "edmundson",
		Language:           "english",
		// No explicit EdmundsonConfigPath — should auto-discover.
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(out, "topic.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "wonderful") {
		t.Errorf("expected bonus-word sentence selected via auto-discovered config, got:\n%s", string(got))
	}
}

// TestImport_LLMFallback_ToExtractive verifies that when the LLM provider
// fails, the system falls back to extractive summarization.
func TestImport_LLMFallback_ToExtractive(t *testing.T) {
	src := t.TempDir()
	writeSampleInputs(t, src)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always 400 to force fallback.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	defer srv.Close()

	writeUserConfig(t, "api_endpoint: "+srv.URL+"\napi_token: tok\n")

	out := filepath.Join(t.TempDir(), "bundle")
	res, err := Import(ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "llm",
		SummarizeAlgorithm: "lsa",
		Language:           "english",
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// The summary should be present but tagged with an extractive source
	// (because LLM failed and we fell back).
	if res.Stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if res.Stats.Fallbacks == 0 {
		t.Errorf("expected fallbacks > 0, got %d", res.Stats.Fallbacks)
	}

	intro, err := os.ReadFile(filepath.Join(out, "intro.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(intro), "> [!summary]") {
		t.Errorf("expected summary callout despite LLM failure")
	}
}
