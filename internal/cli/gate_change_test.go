package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/changegate"
	"github.com/chasedputnam/memphis/internal/config"
)

func writeGateFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// governedDecision is an accepted decision that cites internal/cache/store.go.
func governedDecision(id string) string {
	return `---
schema_version: 1
id: ` + id + `
type: decision
---

# Cache Decision

## Status

Accepted

## Context

The file internal/cache/store.go must cache documents in memory.

## Decision

We SHALL cache documents in memory.

## Consequences

Faster reads.
`
}

func TestComputeGate_StagedGovernedChange_Advisory(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))
	writeGateFile(t, root, "internal/cache/store.go", "package cache\n\nfunc Put() {}\n")
	gitCmd(t, root, "init")
	gitCmd(t, root, "add", "internal/cache/store.go")

	res, count, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceStaged}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("changed files = %d, want 1", count)
	}
	if res.Blocking != 0 {
		t.Errorf("advisory-by-default should not block; Blocking = %d", res.Blocking)
	}
	if !res.Passed() {
		t.Error("gate should pass with only advisory governance findings")
	}
	if !hasCode(res.Issues, changegate.CodeGovernedChange) {
		t.Errorf("expected a %s finding, got %+v", changegate.CodeGovernedChange, res.Issues)
	}
}

func TestComputeGate_PolicyEscalatesToBlocking(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))
	writeGateFile(t, root, "internal/cache/store.go", "package cache\n\nfunc Put() {}\n")
	gitCmd(t, root, "init")
	gitCmd(t, root, "add", "internal/cache/store.go")

	cfg := config.Default()
	cfg.Enforcement.Blocking = []string{changegate.CodeGovernedChange}

	res, _, err := computeGate(root, cfg, changegate.Source{Kind: changegate.SourceStaged}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed() {
		t.Error("policy escalation should make the gate fail")
	}
	if res.Blocking < 1 {
		t.Errorf("Blocking = %d, want >= 1", res.Blocking)
	}
}

func TestComputeGate_ExplicitBypassesGit(t *testing.T) {
	root := t.TempDir() // NOT a git repo
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))

	res, count, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceExplicit, Files: []string{"internal/cache/store.go"}}, true, false)
	if err != nil {
		t.Fatalf("explicit source must not need git: %v", err)
	}
	if count != 1 {
		t.Errorf("changed files = %d, want 1", count)
	}
	if !hasCode(res.Issues, changegate.CodeGovernedChange) {
		t.Errorf("expected governance finding via explicit list, got %+v", res.Issues)
	}
}

func TestComputeGate_NoGitNoExplicit_Errors(t *testing.T) {
	root := t.TempDir() // NOT a git repo
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))

	_, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceStaged}, true, false)
	if err != changegate.ErrNoChangeSource {
		t.Errorf("err = %v, want ErrNoChangeSource", err)
	}
}

func TestComputeGate_CorpusBlockingPlusChangeAdvisory(t *testing.T) {
	root := t.TempDir()
	// A structurally broken decision (missing required sections) → corpus blocking.
	writeGateFile(t, root, "canon/broken.md", `---
schema_version: 1
id: OKF-000000000BAD
type: decision
---

# Broken

## Status

Accepted
`)
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))
	writeGateFile(t, root, "internal/cache/store.go", "package cache\n\nfunc Put() {}\n")
	gitCmd(t, root, "init")
	gitCmd(t, root, "add", "internal/cache/store.go")

	res, _, err := computeGate(root, config.Default(),
		changegate.Source{Kind: changegate.SourceStaged}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed() {
		t.Error("a corpus blocking finding must fail the gate even with only advisory change findings")
	}
	if !hasCode(res.Issues, changegate.CodeGovernedChange) {
		t.Error("change advisory finding should still be present alongside corpus findings")
	}
	// Corpus finding present too (some non-changegate code).
	corpus := false
	for _, iss := range res.Issues {
		if iss.Code != changegate.CodeGovernedChange && iss.Code != changegate.CodeSymbolUnresolved {
			corpus = true
		}
	}
	if !corpus {
		t.Error("expected at least one corpus finding aggregated into the result")
	}
}

func TestComputeGate_ModeOffUnchanged(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "canon/d.md", governedDecision("OKF-000000000AAA"))

	res, count, err := computeGate(root, config.Default(), changegate.Source{}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != -1 {
		t.Errorf("mode off should report changedCount -1, got %d", count)
	}
	if hasCode(res.Issues, changegate.CodeGovernedChange) {
		t.Error("mode off must not produce governance findings")
	}
}

func hasCode(iss []model.Issue, code string) bool {
	for _, i := range iss {
		if i.Code == code {
			return true
		}
	}
	return false
}
