package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/tokens"
)

func TestEngine_SelectsAPIProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(chatResponse{Choices: []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: "summary text"}}}})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cfg := "api_endpoint: " + srv.URL + "\napi_token: tok\nmodel: gpt-4o-mini\n"
	if err := os.WriteFile(filepath.Join(tmp, ConfigFileName), []byte(cfg), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("HOME", tmp)

	e, err := NewEngine(tmp)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if e.ProviderName() != "api" {
		t.Errorf("expected 'api' provider, got %q", e.ProviderName())
	}
	got, err := e.Summarize("Some doc content here.", "Doc Title")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got != "summary text" {
		t.Errorf("got %q", got)
	}
}

func TestEngine_FallsBackToLocalProvider(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	e, err := NewEngine(tmp)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	name := e.ProviderName()
	switch runtime.GOOS {
	case "darwin":
		if name != "apple" && name != "ollama" {
			t.Errorf("expected apple or ollama, got %q", name)
		}
	case "windows":
		if name != "windows" && name != "ollama" {
			t.Errorf("expected windows or ollama, got %q", name)
		}
	default:
		if name != "ollama" {
			t.Errorf("expected ollama, got %q", name)
		}
	}
}

func TestEngine_CustomPromptTemplate(t *testing.T) {
	receivedPrompt := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) > 0 {
			receivedPrompt = req.Messages[0].Content
		}
		body, _ := json.Marshal(chatResponse{Choices: []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: "x"}}}})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	cfg := "api_endpoint: " + srv.URL + "\napi_token: tok\nprompt_template: \"BRIEFLY: {{.Title}} | {{.Content}}\"\n"
	if err := os.WriteFile(filepath.Join(tmp, ConfigFileName), []byte(cfg), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	e, err := NewEngine(tmp)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	_, err = e.Summarize("body text", "MyTitle")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if !strings.Contains(receivedPrompt, "BRIEFLY: MyTitle | body text") {
		t.Errorf("custom template not used, got %q", receivedPrompt)
	}
}

func TestIntelligentTruncate_UnderBudgetReturnsAsIs(t *testing.T) {
	est := tokens.NewEstimator()
	in := "# Title\n\nShort body that comfortably fits.\n"
	got := intelligentTruncate(in, 1000, est)
	if got != in {
		t.Errorf("expected passthrough, got %q", got)
	}
}

// TestIntelligentTruncate_PreservesHeadingsAndFirstParas verifies the spec
// requirement: when content overflows the token budget, we keep headings and
// the first paragraph under each heading rather than just chopping at a byte
// offset. We build a document where the first paragraphs are short cues
// ("KEEP-N") and the bodies are long noise ("DROP-N"). The reduced output
// must keep every "KEEP-N" and drop every "DROP-N".
func TestIntelligentTruncate_PreservesHeadingsAndFirstParas(t *testing.T) {
	est := tokens.NewEstimator()
	var sb strings.Builder
	for i := 1; i <= 4; i++ {
		sb.WriteString("## Section ")
		sb.WriteString(string(rune('A' + i - 1)))
		sb.WriteString("\n\n")
		sb.WriteString("KEEP-")
		sb.WriteString(string(rune('A' + i - 1)))
		sb.WriteString(" first paragraph cue text.\n\n")
		// Noise paragraph that should be dropped.
		sb.WriteString(strings.Repeat("DROP-noise filler. ", 80))
		sb.WriteString("\n\n")
	}
	input := sb.String()

	// Pick a budget that comfortably fits headings + first paragraphs but
	// not the noise paragraphs. The full doc is well over this.
	if est.Count(input) < 200 {
		t.Fatalf("test fixture too small: %d tokens", est.Count(input))
	}
	budget := 120
	got := intelligentTruncate(input, budget, est)

	for _, letter := range []string{"A", "B", "C", "D"} {
		if !strings.Contains(got, "Section "+letter) {
			t.Errorf("missing heading 'Section %s' in reduced output:\n%s", letter, got)
		}
		if !strings.Contains(got, "KEEP-"+letter) {
			t.Errorf("missing first-paragraph cue 'KEEP-%s' in reduced output:\n%s", letter, got)
		}
	}
	if strings.Contains(got, "DROP-noise") {
		t.Errorf("noise paragraph leaked into reduced output:\n%s", got)
	}
	if est.Count(got) > budget {
		t.Errorf("reduced output exceeds budget: %d > %d", est.Count(got), budget)
	}
}

// TestIntelligentTruncate_FallsBackToHardTruncation verifies the last-resort
// path: when even the heading-only reduction overflows, we fall back to a
// token-level truncation and emit the truncation marker.
func TestIntelligentTruncate_FallsBackToHardTruncation(t *testing.T) {
	est := tokens.NewEstimator()
	// No headings — splitSections produces a single preamble section, and
	// its first paragraph is the entire document. assembleReduction can't
	// reduce further, so we fall back.
	body := strings.Repeat("word ", 5000)
	got := intelligentTruncate(body, 200, est)

	if !strings.HasSuffix(got, "[... content truncated ...]") {
		t.Errorf("expected truncation marker, got tail: %q", got[max(0, len(got)-60):])
	}
	if est.Count(got) > 250 { // budget plus marker overhead
		t.Errorf("hard-truncated output too long: %d tokens", est.Count(got))
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
