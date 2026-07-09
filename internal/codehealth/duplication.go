package codehealth

const largeAssertionNLOC = 50 // a test function this long is likely an assertion dump

// duplicationDetectors returns the clone / assertion / error-handling detectors.
func duplicationDetectors() []Detector {
	return []Detector{cloneFindingsDetector, errorHandling, largeAssertionBlock}
}

// cloneFindingsDetector surfaces the precomputed clone findings for a file.
func cloneFindingsDetector(fc *FileContext, in *Inputs) []Finding {
	return in.cloneFindings[fc.Path]
}

// errorHandling emits a finding when any function carries an error-handling
// anti-pattern (empty catch, bare except, ignored error).
func errorHandling(fc *FileContext, _ *Inputs) []Finding {
	if !fc.Metrics.Supported {
		return nil
	}
	var out []Finding
	for _, f := range fc.Metrics.Funcs {
		if len(f.ErrorPatterns) > 0 {
			out = append(out, Finding{Biomarker: "error_handling", Severity: "low", File: fc.Path,
				Function: f.Name, LineStart: f.StartLine, Details: joinStrings(f.ErrorPatterns)})
		}
	}
	return out
}

// largeAssertionBlock flags a long function in a test file (a likely assertion
// dump that should be table-driven).
func largeAssertionBlock(fc *FileContext, _ *Inputs) []Finding {
	if !fc.Metrics.Supported || !isTestFile(fc.Path) {
		return nil
	}
	var out []Finding
	for _, f := range fc.Metrics.Funcs {
		if f.NLOC >= largeAssertionNLOC {
			out = append(out, Finding{Biomarker: "large_assertion_block", Severity: "low",
				File: fc.Path, Function: f.Name, LineStart: f.StartLine})
		}
	}
	return out
}

func joinStrings(xs []string) string {
	s := ""
	for i, x := range xs {
		if i > 0 {
			s += ", "
		}
		s += x
	}
	return s
}
