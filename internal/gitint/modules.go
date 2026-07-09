package gitint

import (
	"sort"
	"strings"
)

// Module is a top-level-directory rollup of file histories.
type Module struct {
	Name            string  `json:"name"`
	FileCount       int     `json:"file_count"`
	HotspotCount    int     `json:"hotspot_count"`
	HotspotDensity  float64 `json:"hotspot_density"`
	AvgChurn        float64 `json:"avg_churn"` // mean CommitsTotal across the module's files
	MedianBusFactor int     `json:"median_bus_factor"`
	PrimaryOwner    string  `json:"primary_owner"`
}

// buildModules groups files by their top-level path segment and rolls up per-
// module metrics. Returned sorted by module name for determinism.
func buildModules(files []*FileHistory) []Module {
	groups := map[string][]*FileHistory{}
	for _, f := range files {
		groups[topSegment(f.Path)] = append(groups[topSegment(f.Path)], f)
	}
	mods := make([]Module, 0, len(groups))
	for name, fs := range groups {
		m := Module{Name: name, FileCount: len(fs)}
		var churnSum, hot int
		busFactors := make([]int, 0, len(fs))
		owners := map[string]int{}
		for _, f := range fs {
			churnSum += f.CommitsTotal
			if f.IsHotspot {
				hot++
			}
			busFactors = append(busFactors, f.BusFactor)
			for a, c := range f.authors {
				owners[a] += c
			}
		}
		m.HotspotCount = hot
		if m.FileCount > 0 {
			m.HotspotDensity = float64(hot) / float64(m.FileCount)
			m.AvgChurn = float64(churnSum) / float64(m.FileCount)
		}
		m.MedianBusFactor = medianInt(busFactors)
		m.PrimaryOwner = topAuthor(owners)
		mods = append(mods, m)
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Name < mods[j].Name })
	return mods
}

// topSegment returns the first path segment (the top-level module); a root-level
// file with no slash is its own module.
func topSegment(path string) string {
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}

// medianInt returns the lower median of a set of ints (deterministic for even n).
func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	return s[(len(s)-1)/2]
}
