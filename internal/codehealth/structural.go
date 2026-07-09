package codehealth

import (
	"fmt"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

// Documented structural thresholds (memphis-owned, tunable).
const (
	largeMethodNLOC      = 60 // with ≥2 CCN
	complexMethodCCN     = 9
	nestingThreshold     = 4 // fires when max nesting > this
	brainMethodNLOC      = 70
	brainMethodCCN       = 9
	bumpyRoadNLOC        = 40
	bumpyRoadNesting     = 3
	complexConditionalN  = 4 // boolean operators in a function
	godClassMethods      = 15
	godClassNLOC         = 200
	godFileDefs          = 20
	primitiveObsessionN  = 4
	lowCohesionMinMethod = 2
)

// structuralDetectors returns the size/complexity/cohesion detectors.
func structuralDetectors() []Detector {
	return []Detector{funcLevelStructural, godFile, classLevelStructural}
}

// funcLevelStructural emits the per-function structural markers.
func funcLevelStructural(fc *FileContext, _ *Inputs) []Finding {
	if !fc.Metrics.Supported {
		return nil
	}
	var out []Finding
	for _, f := range fc.Metrics.Funcs {
		mk := func(bm, sev string) Finding {
			return Finding{Biomarker: bm, Severity: sev, File: fc.Path, Function: f.Name,
				LineStart: f.StartLine, LineEnd: f.EndLine}
		}
		if f.NLOC >= largeMethodNLOC && f.Cyclomatic >= 2 {
			out = append(out, mk("large_method", stepSeverity(f.NLOC, largeMethodNLOC, 120, 200)))
		}
		if f.Cyclomatic >= complexMethodCCN {
			out = append(out, mk("complex_method", stepSeverity(f.Cyclomatic, complexMethodCCN, 15, 25)))
		}
		if f.MaxNesting > nestingThreshold {
			out = append(out, mk("nested_complexity", stepSeverity(f.MaxNesting, nestingThreshold+1, 6, 8)))
		}
		if f.NLOC >= brainMethodNLOC && f.Cyclomatic >= brainMethodCCN {
			out = append(out, mk("brain_method", "high"))
		}
		if f.NLOC >= bumpyRoadNLOC && f.MaxNesting >= bumpyRoadNesting {
			out = append(out, mk("bumpy_road", "low"))
		}
		if f.BoolOps >= complexConditionalN {
			out = append(out, mk("complex_conditional", stepSeverity(f.BoolOps, complexConditionalN, 6, 9)))
		}
		if f.PrimitiveArgs > primitiveObsessionN {
			out = append(out, mk("primitive_obsession", "low"))
		}
	}
	return out
}

// godFile flags files with too many top-level definitions.
func godFile(fc *FileContext, _ *Inputs) []Finding {
	if fc.TopLevel >= godFileDefs {
		return []Finding{{Biomarker: "god_file", Severity: stepSeverity(fc.TopLevel, godFileDefs, 40, 60),
			File: fc.Path, Details: fmt.Sprintf("%d top-level definitions", fc.TopLevel)}}
	}
	return nil
}

// classLevelStructural emits god_class and low_cohesion per class in the file.
func classLevelStructural(fc *FileContext, _ *Inputs) []Finding {
	if !fc.Metrics.Supported {
		return nil
	}
	byClass := map[string][]codeintel.FuncMetrics{}
	for _, f := range fc.Metrics.Funcs {
		if f.Class != "" {
			byClass[f.Class] = append(byClass[f.Class], f)
		}
	}
	var out []Finding
	for _, cls := range sortedKeys(byClass) {
		methods := byClass[cls]
		nloc := 0
		for _, m := range methods {
			nloc += m.NLOC
		}
		if len(methods) >= godClassMethods && nloc >= godClassNLOC {
			out = append(out, Finding{Biomarker: "god_class", Severity: stepSeverity(len(methods), godClassMethods, 25, 40),
				File: fc.Path, Function: cls, Details: fmt.Sprintf("%d methods, %d NLOC", len(methods), nloc)})
		}
		// LCOM4 is only meaningful where field-access data is available (Python/JS/
		// TS). For languages without a field-access profile (e.g. Go), every method
		// has an empty FieldAccess set, which would make lcom4 == method count and
		// fire spuriously — so require some field data before judging cohesion.
		if hasFieldData(methods) && len(methods) >= lowCohesionMinMethod && lcom4(methods) >= 2 {
			out = append(out, Finding{Biomarker: "low_cohesion", Severity: "medium",
				File: fc.Path, Function: cls, Details: "methods form disconnected field clusters"})
		}
	}
	return out
}

// hasFieldData reports whether any method carries field-access information (i.e.
// the language has a field-access profile), so LCOM4 can be measured meaningfully.
func hasFieldData(methods []codeintel.FuncMetrics) bool {
	for _, m := range methods {
		if len(m.FieldAccess) > 0 {
			return true
		}
	}
	return false
}

// lcom4 computes LCOM4: the number of connected components of the graph whose
// nodes are methods and whose edges link methods sharing an accessed field.
// Methods with no field access are isolated nodes. 1 = cohesive; ≥ 2 = split.
func lcom4(methods []codeintel.FuncMetrics) int {
	n := len(methods)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) { parent[find(a)] = find(b) }

	fieldToMethod := map[string]int{}
	for i, m := range methods {
		for _, f := range m.FieldAccess {
			if j, ok := fieldToMethod[f]; ok {
				union(i, j)
			} else {
				fieldToMethod[f] = i
			}
		}
	}
	roots := map[int]bool{}
	for i := 0; i < n; i++ {
		roots[find(i)] = true
	}
	return len(roots)
}

// stepSeverity maps a value to a severity by low/med/high/critical breakpoints.
func stepSeverity(v, low, high, critical int) string {
	switch {
	case v >= critical:
		return "critical"
	case v >= high:
		return "high"
	case v >= low+(high-low)/2:
		return "medium"
	default:
		return "low"
	}
}
