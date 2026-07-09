package codehealth

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

// FileCoverage is parsed per-file line coverage.
type FileCoverage struct {
	Covered int     `json:"covered"`
	Total   int     `json:"total"`
	Rate    float64 `json:"rate"`
}

// Documented coverage thresholds.
const (
	coverageGapRate       = 0.5
	untestedHotspotRate   = 0.5
	coverageGradientScale = 4.0 // deduction = scale × uncovered_fraction (repowise-consistent)
)

// ParseCoverage reads a coverage report, auto-detecting LCOV or Cobertura, and
// returns per-file coverage keyed by file path. Deterministic and offline.
func ParseCoverage(path string) (map[string]FileCoverage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if strings.Contains(text, "<coverage") || strings.HasPrefix(strings.TrimSpace(text), "<?xml") {
		return parseCobertura(text), nil
	}
	return parseLCOV(text), nil
}

// parseLCOV parses LCOV: SF:<file> ... DA:<line>,<hits> ... end_of_record.
func parseLCOV(text string) map[string]FileCoverage {
	out := map[string]FileCoverage{}
	var file string
	var covered, total int
	flush := func() {
		if file != "" && total > 0 {
			out[file] = FileCoverage{Covered: covered, Total: total, Rate: float64(covered) / float64(total)}
		}
		file, covered, total = "", 0, 0
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SF:"):
			flush()
			file = strings.TrimPrefix(line, "SF:")
		case strings.HasPrefix(line, "DA:"):
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) >= 2 {
				total++
				if hits, _ := strconv.Atoi(parts[1]); hits > 0 {
					covered++
				}
			}
		case line == "end_of_record":
			flush()
		}
	}
	flush()
	return out
}

var coberturaClass = regexp.MustCompile(`<class[^>]*filename="([^"]+)"[^>]*line-rate="([0-9.]+)"`)

// parseCobertura parses Cobertura XML class elements' filename + line-rate.
func parseCobertura(text string) map[string]FileCoverage {
	out := map[string]FileCoverage{}
	for _, m := range coberturaClass.FindAllStringSubmatch(text, -1) {
		rate, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		out[m[1]] = FileCoverage{Covered: int(rate * 100), Total: 100, Rate: rate}
	}
	return out
}

// coverageDetectors returns the coverage biomarkers. Each returns nil when the
// file has no coverage data (no report supplied) — REQ-704.
func coverageDetectors() []Detector {
	return []Detector{coverageGap, coverageGradient, untestedHotspot}
}

func coverageGap(fc *FileContext, _ *Inputs) []Finding {
	if fc.Coverage == nil || fc.Coverage.Rate >= coverageGapRate {
		return nil
	}
	return one("coverage_gap", fc.Path, "medium")
}

func coverageGradient(fc *FileContext, _ *Inputs) []Finding {
	if fc.Coverage == nil {
		return nil
	}
	uncovered := 1.0 - fc.Coverage.Rate
	if uncovered <= 0 {
		return nil
	}
	d := coverageGradientScale * uncovered
	return []Finding{{Biomarker: "coverage_gradient", Severity: "medium", File: fc.Path, Deduction: &d}}
}

func untestedHotspot(fc *FileContext, _ *Inputs) []Finding {
	if fc.Coverage == nil || !fc.IsHotspot || fc.Coverage.Rate >= untestedHotspotRate {
		return nil
	}
	return one("untested_hotspot", fc.Path, "high")
}
