package codehealth

import "sort"

// Finding is one biomarker hit on a file. Deduction, when non-nil, overrides the
// severity table (a continuous magnitude, e.g. coverage_gradient).
type Finding struct {
	Biomarker string   `json:"biomarker"`
	Severity  string   `json:"severity"` // low | medium | high | critical
	File      string   `json:"file"`
	Function  string   `json:"function,omitempty"`
	LineStart int      `json:"line_start,omitempty"`
	LineEnd   int      `json:"line_end,omitempty"`
	Details   string   `json:"details,omitempty"`
	Deduction *float64 `json:"-"`
	Impact    float64  `json:"impact"` // per-finding defect deduction after capping (filled by ScoreFile)
}

// Scores holds the three per-file dimension scores, each in [1.0, 10.0].
type Scores struct {
	Defect          float64 `json:"defect"`
	Maintainability float64 `json:"maintainability"`
	Performance     float64 `json:"performance"`
}

// ScoreFile aggregates findings into the three dimension scores and returns the
// per-finding defect impact (parallel to findings). This reproduces repowise's
// score_file: weight each finding, accumulate per category, scale a category's
// contributions down proportionally when it exceeds its cap, clamp to [1,10].
func ScoreFile(findings []Finding) (Scores, []float64) {
	defectScore, defectImpact := scoreDimension(findings, "defect", defectWeight, defectCategory, defectCaps)
	maintScore, _ := scoreDimension(findings, "maintainability", maintWeight, maintCategory, maintCaps)
	perfScore, _ := scoreDimension(findings, "performance", perfWeight, perfCategory, perfCaps)
	return Scores{Defect: defectScore, Maintainability: maintScore, Performance: perfScore}, defectImpact
}

// scoreDimension runs the shared kernel for one dimension over the findings that
// belong to it, returning the score and the per-(original-index) deduction.
func scoreDimension(findings []Finding, dim string, weightOf map[string]float64, catOf map[string]string, caps map[string]float64) (float64, []float64) {
	type entry struct {
		idx    int
		weight float64
	}
	byCat := map[string][]entry{}
	for i, f := range findings {
		if !inDimension(f.Biomarker, dim) {
			continue
		}
		base := severityDeduction[f.Severity]
		if f.Deduction != nil {
			base = *f.Deduction
		}
		w := 1.0
		if wv, ok := weightOf[f.Biomarker]; ok {
			w = wv
		}
		cat := catOf[f.Biomarker]
		if cat == "" {
			cat = "size_and_complexity"
		}
		byCat[cat] = append(byCat[cat], entry{idx: i, weight: base * w})
	}

	perFinding := make([]float64, len(findings))
	total := 0.0
	for _, cat := range sortedKeys(byCat) {
		entries := byCat[cat]
		cap := caps[cat]
		if cap == 0 {
			cap = 1.0
		}
		sum := 0.0
		for _, e := range entries {
			sum += e.weight
		}
		if sum <= cap {
			for _, e := range entries {
				perFinding[e.idx] = e.weight
			}
			total += sum
		} else {
			scale := 0.0
			if sum > 0 {
				scale = cap / sum
			}
			for _, e := range entries {
				perFinding[e.idx] = e.weight * scale
			}
			total += cap
		}
	}
	return clamp(10.0-total, 1.0, 10.0), perFinding
}

// inDimension reports whether a biomarker's deduction feeds a dimension (defaults
// to defect only).
func inDimension(biomarker, dim string) bool {
	dims, ok := dimensions[biomarker]
	if !ok {
		return dim == "defect"
	}
	for _, d := range dims {
		if d == dim {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
