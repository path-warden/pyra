package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/store"
)

func exportStore(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("canon/adr-001.md", `---
schema_version: 1
id: OKF-0000000000AA
type: decision
---

# A Decision

## Status

Accepted

## Context

x

## Decision

We SHALL do it.

## Consequences

y

## Related Decisions

- adr-002
`)
	mk("canon/adr-002.md", "---\nschema_version: 1\nid: OKF-1111111111BB\ntype: decision\n---\n\n# Two\n\n## Status\n\nAccepted\n\n## Context\n\nx\n\n## Decision\n\nWe SHALL.\n\n## Consequences\n\ny\n")
	mk("guides/g.md", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n\nReference body.\n")

	s, err := store.Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestExportDocuments_ReferenceOnly(t *testing.T) {
	s := exportStore(t)
	out := exportDocuments(s)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 reference document line, got %d: %q", len(lines), out)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["tier"] != "reference" {
		t.Errorf("expected reference tier, got %v", rec["tier"])
	}
	// Canon must never appear in the documents export.
	if strings.Contains(out, "OKF-0000000000AA") {
		t.Error("canon artifact leaked into documents export")
	}
}

func TestExportGraph_HasCanonEdges(t *testing.T) {
	s := exportStore(t)
	data, err := exportGraph(s)
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes (2 canon + 1 reference), got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Edges[0]["to"] != "OKF-1111111111BB" {
		t.Errorf("edge should resolve alias adr-002 to canonical id, got %v", g.Edges[0]["to"])
	}
}
