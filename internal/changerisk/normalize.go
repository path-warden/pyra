package changerisk

import "sort"

// MinBaseline is the smallest number of baseline commit scores needed before a
// repo-relative percentile/priority is considered stable. Below it, the ranking
// is reported as unavailable and only the raw score is shown.
const MinBaseline = 20

// Tercile cut points on the repo-relative percentile (0–100).
const (
	elevatedPct = 200.0 / 3.0 // ≈ 66.67
	typicalPct  = 100.0 / 3.0 // ≈ 33.33
)

// Review-priority labels (Memphis wording for the repo-relative terciles).
const (
	PriorityBelow    = "Below typical"
	PriorityTypical  = "Typical"
	PriorityElevated = "Elevated"
)

// Normalizer maps a raw change-risk score to its rank within one repo's own
// recent-commit distribution — the portable signal (the absolute band is not).
type Normalizer struct {
	scores []float64 // sorted ascending
}

// NewNormalizer builds a normalizer from baseline scores (any order).
func NewNormalizer(scores []float64) *Normalizer {
	s := append([]float64(nil), scores...)
	sort.Float64s(s)
	return &Normalizer{scores: s}
}

// Count is the number of baseline scores.
func (n *Normalizer) Count() int { return len(n.scores) }

// Available reports whether the distribution is large enough to rank against.
func (n *Normalizer) Available() bool { return len(n.scores) >= MinBaseline }

// Percentile is the mid-rank percentile (0–100) of score within the
// distribution: ties share the average rank, so identical scores map to one
// percentile. Returns 0 when there is no distribution.
func (n *Normalizer) Percentile(score float64) float64 {
	m := len(n.scores)
	if m == 0 {
		return 0
	}
	below := sort.SearchFloat64s(n.scores, score) // bisect_left
	equal := upperBound(n.scores, score) - below  // bisect_right - below
	return 100.0 * (float64(below) + 0.5*float64(equal)) / float64(m)
}

// Priority is the repo-relative review priority from the score's percentile
// tercile. Callers should gate on Available() before trusting it.
func (n *Normalizer) Priority(score float64) string {
	if len(n.scores) == 0 {
		return PriorityBelow
	}
	switch pct := n.Percentile(score); {
	case pct >= elevatedPct:
		return PriorityElevated
	case pct >= typicalPct:
		return PriorityTypical
	default:
		return PriorityBelow
	}
}

// upperBound returns the index of the first element strictly greater than v
// (bisect_right) in a sorted slice.
func upperBound(s []float64, v float64) int {
	return sort.Search(len(s), func(i int) bool { return s[i] > v })
}
