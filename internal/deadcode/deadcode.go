// Package deadcode turns the code graph's reachability result into an actionable
// dead-code report: unreachable symbols, tiered by confidence, with a cleanup-
// impact estimate and a memphis-unique "governed dead code" flag (an unreachable
// symbol still cited by Accepted Canon).
//
// It does no new graph work — it consumes codegraph.Reachability. It lives
// outside internal/canon/... (the authority path may not depend on it) and is
// deterministic and offline.
package deadcode

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/chasedputnam/memphis/internal/codegraph"
	"github.com/chasedputnam/memphis/internal/codeintel"
)

// Confidence tiers.
const (
	TierHigh   = "high"
	TierMedium = "medium"
	TierLow    = "low"
)

// Candidate is one unreachable symbol reported as likely dead.
type Candidate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Tier     string `json:"tier"`
	Impact   int    `json:"impact"` // source line count
	Governed bool   `json:"governed,omitempty"`
}

// Report is the ranked dead-code result.
type Report struct {
	Candidates  []Candidate `json:"candidates"`
	TotalImpact int         `json:"total_impact"`
}

// symbolIDRe matches an embedded symbol-id, the same literal pattern the grounding
// tools use (no fuzzy matching).
var symbolIDRe = regexp.MustCompile(`[\w.-]+:[^#\s]+#[^@\s]+@\d+`)

// Analyze builds a dead-code report from the graph's unreachable set. root is the
// code root for reference searches; canonBodies are the Canon artifact bodies used
// to flag governed dead code (empty when there is no Canon).
func Analyze(g *codegraph.Graph, ops *codeintel.Ops, root string, canonBodies []string) Report {
	if g == nil {
		return Report{}
	}
	cited := canonCitedSymbols(canonBodies)
	reach := g.Reachability()

	// Precompute, in one pass over ALL source files under the roots (not just
	// definition-bearing ones), the set of files each candidate name appears in.
	// A name is "referenced" when it occurs in a file OTHER than its own
	// definition file — so a doc comment or recursive self-call in the symbol's
	// own file does not count (keeping a genuinely-dead symbol at high tier),
	// while a cross-file dispatch/reflective reference does (medium tier).
	names := map[string]bool{}
	for _, id := range reach.Unreachable {
		if node := g.Symbols[id]; node != nil {
			names[node.Name] = true
		}
	}
	filesWithName := nameFiles(root, names)

	var out []Candidate
	total := 0
	for _, id := range reach.Unreachable {
		node := g.Symbols[id]
		if node == nil || isTestEntry(node.Name) {
			continue
		}
		impact := symbolLines(ops, id)
		hasRefs := referencedElsewhere(filesWithName[node.Name], node.File)
		c := Candidate{
			ID:       id,
			Name:     node.Name,
			Kind:     node.Kind,
			File:     node.File,
			Tier:     classify(node.File, hasRefs),
			Impact:   impact,
			Governed: cited[id],
		}
		out = append(out, c)
		total += impact
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Impact != out[j].Impact {
			return out[i].Impact > out[j].Impact
		}
		return out[i].ID < out[j].ID
	})
	return Report{Candidates: out, TotalImpact: total}
}

// classify assigns a confidence tier: low if defined in a test file; medium if the
// symbol's name still has surviving references (possible dynamic/reflective use or
// use from other dead code); else high.
func classify(file string, hasRefs bool) string {
	if isTestFile(file) {
		return TierLow
	}
	if hasRefs {
		return TierMedium
	}
	return TierHigh
}

// maxScanFileSize bounds the per-file read during the whole-source scan.
const maxScanFileSize = 1 << 20 // 1 MiB

// nameFiles walks all source files under root (skipping VCS/vendor/build dirs and
// oversized/binary files) and records, per candidate name, the set of
// root-relative files whose tokens contain that name. Tokenizing raw text catches
// references in code, strings, and comments alike. One pass, deterministic.
func nameFiles(root string, names map[string]bool) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if len(names) == 0 || root == "" {
		return out
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || info.Size() > maxScanFileSize {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		var cur strings.Builder
		flush := func() {
			if cur.Len() > 0 {
				if tok := cur.String(); names[tok] {
					if out[tok] == nil {
						out[tok] = map[string]bool{}
					}
					out[tok][rel] = true
				}
				cur.Reset()
			}
		}
		for _, r := range string(data) {
			if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				cur.WriteRune(r)
			} else {
				flush()
			}
		}
		flush()
		return nil
	})
	return out
}

// referencedElsewhere reports whether the name appears in any file other than its
// own definition file.
func referencedElsewhere(files map[string]bool, ownFile string) bool {
	for f := range files {
		if f != ownFile {
			return true
		}
	}
	return false
}

// skipDir reports directories the source scan should not descend into.
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".idea", ".vscode":
		return true
	}
	return false
}

// symbolLines returns the number of source lines a symbol spans (0 if unresolved).
func symbolLines(ops *codeintel.Ops, id string) int {
	if ops == nil {
		return 0
	}
	res, err := ops.Source(id, "")
	if err != nil || res.Source == "" {
		return 0
	}
	return strings.Count(res.Source, "\n") + 1
}

// canonCitedSymbols returns the set of symbol-ids literally cited in Canon bodies.
func canonCitedSymbols(bodies []string) map[string]bool {
	out := map[string]bool{}
	for _, b := range bodies {
		for _, id := range symbolIDRe.FindAllString(b, -1) {
			out[id] = true
		}
	}
	return out
}

// isTestEntry reports a test entry point (Test-prefixed function or main).
func isTestEntry(name string) bool {
	return name == "main" || strings.HasPrefix(name, "Test")
}

// isTestFile reports whether a path is a test file by convention.
func isTestFile(path string) bool {
	base := path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		base = path[i+1:]
	}
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasPrefix(base, "test_") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, "Test.java")
}
