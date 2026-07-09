package gitint

import (
	"math"
	"testing"
)

const day = int64(secondsPerDay)

func rec(sha, author string, ts int64, files ...fileDelta) commitRec {
	return commitRec{SHA: sha, Author: author, TS: ts, Files: files}
}

func TestBuildFileHistories_Windows(t *testing.T) {
	asOf := int64(1_000_000_000)
	recs := []commitRec{
		rec("c1", "Ann", asOf, fileDelta{"a.go", 10, 0}),        // now
		rec("c2", "Ann", asOf-40*day, fileDelta{"a.go", 5, 1}),  // 40d ago
		rec("c3", "Ann", asOf-100*day, fileDelta{"a.go", 2, 0}), // 100d ago
	}
	fh := buildFileHistories(recs, asOf)
	a := fh["a.go"]
	if a.CommitsTotal != 3 {
		t.Errorf("CommitsTotal = %d, want 3", a.CommitsTotal)
	}
	if a.Commits30d != 1 {
		t.Errorf("Commits30d = %d, want 1", a.Commits30d)
	}
	if a.Commits90d != 2 {
		t.Errorf("Commits90d = %d, want 2", a.Commits90d)
	}
	if a.LinesAdded != 17 || a.LinesDeleted != 1 {
		t.Errorf("churn = +%d/-%d, want +17/-1", a.LinesAdded, a.LinesDeleted)
	}
	if a.AgeDays != 100 {
		t.Errorf("AgeDays = %d, want 100", a.AgeDays)
	}
}

func TestBuildFileHistories_OwnershipAndBusFactor(t *testing.T) {
	asOf := int64(1_000_000_000)

	// Ann×4, Bob×1 → Ann owns 80% → bus factor 1.
	var recs []commitRec
	for i := 0; i < 4; i++ {
		recs = append(recs, rec("a"+string(rune('0'+i)), "Ann", asOf-int64(i)*day, fileDelta{"f.go", 1, 0}))
	}
	recs = append(recs, rec("b", "Bob", asOf-9*day, fileDelta{"f.go", 1, 0}))
	f := buildFileHistories(recs, asOf)["f.go"]
	if f.PrimaryOwner != "Ann" {
		t.Errorf("owner = %q, want Ann", f.PrimaryOwner)
	}
	if math.Abs(f.PrimaryOwnerPct-0.8) > 1e-9 {
		t.Errorf("owner pct = %v, want 0.8", f.PrimaryOwnerPct)
	}
	if f.ContributorCount != 2 {
		t.Errorf("contributors = %d, want 2", f.ContributorCount)
	}
	if f.BusFactor != 1 {
		t.Errorf("bus factor = %d, want 1 (Ann alone reaches 80%%)", f.BusFactor)
	}
}

func TestBusFactor_EvenSplitAndTwoThirds(t *testing.T) {
	// 3-way even split → need all 3 to reach 80%.
	even := map[string]int{"A": 1, "B": 1, "C": 1}
	if bf := busFactor(even); bf != 3 {
		t.Errorf("even 3-way bus factor = %d, want 3", bf)
	}
	// Ann×2, Bob×1: 0.67 < 0.8, so both needed → 2.
	if bf := busFactor(map[string]int{"Ann": 2, "Bob": 1}); bf != 2 {
		t.Errorf("2:1 bus factor = %d, want 2", bf)
	}
	if bf := busFactor(map[string]int{}); bf != 0 {
		t.Errorf("empty bus factor = %d, want 0", bf)
	}
}

func TestBuildFileHistories_RecentOwner(t *testing.T) {
	asOf := int64(1_000_000_000)
	recs := []commitRec{
		rec("c1", "Old", asOf-200*day, fileDelta{"a.go", 1, 0}), // outside 90d
		rec("c2", "New", asOf-10*day, fileDelta{"a.go", 1, 0}),  // in 90d
	}
	a := buildFileHistories(recs, asOf)["a.go"]
	if a.RecentOwner != "New" {
		t.Errorf("recent owner = %q, want New", a.RecentOwner)
	}

	// A file with no commits in 90d → empty recent owner.
	old := []commitRec{rec("c1", "Old", asOf-200*day, fileDelta{"b.go", 1, 0})}
	b := buildFileHistories(old, asOf)["b.go"]
	if b.RecentOwner != "" {
		t.Errorf("stale file recent owner = %q, want empty", b.RecentOwner)
	}
}

func TestBuildFileHistories_TemporalOrdering(t *testing.T) {
	asOf := int64(1_000_000_000)
	recs := []commitRec{
		rec("r", "Ann", asOf-5*day, fileDelta{"recent.go", 100, 0}), // recent, big
		rec("o", "Ann", asOf-300*day, fileDelta{"old.go", 100, 0}),  // old, big
	}
	fh := buildFileHistories(recs, asOf)
	if fh["recent.go"].TemporalHotspot <= fh["old.go"].TemporalHotspot {
		t.Errorf("recent churn should outweigh equally-sized old churn: recent=%v old=%v",
			fh["recent.go"].TemporalHotspot, fh["old.go"].TemporalHotspot)
	}
}

func TestBuildFileHistories_CoChange(t *testing.T) {
	asOf := int64(1_000_000_000)
	recs := []commitRec{
		rec("c1", "Ann", asOf-1*day, fileDelta{"a.go", 1, 0}, fileDelta{"b.go", 1, 0}),
		rec("c2", "Ann", asOf-2*day, fileDelta{"a.go", 1, 0}, fileDelta{"b.go", 1, 0}),
	}
	a := buildFileHistories(recs, asOf)["a.go"]
	if len(a.CoChange) != 1 || a.CoChange[0].Path != "b.go" || a.CoChange[0].Count != 2 {
		t.Errorf("a.go co-change = %+v, want [{b.go 2}]", a.CoChange)
	}
}
