package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func funcByName(fm FileMetrics, name string) *FuncMetrics {
	for i := range fm.Funcs {
		if fm.Funcs[i].Name == name {
			return &fm.Funcs[i]
		}
	}
	return nil
}

func TestMetrics_GoCyclomaticNestingParams(t *testing.T) {
	dir := t.TempDir()
	src := `package p

func simple(a int) int { return a }

func branchy(a int, b string, c bool) int {
	if a > 0 {
		for i := 0; i < a; i++ {
			if b == "x" && c {
				return i
			}
		}
	}
	switch a {
	case 1:
		return 1
	case 2:
		return 2
	}
	return 0
}
`
	writeFile(t, dir, "p.go", src)
	ops := NewOps(nil, dir)
	fm, err := ops.Metrics("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if !fm.Supported {
		t.Fatal("go should be supported")
	}
	simple := funcByName(fm, "simple")
	if simple == nil || simple.Cyclomatic != 1 {
		t.Errorf("simple cyclomatic = %v, want 1", simple)
	}
	if simple.ParamCount != 1 || simple.PrimitiveArgs != 1 {
		t.Errorf("simple params = %d/%d, want 1/1", simple.ParamCount, simple.PrimitiveArgs)
	}
	b := funcByName(fm, "branchy")
	if b == nil {
		t.Fatal("branchy not found")
	}
	// 1 (base) + if + for + if + && + case1 + case2 = 7.
	if b.Cyclomatic != 7 {
		t.Errorf("branchy cyclomatic = %d, want 7", b.Cyclomatic)
	}
	// if > for > if = depth 3.
	if b.MaxNesting != 3 {
		t.Errorf("branchy nesting = %d, want 3", b.MaxNesting)
	}
	if b.ParamCount != 3 || b.PrimitiveArgs != 3 {
		t.Errorf("branchy params = %d/%d, want 3/3", b.ParamCount, b.PrimitiveArgs)
	}
}

func TestMetrics_GoIgnoredError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "e.go", "package p\n\nfunc f() {\n\t_ = risky()\n}\n\nfunc risky() error { return nil }\n")
	fm, _ := NewOps(nil, dir).Metrics("e.go")
	f := funcByName(fm, "f")
	if f == nil || len(f.ErrorPatterns) == 0 || f.ErrorPatterns[0] != "ignored_error" {
		t.Errorf("expected ignored_error pattern, got %+v", f)
	}
}

func TestMetrics_PythonFieldAccessAndComplexity(t *testing.T) {
	dir := t.TempDir()
	src := `class C:
    def m(self, x):
        if x and self.a:
            self.b = 1
        return self.a
`
	writeFile(t, dir, "c.py", src)
	fm, err := NewOps(nil, dir).Metrics("c.py")
	if err != nil {
		t.Fatal(err)
	}
	if !fm.Supported {
		t.Skip("python grammar not available")
	}
	m := funcByName(fm, "m")
	if m == nil {
		t.Fatal("method m not found")
	}
	// base + if + boolean_operator(and) = 3.
	if m.Cyclomatic != 3 {
		t.Errorf("m cyclomatic = %d, want 3", m.Cyclomatic)
	}
	// Accesses self.a and self.b.
	fa := map[string]bool{}
	for _, f := range m.FieldAccess {
		fa[f] = true
	}
	if !fa["a"] || !fa["b"] {
		t.Errorf("field access = %v, want a and b", m.FieldAccess)
	}
}

func TestMetrics_UnsupportedDegrades(t *testing.T) {
	dir := t.TempDir()
	// A language with no metrics profile (or unknown extension).
	writeFile(t, dir, "x.txt", "hello\n")
	fm, err := NewOps(nil, dir).Metrics("x.txt")
	// Either an extract error (unknown lang) or Supported=false — both are clean.
	if err == nil && fm.Supported {
		t.Errorf("unprofiled file should not be Supported: %+v", fm)
	}
}

func TestMetrics_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "p.go", "package p\n\nfunc f(a int) int {\n\tif a > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n")
	ops := NewOps(nil, dir)
	first, _ := ops.Metrics("p.go")
	for i := 0; i < 3; i++ {
		again, _ := ops.Metrics("p.go")
		if len(again.Funcs) != len(first.Funcs) || again.Funcs[0].Cyclomatic != first.Funcs[0].Cyclomatic {
			t.Fatal("metrics not deterministic")
		}
	}
}
