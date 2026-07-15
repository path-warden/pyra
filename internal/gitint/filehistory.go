package gitint

import (
	"math"
	"sort"
	"strings"
)

// HotspotHalfLifeDays is the half-life of the exponential churn decay in the
// temporal hotspot score.
const HotspotHalfLifeDays = 180.0

const secondsPerDay = 86400

// FileHistory is the per-file behavioral metric bundle derived from git history.
type FileHistory struct {
	Path             string    `json:"path"`
	CommitsTotal     int       `json:"commits_total"`
	Commits30d       int       `json:"commits_30d"`
	Commits90d       int       `json:"commits_90d"`
	LinesAdded       int       `json:"lines_added"`
	LinesDeleted     int       `json:"lines_deleted"`
	FirstCommit      int64     `json:"first_commit"`
	LastCommit       int64     `json:"last_commit"`
	AgeDays          int       `json:"age_days"`
	TemporalHotspot  float64   `json:"temporal_hotspot"`
	PrimaryOwner     string    `json:"primary_owner"`
	PrimaryOwnerPct  float64   `json:"primary_owner_pct"`
	RecentOwner      string    `json:"recent_owner,omitempty"`
	ContributorCount int       `json:"contributor_count"`
	BusFactor        int       `json:"bus_factor"`
	ChurnPercentile  float64   `json:"churn_percentile"`
	IsHotspot        bool      `json:"is_hotspot"`
	ChangeEntropy    float64   `json:"change_entropy"`     // Shannon entropy of per-commit churn (diffusion over time)
	PriorDefectCount int       `json:"prior_defect_count"` // bug-fix commits touching this file in the window
	CoChange         []Partner `json:"co_change,omitempty"`

	// authors is the per-author commit count for this file, retained (unexported,
	// so not serialized) to compute module-level ownership rollups.
	authors map[string]int
	// perCommitChurn accumulates this file's churn per commit, for change entropy.
	perCommitChurn []int
}

// buildFileHistories aggregates commit records into per-file histories, anchored
// to asOf (HEAD's commit time) for all recency windows and time decay. Pure — no
// subprocess — so it is unit-testable from synthetic records.
func buildFileHistories(recs []commitRec, asOf int64) map[string]*FileHistory {
	cut30 := asOf - 30*secondsPerDay
	cut90 := asOf - 90*secondsPerDay

	fh := map[string]*FileHistory{}
	authorCounts := map[string]map[string]int{}
	recentCounts := map[string]map[string]int{}
	cochange := map[string]map[string]int{}

	for _, rec := range recs {
		// Distinct files in this commit (a rename/edit can list a path twice).
		seen := map[string]bool{}
		var uniq []string
		for _, fd := range rec.Files {
			f := fh[fd.Path]
			if f == nil {
				f = &FileHistory{Path: fd.Path}
				fh[fd.Path] = f
			}
			f.LinesAdded += fd.Added
			f.LinesDeleted += fd.Deleted

			if !seen[fd.Path] {
				seen[fd.Path] = true
				uniq = append(uniq, fd.Path)
				f.CommitsTotal++
				if rec.TS >= cut30 {
					f.Commits30d++
				}
				if rec.TS >= cut90 {
					f.Commits90d++
					inc(recentCounts, fd.Path, rec.Author)
				}
				if f.FirstCommit == 0 || rec.TS < f.FirstCommit {
					f.FirstCommit = rec.TS
				}
				if rec.TS > f.LastCommit {
					f.LastCommit = rec.TS
				}
				inc(authorCounts, fd.Path, rec.Author)
				if churn := fd.Added + fd.Deleted; churn > 0 {
					f.perCommitChurn = append(f.perCommitChurn, churn)
				}
				if isFixCommit(rec.Subject) {
					f.PriorDefectCount++
				}
			}
			// Temporal hotspot: decayed per-commit churn (counted per file delta).
			ageDays := math.Max(float64(asOf-rec.TS)/secondsPerDay, 0)
			weight := math.Exp(-math.Ln2 * ageDays / HotspotHalfLifeDays)
			f.TemporalHotspot += weight * math.Min(float64(fd.Added+fd.Deleted)/100.0, 3.0)
		}
		if len(uniq) <= maxFilesPerCommitForCoChange {
			for i := 0; i < len(uniq); i++ {
				for j := i + 1; j < len(uniq); j++ {
					a, b := uniq[i], uniq[j]
					inc(cochange, a, b)
					inc(cochange, b, a)
				}
			}
		}
	}

	for path, f := range fh {
		if f.FirstCommit > 0 {
			f.AgeDays = int((asOf - f.FirstCommit) / secondsPerDay)
		}
		f.ChangeEntropy = entropy(f.perCommitChurn)
		f.perCommitChurn = nil
		ac := authorCounts[path]
		f.authors = ac
		f.ContributorCount = len(ac)
		f.PrimaryOwner, f.PrimaryOwnerPct = primaryOwner(ac, f.CommitsTotal)
		f.BusFactor = busFactor(ac)
		f.RecentOwner = topAuthor(recentCounts[path])
		f.CoChange = partnersFrom(cochange[path])
	}
	return fh
}

// primaryOwner returns the author with the most commits (name-tiebroken) and its
// share of total commits.
func primaryOwner(counts map[string]int, total int) (string, float64) {
	name := topAuthor(counts)
	if name == "" || total == 0 {
		return name, 0
	}
	return name, float64(counts[name]) / float64(total)
}

// busFactor is the minimum number of top authors whose cumulative commit share
// first reaches 80%.
func busFactor(counts map[string]int) int {
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}
	threshold := 0.8 * float64(total)
	running, bus := 0, 0
	for _, a := range rankAuthors(counts) {
		running += counts[a]
		bus++
		if float64(running) >= threshold {
			break
		}
	}
	return bus
}

// topAuthor returns the highest-count author, tie-broken by name ascending, or
// "" when there are none.
func topAuthor(counts map[string]int) string {
	ranked := rankAuthors(counts)
	if len(ranked) == 0 {
		return ""
	}
	return ranked[0]
}

// rankAuthors returns author names ordered by count descending, then name
// ascending — a deterministic order independent of map iteration.
func rankAuthors(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	return names
}

func partnersFrom(m map[string]int) []Partner {
	partners := make([]Partner, 0, len(m))
	for p, c := range m {
		partners = append(partners, Partner{Path: p, Count: c})
	}
	sort.Slice(partners, func(i, j int) bool {
		if partners[i].Count != partners[j].Count {
			return partners[i].Count > partners[j].Count
		}
		return partners[i].Path < partners[j].Path
	})
	return partners
}

func inc(m map[string]map[string]int, k1, k2 string) {
	if m[k1] == nil {
		m[k1] = map[string]int{}
	}
	m[k1][k2]++
}

// fixWords are the whole-word bug-fix markers, matched on word boundaries so that
// "prefix"/"debug"/"dispatch" do not count.
var fixWords = map[string]bool{
	"fix": true, "fixes": true, "fixed": true, "bug": true, "bugfix": true,
	"hotfix": true, "revert": true, "reverts": true, "patch": true,
}

// isFixCommit reports whether a commit subject looks like a bug fix, by message
// convention (documented, not learned): a whole-word "fix"/"bug"/"hotfix"/... token.
func isFixCommit(subject string) bool {
	for _, w := range strings.FieldsFunc(strings.ToLower(subject), func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		if fixWords[w] {
			return true
		}
	}
	return false
}

// entropy is the Shannon entropy (base 2) of a churn distribution; 0 when there
// is nothing to measure (no churn or a single commit).
func entropy(perCommit []int) float64 {
	total := 0
	for _, c := range perCommit {
		total += c
	}
	if total <= 0 || len(perCommit) < 2 {
		return 0.0
	}
	var h float64
	for _, c := range perCommit {
		if c > 0 {
			frac := float64(c) / float64(total)
			h -= frac * math.Log2(frac)
		}
	}
	return h
}
