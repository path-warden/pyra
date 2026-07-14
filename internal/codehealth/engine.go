// Package codehealth scores every source file by running deterministic biomarker
// detectors over the layers pyra already owns (codeintel, gitint, codegraph,
// canon) and aggregating findings with a ported repowise scoring kernel into
// three independently-capped dimensions.
//
// It lives outside internal/canon/...; the authority path may not depend on it
// (a boundary test enforces this). The scoring kernel constants are a faithful
// port of repowise (AGPL-3.0), confined to kernel_constants.go; the biomarkers
// are pyra's own deterministic detectors.
package codehealth

import (
	"sort"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/store"
)

// FileContext is the per-file input a biomarker detector reads.
type FileContext struct {
	Path      string
	NLOC      int
	Metrics   codeintel.FileMetrics
	TopLevel  int // top-level definitions in the file (for god_file)
	Git       *gitint.FileHistory
	IsHotspot bool
	Governed  bool
	Coverage  *FileCoverage
	graphNode bool // reserved for graph-derived detectors
}

// Inputs bundles the whole-run analysis inputs.
type Inputs struct {
	Ops      *codeintel.Ops
	History  *gitint.History
	Graph    *codegraph.Graph
	Store    *store.Store
	Roots    []string
	Root     string // store root, for reading source (clone detection)
	Coverage map[string]FileCoverage
	// detectors is the registered biomarker roster; nil uses DefaultDetectors().
	detectors []Detector
	// cloneFindings is precomputed once per run (clones are cross-file).
	cloneFindings map[string][]Finding
}

// Detector is one biomarker: it inspects a file context (and shared inputs) and
// returns any findings.
type Detector func(fc *FileContext, in *Inputs) []Finding

// FileHealth is a scored file.
type FileHealth struct {
	Path            string    `json:"path"`
	NLOC            int       `json:"nloc"`
	Defect          float64   `json:"defect"`
	Maintainability float64   `json:"maintainability"`
	Performance     float64   `json:"performance"`
	IsHotspot       bool      `json:"is_hotspot"`
	Findings        []Finding `json:"findings,omitempty"`
	TopMarker       string    `json:"top_marker,omitempty"`
	Suggestion      string    `json:"suggestion,omitempty"`
}

// Report is the whole-repo health result.
type Report struct {
	Files          []FileHealth `json:"files"`
	AverageHealth  float64      `json:"average_health"`
	HotspotHealth  float64      `json:"hotspot_health"`
	Worst          *FileHealth  `json:"worst,omitempty"`
	FileCount      int          `json:"file_count"`
	Contradictions []string     `json:"contradictory_decisions,omitempty"` // repo-level governance
}

// Analyze builds a health report over the code roots. Deterministic and offline.
func Analyze(in Inputs) (Report, error) {
	if in.detectors == nil {
		in.detectors = DefaultDetectors()
	}
	contexts, err := buildContexts(&in)
	if err != nil {
		return Report{}, err
	}
	in.cloneFindings = detectClones(&in, contexts)

	var files []FileHealth
	for _, fc := range contexts {
		var findings []Finding
		for _, d := range in.detectors {
			findings = append(findings, d(fc, &in)...)
		}
		scores, impact := ScoreFile(findings)
		for i := range findings {
			findings[i].Impact = impact[i]
		}
		sortFindings(findings)
		top, suggestion := topMarker(findings)
		files = append(files, FileHealth{
			Path: fc.Path, NLOC: fc.NLOC,
			Defect: scores.Defect, Maintainability: scores.Maintainability, Performance: scores.Performance,
			IsHotspot: fc.IsHotspot, Findings: findings, TopMarker: top, Suggestion: suggestion,
		})
	}

	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Defect != files[j].Defect {
			return files[i].Defect < files[j].Defect
		}
		return files[i].Path < files[j].Path
	})

	rep := assemble(files)
	rep.Contradictions = detectContradictions(&in)
	return rep, nil
}

// buildContexts enumerates files (via codeintel Map) and gathers each file's
// metrics, git history, hotspot flag, governance, and coverage.
func buildContexts(in *Inputs) ([]*FileContext, error) {
	seen := map[string]bool{}
	var order []string
	topLevel := map[string]int{}
	for _, root := range in.Roots {
		maps, err := in.Ops.Map(root, "", "", false)
		if err != nil {
			continue
		}
		for _, fm := range maps {
			if !seen[fm.File] {
				seen[fm.File] = true
				order = append(order, fm.File)
			}
			for _, e := range fm.Entries {
				if e.Parent == nil {
					topLevel[fm.File]++
				}
			}
		}
	}
	sort.Strings(order)

	hotspots := map[string]bool{}
	governed := governedFiles(in, order)
	if in.History != nil {
		for _, h := range in.History.Hotspots() {
			hotspots[h.Path] = true
		}
	}

	var out []*FileContext
	for _, path := range order {
		fc := &FileContext{Path: path, TopLevel: topLevel[path], IsHotspot: hotspots[path]}
		if fm, err := in.Ops.Metrics(path); err == nil {
			fc.Metrics = fm
		}
		fc.NLOC = fileNLOC(fc.Metrics)
		if in.History != nil {
			fc.Git = in.History.File(path)
		}
		if in.Coverage != nil {
			if cov, ok := in.Coverage[path]; ok {
				c := cov
				fc.Coverage = &c
			}
		}
		fc.Governed = governed[path]
		out = append(out, fc)
	}
	return out, nil
}

// fileNLOC approximates a file's size as the sum of its function NLOCs (min 1),
// used only for KPI weighting.
func fileNLOC(fm codeintel.FileMetrics) int {
	n := 0
	for _, f := range fm.Funcs {
		n += f.NLOC
	}
	if n < 1 {
		n = 1
	}
	return n
}

// assemble computes the KPIs from scored files.
func assemble(files []FileHealth) Report {
	r := Report{Files: files, FileCount: len(files)}
	if len(files) == 0 {
		r.AverageHealth, r.HotspotHealth = 10.0, 10.0
		return r
	}
	r.AverageHealth = nlocWeighted(files, func(f FileHealth) bool { return true })
	r.HotspotHealth = nlocWeighted(files, func(f FileHealth) bool { return f.IsHotspot })
	// Callers pass files already sorted by Defect ascending, so files[0] is the
	// worst performer.
	w := files[0]
	r.Worst = &w
	return r
}

func nlocWeighted(files []FileHealth, include func(FileHealth) bool) float64 {
	var sum, wsum float64
	for _, f := range files {
		if !include(f) {
			continue
		}
		w := float64(f.NLOC)
		if w < 1 {
			w = 1
		}
		sum += f.Defect * w
		wsum += w
	}
	if wsum == 0 {
		return 10.0
	}
	return round2(sum / wsum)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Impact != fs[j].Impact {
			return fs[i].Impact > fs[j].Impact
		}
		if fs[i].Biomarker != fs[j].Biomarker {
			return fs[i].Biomarker < fs[j].Biomarker
		}
		return fs[i].LineStart < fs[j].LineStart
	})
}
