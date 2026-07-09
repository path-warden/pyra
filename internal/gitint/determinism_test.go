package gitint

import (
	"encoding/json"
	"testing"
)

// TestNew_DeterministicAcrossRuns builds the index twice over the same repo and
// asserts byte-identical output — the core determinism guarantee (REQ-602),
// which rests on anchoring windows to HEAD's commit time rather than wall-clock.
func TestNew_DeterministicAcrossRuns(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	// Fixed commit dates so windows are computed against HEAD, reproducibly.
	writeF(t, root, "a.go", "package a\n")
	writeF(t, root, "b.go", "package b\n")
	gitT(t, root, "add", ".")
	commitAt(t, root, "seed", 1_700_000_000)
	for i := 0; i < 5; i++ {
		writeF(t, root, "a.go", "package a\n// v"+itoa(i)+"\n")
		gitT(t, root, "add", ".")
		commitAt(t, root, "edit", int64(1_700_000_100+i))
	}

	snapshot := func() string {
		h, ok := New(root, 100)
		if !ok {
			t.Fatal("index should build")
		}
		payload := map[string]any{
			"asOf":     h.AsOf(),
			"files":    h.Files(),
			"hotspots": h.Hotspots(),
			"modules":  h.Modules(),
		}
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	first := snapshot()
	for i := 0; i < 3; i++ {
		if again := snapshot(); again != first {
			t.Fatalf("index not deterministic across runs:\n%s\nvs\n%s", first, again)
		}
	}
}

// TestNew_WindowsAnchoredToHead proves recency windows use HEAD's commit time,
// not wall-clock: a repo whose newest commit is far in the past still classifies
// its commits as within the 90-day window relative to that HEAD.
func TestNew_WindowsAnchoredToHead(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	writeF(t, root, "a.go", "package a\n")
	gitT(t, root, "add", ".")
	// A HEAD committed years ago; asOf must anchor here, not on time.Now().
	oldTS := int64(1_500_000_000) // 2017
	commitAt(t, root, "old-head", oldTS)

	h, ok := New(root, 100)
	if !ok {
		t.Fatal("index should build")
	}
	if h.AsOf() != oldTS {
		t.Errorf("asOf = %d, want the old HEAD ts %d", h.AsOf(), oldTS)
	}
	// The single commit is at asOf, so it is inside the 90-day window measured
	// from asOf — a wall-clock anchor would place it ~years outside and report 0.
	if f := h.File("a.go"); f == nil || f.Commits90d != 1 {
		t.Errorf("a.go Commits90d = %v, want 1 (window anchored to HEAD)", f)
	}
}
