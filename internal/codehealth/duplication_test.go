package codehealth

import (
	"strings"
	"testing"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

func TestClones_PlantedDuplicateFound(t *testing.T) {
	root := t.TempDir()
	// Two files with an identical, sizeable block.
	block := "\tx := 0\n\tfor i := 0; i < 100; i++ {\n\t\tx = x + i*2 - 1\n\t\tif x > 50 {\n\t\t\tx = x / 2\n\t\t}\n\t}\n"
	writeFile(t, root, "a.go", "package p\n\nfunc A() {\n"+block+"}\n")
	writeFile(t, root, "b.go", "package p\n\nfunc B() {\n"+block+"}\n")
	ops := codeintel.NewOps(nil, root)

	contexts, _ := buildContexts(&Inputs{Ops: ops, Roots: []string{root}, Root: root})
	clones := detectClones(&Inputs{Root: root}, contexts)
	if len(clones["a.go"]) == 0 || len(clones["b.go"]) == 0 {
		t.Fatalf("planted clone not found: %v", clones)
	}
	if clones["a.go"][0].Biomarker != "dry_violation" {
		t.Errorf("expected dry_violation, got %s", clones["a.go"][0].Biomarker)
	}
}

func TestClones_NoFalsePositiveBelowWindow(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package p\n\nfunc A() int { return 1 }\n")
	writeFile(t, root, "b.go", "package p\n\nfunc B() int { return 2 }\n")
	ops := codeintel.NewOps(nil, root)
	contexts, _ := buildContexts(&Inputs{Ops: ops, Roots: []string{root}, Root: root})
	if clones := detectClones(&Inputs{Root: root}, contexts); len(clones) != 0 {
		t.Errorf("short distinct files should not clone: %v", clones)
	}
}

func TestErrorHandling_Detected(t *testing.T) {
	fc, _ := metricsCtx(t, "e.go", "package p\n\nfunc f() {\n\t_ = risky()\n}\n\nfunc risky() error { return nil }\n", 0)
	got := errorHandling(fc, nil)
	if len(got) != 1 || got[0].Biomarker != "error_handling" {
		t.Errorf("expected error_handling, got %v", got)
	}
	if !strings.Contains(got[0].Details, "ignored_error") {
		t.Errorf("details should name the pattern: %q", got[0].Details)
	}
}

func TestLargeAssertionBlock_TestFileOnly(t *testing.T) {
	var b strings.Builder
	b.WriteString("package p\n\nfunc TestBig(t *T) {\n")
	for i := 0; i < 55; i++ {
		b.WriteString("\tassert(x)\n")
	}
	b.WriteString("}\n")
	// In a test file → fires.
	fc, _ := metricsCtx(t, "x_test.go", b.String(), 0)
	if len(largeAssertionBlock(fc, nil)) == 0 {
		t.Error("large assertion block should fire in a test file")
	}
	// Same content in a non-test file → does not fire.
	fc2, _ := metricsCtx(t, "x.go", b.String(), 0)
	if len(largeAssertionBlock(fc2, nil)) != 0 {
		t.Error("large_assertion_block should only fire in test files")
	}
}

func TestErrorHandling_ReportsEveryOffendingFunction(t *testing.T) {
	fc, _ := metricsCtx(t, "e.go",
		"package p\n\nfunc a() { _ = risky() }\n\nfunc b() { _ = risky() }\n\nfunc risky() error { return nil }\n", 0)
	got := errorHandling(fc, nil)
	if len(got) != 2 {
		t.Errorf("both offending functions should be reported, got %d: %+v", len(got), got)
	}
}
