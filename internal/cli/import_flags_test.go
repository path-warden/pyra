package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/chasedputnam/pyra/internal/changelog"
)

// resetCommandFlags returns a cobra command's flags to their declared defaults
// and clears the Changed bit. Necessary because rootCmd is a package singleton
// shared across tests — values set by a previous Execute() otherwise leak into
// the next one.
func resetCommandFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
}

// writeImportSource writes a small markdown corpus into dir for import tests.
func writeImportSource(t *testing.T, dir string) {
	t.Helper()
	body := "# Doc\n\nFirst sentence about the topic. Second sentence with more detail. Third sentence wraps it up.\n"
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
}

// TestImportCLI_ForwardsSummarizeAlgorithmFlag verifies the --summarize-algorithm
// flag on the import subcommand reaches the importer and is persisted to the
// changelog. The CLI is the only layer that wires this flag, so this test
// guards against regressions in the cobra/runImport plumbing.
func TestImportCLI_ForwardsSummarizeAlgorithmFlag(t *testing.T) {
	resetCommandFlags(importCmd)
	t.Setenv("HOME", t.TempDir())

	src := t.TempDir()
	writeImportSource(t, src)
	out := filepath.Join(t.TempDir(), "bundle")

	rootCmd.SetArgs([]string{
		"import", src,
		"--out", out,
		"--stable-timestamps",
		"--summarize", "extractive",
		"--summarize-algorithm", "luhn",
		"--language", "english",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	cl, err := changelog.ReadChangelog(out)
	if err != nil {
		t.Fatalf("ReadChangelog: %v", err)
	}
	if cl.SummarizeAlgorithm != "luhn" {
		t.Errorf("SummarizeAlgorithm = %q, want %q", cl.SummarizeAlgorithm, "luhn")
	}
	if cl.SummarizeMode != "extractive" {
		t.Errorf("SummarizeMode = %q, want %q", cl.SummarizeMode, "extractive")
	}

	// Sanity: the bundle exists and has a summary callout, proving the flag
	// not only landed in the changelog but also drove summarization.
	bodyBytes, err := os.ReadFile(filepath.Join(out, "doc.md"))
	if err != nil {
		t.Fatalf("read doc.md: %v", err)
	}
	if !strings.Contains(string(bodyBytes), "> [!summary]") {
		t.Errorf("expected summary callout in imported doc")
	}
}

// TestImportCLI_ForwardsLanguageFlag verifies the --language flag is wired
// from cobra into the importer and persisted to the changelog.
func TestImportCLI_ForwardsLanguageFlag(t *testing.T) {
	resetCommandFlags(importCmd)
	t.Setenv("HOME", t.TempDir())

	src := t.TempDir()
	writeImportSource(t, src)
	out := filepath.Join(t.TempDir(), "bundle")

	// Use a language with no embedded stopword data (french); the importer
	// degrades gracefully — what matters here is that the flag reaches the
	// changelog so a future update reuses the same setting.
	rootCmd.SetArgs([]string{
		"import", src,
		"--out", out,
		"--stable-timestamps",
		"--summarize", "extractive",
		"--summarize-algorithm", "lsa",
		"--language", "french",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	cl, err := changelog.ReadChangelog(out)
	if err != nil {
		t.Fatalf("ReadChangelog: %v", err)
	}
	if cl.Language != "french" {
		t.Errorf("Language = %q, want %q", cl.Language, "french")
	}
}

// TestImportCLI_LLMMode_EndToEnd verifies that --summarize=llm at the CLI
// layer reaches the importer, picks the API provider configured in
// ~/.config/pyra/llm.config (under a test HOME), and the LLM-produced text
// lands in the bundle. This is the only test that exercises the full
// CLI → importer → summarize/llm → provider stack.
func TestImportCLI_LLMMode_EndToEnd(t *testing.T) {
	resetCommandFlags(importCmd)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer cli-test-token" {
			t.Errorf("unexpected auth header: %q", got)
		}
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "CLI-routed LLM summary."}},
			},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// Place llm.config under a fake HOME so the writer doesn't wipe it
	// during bundle cleanup (it wipes only OutDir).
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "pyra")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := "api_endpoint: " + srv.URL + "\napi_token: cli-test-token\nmodel: gpt-4o-mini\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "llm.config"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write llm.config: %v", err)
	}
	t.Setenv("HOME", home)

	src := t.TempDir()
	writeImportSource(t, src)
	out := filepath.Join(t.TempDir(), "bundle")

	rootCmd.SetArgs([]string{
		"import", src,
		"--out", out,
		"--stable-timestamps",
		"--summarize", "llm",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if calls == 0 {
		t.Fatal("LLM provider was never called — --summarize=llm flag not plumbed through CLI")
	}

	bodyBytes, err := os.ReadFile(filepath.Join(out, "doc.md"))
	if err != nil {
		t.Fatalf("read doc.md: %v", err)
	}
	if !strings.Contains(string(bodyBytes), "CLI-routed LLM summary") {
		t.Errorf("LLM summary not present in bundle. Got:\n%s", string(bodyBytes))
	}

	cl, err := changelog.ReadChangelog(out)
	if err != nil {
		t.Fatalf("ReadChangelog: %v", err)
	}
	if cl.SummarizeMode != "llm" {
		t.Errorf("SummarizeMode = %q, want llm", cl.SummarizeMode)
	}
}
