package cli

import (
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/parse"
)

func TestPromoteScaffold_SeedsBodyIntoProseSection(t *testing.T) {
	reg := artifacts.Default()
	body := "This is the original ingested guide content about caching."
	content := promoteScaffold("OKF-0123456789AB", "decision", "Caching", body, reg["decision"])

	if !strings.Contains(content, body) {
		t.Errorf("source body not seeded into artifact:\n%s", content)
	}
	p := parse.Parse([]byte(content))
	c := classify.Classify(p, reg)
	if c.Type != "decision" {
		t.Errorf("promoted artifact misclassified as %q", c.Type)
	}
	// The body should land in Context (decision's primary prose section).
	if ctx, ok := p.Section("context"); !ok || !strings.Contains(ctx, body) {
		t.Errorf("body not in context section: ok=%v body=%q", ok, ctx)
	}
}

func TestPrimaryProseSection(t *testing.T) {
	reg := artifacts.Default()
	if got := primaryProseSection(reg["decision"]); got != "context" {
		t.Errorf("decision prose section: got %q", got)
	}
	// requirement's first required prose section (not status/requirements) is problem.
	if got := primaryProseSection(reg["requirement"]); got != "problem" {
		t.Errorf("requirement prose section: got %q", got)
	}
}

func TestStripConceptChrome(t *testing.T) {
	in := "# A Title\n\n> [!summary]\n> a summary line\n\nReal content here.\n"
	got := stripConceptChrome(in)
	if strings.Contains(got, "# A Title") {
		t.Errorf("leading title not stripped: %q", got)
	}
	if strings.Contains(got, "summary") {
		t.Errorf("summary callout not stripped: %q", got)
	}
	if got != "Real content here." {
		t.Errorf("got %q", got)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Caching Decision":  "caching-decision",
		"  Hello, World!  ": "hello-world",
		"":                  "fallback-id",
	}
	for in, want := range cases {
		if got := slug(in, "FALLBACK-ID"); got != want {
			t.Errorf("slug(%q)=%q want %q", in, got, want)
		}
	}
}
