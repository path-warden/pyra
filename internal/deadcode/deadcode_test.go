package deadcode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/codeintel"
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

// deadRepo: Used() (exported, reachable) calls helper(); orphan() is private and
// unreferenced (dead, high); dynamicish() is private but textually referenced in a
// string/comment (medium); testHelper is in a test file (low).
func deadRepo(t *testing.T) (*codegraph.Graph, *codeintel.Ops, string) {
	t.Helper()
	root := t.TempDir()
	// orphan: private, referenced nowhere → high. dynamicish: referenced only
	// textually (a string, e.g. a reflective lookup) in a DIFFERENT, definition-less
	// file that the graph never sees → the whole-source scan still finds it → medium.
	// A doc comment repeating orphan's own name (same file) must NOT demote it.
	write(t, root, "app.go", "package p\n\nfunc Used() { helper() }\n\nfunc helper() {}\n\n// orphan does nothing.\nfunc orphan() { return }\n\nfunc dynamicish() {}\n")
	write(t, root, "dispatch.go", "package p\n\n// dynamicish is dispatched dynamically by name.\nvar _ = \"dynamicish\"\n")
	write(t, root, "x_test.go", "package p\n\nfunc testHelper() {}\n\nfunc TestX(t *T) {}\n")
	ops := codeintel.NewOps(nil, root)
	g, err := codegraph.Build(ops, []string{root}, codegraph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return g, ops, root
}

func candByName(r Report, name string) *Candidate {
	for i := range r.Candidates {
		if r.Candidates[i].Name == name {
			return &r.Candidates[i]
		}
	}
	return nil
}

func TestAnalyze_TiersAndExclusions(t *testing.T) {
	g, ops, root := deadRepo(t)
	rep := Analyze(g, ops, root, nil)

	// helper() is reachable (called by exported Used) → not a candidate.
	if candByName(rep, "helper") != nil {
		t.Error("reachable helper should not be a dead-code candidate")
	}
	// Test entry points excluded.
	if candByName(rep, "TestX") != nil || candByName(rep, "main") != nil {
		t.Error("Test*/main must be excluded")
	}
	// orphan: private, unreferenced → high.
	if o := candByName(rep, "orphan"); o == nil || o.Tier != TierHigh {
		t.Errorf("orphan should be high-confidence dead, got %+v", o)
	}
	// dynamicish: textually referenced → medium.
	if d := candByName(rep, "dynamicish"); d == nil || d.Tier != TierMedium {
		t.Errorf("dynamicish should be medium (textual ref), got %+v", d)
	}
	// testHelper: in a test file → low.
	if th := candByName(rep, "testHelper"); th == nil || th.Tier != TierLow {
		t.Errorf("testHelper should be low (test file), got %+v", th)
	}
}

func TestAnalyze_ImpactAndRanking(t *testing.T) {
	g, ops, root := deadRepo(t)
	rep := Analyze(g, ops, root, nil)
	// Impact is a positive line count for a resolvable symbol.
	if o := candByName(rep, "orphan"); o == nil || o.Impact < 1 {
		t.Errorf("orphan impact = %v, want >= 1", o)
	}
	// Ranked by impact desc, then id.
	for i := 1; i < len(rep.Candidates); i++ {
		if rep.Candidates[i-1].Impact < rep.Candidates[i].Impact {
			t.Fatalf("candidates not ranked by impact desc at %d", i)
		}
	}
	if rep.TotalImpact <= 0 {
		t.Errorf("total impact = %d, want > 0", rep.TotalImpact)
	}
}

func TestAnalyze_GovernedDeadCode(t *testing.T) {
	g, ops, root := deadRepo(t)
	orphan := candByName(Analyze(g, ops, root, nil), "orphan")
	// A Canon body citing orphan's symbol-id → governed.
	rep := Analyze(g, ops, root, []string{"This decision governs `" + orphan.ID + "`."})
	og := candByName(rep, "orphan")
	if og == nil || !og.Governed {
		t.Errorf("orphan cited by Canon should be governed dead code, got %+v", og)
	}
	// No Canon → not governed.
	if candByName(Analyze(g, ops, root, nil), "orphan").Governed {
		t.Error("no Canon → no governed marks")
	}
}

func TestAnalyze_EmptyGraph(t *testing.T) {
	if rep := Analyze(&codegraph.Graph{}, nil, "", nil); len(rep.Candidates) != 0 || rep.TotalImpact != 0 {
		t.Errorf("empty graph should yield empty report, got %+v", rep)
	}
	if rep := Analyze(nil, nil, "", nil); len(rep.Candidates) != 0 {
		t.Error("nil graph should yield empty report")
	}
}
