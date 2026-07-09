package codegraph

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func snapshotJSON(t *testing.T, g *Graph) string {
	t.Helper()
	payload := map[string]any{
		"order":        g.Order,
		"file_edges":   g.FileEdges,
		"centrality":   g.PageRank(),
		"communities":  g.Communities(),
		"cycles":       g.Cycles(),
		"reachability": g.Reachability(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGraph_DeterministicAcrossRuns(t *testing.T) {
	root, ops := fixtureRepo(t)
	g1, _ := Build(ops, []string{root}, Options{})
	first := snapshotJSON(t, g1)
	for i := 0; i < 3; i++ {
		g2, _ := Build(ops, []string{root}, Options{})
		if again := snapshotJSON(t, g2); again != first {
			t.Fatalf("graph + analyses not deterministic across builds:\n%s\nvs\n%s", first, again)
		}
	}
}

func TestGraph_RootOrderIndependent(t *testing.T) {
	root, ops := fixtureRepo(t)
	a := filepath.Join(root, "store")
	b := filepath.Join(root, "app")

	gAB, _ := Build(ops, []string{a, b}, Options{})
	gBA, _ := Build(ops, []string{b, a}, Options{})
	if snapshotJSON(t, gAB) != snapshotJSON(t, gBA) {
		t.Error("graph output must be independent of root ordering")
	}
}
