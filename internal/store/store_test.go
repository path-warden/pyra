package store

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/search"
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

func canonDecision(id string) string {
	return `---
schema_version: 1
id: ` + id + `
type: decision
---

# A Decision

## Status

Accepted

## Context

We index things.

## Decision

We SHALL index.

## Consequences

Searchable.
`
}

func referenceConcept(title string) string {
	return "---\ntype: Guide\ntitle: " + title + "\ntags: [search]\n---\n\n# " + title + "\n\n> [!summary]\n> A guide about searching documents.\n\nThis guide explains document search and indexing.\n"
}

func TestLoad_SeparatesTiers(t *testing.T) {
	root := t.TempDir()
	write(t, root, "canon/adr-001.md", canonDecision("OKF-0123456789AB"))
	write(t, root, "guides/search.md", referenceConcept("Searching"))

	s, err := Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if len(s.Canon) != 1 {
		t.Errorf("expected 1 canon item, got %d", len(s.Canon))
	}
	if s.Canon[0].Tier != TierCanon {
		t.Error("canon item has wrong tier")
	}
	if len(s.Reference) != 1 {
		t.Errorf("expected 1 reference item, got %d (%+v)", len(s.Reference), s.Reference)
	}
	if s.Reference[0].Tier != TierReference {
		t.Error("reference item has wrong tier")
	}
	if !s.HasCanon() {
		t.Error("HasCanon should be true")
	}
}

func TestLoad_CanonFilesNotInReference(t *testing.T) {
	root := t.TempDir()
	write(t, root, "canon/adr-001.md", canonDecision("OKF-0123456789AB"))

	s, err := Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	for _, ref := range s.Reference {
		if filepath.Dir(ref.Path) == "canon" {
			t.Errorf("canon file leaked into reference tier: %s", ref.Path)
		}
	}
}

func TestLoad_ZeroCanonBehavesLegacy(t *testing.T) {
	root := t.TempDir()
	write(t, root, "guides/a.md", referenceConcept("A"))
	write(t, root, "guides/b.md", referenceConcept("B"))

	s, err := Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if s.HasCanon() {
		t.Error("expected no canon")
	}
	if len(s.Reference) != 2 {
		t.Errorf("expected 2 reference concepts, got %d", len(s.Reference))
	}
}

func TestRebuild_Idempotent(t *testing.T) {
	root := t.TempDir()
	write(t, root, "guides/search.md", referenceConcept("Searching"))
	write(t, root, "guides/index2.md", referenceConcept("Indexing"))

	s, err := Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	before := searchIDs(s, "document")
	if len(before) == 0 {
		t.Fatal("expected search hits before rebuild")
	}
	if err := s.Rebuild(); err != nil {
		t.Fatal(err)
	}
	after := searchIDs(s, "document")
	if len(before) != len(after) {
		t.Fatalf("result count changed after rebuild: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("result %d changed: %q -> %q", i, before[i], after[i])
		}
	}
}

func searchIDs(s *Store, q string) []string {
	res := s.Search(q, search.SearchOptions{Limit: 20})
	ids := make([]string, 0, len(res))
	for _, r := range res {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	return ids
}
