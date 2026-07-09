package changegate

import (
	"regexp"
	"sort"
	"strings"

	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/codeintel"
	"github.com/chasedputnam/memphis/internal/store"
)

// symbolIDRe matches a grove-style symbol-id embedded in prose, the same pattern
// the grounding tools use. Governance parses each match's path component.
var symbolIDRe = regexp.MustCompile(`[\w.-]+:[^#\s]+#[^@\s]+@\d+`)

// Evaluate resolves which Accepted Canon artifacts govern each changed file and
// returns the findings as classified-ready model.Issues (severity "warning"),
// sorted deterministically. It iterates the full Canon set in load order — never
// the fuzzy search index — so results are complete and reproducible.
func Evaluate(s *store.Store, ops *codeintel.Ops, files []string) []model.Issue {
	if s == nil || len(files) == 0 {
		return nil
	}
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	findings := governanceFindings(s, fileSet)
	// Drift over touched code is folded in by the drift pass (see drift.go).
	findings = append(findings, driftFindings(s, ops, fileSet)...)

	iss := issues(findings)
	sortIssues(iss)
	return iss
}

// governanceFindings emits one CodeGovernedChange finding per (changed file,
// governing live artifact). A superseded governing artifact resolves to its live
// successor; one with no live successor is never cited (no dead authority).
func governanceFindings(s *store.Store, fileSet map[string]bool) []Finding {
	var out []Finding
	for i := range s.Canon {
		item := &s.Canon[i]
		gov := governedFiles(item, fileSet)
		if len(gov) == 0 {
			continue
		}
		cited := item
		if item.Status == "superseded" {
			succ := s.Successor(item.ID)
			if succ == nil || !isGoverning(succ.Status) {
				continue // dead/draft authority with no live successor is never cited
			}
			cited = succ
		} else if !isGoverning(item.Status) {
			continue // only Accepted (or status-less requirements/designs) govern
		}
		for _, f := range gov {
			out = append(out, Finding{
				Code:     CodeGovernedChange,
				File:     f,
				Artifact: cited.ID,
				Type:     cited.Type,
				Status:   cited.Status,
				Title:    cited.Title,
			})
		}
	}
	return out
}

// governedFiles returns, sorted, the changed files this artifact governs: those
// it cites by a symbol-id path or by a boundary-delimited literal path mention.
func governedFiles(item *store.Item, fileSet map[string]bool) []string {
	symPaths := citedSymbolPaths(item.Body)
	var out []string
	for f := range fileSet {
		if symPaths[f] || bodyMentionsFile(item.Body, f) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// citedSymbolPaths returns the set of file paths named by symbol-ids in a body.
func citedSymbolPaths(body string) map[string]bool {
	out := map[string]bool{}
	for _, sid := range symbolIDRe.FindAllString(body, -1) {
		if p, _, _, ok := codeintel.ParseID(sid); ok {
			out[p] = true
		}
	}
	return out
}

// bodyMentionsFile reports whether body cites the literal file path f on path
// boundaries — so "a.go" does not match inside "aaa.go" or "x/a.go", and
// "store.go" still matches inside a symbol-id like "…store.go#Put@42". This is a
// literal match (REQ-504), just boundary-aware to cut the worst false positives.
func bodyMentionsFile(body, f string) bool {
	for idx := 0; ; {
		i := strings.Index(body[idx:], f)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(f)
		if boundedLeft(body, start) && boundedRight(body, end) {
			return true
		}
		idx = start + 1
	}
}

// boundedLeft reports whether the byte before pos cannot be part of a longer path
// segment (so the match is not the tail of a deeper path or a longer word).
func boundedLeft(s string, pos int) bool {
	if pos == 0 {
		return true
	}
	return !isPathByte(s[pos-1]) && s[pos-1] != '/'
}

// boundedRight reports whether the byte at pos cannot extend the filename.
func boundedRight(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	c := s[pos]
	// '#' begins a symbol-id suffix and is a valid terminator; '.' and '/' would
	// extend into a different file/dir; alphanumerics/-/_ extend the name.
	if c == '#' {
		return true
	}
	return !isPathByte(c) && c != '/'
}

// isPathByte reports whether c can appear inside a filename segment.
func isPathByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '_', c == '-', c == '.':
		return true
	}
	return false
}

// isGoverning reports whether an artifact with this lifecycle status actively
// governs code. Accepted decisions govern; requirements/designs that carry no
// status section (status == "") govern; draft/rejected/deprecated/retired/
// superseded statuses do not (a superseded artifact resolves to its successor
// first). This scopes governance to Accepted authority per the requirement, so a
// proposed or rejected decision never reads as governing a change.
func isGoverning(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "accepted":
		return true
	}
	return false
}

// isDead reports whether a lifecycle status is retired/superseded (not live).
func isDead(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "superseded", "retired", "deprecated", "rejected":
		return true
	}
	return false
}

// sortIssues orders findings deterministically by (Path, Code, Message).
func sortIssues(iss []model.Issue) {
	sort.SliceStable(iss, func(i, j int) bool {
		if iss[i].Path != iss[j].Path {
			return iss[i].Path < iss[j].Path
		}
		if iss[i].Code != iss[j].Code {
			return iss[i].Code < iss[j].Code
		}
		return iss[i].Message < iss[j].Message
	})
}
