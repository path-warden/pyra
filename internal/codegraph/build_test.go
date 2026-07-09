package codegraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureRepo builds a small Go tree: store.Put is referenced by app.Use;
// util.Helper is unreferenced; two files each define Get (name collision).
func fixtureRepo(t *testing.T) (string, *codeintel.Ops) {
	t.Helper()
	root := t.TempDir()
	write(t, root, "store/store.go", "package store\n\nfunc Put() {}\n\nfunc Get() {}\n")
	write(t, root, "cache/cache.go", "package cache\n\nfunc Get() {}\n")
	write(t, root, "app/app.go", "package app\n\nfunc Use() { Put(); Get() }\n")
	write(t, root, "util/util.go", "package util\n\nfunc helper() {}\n")
	return root, codeintel.NewOps(nil, root)
}

func findByName(g *Graph, name string) []*SymbolNode {
	var out []*SymbolNode
	for _, id := range g.Order {
		if g.Symbols[id].Name == name {
			out = append(out, g.Symbols[id])
		}
	}
	return out
}

func TestBuild_NodesEdgesContainment(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, err := Build(ops, []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Symbol nodes: Put, Get(store), Get(cache), Use, helper = 5.
	if g.NodeCount() != 5 {
		t.Fatalf("nodes = %d, want 5: %v", g.NodeCount(), g.Order)
	}
	// Containment: app/app.go defines exactly Use.
	if ids := g.Files["app/app.go"]; len(ids) != 1 {
		t.Errorf("app.go defines %d symbols, want 1", len(ids))
	}
	// Use references Put and Get; Get collides (store + cache) → edge to BOTH.
	use := findByName(g, "Use")[0]
	targets := map[string]bool{}
	for _, id := range use.Out {
		targets[g.Symbols[id].Name+"@"+g.Symbols[id].File] = true
	}
	if !targets["Put@store/store.go"] {
		t.Error("Use should reference store.Put")
	}
	if !targets["Get@store/store.go"] || !targets["Get@cache/cache.go"] {
		t.Errorf("Use.Get should edge to BOTH Get defs (collision), got %v", use.Out)
	}
}

func TestBuild_FileEdges(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, _ := Build(ops, []string{root}, Options{})
	// app/app.go depends on store and cache (via Put/Get), not on itself.
	deps := map[string]bool{}
	for _, f := range g.FileEdges["app/app.go"] {
		deps[f] = true
	}
	if !deps["store/store.go"] || !deps["cache/cache.go"] {
		t.Errorf("app.go file edges = %v, want store + cache", g.FileEdges["app/app.go"])
	}
	if deps["app/app.go"] {
		t.Error("file edges must exclude self-loops")
	}
}

func TestBuild_UnresolvedNameNoEdge(t *testing.T) {
	root := t.TempDir()
	// References a name (Missing) that nothing defines → no edge.
	write(t, root, "a.go", "package a\n\nfunc F() { Missing() }\n")
	g, _ := Build(codeintel.NewOps(nil, root), []string{root}, Options{})
	f := findByName(g, "F")[0]
	if len(f.Out) != 0 {
		t.Errorf("unresolved reference should add no edge, got %v", f.Out)
	}
}

func TestBuild_NodeCapTruncates(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, _ := Build(ops, []string{root}, Options{NodeCap: 3})
	if !g.Truncated {
		t.Error("NodeCap 3 over 5 nodes should set Truncated")
	}
	if g.NodeCount() != 3 {
		t.Errorf("capped node count = %d, want 3", g.NodeCount())
	}
	// No edge should point at a pruned node.
	for _, id := range g.Order {
		for _, t2 := range g.Symbols[id].Out {
			if _, ok := g.Symbols[t2]; !ok {
				t.Errorf("edge to pruned node %s", t2)
			}
		}
	}
}

func TestIsExported(t *testing.T) {
	cases := []struct {
		lang, name, parent string
		want               bool
	}{
		{"go", "Put", "", true},
		{"go", "put", "", false},
		{"python", "public_fn", "", true},
		{"python", "_private", "", false},
		{"typescript", "_hidden", "", false},
		{"java", "anything", "", true},       // top-level → public
		{"java", "method", "MyClass", false}, // nested → not
		{"rust", "top", "", true},
		{"unknownlang", "x", "", true}, // conservative default: top-level public
	}
	for _, c := range cases {
		if got := isExported(c.lang, c.name, c.parent); got != c.want {
			t.Errorf("isExported(%s,%s,%q) = %v, want %v", c.lang, c.name, c.parent, got, c.want)
		}
	}
}
