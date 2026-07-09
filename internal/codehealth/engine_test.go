package codehealth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileByPath(r Report, name string) *FileHealth {
	for i := range r.Files {
		if r.Files[i].Path == name {
			return &r.Files[i]
		}
	}
	return nil
}

func TestAnalyze_RankingKPIsAndSuggestion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "bad.go", "package p\n\nfunc A() {}\n")
	writeFile(t, root, "good.go", "package p\n\nfunc B() {}\n")
	ops := codeintel.NewOps(nil, root)

	// Inject a detector that flags bad.go with a critical god_class.
	inject := func(fc *FileContext, in *Inputs) []Finding {
		if fc.Path == "bad.go" {
			return []Finding{{Biomarker: "god_class", Severity: "critical", File: fc.Path}}
		}
		return nil
	}
	rep, err := Analyze(Inputs{Ops: ops, Roots: []string{root}, detectors: []Detector{inject}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(rep.Files))
	}
	// bad.go ranks first (lowest defect) and carries the suggestion.
	if rep.Files[0].Path != "bad.go" {
		t.Errorf("worst file = %s, want bad.go", rep.Files[0].Path)
	}
	bad := fileByPath(rep, "bad.go")
	if bad.Defect >= 10.0 {
		t.Errorf("bad.go defect = %v, want < 10", bad.Defect)
	}
	if bad.TopMarker != "god_class" || bad.Suggestion != "Extract Class" {
		t.Errorf("suggestion = %q/%q, want god_class/Extract Class", bad.TopMarker, bad.Suggestion)
	}
	good := fileByPath(rep, "good.go")
	if good.Defect != 10.0 || len(good.Findings) != 0 {
		t.Errorf("good.go should be clean 10.0, got %v", good.Defect)
	}
	if rep.Worst == nil || rep.Worst.Path != "bad.go" {
		t.Errorf("worst = %+v, want bad.go", rep.Worst)
	}
	// Average health is NLOC-weighted and below 10 (bad.go drags it).
	if rep.AverageHealth >= 10.0 {
		t.Errorf("average health = %v, want < 10", rep.AverageHealth)
	}
}

func TestAnalyze_EmptyRepo(t *testing.T) {
	root := t.TempDir()
	ops := codeintel.NewOps(nil, root)
	rep, err := Analyze(Inputs{Ops: ops, Roots: []string{root}, detectors: []Detector{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 0 || rep.AverageHealth != 10.0 {
		t.Errorf("empty repo = %+v, want no files, avg 10", rep)
	}
}

func TestAnalyze_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package p\n\nfunc A() {}\n")
	writeFile(t, root, "b.go", "package p\n\nfunc B() {}\n")
	ops := codeintel.NewOps(nil, root)
	det := []Detector{func(fc *FileContext, in *Inputs) []Finding {
		return []Finding{{Biomarker: "churn_risk", Severity: "high", File: fc.Path}}
	}}
	first, _ := Analyze(Inputs{Ops: ops, Roots: []string{root}, detectors: det})
	for i := 0; i < 3; i++ {
		again, _ := Analyze(Inputs{Ops: ops, Roots: []string{root}, detectors: det})
		if again.Files[0].Path != first.Files[0].Path || again.AverageHealth != first.AverageHealth {
			t.Fatal("analyze not deterministic")
		}
	}
}
