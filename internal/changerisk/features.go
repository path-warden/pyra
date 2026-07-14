// Package changerisk computes a deterministic, offline, pre-merge change-risk
// signal from the shape of a diff (Kamei just-in-time metrics), a repo-relative
// ranking, and actionable PR directives.
//
// It lives outside internal/canon/... (like internal/codeintel and
// internal/changegate): it depends on internal/store, internal/codeintel, and
// internal/gitint, none of which the authority path may import. A boundary test
// enforces that internal/canon never depends on this package.
//
// The scoring model (feature standardization, the logistic formula, and its
// learned constants in model_constants.go) is a faithful port of repowise's
// change-risk model so Pyra is parity-testable against it; the constants are
// confined to that one file so they can be replaced later.
package changerisk

import (
	"math"
	"os/exec"
	"strings"
)

// ChangeFeatures holds the Kamei change metrics for one change (a commit or a
// base..head range). Field shape mirrors repowise's ChangeFeatures for parity.
type ChangeFeatures struct {
	LA      int     // lines added
	LD      int     // lines deleted
	NF      int     // files touched
	ND      int     // distinct directories touched
	NS      int     // distinct top-level subsystems touched
	Entropy float64 // Shannon entropy of the per-file churn distribution
	// Exp is the author's prior-commit count. nil means unknown (e.g. a staged
	// diff with no commit/author yet); the scorer treats unknown as neutral
	// rather than imputing inexperience.
	Exp     *int
	Author  string
	Subject string
	Ref     string
}

// fileChange is one (path, additions, deletions) row of a diff.
type fileChange struct {
	Path          string
	Adds, Deletes int
}

// git runs a git command at root and returns stdout (empty on error).
func git(root string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// parseNumstat parses `git ... --numstat` output into file changes, filtering by
// extension suffixes when exts is non-empty. Binary rows ("-\t-\tpath") count 0.
func parseNumstat(numstat string, exts []string) []fileChange {
	var changes []fileChange
	for _, row := range strings.Split(numstat, "\n") {
		if row == "" {
			continue
		}
		parts := strings.Split(row, "\t")
		if len(parts) != 3 {
			continue
		}
		path := parts[2]
		if len(exts) > 0 && !hasAnySuffix(path, exts) {
			continue
		}
		changes = append(changes, fileChange{
			Path:    path,
			Adds:    atoiOrZero(parts[0]),
			Deletes: atoiOrZero(parts[1]),
		})
	}
	return changes
}

// featuresFromChanges builds the diffusion metrics from a set of file changes.
// exp/author/subject/ref are supplied by the caller (they are not derivable from
// the numstat alone).
func featuresFromChanges(changes []fileChange, exp *int, author, subject, ref string) ChangeFeatures {
	la, ld, nf := 0, 0, 0
	dirs := map[string]bool{}
	subs := map[string]bool{}
	var perFile []int
	for _, c := range changes {
		la += c.Adds
		ld += c.Deletes
		nf++
		if churn := c.Adds + c.Deletes; churn > 0 {
			perFile = append(perFile, churn)
		}
		segs := strings.Split(c.Path, "/")
		dirs[strings.Join(segs[:len(segs)-1], "/")] = true
		subs[segs[0]] = true
	}
	return ChangeFeatures{
		LA: la, LD: ld, NF: nf,
		ND: len(dirs), NS: len(subs),
		Entropy: entropy(perFile),
		Exp:     exp, Author: author, Subject: subject, Ref: ref,
	}
}

// entropy is the Shannon entropy (base 2) of the per-file churn distribution;
// 0 when there is no churn or fewer than two files (no diffusion to measure).
func entropy(perFile []int) float64 {
	total := 0
	for _, p := range perFile {
		total += p
	}
	if total <= 0 || len(perFile) < 2 {
		return 0.0
	}
	var h float64
	for _, p := range perFile {
		if p > 0 {
			frac := float64(p) / float64(total)
			h -= frac * math.Log2(frac)
		}
	}
	return h
}

// ChangeMode selects which kind of change to score.
type ChangeMode int

const (
	ModeStaged ChangeMode = iota // the git staged index
	ModeCommit                   // a single commit (SHA)
	ModeRange                    // a base..head range
	ModeSince                    // working tree vs a ref
	ModeFiles                    // an explicit file list (no line counts)
)

// Change identifies the change to score.
type Change struct {
	Mode  ChangeMode
	SHA   string   // ModeCommit
	Base  string   // ModeRange
	Head  string   // ModeRange
	Ref   string   // ModeSince
	Files []string // ModeFiles
}

// Anchor returns the git ref to sample the repo-relative baseline from, and the
// sha to exclude from that baseline (its own score), for this change.
func (c Change) Anchor() (anchor, exclude string) {
	switch c.Mode {
	case ModeCommit:
		return c.SHA, c.SHA
	case ModeRange:
		return c.Head, ""
	default:
		return "HEAD", ""
	}
}

// Extract computes the change's features and the list of changed file paths (in
// numstat order). Modes without a commit author (staged / since / files) score
// experience as unknown.
func (c Change) Extract(root string, exts []string) (ChangeFeatures, []string) {
	if c.Mode == ModeFiles {
		// No diff to measure line counts; build diffusion from the paths alone.
		changes := make([]fileChange, 0, len(c.Files))
		for _, p := range c.Files {
			if p != "" {
				changes = append(changes, fileChange{Path: p})
			}
		}
		return featuresFromChanges(changes, nil, "", "", "files"), pathsOf(changes)
	}

	var numstat, author, subject, ref, upto string
	switch c.Mode {
	case ModeCommit:
		author, subject = commitMeta(root, c.SHA)
		numstat = git(root, "show", c.SHA, "--no-renames", "--numstat", "--format=")
		upto = strings.TrimSpace(git(root, "rev-parse", "--verify", "--quiet", c.SHA+"^"))
		ref = c.SHA
		// For the root commit (no parent), author experience is 0 — counting from
		// the commit itself would include the commit being scored.
	case ModeRange:
		numstat = git(root, "diff", "--no-renames", "--numstat", c.Base+".."+c.Head)
		author, subject = commitMeta(root, c.Head)
		upto, ref = c.Base, c.Base+".."+c.Head
	case ModeSince:
		numstat = git(root, "diff", "--no-renames", "--numstat", c.Ref)
		ref = "since " + c.Ref
	default: // ModeStaged
		numstat = git(root, "diff", "--cached", "--no-renames", "--numstat")
		ref = "staged"
	}
	changes := parseNumstat(numstat, exts)
	var exp *int
	if c.Mode == ModeCommit || c.Mode == ModeRange {
		exp = authorExperience(root, author, upto)
	}
	return featuresFromChanges(changes, exp, author, subject, ref), pathsOf(changes)
}

func pathsOf(changes []fileChange) []string {
	paths := make([]string, 0, len(changes))
	for _, ch := range changes {
		paths = append(paths, ch.Path)
	}
	return paths
}

// ExtractStaged builds features for the staged index (Exp unknown).
func ExtractStaged(root string, exts []string) ChangeFeatures {
	f, _ := Change{Mode: ModeStaged}.Extract(root, exts)
	return f
}

// ExtractCommit builds features for a single commit.
func ExtractCommit(root, sha string, exts []string) ChangeFeatures {
	f, _ := Change{Mode: ModeCommit, SHA: sha}.Extract(root, exts)
	return f
}

// ExtractRange builds features for a base..head range scored as one change.
func ExtractRange(root, base, head string, exts []string) ChangeFeatures {
	f, _ := Change{Mode: ModeRange, Base: base, Head: head}.Extract(root, exts)
	return f
}

// commitMeta returns the author name and subject of a commit.
func commitMeta(root, ref string) (author, subject string) {
	meta := strings.Trim(git(root, "show", "-s", "--format=%an%x00%s", ref), "\n")
	author, subject, _ = strings.Cut(meta, "\x00")
	return author, subject
}

// authorExperience returns the author's prior-commit count reachable from upto,
// or nil when the author is unknown.
func authorExperience(root, author, upto string) *int {
	if author == "" {
		return nil
	}
	out := strings.TrimSpace(git(root, "rev-list", "--count", "--author", author, "--no-merges", upto))
	n := atoiOrZero(out)
	return &n
}

func hasAnySuffix(s string, suffixes []string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func atoiOrZero(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
