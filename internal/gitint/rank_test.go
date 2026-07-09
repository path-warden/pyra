package gitint

import "testing"

// mkFile is a FileHistory with enough fields set to drive ranking.
func mkFile(path string, temporal float64, total, c90 int) *FileHistory {
	return &FileHistory{Path: path, TemporalHotspot: temporal, CommitsTotal: total, Commits90d: c90}
}

func TestRankFiles_TopQuartileHotspot(t *testing.T) {
	// 4 files; only the top-temporal one is in the top quartile (pct 0.75).
	files := []*FileHistory{
		mkFile("a", 0.5, 6, 3),
		mkFile("b", 1.0, 6, 3),
		mkFile("c", 2.0, 6, 3),
		mkFile("d", 9.0, 6, 3), // highest → rank 3 → pct 0.75
	}
	rankFiles(files)
	byPath := map[string]*FileHistory{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	if !byPath["d"].IsHotspot {
		t.Error("highest-churn file d should be a hotspot")
	}
	for _, p := range []string{"a", "b", "c"} {
		if byPath[p].IsHotspot {
			t.Errorf("%s (below top quartile) should not be a hotspot (pct %.2f)", p, byPath[p].ChurnPercentile)
		}
	}
}

func TestRankFiles_QuietRepoFloorsSuppress(t *testing.T) {
	// Top-quartile by rank but below activity floors → no hotspot.
	files := []*FileHistory{
		mkFile("a", 0.1, 1, 1),
		mkFile("b", 0.2, 1, 1),
		mkFile("c", 0.3, 1, 1),
		mkFile("d", 0.4, 1, 1), // top quartile, but only 1 commit
	}
	rankFiles(files)
	for _, f := range files {
		if f.IsHotspot {
			t.Errorf("%s should not be a hotspot below floors (total=%d, 90d=%d)", f.Path, f.CommitsTotal, f.Commits90d)
		}
	}
}

func TestRankHotspots_StableOrderUnderShuffle(t *testing.T) {
	// 8 files: 6 low (ranks 0..5 → pct ≤ 0.625, not hotspots) + 2 tied high
	// (ranks 6,7 → pct ≥ 0.75, both hotspots). The tie exercises the path
	// tiebreak in rankHotspots.
	build := func() []*FileHistory {
		fs := []*FileHistory{
			mkFile("z", 9.0, 6, 3), // tie with a
			mkFile("a", 9.0, 6, 3),
		}
		for i := 0; i < 6; i++ {
			fs = append(fs, mkFile("low"+string(rune('0'+i)), float64(i)*0.1, 6, 3))
		}
		return fs
	}
	f1 := build()
	rankFiles(f1)
	h1 := rankHotspots(f1)

	f2 := build()
	f2[0], f2[1] = f2[1], f2[0] // different input order
	rankFiles(f2)
	h2 := rankHotspots(f2)

	if len(h1) != 2 || len(h2) != 2 {
		t.Fatalf("hotspot counts = %d / %d, want 2 / 2", len(h1), len(h2))
	}
	for i := range h1 {
		if h1[i].Path != h2[i].Path {
			t.Errorf("order differs at %d: %s vs %s", i, h1[i].Path, h2[i].Path)
		}
	}
	// Temporal tie broken by path ascending: a before z.
	if h1[0].Path != "a" || h1[1].Path != "z" {
		t.Errorf("tie order = %s,%s want a,z", h1[0].Path, h1[1].Path)
	}
}
