package gitint

import "sort"

// Hotspot classification constants (documented, uncalibrated — a hotspot is a
// ranking within this repo, not an absolute verdict).
const (
	HotspotPercentile = 0.75 // top quartile by churn
	HotspotFloorTotal = 5    // ≥5 commits in the window
	HotspotFloor90d   = 2    // ≥2 commits in the last 90 days
)

// rankFiles assigns each file its repo-relative churn percentile and hotspot
// flag, in place. Files are ranked by temporal hotspot score (recent-commit
// count as tiebreak); a hotspot must clear the top-quartile cut AND the absolute
// activity floors, so a quiet repo flags nothing.
func rankFiles(files []*FileHistory) {
	total := len(files)
	if total == 0 {
		return
	}
	order := make([]*FileHistory, total)
	copy(order, files)
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].TemporalHotspot != order[j].TemporalHotspot {
			return order[i].TemporalHotspot < order[j].TemporalHotspot
		}
		return order[i].Commits90d < order[j].Commits90d
	})
	for rank, f := range order {
		f.ChurnPercentile = float64(rank) / float64(total)
		f.IsHotspot = f.ChurnPercentile >= HotspotPercentile &&
			f.CommitsTotal >= HotspotFloorTotal &&
			f.Commits90d >= HotspotFloor90d
	}
}

// rankHotspots returns the hotspot files sorted by temporal hotspot score
// descending, tie-broken by path ascending for a stable order.
func rankHotspots(files []*FileHistory) []*FileHistory {
	var hot []*FileHistory
	for _, f := range files {
		if f.IsHotspot {
			hot = append(hot, f)
		}
	}
	sort.SliceStable(hot, func(i, j int) bool {
		if hot[i].TemporalHotspot != hot[j].TemporalHotspot {
			return hot[i].TemporalHotspot > hot[j].TemporalHotspot
		}
		return hot[i].Path < hot[j].Path
	})
	return hot
}
