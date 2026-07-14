package canon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/validate"
	"github.com/chasedputnam/pyra/internal/config"
)

// Conformance fixtures are verbatim copies of real rac-core artifacts under
// testdata/conformance. These tests pin pyra's Canon engine to rac-core's
// actual on-disk format, guarding against semantic drift between the two
// implementations (the largest risk identified in the design).

// expectedTypes maps a fixture filename to the type rac-core assigns it.
var expectedTypes = map[string]string{
	"adr-001-markdown-first.md":   "decision",
	"adr-002-ai-optional.md":      "decision",
	"adr-004-artifact-model.md":   "decision",
	"rac-artifact-trust-model.md": "requirement",
}

func loadConformanceCorpus(t *testing.T) []Artifact {
	t.Helper()
	src := filepath.Join("testdata", "conformance")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	// Stage into a temp store under a canon root so LoadCorpus picks them up.
	root := t.TempDir()
	canonDir := filepath.Join(root, "canon")
	if err := os.MkdirAll(canonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(canonDir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	arts, err := LoadCorpus(root, config.Default())
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	return arts
}

func TestConformance_TypeInferenceMatchesRacCore(t *testing.T) {
	arts := loadConformanceCorpus(t)
	if len(arts) != len(expectedTypes) {
		t.Fatalf("expected %d fixtures, loaded %d", len(expectedTypes), len(arts))
	}
	byName := map[string]Artifact{}
	for _, a := range arts {
		byName[filepath.Base(a.Path)] = a
	}
	for name, want := range expectedTypes {
		a, ok := byName[name]
		if !ok {
			t.Errorf("fixture %s not loaded", name)
			continue
		}
		if a.Type != want {
			t.Errorf("%s: classified as %q, rac-core type is %q (conf %.2f)", name, a.Type, want, a.Confidence)
		}
	}
}

func TestConformance_RealArtifactsHaveNoErrors(t *testing.T) {
	arts := loadConformanceCorpus(t)
	for _, a := range arts {
		// Frontmatter on real rac-core artifacts must parse cleanly.
		for _, iss := range a.LoadIssues {
			if iss.Severity == model.SeverityError {
				t.Errorf("%s: unexpected load error %s: %s", a.Path, iss.Code, iss.Message)
			}
		}
		// rac-core's own artifacts must produce no error-severity findings under
		// the ported validator (warnings such as not-singular are allowed).
		for _, iss := range validate.Validate(a.Product, a.Classification, validate.Options{}) {
			if iss.Severity == model.SeverityError {
				t.Errorf("%s: validation error drift [%s]: %s", a.Path, iss.Code, iss.Message)
			}
		}
	}
}

func TestConformance_OpaqueIDsValidate(t *testing.T) {
	arts := loadConformanceCorpus(t)
	for _, a := range arts {
		for _, iss := range a.LoadIssues {
			if iss.Code == "invalid_id" {
				t.Errorf("%s: rac-core opaque id rejected: %s", a.Path, iss.Message)
			}
		}
	}
}
