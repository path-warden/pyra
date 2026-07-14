package retrieval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, root string) *store.Store {
	t.Helper()
	s, err := store.Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAssemble_AuthorityFirstRanking(t *testing.T) {
	root := t.TempDir()
	// Both mention "indexing"; canon should rank first at equal relevance.
	write(t, root, "canon/spec.md", `---
schema_version: 1
id: OKF-0123456789AB
type: requirement
---

# Indexing Spec

## Problem

Lookups are slow.

## Requirements

- [REQ-001] The system SHALL perform indexing of documents.
`)
	write(t, root, "guides/indexing.md", "---\ntype: Guide\ntitle: Indexing Guide\n---\n\n# Indexing Guide\n\nA guide about indexing of documents.\n")

	s := load(t, root)
	res := Assemble(s, "indexing documents", Options{})
	if len(res.Items) == 0 {
		t.Fatal("no items assembled")
	}
	// Find positions of canon vs reference.
	canonPos, refPos := -1, -1
	for i, it := range res.Items {
		if it.Tier == "canon" && canonPos == -1 {
			canonPos = i
		}
		if it.Tier == "reference" && refPos == -1 {
			refPos = i
		}
	}
	if canonPos == -1 {
		t.Fatal("canon item not in results")
	}
	if refPos != -1 && canonPos > refPos {
		t.Errorf("canon (%d) should rank before reference (%d)", canonPos, refPos)
	}
}

func TestAssemble_CanonHasCitation(t *testing.T) {
	root := t.TempDir()
	write(t, root, "canon/spec.md", `---
schema_version: 1
id: OKF-0123456789AB
type: requirement
---

# Auth Spec

## Problem

Users are unauthenticated.

## Requirements

- [REQ-001] The system SHALL authenticate users.
`)
	s := load(t, root)
	res := Assemble(s, "authenticate", Options{})
	if len(res.Items) == 0 {
		t.Fatal("no items")
	}
	c := res.Items[0].Citation
	if c == nil {
		t.Fatal("canon item missing citation")
	} else if c.ID != "OKF-0123456789AB" || c.Type != "requirement" || c.Status != "" {
		t.Errorf("bad citation: %+v", c)
	}
}

func TestAssemble_RequirementTextVerbatim(t *testing.T) {
	root := t.TempDir()
	reqText := "[REQ-001] The system SHALL authenticate every user before granting access to any protected resource."
	write(t, root, "canon/spec.md", `---
schema_version: 1
id: OKF-0123456789AB
type: requirement
---

# Auth

## Problem

Some lengthy prose that exists only to pad the body well beyond the tiny token budget so that compression would otherwise be required to fit this artifact into context.

## Requirements

`+reqText+`
`)
	s := load(t, root)
	// Tiny budget forces compression; req text must remain verbatim or be deferred.
	res := Assemble(s, "authenticate user", Options{TokenBudget: 40})
	for _, it := range res.Items {
		if strings.Contains(it.Body, "[REQ-001]") && !strings.Contains(it.Body, reqText) {
			t.Errorf("requirement statement was altered under compression:\n%q", it.Body)
		}
	}
}

func TestAssemble_OverflowToFollowup(t *testing.T) {
	root := t.TempDir()
	for i, name := range []string{"a", "b", "c", "d"} {
		_ = i
		write(t, root, "guides/"+name+".md",
			"---\ntype: Guide\ntitle: "+name+"\n---\n\n# "+name+"\n\n"+strings.Repeat("indexing documents and search content here. ", 50)+"\n")
	}
	s := load(t, root)
	res := Assemble(s, "indexing documents", Options{TokenBudget: 60, Compression: "none"})
	if len(res.SuggestedFollowup) == 0 {
		t.Errorf("expected overflow into suggested_followup, got items=%d followup=%d",
			len(res.Items), len(res.SuggestedFollowup))
	}
	if res.TotalTokens > 60 {
		t.Errorf("assembled tokens %d exceed budget 60", res.TotalTokens)
	}
}

func TestAssemble_SupersededResolvesToSuccessor(t *testing.T) {
	root := t.TempDir()
	write(t, root, "canon/old.md", `---
schema_version: 1
id: OKF-0000000000AA
type: decision
---

# Old Decision

## Status

Superseded

## Context

Old approach to caching.

## Decision

We used the old caching approach.

## Consequences

Replaced later.
`)
	write(t, root, "canon/new.md", `---
schema_version: 1
id: OKF-1111111111BB
type: decision
---

# New Decision

## Status

Accepted

## Supersedes

OKF-0000000000AA

## Context

New approach to caching.

## Decision

We SHALL use the new caching approach.

## Consequences

Better.
`)
	s := load(t, root)
	res := Assemble(s, "caching approach", Options{})
	for _, it := range res.Items {
		if it.ID == "OKF-0000000000AA" {
			t.Errorf("superseded artifact returned bare instead of its successor")
		}
	}
	// The successor should be present.
	found := false
	for _, it := range res.Items {
		if it.ID == "OKF-1111111111BB" {
			found = true
		}
	}
	if !found {
		t.Errorf("successor not present in results: %+v", res.Items)
	}
}
