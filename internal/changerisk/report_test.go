package changerisk

import (
	"testing"

	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/codeintel"
	"github.com/chasedputnam/memphis/internal/config"
	"github.com/chasedputnam/memphis/internal/store"
)

func TestAssess_StagedWithDirectives(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	// A committed baseline so ranking is possible.
	commit(t, root, "seed", map[string]string{"cache/store.go": "package cache\nfunc Put(){}\n"})
	// Governing Canon + a dependent, then stage an untested change to store.go.
	writeF(t, root, "canon/d.md", `---
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
	writeF(t, root, "app/user.go", "package app\nfunc U(){ Put() }\n")
	commit(t, root, "add deps", map[string]string{"app/user.go": "package app\nfunc U(){ Put() }\n"})
	writeF(t, root, "cache/store.go", "package cache\nfunc Put(){ _ = 1 }\n")
	gitT(t, root, "add", "cache/store.go")

	st, err := store.Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ops := codeintel.NewOps(nil, root)

	rep, err := Assess(root, root, Change{Mode: ModeStaged}, st, ops, Options{Baseline: 50})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Score <= 0 {
		t.Errorf("expected a positive raw score, got %v", rep.Score)
	}
	// Directives present: missing_tests (store.go untested) + will_break (user.go) +
	// governance (Canon governs store.go).
	codes := map[string]bool{}
	for _, d := range rep.Directives {
		codes[d.Code] = true
		if d.File == "" {
			t.Errorf("directive %s has empty File", d.Code)
		}
	}
	for _, want := range []string{CodeMissingTests, CodeWillBreak, CodeGovernanceRisk} {
		if !codes[want] {
			t.Errorf("missing directive %s; got %+v", want, rep.Directives)
		}
	}
}

func TestReport_Issues_HeadlineAndPaths(t *testing.T) {
	rep := Report{
		Score: 7.4, Priority: PriorityElevated, Percentile: 89, RankKnown: true,
		Directives: []Finding{{Code: CodeMissingTests, File: "a.go", Message: "a.go changed without a test"}},
	}
	iss := rep.Issues()
	if len(iss) != 2 {
		t.Fatalf("want 2 issues (headline + 1 directive), got %d", len(iss))
	}
	if iss[0].Code != CodeRisk {
		t.Errorf("first issue should be the %s headline, got %s", CodeRisk, iss[0].Code)
	}
	if !contains(iss[0].Message, "Elevated") || !contains(iss[0].Message, "89") || !contains(iss[0].Message, "7.4") {
		t.Errorf("headline missing priority/percentile/raw: %q", iss[0].Message)
	}
	if iss[1].Path != "a.go" || iss[1].Severity != model.SeverityWarning {
		t.Errorf("directive issue wrong Path/severity: %+v", iss[1])
	}
}

func TestReport_Issues_UnavailableRank(t *testing.T) {
	rep := Report{Score: 3.0, RankKnown: false}
	iss := rep.Issues()
	if !contains(iss[0].Message, "unavailable") {
		t.Errorf("headline should say ranking unavailable: %q", iss[0].Message)
	}
}

func TestAssess_NonGitDegrades(t *testing.T) {
	root := t.TempDir() // not a git repo
	rep, err := Assess(root, root, Change{Mode: ModeStaged}, nil, nil, Options{})
	if err != nil {
		t.Fatalf("non-git Assess should not error: %v", err)
	}
	if rep.RankKnown {
		t.Error("no git history → ranking must be unavailable")
	}
}
