package changerisk

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chasedputnam/pyra/internal/changegate"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/store"
)

// Stable directive/finding codes. All advisory by default; escalatable via the
// store's enforcement policy.
const (
	CodeRisk           = "change-risk"            // the score headline (repo-level)
	CodeMissingTests   = "risk-missing-tests"     // changed source without its test in the diff
	CodeMissingCoChg   = "risk-missing-cochanges" // frequent co-change partners absent from the diff
	CodeWillBreak      = "risk-will-break"        // structural dependents of changed symbols
	CodeGovernanceRisk = "risk-governance"        // change touches Accepted Canon
)

const (
	coChangeSupport = 2 // minimum shared commits to count as a co-change partner
	maxPartners     = 5 // cap partners reported per file
	maxDependents   = 10
)

// Finding is one change-risk directive before policy classification.
type Finding struct {
	Code    string
	File    string // the changed file the directive is about ("" for repo-level)
	Message string
}

// missingTestsDirectives flags changed source files whose conventional test file
// is not part of the same change.
func missingTestsDirectives(changed []string, changeSet map[string]bool) []Finding {
	var out []Finding
	for _, f := range changed {
		if !isSourceNeedingTest(f) {
			continue
		}
		candidates := testCandidates(f)
		if len(candidates) == 0 {
			continue // language demands no separate test file (e.g. Rust in-file)
		}
		if anyInSet(candidates, changeSet) {
			continue
		}
		out = append(out, Finding{
			Code:    CodeMissingTests,
			File:    f,
			Message: fmt.Sprintf("%s changed without a test (expected one of: %s) in the change", f, strings.Join(candidates, ", ")),
		})
	}
	return out
}

// codeGraph is a name-level file adjacency built from one codeintel Map walk:
// which names each file defines/references, and which files reference each name.
type codeGraph struct {
	defs      map[string]map[string]bool // file -> defined names
	refs      map[string]map[string]bool // file -> referenced names
	refByName map[string]map[string]bool // name -> files referencing it
}

// buildCodeGraph runs one codeintel Map over root. Returns nil when ops is nil
// or the walk fails (callers then skip code-derived directives).
func buildCodeGraph(ops *codeintel.Ops, root string) *codeGraph {
	if ops == nil {
		return nil
	}
	maps, err := ops.Map(root, "", "", false)
	if err != nil {
		return nil
	}
	g := &codeGraph{
		defs:      map[string]map[string]bool{},
		refs:      map[string]map[string]bool{},
		refByName: map[string]map[string]bool{},
	}
	for _, fm := range maps {
		for _, e := range fm.Entries {
			if g.defs[fm.File] == nil {
				g.defs[fm.File] = map[string]bool{}
			}
			g.defs[fm.File][e.Name] = true
			for _, r := range e.References {
				if g.refs[fm.File] == nil {
					g.refs[fm.File] = map[string]bool{}
				}
				g.refs[fm.File][r] = true
				if g.refByName[r] == nil {
					g.refByName[r] = map[string]bool{}
				}
				g.refByName[r][fm.File] = true
			}
		}
	}
	return g
}

// importLinked reports whether files a and b share a structural edge: one
// references a name the other defines.
func (g *codeGraph) importLinked(a, b string) bool {
	return intersects(g.defs[a], g.refs[b]) || intersects(g.defs[b], g.refs[a])
}

// willBreakDirectives reports files that structurally reference a name defined in
// a changed file and are themselves outside the change set.
func willBreakDirectives(g *codeGraph, changed []string, changeSet map[string]bool) []Finding {
	if g == nil {
		return nil
	}
	var out []Finding
	for _, f := range changed {
		deps := map[string]bool{}
		for name := range g.defs[f] {
			for dep := range g.refByName[name] {
				if dep == f || changeSet[dep] {
					continue
				}
				deps[dep] = true
			}
		}
		if len(deps) == 0 {
			continue
		}
		list := sortedKeys(deps)
		if len(list) > maxDependents {
			list = list[:maxDependents]
		}
		out = append(out, Finding{
			Code:    CodeWillBreak,
			File:    f,
			Message: fmt.Sprintf("%s is referenced by %d file(s) outside the change: %s", f, len(deps), strings.Join(list, ", ")),
		})
	}
	return out
}

// missingCoChangeDirectives reports frequent co-change partners of changed files
// that are absent from the change set, excluding structurally import-linked pairs
// (so only hidden coupling is surfaced).
func missingCoChangeDirectives(h *gitint.History, g *codeGraph, changed []string, changeSet map[string]bool) []Finding {
	if h == nil {
		return nil
	}
	var out []Finding
	for _, f := range changed {
		var absent []string
		for _, p := range h.CoChangePartners(f) {
			if p.Count < coChangeSupport {
				break // partners are sorted by count desc
			}
			if changeSet[p.Path] {
				continue
			}
			if g != nil && g.importLinked(f, p.Path) {
				continue // structural link, not hidden coupling
			}
			absent = append(absent, p.Path)
			if len(absent) >= maxPartners {
				break
			}
		}
		if len(absent) == 0 {
			continue
		}
		out = append(out, Finding{
			Code:    CodeMissingCoChg,
			File:    f,
			Message: fmt.Sprintf("%s usually changes with, but the change omits: %s", f, strings.Join(absent, ", ")),
		})
	}
	return out
}

// governanceDirectives reuses the change-aware gate's governance resolution: each
// governing-Canon finding over the change set becomes a governance_risk directive.
func governanceDirectives(st *store.Store, ops *codeintel.Ops, files []string) []Finding {
	if st == nil {
		return nil
	}
	var out []Finding
	for _, iss := range changegate.Evaluate(st, ops, files) {
		if iss.Code != changegate.CodeGovernedChange {
			continue
		}
		out = append(out, Finding{
			Code:    CodeGovernanceRisk,
			File:    iss.Path,
			Message: iss.Message,
		})
	}
	return out
}

// --- helpers -------------------------------------------------------------

func intersects(a, b map[string]bool) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	// Iterate the smaller set.
	if len(a) > len(b) {
		a, b = b, a
	}
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

func anyInSet(items []string, set map[string]bool) bool {
	for _, i := range items {
		if set[i] {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
