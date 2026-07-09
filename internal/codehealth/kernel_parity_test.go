package codehealth

import (
	"math"
	"testing"
)

func fptr(v float64) *float64 { return &v }

// caseFindings builds the finding set for each parity case (mirrors the generator).
func caseFindings(i int) []Finding {
	switch i {
	case 0:
		return []Finding{{Biomarker: "god_class", Severity: "high"}}
	case 1:
		return []Finding{
			{Biomarker: "large_method", Severity: "medium"},
			{Biomarker: "complex_method", Severity: "high"},
			{Biomarker: "nested_complexity", Severity: "high"},
		}
	case 2:
		return []Finding{
			{Biomarker: "co_change_scatter", Severity: "critical"},
			{Biomarker: "change_entropy", Severity: "high"},
			{Biomarker: "churn_risk", Severity: "high"},
			{Biomarker: "ownership_risk", Severity: "medium"},
		}
	case 3:
		return []Finding{{Biomarker: "coverage_gradient", Severity: "medium", Deduction: fptr(0.8)}}
	case 4:
		return []Finding{
			{Biomarker: "god_class", Severity: "critical"},
			{Biomarker: "large_method", Severity: "high"},
			{Biomarker: "dry_violation", Severity: "medium"},
			{Biomarker: "error_handling", Severity: "low"},
		}
	default:
		return nil
	}
}

// goldens generated from repowise score_file over the ported constants.
var goldens = []struct {
	i                   int
	defect, maint, perf float64
}{
	{0, 8.644, 8.8, 10.0},
	{1, 6.892, 8.1, 10.0},
	{2, 6.5, 10.0, 10.0},
	{3, 9.2, 10.0, 10.0},
	{4, 5.74, 5.8, 10.0},
	{5, 10.0, 10.0, 10.0},
}

func TestScoreFile_ParityWithRepowise(t *testing.T) {
	const tol = 0.05
	for _, g := range goldens {
		s, _ := ScoreFile(caseFindings(g.i))
		if math.Abs(s.Defect-g.defect) > tol {
			t.Errorf("case %d defect = %.4f, want %.4f", g.i, s.Defect, g.defect)
		}
		if math.Abs(s.Maintainability-g.maint) > tol {
			t.Errorf("case %d maintainability = %.4f, want %.4f", g.i, s.Maintainability, g.maint)
		}
		if math.Abs(s.Performance-g.perf) > tol {
			t.Errorf("case %d performance = %.4f, want %.4f", g.i, s.Performance, g.perf)
		}
	}
}

func TestScoreFile_CategoryCapScalesProportionally(t *testing.T) {
	// Three organizational criticals would deduct 3×2.0×weights ≫ the 3.5 cap.
	fs := []Finding{
		{Biomarker: "churn_risk", Severity: "critical"},
		{Biomarker: "change_entropy", Severity: "critical"},
		{Biomarker: "co_change_scatter", Severity: "critical"},
	}
	s, impact := ScoreFile(fs)
	if math.Abs(s.Defect-(10.0-3.5)) > 1e-9 {
		t.Errorf("capped organizational defect = %v, want 6.5", s.Defect)
	}
	// Per-finding impacts sum to the cap.
	sum := 0.0
	for _, d := range impact {
		sum += d
	}
	if math.Abs(sum-3.5) > 1e-9 {
		t.Errorf("impacts sum = %v, want 3.5 (the cap)", sum)
	}
}

func TestScoreFile_PerformanceEmpty(t *testing.T) {
	s, _ := ScoreFile([]Finding{{Biomarker: "god_class", Severity: "critical"}})
	if s.Performance != 10.0 {
		t.Errorf("performance = %v, want 10.0 (no detectors)", s.Performance)
	}
}
