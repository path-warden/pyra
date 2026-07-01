package codeintel

import "testing"

// TestAllLanguages_TagsCompileAndExtract guards against grammar/query version
// drift: every provisioned language's tags query must compile against its
// gotreesitter grammar and extract the expected definition.
func TestAllLanguages_TagsCompileAndExtract(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		src     string
		defName string
		defKind string
	}{
		{"go", "a.go", "package p\nfunc Alpha() {}\n", "Alpha", "function"},
		{"python", "a.py", "def beta():\n    pass\n", "beta", "function"},
		{"javascript", "a.js", "function gamma() {}\n", "gamma", "function"},
		{"typescript", "a.ts", "function delta(): void {}\n", "delta", "function"},
		{"tsx", "a.tsx", "function epsilon() { return null; }\n", "epsilon", "function"},
		{"java", "A.java", "class Zeta {\n  void eta() {}\n}\n", "Zeta", "class"},
		{"rust", "a.rs", "fn theta() {}\n", "theta", "function"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, file := writeFixture(t, tc.file, tc.src)
			o := NewOps(nil, dir)
			syms, _, err := o.eng.Extract(file, tc.file)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			found := false
			for _, s := range syms {
				if s.IsDefinition && s.Name == tc.defName {
					found = true
					if s.Kind != tc.defKind {
						t.Errorf("%s: kind = %q, want %q", tc.defName, s.Kind, tc.defKind)
					}
				}
			}
			if !found {
				t.Errorf("did not extract definition %q from %s (got %+v)", tc.defName, tc.name, syms)
			}
		})
	}
}

// TestPython_LocalsAndImportsCompile ensures the optional queries either compile
// (become usable) or degrade to nil without error — never break extraction.
func TestPython_LocalsAndImportsCompile(t *testing.T) {
	lang, err := DefaultRegistry().ForFile("x.py")
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(nil)
	cq, err := eng.compiled(lang)
	if err != nil {
		t.Fatalf("tags must compile: %v", err)
	}
	_ = cq.locals  // may be nil (graceful)
	_ = cq.imports // may be nil (graceful)
}
