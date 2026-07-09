package changerisk

import (
	"math"
	"testing"
)

func iptr(n int) *int { return &n }

// goldenScores are ChangeFeatures → expected raw score, generated from repowise's
// change_risk/model.py score_change() over the same _CONSTANTS this package ports
// (see model_constants.go). This pins Memphis's ScoreChange to repowise's model:
// if either the port math or the constants drift, this fails.
var goldenScores = []struct {
	f    ChangeFeatures
	want float64
}{
	{ChangeFeatures{LA: 0, LD: 0, NF: 0, ND: 0, NS: 0, Entropy: 0.0, Exp: iptr(0)}, 1.4},
	{ChangeFeatures{LA: 10, LD: 2, NF: 1, ND: 1, NS: 1, Entropy: 0.0, Exp: iptr(5)}, 4.1},
	{ChangeFeatures{LA: 500, LD: 300, NF: 20, ND: 8, NS: 4, Entropy: 3.5, Exp: iptr(1)}, 9.2},
	{ChangeFeatures{LA: 10, LD: 2, NF: 1, ND: 1, NS: 1, Entropy: 0.0, Exp: nil}, 4.0}, // exp unknown
	{ChangeFeatures{LA: 42, LD: 7, NF: 3, ND: 2, NS: 2, Entropy: 1.5, Exp: iptr(50)}, 6.5},
}

func TestScoreChange_ParityWithRepowise(t *testing.T) {
	const tol = 0.05 // documented parity tolerance
	for _, g := range goldenScores {
		got := ScoreChange(g.f).Score
		if math.Abs(got-g.want) > tol {
			t.Errorf("ScoreChange(%+v).Score = %v, want %v (±%v)", g.f, got, g.want, tol)
		}
	}
}

func TestScoreChange_Deterministic(t *testing.T) {
	f := ChangeFeatures{LA: 42, LD: 7, NF: 3, ND: 2, NS: 2, Entropy: 1.5, Exp: iptr(50)}
	first := ScoreChange(f)
	for i := 0; i < 5; i++ {
		got := ScoreChange(f)
		if got.Score != first.Score || got.Probability != first.Probability {
			t.Fatalf("nondeterministic: %v vs %v", got, first)
		}
	}
}

func TestScoreChange_UnknownExpIsNeutral(t *testing.T) {
	base := ChangeFeatures{LA: 10, LD: 2, NF: 1, ND: 1, NS: 1, Entropy: 0.0, Exp: nil}
	r := ScoreChange(base)
	var expDriver *RiskDriver
	for i := range r.Drivers {
		if r.Drivers[i].Feature == "exp" {
			expDriver = &r.Drivers[i]
		}
	}
	if expDriver == nil {
		t.Fatal("no exp driver")
	}
	if expDriver.Known {
		t.Error("exp driver should be Known=false when Exp is nil")
	}
	if expDriver.Contribution != 0 {
		t.Errorf("unknown exp should contribute 0, got %v", expDriver.Contribution)
	}
}

func TestTopDrivers_SortedByAbsContribution(t *testing.T) {
	// Large diffuse change: la should dominate.
	r := ScoreChange(ChangeFeatures{LA: 500, LD: 300, NF: 20, ND: 8, NS: 4, Entropy: 3.5, Exp: iptr(1)})
	top := r.TopDrivers()
	for i := 1; i < len(top); i++ {
		if math.Abs(top[i-1].Contribution) < math.Abs(top[i].Contribution) {
			t.Fatalf("drivers not sorted by |contribution|: %+v", top)
		}
	}
	if top[0].Feature != "la" {
		t.Errorf("top driver = %q, want la", top[0].Feature)
	}
}
