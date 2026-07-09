package codegraph

import (
	"reflect"
	"testing"
)

func TestCycles_MutualSelfAndAcyclic(t *testing.T) {
	g := graphFromEdges(map[string][]string{
		"a": {"b"},
		"b": {"a"}, // a↔b cycle
		"c": {"c"}, // self-loop
		"x": {"y"},
		"y": {"z"}, // acyclic chain x→y→z
	})
	cycles := g.Cycles()
	// Ordered by smallest member: {a,b} then {c}.
	if len(cycles) != 2 {
		t.Fatalf("cycles = %v, want 2 (a-b, c)", cycles)
	}
	if !reflect.DeepEqual(cycles[0], []string{"a", "b"}) {
		t.Errorf("first cycle = %v, want [a b]", cycles[0])
	}
	if !reflect.DeepEqual(cycles[1], []string{"c"}) {
		t.Errorf("second cycle = %v, want [c] (self-loop)", cycles[1])
	}
	// The acyclic chain contributes nothing.
	for _, c := range cycles {
		for _, m := range c {
			if m == "x" || m == "y" || m == "z" {
				t.Errorf("acyclic node %s should not appear in a cycle", m)
			}
		}
	}
}

func TestCycles_Deterministic(t *testing.T) {
	g := graphFromEdges(map[string][]string{
		"a": {"b"}, "b": {"c"}, "c": {"a"}, // 3-cycle
		"d": {"e"}, "e": {"d"},
	})
	first := g.Cycles()
	for i := 0; i < 3; i++ {
		if again := g.Cycles(); !reflect.DeepEqual(again, first) {
			t.Fatalf("nondeterministic cycles: %v vs %v", again, first)
		}
	}
	if len(first) != 2 {
		t.Fatalf("want 2 cycles, got %v", first)
	}
}

func TestCycles_None(t *testing.T) {
	g := graphFromEdges(map[string][]string{"x": {"y"}, "y": {"z"}})
	if cycles := g.Cycles(); len(cycles) != 0 {
		t.Errorf("acyclic graph should report no cycles, got %v", cycles)
	}
}
