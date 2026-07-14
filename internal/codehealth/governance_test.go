package codehealth

import (
	"path/filepath"
	"testing"

	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/store"
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

func TestUngovernedHotspot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "canon/d.md", "---\nschema_version: 1\nid: OKF-000000000AAA\ntype: decision\n---\n\n# D\n\n## Status\n\nAccepted\n\n## Context\n\nGoverns internal/cache/store.go.\n\n## Decision\n\nWe SHALL cache.\n\n## Consequences\n\nFast.\n")
	st := loadStore(t, root)
	ops := codeintel.NewOps(nil, root)
	in := &Inputs{Store: st, Ops: ops}

	// A hotspot with no governing Canon → flagged.
	un := &FileContext{Path: "other/file.go", IsHotspot: true, Governed: false}
	if len(ungovernedHotspot(un, in)) == 0 {
		t.Error("ungoverned hotspot should be flagged")
	}
	// A governed hotspot → not flagged.
	gov := &FileContext{Path: "internal/cache/store.go", IsHotspot: true, Governed: true}
	if len(ungovernedHotspot(gov, in)) != 0 {
		t.Error("governed hotspot should not be flagged")
	}
}

func TestUngovernedHotspot_NoCanonOmitted(t *testing.T) {
	root := t.TempDir()
	st := loadStore(t, root) // no canon files
	in := &Inputs{Store: st, Ops: codeintel.NewOps(nil, root)}
	fc := &FileContext{Path: "x.go", IsHotspot: true, Governed: false}
	if len(ungovernedHotspot(fc, in)) != 0 {
		t.Error("no-Canon store must not flag ungoverned hotspots")
	}
}

func TestStaleGovernance(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "canon/d.md", "---\nschema_version: 1\nid: OKF-000000000AAA\ntype: decision\n---\n\n# D\n\n## Status\n\nAccepted\n\n## Context\n\nx\n\n## Decision\n\nWe SHALL x.\n\n## Consequences\n\ny\n")
	st := loadStore(t, root)
	in := &Inputs{Store: st, Ops: codeintel.NewOps(nil, root)}
	// A governed, heavily-churned file → stale governance.
	fc := &FileContext{Path: "x.go", Governed: true, Git: &gitint.FileHistory{Commits90d: 8}}
	if len(staleGovernance(fc, in)) == 0 {
		t.Error("stale_governance should fire on a churny governed file")
	}
	// Ungoverned → not flagged.
	fc2 := &FileContext{Path: "y.go", Governed: false, Git: &gitint.FileHistory{Commits90d: 8}}
	if len(staleGovernance(fc2, in)) != 0 {
		t.Error("stale_governance requires governance")
	}
}

func TestContradictoryDecision(t *testing.T) {
	root := t.TempDir()
	// new supersedes old, but old is still Accepted (not superseded) → contradiction.
	writeFile(t, root, "canon/old.md", "---\nschema_version: 1\nid: OKF-000000000OLD\ntype: decision\n---\n\n# Old\n\n## Status\n\nAccepted\n\n## Context\n\nx\n\n## Decision\n\nWe SHALL old.\n\n## Consequences\n\ny\n")
	writeFile(t, root, "canon/new.md", "---\nschema_version: 1\nid: OKF-000000000NEW\ntype: decision\n---\n\n# New\n\n## Status\n\nAccepted\n\n## Supersedes\n\nOKF-000000000OLD\n\n## Context\n\nx\n\n## Decision\n\nWe SHALL new.\n\n## Consequences\n\ny\n")
	st := loadStore(t, root)
	got := detectContradictions(&Inputs{Store: st})
	found := false
	for _, p := range got {
		if filepath.Base(p) == "new.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected new.md flagged as contradictory (supersedes a live target); got %v", got)
	}
}
