package codehealth

import (
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/codeintel"
)

// metricsCtx builds a FileContext from a written source file.
func metricsCtx(t *testing.T, rel, src string, topLevel int) (*FileContext, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, rel, src)
	fm, err := codeintel.NewOps(nil, root).Metrics(rel)
	if err != nil {
		t.Fatal(err)
	}
	return &FileContext{Path: rel, Metrics: fm, TopLevel: topLevel}, root
}

func biomarkers(fs []Finding) map[string]bool {
	m := map[string]bool{}
	for _, f := range fs {
		m[f.Biomarker] = true
	}
	return m
}

func TestStructural_LargeComplexNested(t *testing.T) {
	// A function with a deep, branchy, long body.
	var b strings.Builder
	b.WriteString("package p\n\nfunc big(a int) int {\n")
	for i := 0; i < 70; i++ {
		b.WriteString("\tif a > 0 {\n\t\tif a > 1 {\n\t\t\tif a > 2 {\n\t\t\t\tif a > 3 {\n\t\t\t\t\tif a > 4 {\n\t\t\t\t\t\ta++\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n")
	}
	b.WriteString("\treturn a\n}\n")
	fc, _ := metricsCtx(t, "big.go", b.String(), 0)

	got := biomarkers(funcLevelStructural(fc, nil))
	for _, want := range []string{"large_method", "complex_method", "nested_complexity", "brain_method"} {
		if !got[want] {
			t.Errorf("expected %s on a big deep function; got %v", want, got)
		}
	}
}

func TestStructural_SmallCleanNoFindings(t *testing.T) {
	fc, _ := metricsCtx(t, "s.go", "package p\n\nfunc small(a int) int { return a }\n", 0)
	if got := funcLevelStructural(fc, nil); len(got) != 0 {
		t.Errorf("clean function should have no structural findings, got %v", got)
	}
}

func TestStructural_GodFileThreshold(t *testing.T) {
	if got := godFile(&FileContext{Path: "x.go", TopLevel: godFileDefs}, nil); len(got) != 1 {
		t.Errorf("god_file should fire at %d defs", godFileDefs)
	}
	if got := godFile(&FileContext{Path: "x.go", TopLevel: godFileDefs - 1}, nil); len(got) != 0 {
		t.Errorf("god_file should not fire below %d defs", godFileDefs)
	}
}

func TestStructural_ComplexConditional(t *testing.T) {
	fc, _ := metricsCtx(t, "c.go", "package p\n\nfunc f(a, b, c, d, e bool) bool {\n\treturn a && b || c && d || e\n}\n", 0)
	if !biomarkers(funcLevelStructural(fc, nil))["complex_conditional"] {
		t.Errorf("expected complex_conditional for many boolean operators")
	}
}

func TestStructural_PrimitiveObsession(t *testing.T) {
	fc, _ := metricsCtx(t, "p.go", "package p\n\nfunc f(a int, b int, c string, d bool, e float64) {}\n", 0)
	if !biomarkers(funcLevelStructural(fc, nil))["primitive_obsession"] {
		t.Errorf("expected primitive_obsession for 5 primitive params")
	}
}

func TestLCOM4_CohesiveVsSplit(t *testing.T) {
	// Cohesive: two methods share field "a".
	cohesive := []codeintel.FuncMetrics{
		{Name: "m1", FieldAccess: []string{"a"}},
		{Name: "m2", FieldAccess: []string{"a", "b"}},
	}
	if lcom4(cohesive) != 1 {
		t.Errorf("cohesive class LCOM4 = %d, want 1", lcom4(cohesive))
	}
	// Split: two methods touch disjoint fields → 2 components.
	split := []codeintel.FuncMetrics{
		{Name: "m1", FieldAccess: []string{"a"}},
		{Name: "m2", FieldAccess: []string{"b"}},
	}
	if lcom4(split) != 2 {
		t.Errorf("split class LCOM4 = %d, want 2", lcom4(split))
	}
}

func TestStructural_GoNoLowCohesionFalsePositive(t *testing.T) {
	// A Go struct with several methods must NOT fire low_cohesion (Go has no
	// field-access profile, so cohesion can't be measured).
	fc, _ := metricsCtx(t, "s.go",
		"package p\n\ntype T struct{ a, b int }\n\nfunc (t *T) M1() int { return t.a }\n\nfunc (t *T) M2() int { return t.b }\n\nfunc (t *T) M3() int { return 0 }\n", 0)
	for _, f := range classLevelStructural(fc, nil) {
		if f.Biomarker == "low_cohesion" {
			t.Errorf("Go code must not fire low_cohesion (no field-access data): %+v", f)
		}
	}
}
