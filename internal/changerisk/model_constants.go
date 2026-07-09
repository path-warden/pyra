package changerisk

// This file is the ONLY change-risk source ported from repowise, and the only
// place its learned constants live. It is isolated so the constants can be
// replaced (e.g. by Memphis-owned calibration) without touching features,
// scoring math, ranking, directives, or integration.
//
// PORTED FROM repowise (AGPL-3.0):
//   packages/core/src/repowise/core/analysis/change_risk/model.py  (_CONSTANTS)
// Calibrated offline (2026-05-30) on a 7-repo, 5-language slice of repowise's
// defect corpus (AG-SZZ bug-inducing commits as labels, leave-one-repo-out).
// The parity test (model_parity_test.go) pins Memphis's ScoreChange output to
// repowise's for these constants; if repowise recalibrates, regenerate both the
// constants here AND the parity fixtures together.
//
// Parallel arrays: Features / Log1p / Mean / Std / Coef are index-aligned.

type modelConstants struct {
	Features  []string
	Log1p     []bool
	Mean      []float64
	Std       []float64
	Coef      []float64
	Intercept float64
}

var portedConstants = modelConstants{
	Features:  []string{"la", "ld", "nf", "nd", "ns", "entropy", "exp"},
	Log1p:     []bool{true, true, true, true, true, false, true},
	Mean:      []float64{2.443, 1.723, 1.006, 0.795, 0.806, 0.537, 3.011},
	Std:       []float64{1.414, 1.380, 0.465, 0.237, 0.285, 0.776, 2.270},
	Coef:      []float64{1.1241, 0.0151, -0.1103, -0.0310, -0.0672, 0.1483, -0.0702},
	Intercept: -0.3797,
}
