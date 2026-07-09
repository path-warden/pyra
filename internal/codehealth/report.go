package codehealth

// refactoring maps a biomarker to a named refactoring suggestion (label only, no
// generated code).
var refactoring = map[string]string{
	"god_class":         "Extract Class",
	"low_cohesion":      "Extract Class",
	"large_method":      "Extract Helper",
	"brain_method":      "Extract Helper",
	"complex_method":    "Extract Helper",
	"nested_complexity": "Extract Helper / Guard Clauses",
	"god_file":          "Split File",
	"dry_violation":     "Extract Shared Helper",
	"cyclic_dependency": "Break Cycle",
}

// topMarker returns the highest-impact finding's biomarker and its refactoring
// suggestion. Findings must already be sorted by impact descending.
func topMarker(findings []Finding) (marker, suggestion string) {
	if len(findings) == 0 {
		return "", ""
	}
	marker = findings[0].Biomarker
	return marker, refactoring[marker]
}
