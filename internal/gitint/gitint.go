// Package gitint is a deterministic, offline git-history intelligence layer. From
// one bounded `git log --numstat` walk it derives per-file behavioral metrics
// (churn, ownership, bus factor, temporal hotspot, co-change), a repo-relative
// hotspot ranking, and top-level-module rollups — all anchored to HEAD's commit
// timestamp so re-runs on identical repository state are byte-identical.
//
// It is pure git (no internal/codeintel import) and lives outside
// internal/canon/...; the authority path may not depend on it (a boundary test
// enforces this). It is an independent implementation from public method
// descriptions and carries no learned constants.
package gitint

import (
	"os/exec"
	"sort"
	"strings"
)

// maxFilesPerCommitForCoChange caps co-change pairing: a commit touching more
// files than this is too diffuse to imply coupling and would cost O(n²) pairs,
// so it contributes to churn/ownership but not to co-change.
const maxFilesPerCommitForCoChange = 50

// DefaultWindow is the default number of recent commits walked.
const DefaultWindow = 1000

// Partner is a co-change partner of a file with the number of shared commits.
type Partner struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// Ownership is the ownership view returned by OwnershipAt for a file or a module.
type Ownership struct {
	Path             string  `json:"path"`
	IsModule         bool    `json:"is_module"`
	PrimaryOwner     string  `json:"primary_owner"`
	PrimaryOwnerPct  float64 `json:"primary_owner_pct,omitempty"`
	RecentOwner      string  `json:"recent_owner,omitempty"`
	ContributorCount int     `json:"contributor_count"`
	BusFactor        int     `json:"bus_factor"`
	Module           *Module `json:"module,omitempty"`
}

// History is the built, in-memory git-intelligence index for one repo window.
type History struct {
	root    string
	asOf    int64
	capped  bool
	files   map[string]*FileHistory
	ordered []*FileHistory // sorted by path
}

// New builds the index from one bounded git-log walk. ok=false when root is not a
// git repository (callers then degrade). Additive: the change-risk-facing methods
// (Churn / CoChangePartners / AuthorCommits) keep their original behavior.
func New(root string, window int) (*History, bool) {
	if window <= 0 {
		window = DefaultWindow
	}
	empty := &History{root: root, files: map[string]*FileHistory{}}
	recs, asOf, capped, ok := walk(root, window)
	if !ok {
		return empty, false
	}
	files := buildFileHistories(recs, asOf)
	ordered := make([]*FileHistory, 0, len(files))
	for _, f := range files {
		ordered = append(ordered, f)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	rankFiles(ordered)
	return &History{root: root, asOf: asOf, capped: capped, files: files, ordered: ordered}, true
}

// AsOf returns the anchor timestamp (HEAD's commit time) used for all windows.
func (h *History) AsOf() int64 { return h.asOf }

// Capped reports whether the walk hit the window bound.
func (h *History) Capped() bool { return h.capped }

// Churn returns the number of commits in the window that touched path.
func (h *History) Churn(path string) int {
	if f := h.files[path]; f != nil {
		return f.CommitsTotal
	}
	return 0
}

// CoChangePartners returns files that changed together with path, sorted by
// shared-commit count descending then path.
func (h *History) CoChangePartners(path string) []Partner {
	if f := h.files[path]; f != nil {
		return f.CoChange
	}
	return nil
}

// AuthorCommits returns the author's commit count reachable from upto, or 0 when
// unknown. This is a direct git query, independent of the built index.
func (h *History) AuthorCommits(author, upto string) int {
	if author == "" {
		return 0
	}
	out, err := exec.Command("git", "-C", h.root,
		"rev-list", "--count", "--author", author, "--no-merges", upto).Output()
	if err != nil {
		return 0
	}
	return atoiOrZero(strings.TrimSpace(string(out)))
}

// File returns the history for a path, or nil if it was not touched in the window.
func (h *History) File(path string) *FileHistory { return h.files[path] }

// Files returns all indexed file histories, sorted by path.
func (h *History) Files() []*FileHistory { return h.ordered }

// Hotspots returns the hotspot files ranked by temporal score descending.
func (h *History) Hotspots() []*FileHistory { return rankHotspots(h.ordered) }

// Modules returns the top-level-directory rollups, sorted by name.
func (h *History) Modules() []Module { return buildModules(h.ordered) }

// OwnershipAt returns the ownership view for a path: a file's own ownership when
// the path is an indexed file, otherwise the rollup for the module (top-level
// directory) the path falls under.
func (h *History) OwnershipAt(path string) Ownership {
	if f := h.files[path]; f != nil {
		return Ownership{
			Path:             path,
			PrimaryOwner:     f.PrimaryOwner,
			PrimaryOwnerPct:  f.PrimaryOwnerPct,
			RecentOwner:      f.RecentOwner,
			ContributorCount: f.ContributorCount,
			BusFactor:        f.BusFactor,
		}
	}
	seg := topSegment(strings.TrimRight(path, "/"))
	for _, m := range h.Modules() {
		if m.Name == seg {
			mod := m
			return Ownership{
				Path:         path,
				IsModule:     true,
				PrimaryOwner: mod.PrimaryOwner,
				BusFactor:    mod.MedianBusFactor,
				Module:       &mod,
			}
		}
	}
	return Ownership{Path: path, IsModule: true}
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func atoiOrZero(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
