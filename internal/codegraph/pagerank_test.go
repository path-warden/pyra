package codegraph

import (
	"testing"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

func scoreOfName(c []Centrality, g *Graph, name string) float64 {
	var best float64
	for _, x := range c {
		if g.Symbols[x.ID].Name == name && x.Score > best {
			best = x.Score
		}
	}
	return best
}

func TestPageRank_HubOutranksLeaf(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, _ := Build(ops, []string{root}, Options{})
	c := g.PageRank()
	if len(c) != g.NodeCount() {
		t.Fatalf("centrality entries = %d, want %d", len(c), g.NodeCount())
	}
	// Put is referenced (by Use); helper is referenced by nothing.
	if scoreOfName(c, g, "Put") <= scoreOfName(c, g, "helper") {
		t.Errorf("referenced Put should outrank unreferenced helper: Put=%v helper=%v",
			scoreOfName(c, g, "Put"), scoreOfName(c, g, "helper"))
	}
	// Scores are sorted descending.
	for i := 1; i < len(c); i++ {
		if c[i-1].Score < c[i].Score {
			t.Fatalf("centrality not sorted at %d", i)
		}
	}
}

func TestPageRank_Deterministic(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, _ := Build(ops, []string{root}, Options{})
	first := g.PageRank()
	for i := 0; i < 3; i++ {
		again := g.PageRank()
		if len(again) != len(first) {
			t.Fatal("length changed")
		}
		for j := range first {
			if again[j].ID != first[j].ID || again[j].Score != first[j].Score {
				t.Fatalf("nondeterministic at %d: %+v vs %+v", j, again[j], first[j])
			}
		}
	}
}

func TestTopCentral_LimitAndEmpty(t *testing.T) {
	root, ops := fixtureRepo(t)
	g, _ := Build(ops, []string{root}, Options{})
	if top := g.TopCentral(2); len(top) != 2 {
		t.Errorf("TopCentral(2) = %d, want 2", len(top))
	}
	// Empty graph → no centrality, no panic.
	empty, _ := Build(codeintel.NewOps(nil, t.TempDir()), []string{t.TempDir()}, Options{})
	if len(empty.PageRank()) != 0 {
		t.Error("empty graph PageRank should be empty")
	}
}
