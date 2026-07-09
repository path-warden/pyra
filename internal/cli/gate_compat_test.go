package cli

import (
	"encoding/json"
	"testing"

	"github.com/chasedputnam/memphis/internal/canon/gate"
	"github.com/chasedputnam/memphis/internal/changegate"
	"github.com/chasedputnam/memphis/internal/config"
)

// TestComputeGate_ModeOffEqualsCorpusGate proves the change-aware feature is
// additive: with no change flags, computeGate returns byte-identical output to
// the corpus gate (gate.Run) on the same repository state (REQ-105).
func TestComputeGate_ModeOffEqualsCorpusGate(t *testing.T) {
	root := t.TempDir()
	// A small fixed corpus: one clean decision and one with an advisory warning.
	writeGateFile(t, root, "canon/a.md", governedDecision("OKF-000000000AAA"))
	writeGateFile(t, root, "canon/b.md", `---
schema_version: 1
id: OKF-000000000BBB
type: requirement
---

# Req

## Problem

Something is slow.

## Requirements

- [REQ-001] The system SHALL be fast and SHALL be small.
`)

	cfg := config.Default()

	want, err := gate.Run(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, count, err := computeGate(root, cfg, changegate.Source{}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != -1 {
		t.Errorf("mode-off changedCount = %d, want -1", count)
	}

	wj, _ := json.Marshal(want)
	gj, _ := json.Marshal(got)
	if string(wj) != string(gj) {
		t.Errorf("mode-off gate output differs from corpus gate:\ncorpus: %s\nmode-off: %s", wj, gj)
	}
}
