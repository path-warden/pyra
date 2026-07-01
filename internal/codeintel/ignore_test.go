package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalk_HonorsRootGitignoreFromSubdir(t *testing.T) {
	root := t.TempDir()
	// Root .gitignore ignores the vendor/ subtree and any node_modules dir.
	mustWrite(t, root, ".gitignore", "vendored/\nnode_modules\n")
	mustWrite(t, root, "app/keep.go", "package app\nfunc Keep() {}\n")
	mustWrite(t, root, "app/vendored/skip.go", "package v\nfunc Skip() {}\n")
	mustWrite(t, root, "app/node_modules/dep.go", "package d\nfunc Dep() {}\n")

	o := NewOps(nil, root)
	// Search the subdirectory: the ROOT .gitignore must still apply.
	syms, err := o.Symbols(filepath.Join(root, "app"), "", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range syms {
		if s.Name == "Skip" {
			t.Errorf("vendored/ (root .gitignore) should be ignored, got %s", s.ID)
		}
		if s.Name == "Dep" {
			t.Errorf("node_modules (root .gitignore) should be ignored, got %s", s.ID)
		}
	}
	if !hasSymbol(syms, "Keep") {
		t.Errorf("expected Keep to be found, got %v", ids(syms))
	}
}

func TestWalk_HonorsNestedGitignore(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "pkg/keep.go", "package pkg\nfunc Keep() {}\n")
	mustWrite(t, root, "pkg/.gitignore", "*.gen.go\n")
	mustWrite(t, root, "pkg/thing.gen.go", "package pkg\nfunc Generated() {}\n")

	o := NewOps(nil, root)
	syms, err := o.Symbols(root, "", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if hasSymbol(syms, "Generated") {
		t.Errorf("*.gen.go (nested .gitignore) should be ignored, got %v", ids(syms))
	}
	if !hasSymbol(syms, "Keep") {
		t.Errorf("expected Keep, got %v", ids(syms))
	}
}

func TestConfinedPath_RejectsEscape(t *testing.T) {
	rootA := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, outside, "secret.go", "package s\nfunc Secret() {}\n")

	o := NewOps(nil, rootA)
	escape := filepath.Join(outside, "secret.go")

	if _, err := o.Outline(escape, "", 1); err == nil {
		t.Error("Outline of a file outside the root should be rejected")
	}
	if _, err := o.Check(escape); err == nil {
		t.Error("Check of a file outside the root should be rejected")
	}
	if _, err := o.Source(escape, "Secret"); err == nil {
		t.Error("Source of a file outside the root should be rejected")
	}
}

func TestConfinedPath_AllowsInsideRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "pkg/a.go", "package pkg\nfunc A() {}\n")
	o := NewOps(nil, root)
	// A store-relative path must resolve against the root (not the CWD).
	rows, err := o.Outline("pkg/a.go", "", 0)
	if err != nil {
		t.Fatalf("relative path under root should resolve: %v", err)
	}
	if len(rows) == 0 || rows[0]["name"] != "A" {
		t.Errorf("expected A, got %v", rows)
	}
}

// --- helpers ---

func mustWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasSymbol(syms []Symbol, name string) bool {
	for _, s := range syms {
		if s.Name == name {
			return true
		}
	}
	return false
}

func ids(syms []Symbol) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = s.ID
	}
	return out
}
