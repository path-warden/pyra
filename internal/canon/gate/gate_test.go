package gate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func writeCanon(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_CleanCorpusPasses(t *testing.T) {
	root := t.TempDir()
	writeCanon(t, root, "canon/adr-001.md", `---
schema_version: 1
id: OKF-0123456789AB
type: decision
---

# Use Markdown

## Status

Accepted

## Context

We need portability.

## Decision

We SHALL use Markdown.

## Consequences

Portable.
`)
	res, err := Run(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed() {
		t.Fatalf("expected pass, got blocking=%d issues=%+v", res.Blocking, res.Issues)
	}
	if res.ArtifactCount != 1 {
		t.Errorf("expected 1 artifact, got %d", res.ArtifactCount)
	}
}

func TestRun_BlockingOnMissingSection(t *testing.T) {
	root := t.TempDir()
	// Decision missing ## Decision -> missing_required_section (error -> blocking).
	writeCanon(t, root, "canon/adr-002.md", `---
schema_version: 1
id: OKF-0123456789AB
type: decision
---

# Bad

## Status

Accepted

## Context

x

## Consequences

y
`)
	res, err := Run(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed() {
		t.Fatal("expected blocking failure")
	}
	if res.Blocking < 1 {
		t.Errorf("expected >=1 blocking, got %d", res.Blocking)
	}
}

const lowercaseReqSpec = `---
schema_version: 1
id: OKF-0123456789AB
type: requirement
---

# Spec

## Problem

Something is slow.

## Requirements

- [REQ-001] The system shall do a thing.
`

func TestRun_DisabledRuleDropsFinding(t *testing.T) {
	root := t.TempDir()
	writeCanon(t, root, "canon/spec.md", lowercaseReqSpec)

	// lowercase "shall" -> requirement-normative-keyword error (blocking) by default.
	def, err := Run(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if def.Passed() {
		t.Fatal("expected normative-keyword blocking by default")
	}

	cfg := config.Default()
	cfg.Enforcement.Disabled = []string{"requirement-normative-keyword"}
	dis, err := Run(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range dis.Issues {
		if iss.Code == "requirement-normative-keyword" {
			t.Error("requirement-normative-keyword should be dropped when disabled")
		}
	}
}

func TestRun_AdvisoryDemotesError(t *testing.T) {
	root := t.TempDir()
	writeCanon(t, root, "canon/spec.md", lowercaseReqSpec)

	cfg := config.Default()
	cfg.Enforcement.Advisory = []string{"requirement-normative-keyword"}
	res, err := Run(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range res.Issues {
		if iss.Code == "requirement-normative-keyword" && iss.Severity == "error" {
			t.Errorf("normative-keyword should be demoted to advisory, got error")
		}
	}
	if res.Advisory < 1 {
		t.Errorf("expected advisory count >=1, got %d", res.Advisory)
	}
}

func TestRun_EmptyCorpusPasses(t *testing.T) {
	res, err := Run(t.TempDir(), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed() || res.ArtifactCount != 0 {
		t.Errorf("expected clean empty corpus, got %+v", res)
	}
}
