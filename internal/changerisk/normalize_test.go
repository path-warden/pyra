package changerisk

import (
	"math"
	"testing"
)

func TestNormalizer_MidRankPercentileWithTies(t *testing.T) {
	// Distribution: 1,2,2,2,3. Score 2 → below=1, equal=3 → 100*(1+1.5)/5 = 50.
	n := NewNormalizer([]float64{3, 2, 2, 1, 2})
	if pct := n.Percentile(2); math.Abs(pct-50.0) > 1e-9 {
		t.Errorf("percentile(2) = %v, want 50", pct)
	}
	// Score below all → 0; above all → mid-rank of the top.
	if pct := n.Percentile(0); pct != 0 {
		t.Errorf("percentile(0) = %v, want 0", pct)
	}
	if pct := n.Percentile(3); math.Abs(pct-90.0) > 1e-9 { // below=4, equal=1 → 100*(4+0.5)/5
		t.Errorf("percentile(3) = %v, want 90", pct)
	}
}

func TestNormalizer_EmptyDistribution(t *testing.T) {
	n := NewNormalizer(nil)
	if n.Percentile(5) != 0 {
		t.Error("empty distribution percentile should be 0")
	}
	if n.Available() {
		t.Error("empty distribution must not be Available")
	}
}

func TestNormalizer_TercileBoundaries(t *testing.T) {
	// 100 evenly spaced scores 0.1..10.0 so percentiles are well-defined.
	var scores []float64
	for i := 1; i <= 100; i++ {
		scores = append(scores, float64(i)/10.0)
	}
	n := NewNormalizer(scores)
	// A score in the top third ranks Elevated; bottom third Below typical.
	if p := n.Priority(9.5); p != PriorityElevated {
		t.Errorf("Priority(9.5) = %q, want Elevated", p)
	}
	if p := n.Priority(1.0); p != PriorityBelow {
		t.Errorf("Priority(1.0) = %q, want Below typical", p)
	}
	if p := n.Priority(5.0); p != PriorityTypical {
		t.Errorf("Priority(5.0) = %q, want Typical", p)
	}
}

func TestNormalizer_AvailabilityThreshold(t *testing.T) {
	small := make([]float64, MinBaseline-1)
	if NewNormalizer(small).Available() {
		t.Errorf("%d scores should be unavailable (< MinBaseline %d)", MinBaseline-1, MinBaseline)
	}
	ok := make([]float64, MinBaseline)
	if !NewNormalizer(ok).Available() {
		t.Errorf("%d scores should be available (== MinBaseline)", MinBaseline)
	}
}

func TestBaselineScores_ExcludesTargetAndScores(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	for i := 0; i < 3; i++ {
		writeF(t, root, "a.go", "package a\n// v"+itoa(i)+"\n")
		gitT(t, root, "add", ".")
		gitT(t, root, "commit", "-m", "c"+itoa(i))
	}
	all := BaselineScores(root, "HEAD", 50, nil, "")
	if len(all) != 3 {
		t.Fatalf("baseline scored %d commits, want 3", len(all))
	}
	// Exclude HEAD → one fewer.
	head := ""
	// resolve HEAD sha
	head = gitRevParse(t, root, "HEAD")
	excl := BaselineScores(root, "HEAD", 50, nil, head)
	if len(excl) != 2 {
		t.Errorf("excluding HEAD scored %d, want 2", len(excl))
	}
}

func gitRevParse(t *testing.T, root, ref string) string {
	t.Helper()
	out := git(root, "rev-parse", ref)
	return trimNL(out)
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
