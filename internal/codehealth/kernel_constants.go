package codehealth

// This file is the ONLY code-health source ported from repowise, and the only
// place its calibrated scoring constants live. It is isolated so the constants
// can be replaced (e.g. by pyra-owned calibration) without touching the
// biomarkers, composition, or surfaces.
//
// PORTED FROM repowise (AGPL-3.0):
//   packages/core/src/repowise/core/analysis/health/scoring.py
// Calibrated offline against repowise's defect corpus. The parity test
// (kernel_parity_test.go) pins Pyra's ScoreFile to repowise's score_file for
// these constants; regenerate both together if repowise recalibrates.

// Severity deduction table (base deduction before the per-biomarker multiplier).
var severityDeduction = map[string]float64{
	"low":      0.3,
	"medium":   0.7,
	"high":     1.2,
	"critical": 2.0,
}

// --- Defect dimension (the surfaced score) ---

var defectCaps = map[string]float64{
	"organizational":         3.5,
	"structural_complexity":  2.5,
	"test_coverage":          2.0,
	"test_coverage_gradient": 2.0,
	"size_and_complexity":    1.5,
	"duplication":            1.0,
	"test_quality":           0.5,
	"error_handling":         0.5,
}

var defectWeight = map[string]float64{
	"co_change_scatter":      1.8,
	"change_entropy":         1.51,
	"ownership_risk":         1.38,
	"nested_complexity":      1.34,
	"complex_conditional":    1.33,
	"large_method":           1.25,
	"complex_method":         1.21,
	"function_hotspot":       1.16,
	"god_class":              1.13,
	"prior_defect":           1.0,
	"untested_hotspot":       1.3,
	"churn_risk":             1.2,
	"code_age_volatility":    1.1,
	"developer_congestion":   0.5,
	"low_cohesion":           0.5,
	"brain_method":           0.5,
	"bumpy_road":             0.5,
	"primitive_obsession":    0.5,
	"dry_violation":          0.5,
	"knowledge_loss":         0.4,
	"error_handling":         0.5,
	"contradictory_decision": 1.0,
	"stale_governance":       0.9,
	"ungoverned_hotspot":     0.7,
}

var defectCategory = map[string]string{
	"brain_method":               "structural_complexity",
	"low_cohesion":               "structural_complexity",
	"god_class":                  "structural_complexity",
	"nested_complexity":          "structural_complexity",
	"bumpy_road":                 "structural_complexity",
	"complex_conditional":        "structural_complexity",
	"complex_method":             "size_and_complexity",
	"large_method":               "size_and_complexity",
	"primitive_obsession":        "size_and_complexity",
	"god_file":                   "size_and_complexity", // pyra-specific (documented)
	"dry_violation":              "duplication",
	"untested_hotspot":           "test_coverage",
	"coverage_gap":               "test_coverage",
	"coverage_gradient":          "test_coverage_gradient",
	"developer_congestion":       "organizational",
	"knowledge_loss":             "organizational",
	"hidden_coupling":            "organizational",
	"function_hotspot":           "organizational",
	"code_age_volatility":        "organizational",
	"ownership_risk":             "organizational",
	"churn_risk":                 "organizational",
	"change_entropy":             "organizational",
	"co_change_scatter":          "organizational",
	"prior_defect":               "organizational",
	"large_assertion_block":      "test_quality",
	"duplicated_assertion_block": "test_quality",
	"error_handling":             "error_handling",
	"ungoverned_hotspot":         "organizational",
	"stale_governance":           "organizational",
	"contradictory_decision":     "organizational",
}

// dimensions maps a biomarker to the dimensions its deduction feeds. Unlisted
// biomarkers default to defect only.
var dimensions = map[string][]string{
	"low_cohesion":        {"defect", "maintainability"},
	"brain_method":        {"defect", "maintainability"},
	"primitive_obsession": {"defect", "maintainability"},
	"dry_violation":       {"defect", "maintainability"},
	"error_handling":      {"defect", "maintainability"},
	"god_class":           {"defect", "maintainability"},
	"large_method":        {"defect", "maintainability"},
	"nested_complexity":   {"defect", "maintainability"},
}

// --- Maintainability dimension ---

var maintWeight = map[string]float64{
	"low_cohesion":        1.0,
	"brain_method":        1.0,
	"primitive_obsession": 1.0,
	"dry_violation":       1.0,
	"error_handling":      1.0,
	"god_class":           1.0,
	"large_method":        1.0,
	"nested_complexity":   1.0,
}

var maintCategory = map[string]string{
	"brain_method":        "structural_complexity",
	"low_cohesion":        "structural_complexity",
	"god_class":           "structural_complexity",
	"nested_complexity":   "structural_complexity",
	"large_method":        "structural_complexity",
	"primitive_obsession": "size_and_complexity",
	"dry_violation":       "duplication",
	"error_handling":      "error_handling",
}

var maintCaps = map[string]float64{
	"structural_complexity": 4.0,
	"size_and_complexity":   2.0,
	"duplication":           2.0,
	"error_handling":        2.0,
}

// --- Performance dimension (detectors deferred; tables kept for wiring) ---

var perfWeight = map[string]float64{}
var perfCategory = map[string]string{}
var perfCaps = map[string]float64{"performance": 1.0}
