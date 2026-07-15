package cli

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/changegate"
	"github.com/chasedputnam/pyra/internal/changerisk"
	"github.com/chasedputnam/pyra/internal/config"
)

// stageRiskRepo builds a git repo with a baseline commit, a dependent file, a
// governing Canon artifact, and a staged, untested change to store.go.
func stageRiskRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init")
	writeGateFile(t, root, "cache/store.go", "package cache\nfunc Put(){}\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "seed")

	writeGateFile(t, root, "canon/d.md", `---
schema_version: 1
id: OKF-000000000AAA
type: decision
---

# Cache

## Status

Accepted

## Context

The file cache/store.go must cache in memory.

## Decision

We SHALL cache in memory.

## Consequences

Fast.
`)
	writeGateFile(t, root, "app/user.go", "package app\nfunc U(){ Put() }\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "deps+canon")

	// Stage an untested change to a governed, depended-on file.
	writeGateFile(t, root, "cache/store.go", "package cache\nfunc Put(){ _ = 1 }\n")
	gitCmd(t, root, "add", "cache/store.go")
	return root
}

func TestComputeGate_RiskMergesHeadlineAndDirectives(t *testing.T) {
	root := stageRiskRepo(t)

	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceStaged}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocking != 0 {
		t.Errorf("risk directives are advisory by default; Blocking = %d", res.Blocking)
	}
	// The headline and at least the governance + missing-tests directives merge in.
	want := []string{changerisk.CodeRisk, changerisk.CodeGovernanceRisk, changerisk.CodeMissingTests}
	for _, code := range want {
		if !hasCode(res.Issues, code) {
			t.Errorf("expected %s in merged result; got %+v", code, res.Issues)
		}
	}
}

func TestComputeGate_RiskPolicyEscalates(t *testing.T) {
	root := stageRiskRepo(t)
	cfg := config.Default()
	cfg.Enforcement.Blocking = []string{changerisk.CodeGovernanceRisk}

	res, _, err := computeGate(root, cfg, changegate.Source{Kind: changegate.SourceStaged}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed() {
		t.Error("escalating a risk directive to blocking should fail the gate")
	}
	if res.Blocking < 1 {
		t.Errorf("Blocking = %d, want >= 1", res.Blocking)
	}
}

func TestComputeGate_RiskOffNoRiskFindings(t *testing.T) {
	root := stageRiskRepo(t)
	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceStaged}, true, false) // risk=false
	if err != nil {
		t.Fatal(err)
	}
	if hasCode(res.Issues, changerisk.CodeRisk) {
		t.Error("risk off must not produce a change-risk headline")
	}
}

func TestComputeGate_RiskExplicitFilesDegrades(t *testing.T) {
	root := t.TempDir() // NOT a git repo
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))

	// Explicit files + risk: no git, ModeFiles → must not fail, still merges.
	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceExplicit, Files: []string{"internal/cache/store.go"}}, true, true)
	if err != nil {
		t.Fatalf("risk over explicit files in a non-git dir should not error: %v", err)
	}
	if !hasCode(res.Issues, changerisk.CodeRisk) {
		t.Error("risk headline should still be emitted for explicit files")
	}
}
