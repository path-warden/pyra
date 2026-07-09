package codegraph

import "testing"

// twoClusters: {a1,a2,a3} dense, {b1,b2,b3} dense, one bridge a1↔b1.
func twoClusters() *Graph {
	return graphFromEdges(map[string][]string{
		"a1": {"a2", "a3", "b1"},
		"a2": {"a1", "a3"},
		"a3": {"a1", "a2"},
		"b1": {"b2", "b3"},
		"b2": {"b1", "b3"},
		"b3": {"b1", "b2"},
	})
}

func TestCommunities_TwoClusters(t *testing.T) {
	g := twoClusters()
	cs := g.Communities()
	if len(cs) != 2 {
		t.Fatalf("communities = %d, want 2: %+v", len(cs), cs)
	}
	// The a-cluster shares one community; the b-cluster another.
	if communityOf(cs, "a2") != communityOf(cs, "a3") {
		t.Error("a2 and a3 should share a community")
	}
	if communityOf(cs, "b2") != communityOf(cs, "b3") {
		t.Error("b2 and b3 should share a community")
	}
	if communityOf(cs, "a2") == communityOf(cs, "b2") {
		t.Error("the two clusters should be different communities")
	}
}

func TestCommunities_Deterministic(t *testing.T) {
	g := twoClusters()
	first := g.Communities()
	for i := 0; i < 3; i++ {
		again := g.Communities()
		if len(again) != len(first) {
			t.Fatalf("community count changed: %d vs %d", len(again), len(first))
		}
		for j := range first {
			if first[j].ID != again[j].ID || len(first[j].Members) != len(again[j].Members) {
				t.Fatalf("community %d differs across runs", j)
			}
			for k := range first[j].Members {
				if first[j].Members[k] != again[j].Members[k] {
					t.Fatalf("member order differs in community %d", j)
				}
			}
		}
	}
}

func TestCommunities_Empty(t *testing.T) {
	if cs := (&Graph{}).Communities(); cs != nil {
		t.Errorf("empty graph communities = %v, want nil", cs)
	}
}
