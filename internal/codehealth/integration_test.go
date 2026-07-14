package codehealth

import (
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/codeintel"
)

// TestAnalyze_FullRosterSeededUnhealthyFile drives the whole engine with the
// default roster over a repo containing one clearly-unhealthy file and asserts it
// ranks worst with a structural top marker and a suggestion.
func TestAnalyze_FullRosterSeededUnhealthyFile(t *testing.T) {
	root := t.TempDir()

	// A god-file: many top-level definitions, one huge deeply-nested function.
	var b strings.Builder
	b.WriteString("package p\n")
	for i := 0; i < 25; i++ {
		b.WriteString("func F")
		b.WriteByte(byte('a' + i%26))
		b.WriteString("() {}\n")
	}
	b.WriteString("\nfunc huge(a int) int {\n")
	for i := 0; i < 80; i++ {
		b.WriteString("\tif a > 0 { if a > 1 { if a > 2 { if a > 3 { if a > 4 { a++ } } } } }\n")
	}
	b.WriteString("\treturn a\n}\n")
	writeFile(t, root, "bad.go", b.String())
	writeFile(t, root, "good.go", "package p\n\nfunc Ok() int { return 1 }\n")

	ops := codeintel.NewOps(nil, root)
	rep, err := Analyze(Inputs{Ops: ops, Roots: []string{root}, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(rep.Files))
	}
	bad := rep.Files[0]
	if bad.Path != "bad.go" {
		t.Fatalf("worst file = %s, want bad.go", bad.Path)
	}
	if bad.Defect >= 9.0 {
		t.Errorf("bad.go defect = %v, want clearly unhealthy", bad.Defect)
	}
	if bad.Maintainability >= 10.0 {
		t.Errorf("bad.go maintainability should be dinged, got %v", bad.Maintainability)
	}
	// It should carry structural findings and a refactoring suggestion.
	marks := biomarkers(bad.Findings)
	if !marks["god_file"] && !marks["large_method"] && !marks["complex_method"] {
		t.Errorf("expected structural findings on bad.go, got %v", marks)
	}
	if bad.Suggestion == "" {
		t.Errorf("expected a refactoring suggestion, got none (top=%s)", bad.TopMarker)
	}
	// good.go stays healthy.
	for _, f := range rep.Files {
		if f.Path == "good.go" && f.Defect != 10.0 {
			t.Errorf("good.go should be 10.0, got %v", f.Defect)
		}
	}
	// Performance dimension is present-but-empty everywhere.
	if bad.Performance != 10.0 {
		t.Errorf("performance should be 10.0 (no detectors), got %v", bad.Performance)
	}
}
