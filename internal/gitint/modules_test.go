package gitint

import (
	"math"
	"testing"
)

func fileWith(path string, total, bus int, hotspot bool, authors map[string]int) *FileHistory {
	return &FileHistory{
		Path: path, CommitsTotal: total, BusFactor: bus, IsHotspot: hotspot, authors: authors,
	}
}

func TestBuildModules_GroupingAndRollups(t *testing.T) {
	files := []*FileHistory{
		fileWith("internal/a.go", 10, 1, true, map[string]int{"Ann": 8, "Bob": 2}),
		fileWith("internal/b.go", 4, 2, false, map[string]int{"Bob": 4}),
		fileWith("cmd/main.go", 2, 1, false, map[string]int{"Cy": 2}),
	}
	mods := buildModules(files)
	if len(mods) != 2 {
		t.Fatalf("modules = %d, want 2 (internal, cmd)", len(mods))
	}
	// Sorted by name: cmd, internal.
	if mods[0].Name != "cmd" || mods[1].Name != "internal" {
		t.Fatalf("module order = %s,%s want cmd,internal", mods[0].Name, mods[1].Name)
	}
	in := mods[1]
	if in.FileCount != 2 {
		t.Errorf("internal FileCount = %d, want 2", in.FileCount)
	}
	if in.HotspotCount != 1 || math.Abs(in.HotspotDensity-0.5) > 1e-9 {
		t.Errorf("internal hotspots = %d density %.2f, want 1 / 0.5", in.HotspotCount, in.HotspotDensity)
	}
	if math.Abs(in.AvgChurn-7.0) > 1e-9 { // (10+4)/2
		t.Errorf("internal AvgChurn = %v, want 7", in.AvgChurn)
	}
	// Owner across internal: Ann 8, Bob 2+4=6 → Ann.
	if in.PrimaryOwner != "Ann" {
		t.Errorf("internal owner = %q, want Ann", in.PrimaryOwner)
	}
}

func TestMedianInt(t *testing.T) {
	if m := medianInt([]int{3, 1, 2}); m != 2 {
		t.Errorf("median(1,2,3) = %d, want 2", m)
	}
	if m := medianInt([]int{4, 1, 3, 2}); m != 2 { // lower median of even set
		t.Errorf("median(1,2,3,4) = %d, want 2 (lower)", m)
	}
	if m := medianInt(nil); m != 0 {
		t.Errorf("median(empty) = %d, want 0", m)
	}
}
