package changegate

import (
	"reflect"
	"testing"

	"github.com/chasedputnam/memphis/internal/config"
	"github.com/chasedputnam/memphis/internal/store"
)

func loadStore(t *testing.T, root string) *store.Store {
	t.Helper()
	s, err := store.Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// canonDecision renders an accepted decision whose body embeds the given prose
// (used to inject a symbol-id or file-path citation).
func canonDecision(t *testing.T, root, rel, id, body string) {
	t.Helper()
	writeFile(t, root, rel, `---
schema_version: 1
id: `+id+`
type: decision
---

# Decision `+id+`

## Status

Accepted

## Context

`+body+`

## Decision

We SHALL keep it that way.

## Consequences

Fine.
`)
}

func TestEvaluate_CiteBySymbolID(t *testing.T) {
	root := t.TempDir()
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"This governs `go:internal/cache/store.go#Put@42`.")
	s := loadStore(t, root)

	got := Evaluate(s, nil, []string{"internal/cache/store.go"})
	if len(got) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(got), got)
	}
	if got[0].Code != CodeGovernedChange || got[0].Path != "internal/cache/store.go" {
		t.Errorf("unexpected finding: %+v", got[0])
	}
}

func TestEvaluate_CiteByPath(t *testing.T) {
	root := t.TempDir()
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"The file internal/cache/store.go must cache in memory.")
	s := loadStore(t, root)

	got := Evaluate(s, nil, []string{"internal/cache/store.go"})
	if len(got) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(got), got)
	}
}

func TestEvaluate_NoGovernance(t *testing.T) {
	root := t.TempDir()
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"This decision is about networking, not caching.")
	s := loadStore(t, root)

	got := Evaluate(s, nil, []string{"internal/cache/store.go"})
	if len(got) != 0 {
		t.Errorf("expected no findings, got %+v", got)
	}
}

func TestEvaluate_PathBoundary_NoFalsePositive(t *testing.T) {
	root := t.TempDir()
	// Body mentions "aaa.go" and "x/store.go"; neither should govern "a.go" or
	// "store.go" respectively via a substring match.
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"See aaa.go and pkg/x/store.go for details.")
	s := loadStore(t, root)

	if got := Evaluate(s, nil, []string{"a.go"}); len(got) != 0 {
		t.Errorf("a.go should not match inside aaa.go: %+v", got)
	}
	if got := Evaluate(s, nil, []string{"store.go"}); len(got) != 0 {
		t.Errorf("store.go should not match inside pkg/x/store.go: %+v", got)
	}
}

func TestEvaluate_SupersededResolvesToSuccessor(t *testing.T) {
	root := t.TempDir()
	// Old (superseded) governs the file; new supersedes old and is accepted.
	writeFile(t, root, "canon/old.md", `---
schema_version: 1
id: OKF-000000000OLD
type: decision
---

# Old

## Status

Superseded

## Context

Governs internal/cache/store.go the old way.

## Decision

We used the old way.

## Consequences

Replaced.
`)
	writeFile(t, root, "canon/new.md", `---
schema_version: 1
id: OKF-000000000NEW
type: decision
---

# New

## Status

Accepted

## Supersedes

OKF-000000000OLD

## Context

Also governs internal/cache/store.go the new way.

## Decision

We SHALL use the new way.

## Consequences

Better.
`)
	s := loadStore(t, root)

	got := Evaluate(s, nil, []string{"internal/cache/store.go"})
	ids := map[string]bool{}
	for _, f := range got {
		// Extract governing artifact from the message via the finding's fields is
		// not exposed; assert the old ID never appears and the new one does.
		if containsID(f.Message, "OKF-000000000OLD") {
			t.Errorf("superseded artifact OKF-000000000OLD should not be cited: %q", f.Message)
		}
		if containsID(f.Message, "OKF-000000000NEW") {
			ids["new"] = true
		}
	}
	if !ids["new"] {
		t.Errorf("successor OKF-000000000NEW should govern the file: %+v", got)
	}
}

func TestEvaluate_Deterministic(t *testing.T) {
	root := t.TempDir()
	canonDecision(t, root, "canon/a.md", "OKF-0000000000A1",
		"Governs internal/cache/store.go and internal/api/handler.go.")
	canonDecision(t, root, "canon/b.md", "OKF-0000000000B2",
		"Also governs internal/cache/store.go.")
	s := loadStore(t, root)

	files := []string{"internal/api/handler.go", "internal/cache/store.go"}
	first := Evaluate(s, nil, files)
	for i := 0; i < 5; i++ {
		again := Evaluate(s, nil, files)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("nondeterministic output:\n%+v\nvs\n%+v", first, again)
		}
	}
}

func containsID(s, id string) bool {
	return len(s) >= len(id) && (indexOf(s, id) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
