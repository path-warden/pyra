package codegraph

import (
	"testing"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

func contains(ids []string, name string, g *Graph) bool {
	for _, id := range ids {
		if g.Symbols[id].Name == name {
			return true
		}
	}
	return false
}

func TestReachability_ExportedRootsReachClosure(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, _ := Build(ops, []string{root}, Options{})
	r := g.Reachability()

	// Exported Go funcs (Put, Get, Use) are entry points; private helper is not.
	if !contains(r.EntryPoints, "Use", g) || !contains(r.EntryPoints, "Put", g) {
		t.Errorf("entry points should include exported Use/Put: %v", r.EntryPoints)
	}
	if contains(r.EntryPoints, "helper", g) {
		t.Error("private helper should not be an entry point")
	}
	// Use → Put/Get are reachable; helper (unexported, unreferenced) is not.
	if !contains(r.Reachable, "Put", g) || !contains(r.Reachable, "Use", g) {
		t.Errorf("Put and Use should be reachable: %v", r.Reachable)
	}
	if !contains(r.Unreachable, "helper", g) {
		t.Errorf("helper should be unreachable: %v", r.Unreachable)
	}
	if contains(r.Reachable, "helper", g) {
		t.Error("helper must not be reachable")
	}
}

func TestReachability_NoEntryPoints(t *testing.T) {
	root := t.TempDir()
	// Only unexported Go funcs, none named main, none referenced from outside.
	write(t, root, "a.go", "package a\n\nfunc one() { two() }\n\nfunc two() {}\n")
	g, _ := Build(codeintel.NewOps(nil, root), []string{root}, Options{})
	r := g.Reachability()
	if len(r.EntryPoints) != 0 {
		t.Errorf("no exported/main symbols → no entry points, got %v", r.EntryPoints)
	}
	if len(r.Reachable) != 0 {
		t.Errorf("no entry points → empty reachable, got %v", r.Reachable)
	}
	if len(r.Unreachable) != g.NodeCount() {
		t.Errorf("all %d nodes should be unreachable, got %d", g.NodeCount(), len(r.Unreachable))
	}
}

func TestReachability_MainIsEntryPoint(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.go", "package main\n\nfunc main() { run() }\n\nfunc run() {}\n")
	g, _ := Build(codeintel.NewOps(nil, root), []string{root}, Options{})
	r := g.Reachability()
	if !contains(r.EntryPoints, "main", g) {
		t.Errorf("main should be an entry point: %v", r.EntryPoints)
	}
	// run is lowercase (not exported) but reachable from main.
	if !contains(r.Reachable, "run", g) {
		t.Errorf("run should be reachable from main: %v", r.Reachable)
	}
}
