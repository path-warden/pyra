package store

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

// supersededDecision renders a superseded decision whose ## Supersedes section
// points at the given target id (empty target = no supersedes section).
func supersededDecision(id, supersedes string) string {
	s := `---
schema_version: 1
id: ` + id + `
type: decision
---

# Decision ` + id + `

## Status

Superseded
`
	if supersedes != "" {
		s += "\n## Supersedes\n\n" + supersedes + "\n"
	}
	s += `
## Context

Context.

## Decision

We SHALL do the thing.

## Consequences

Consequences.
`
	return s
}

func acceptedDecision(id, supersedes string) string {
	s := `---
schema_version: 1
id: ` + id + `
type: decision
---

# Decision ` + id + `

## Status

Accepted
`
	if supersedes != "" {
		s += "\n## Supersedes\n\n" + supersedes + "\n"
	}
	s += `
## Context

Context.

## Decision

We SHALL do the thing.

## Consequences

Consequences.
`
	return s
}

func loadStore(t *testing.T, root string) *Store {
	t.Helper()
	s, err := Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSuccessor_NoSupersession(t *testing.T) {
	root := t.TempDir()
	write(t, root, "canon/a.md", acceptedDecision("OKF-0000000000AA", ""))
	s := loadStore(t, root)

	if got := s.Successor("OKF-0000000000AA"); got != nil {
		t.Errorf("accepted artifact should have no successor, got %s", got.ID)
	}
	if got := s.Successor("OKF-UNKNOWNXXXXX"); got != nil {
		t.Errorf("unknown id should return nil, got %s", got.ID)
	}
}

func TestSuccessor_SingleHop(t *testing.T) {
	root := t.TempDir()
	write(t, root, "canon/old.md", supersededDecision("OKF-0000000000AA", ""))
	write(t, root, "canon/new.md", acceptedDecision("OKF-1111111111BB", "OKF-0000000000AA"))
	s := loadStore(t, root)

	got := s.Successor("OKF-0000000000AA")
	if got == nil {
		t.Fatal("superseded artifact should resolve to its successor")
	}
	if got.ID != "OKF-1111111111BB" {
		t.Errorf("successor = %s, want OKF-1111111111BB", got.ID)
	}
}

func TestSuccessor_MultiHopChain(t *testing.T) {
	root := t.TempDir()
	// v1 <- v2 <- v3 (v3 accepted). v1 and v2 superseded.
	write(t, root, "canon/v1.md", supersededDecision("OKF-000000000001", ""))
	write(t, root, "canon/v2.md", supersededDecision("OKF-000000000002", "OKF-000000000001"))
	write(t, root, "canon/v3.md", acceptedDecision("OKF-000000000003", "OKF-000000000002"))
	s := loadStore(t, root)

	got := s.Successor("OKF-000000000001")
	if got == nil {
		t.Fatal("chain head should resolve to the terminal live artifact")
	}
	if got.ID != "OKF-000000000003" {
		t.Errorf("terminal successor = %s, want OKF-000000000003", got.ID)
	}
}

func TestSuccessor_CycleGuardTerminates(t *testing.T) {
	root := t.TempDir()
	// Pathological cycle: each supersedes the other, both superseded.
	write(t, root, "canon/a.md", supersededDecision("OKF-0000000000AA", "OKF-0000000000BB"))
	write(t, root, "canon/b.md", supersededDecision("OKF-0000000000BB", "OKF-0000000000AA"))
	s := loadStore(t, root)

	// The visited-set guard must break the cycle: the call returns rather than
	// looping forever. The exact terminal artifact is unimportant here.
	_ = s.Successor("OKF-0000000000AA")
}
