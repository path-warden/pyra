package cli

import (
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/parse"
	"github.com/chasedputnam/pyra/internal/canon/validate"
)

func TestScaffold_RoundTripsCleanForEveryType(t *testing.T) {
	reg := artifacts.Default()
	for _, typ := range reg.Types() {
		spec := reg[typ]
		content := scaffold("OKF-0123456789AB", typ, "Test Title", spec)

		p := parse.Parse([]byte(content))
		c := classify.Classify(p, reg)
		if c.Type != typ {
			t.Errorf("%s: scaffold classified as %q (conf %.2f)", typ, c.Type, c.Confidence)
		}
		issues := validate.Validate(p, c, validate.Options{})
		for _, iss := range issues {
			// A freshly scaffolded artifact must not have structural errors.
			if iss.Severity == "error" {
				t.Errorf("%s: scaffold produced error %s: %s", typ, iss.Code, iss.Message)
			}
		}
		if !strings.Contains(content, "id: OKF-0123456789AB") {
			t.Errorf("%s: id not written into frontmatter", typ)
		}
	}
}

func TestTitleize(t *testing.T) {
	cases := map[string]string{
		"context":           "Context",
		"related decisions": "Related Decisions",
		"status":            "Status",
	}
	for in, want := range cases {
		if got := titleize(in); got != want {
			t.Errorf("titleize(%q)=%q want %q", in, got, want)
		}
	}
}

func TestHumanizeFilename(t *testing.T) {
	if got := humanizeFilename("/x/adr-001-markdown-first.md"); got != "Adr 001 Markdown First" {
		t.Errorf("got %q", got)
	}
}
