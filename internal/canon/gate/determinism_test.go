package gate

import (
	"reflect"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func TestRun_PureReferenceBundlePasses(t *testing.T) {
	// A bundle with only Reference concepts (no canon root) imposes no gate.
	root := t.TempDir()
	writeCanon(t, root, "guides/a.md", "---\ntype: Guide\n---\n\n# A\n\nReference content.\n")
	writeCanon(t, root, "guides/b.md", "---\ntype: Guide\n---\n\n# B\n\nMore reference content.\n")

	res, err := Run(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed() {
		t.Errorf("pure-reference bundle must pass the gate, got blocking=%d", res.Blocking)
	}
	if res.ArtifactCount != 0 {
		t.Errorf("expected 0 canon artifacts, got %d", res.ArtifactCount)
	}
}

func TestRun_DeterministicOutput(t *testing.T) {
	root := t.TempDir()
	writeCanon(t, root, "canon/adr-001.md", `---
schema_version: 1
id: OKF-0000000000AA
type: decision
---

# First

## Status

Accepted

## Context

Context one.

## Decision

We SHALL do one. We SHALL also do two and SHALL do three.

## Consequences

ok

## Related Decisions

- adr-002
`)
	writeCanon(t, root, "canon/adr-002.md", `---
schema_version: 1
id: OKF-1111111111BB
type: decision
---

# Second

## Status

Accepted

## Context

Context two.

## Decision

We shall do something lowercase.

## Consequences

ok
`)

	first, err := Run(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if first.Blocking != second.Blocking || first.Advisory != second.Advisory {
		t.Fatalf("counts differ across runs: %+v vs %+v", first, second)
	}
	if !reflect.DeepEqual(first.Issues, second.Issues) {
		t.Errorf("issue output is non-deterministic across identical runs")
	}
}
