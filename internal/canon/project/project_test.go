package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/identity"
	"github.com/chasedputnam/pyra/internal/config"
)

func writeSrc(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sliceHas(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func TestResolveType(t *testing.T) {
	reg := artifacts.Default()
	cases := []struct {
		name     string
		path     string
		override string
		want     string
		wantErr  bool
	}{
		{"requirements", "specs/feat/requirements.md", "", artifacts.TypeRequirement, false},
		{"design", "specs/feat/design.md", "", artifacts.TypeDesign, false},
		{"kiro requirements", ".kiro/specs/feat/requirements.md", "", artifacts.TypeRequirement, false},
		{"override decision", "specs/feat/notes.md", "decision", artifacts.TypeDecision, false},
		{"override case-insensitive", "specs/feat/requirements.md", "Design", artifacts.TypeDesign, false},
		{"unknown basename", "specs/feat/tasks.md", "", "", true},
		{"unknown override", "specs/feat/requirements.md", "bogus", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveType(tc.path, tc.override, reg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got type %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveType(%q, %q) = %q, want %q", tc.path, tc.override, got, tc.want)
			}
		})
	}
}

func TestResolveType_UnknownListsSupported(t *testing.T) {
	reg := artifacts.Default()
	_, err := resolveType("specs/feat/tasks.md", "", reg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requirements.md") || !strings.Contains(err.Error(), "design.md") {
		t.Errorf("error should mention supported docs: %v", err)
	}
}

func TestMapTargetPath(t *testing.T) {
	cfg := config.Default() // canon root "canon"
	cases := []struct {
		src  string
		want string
	}{
		{"specs/canon-automation/requirements.md", filepath.Join("/store", "canon", "canon-automation", "requirements.md")},
		{".kiro/specs/canon-automation/requirements.md", filepath.Join("/store", "canon", "canon-automation", "requirements.md")},
		{"specs/feat/design.md", filepath.Join("/store", "canon", "feat", "design.md")},
	}
	for _, tc := range cases {
		got := mapTargetPath(cfg, "/store", tc.src)
		if got != tc.want {
			t.Errorf("mapTargetPath(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}

func TestMapTargetPath_CustomCanonRoot(t *testing.T) {
	cfg := config.Default()
	cfg.CanonRoots = []string{"rac", "decisions"}
	got := mapTargetPath(cfg, "/store", "specs/feat/requirements.md")
	want := filepath.Join("/store", "rac", "feat", "requirements.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProject_MintsThenReusesID(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	cfg.RepositoryKey = "PROJ"
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feature\n\n## Problem\n\nUsers cannot do X.\n\n## Requirements\n\n[REQ-001] The system SHALL authenticate every user before access.\n")

	r1, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatalf("first Project: %v", err)
	}
	if !identity.ValidID(r1.ID) {
		t.Fatalf("minted id not well-formed: %q", r1.ID)
	}
	if !strings.HasPrefix(r1.ID, "PROJ-") {
		t.Errorf("minted id should use repository_key: %q", r1.ID)
	}
	if !r1.Created {
		t.Error("first projection should report Created")
	}
	if _, err := os.Stat(r1.ArtifactPath); err != nil {
		t.Fatalf("artifact not written: %v", err)
	}

	r2, err := Project(cfg, src, Options{Store: store, Write: true})
	if err != nil {
		t.Fatalf("second Project: %v", err)
	}
	if r2.ID != r1.ID {
		t.Errorf("id not reused on re-projection: %q != %q", r2.ID, r1.ID)
	}
	if r2.Created {
		t.Error("re-projection over an existing artifact should not report Created")
	}
}

func TestProject_RequirementLinesVerbatim(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	line := "[REQ-007] The system SHALL preserve this exact wording, verbatim and unaltered."
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nP.\n\n## Requirements\n\n"+line+"\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(r.ArtifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), line) {
		t.Errorf("requirement line not preserved verbatim.\nwant substring: %q\ngot:\n%s", line, got)
	}
}

func TestProject_MissingRequiredSectionIsIncomplete(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	// No "## Problem": the required problem section cannot be filled from source.
	writeSrc(t, src, "# Feat\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if !sliceHas(r.IncompleteSections, "problem") {
		t.Errorf("expected 'problem' in IncompleteSections, got %v", r.IncompleteSections)
	}
	got, _ := os.ReadFile(r.ArtifactPath)
	if !strings.Contains(string(got), "## Problem") {
		t.Error("artifact should still emit a Problem section")
	}
	if !strings.Contains(string(got), "TODO") {
		t.Error("unfilled required section should carry a TODO placeholder")
	}
}

func TestProject_DesignType(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "design.md")
	writeSrc(t, src, "# Feat Design\n\n## Context\n\nCtx.\n\n## User Need\n\nNeed.\n\n## Design\n\nThe design.\n\n## Constraints\n\nNone.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if r.Type != artifacts.TypeDesign {
		t.Errorf("Type=%q, want design", r.Type)
	}
	if len(r.IncompleteSections) != 0 {
		t.Errorf("all design required sections were present; IncompleteSections=%v", r.IncompleteSections)
	}
	got, _ := os.ReadFile(r.ArtifactPath)
	for _, want := range []string{"type: design", "## Context", "## User Need", "## Design", "## Constraints"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("design artifact missing %q\n%s", want, got)
		}
	}
}

func decisionDoc(id string) string {
	return "---\nschema_version: 1\nid: " + id + "\ntype: decision\n---\n\n# Decision\n\n## Context\n\nctx\n\n## Decision\n\nWe SHALL do it.\n\n## Consequences\n\nok\n"
}

func designDoc(id string) string {
	return "---\nschema_version: 1\nid: " + id + "\ntype: design\n---\n\n# Design\n\n## Context\n\nctx\n\n## User Need\n\nneed\n\n## Design\n\nd\n\n## Constraints\n\nc\n"
}

func TestProject_InfersRelationshipFromLiteralID(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	writeSrc(t, filepath.Join(store, "canon", "dec.md"), decisionDoc("OKF-0000000000AA"))
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nRelates to OKF-0000000000AA which we build on.\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range r.InferredEdges {
		if e.Target == "OKF-0000000000AA" && e.Section == "related decisions" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected inferred related-decisions edge, got %v", r.InferredEdges)
	}
	got, _ := os.ReadFile(r.ArtifactPath)
	if !strings.Contains(string(got), "## Related Decisions") || !strings.Contains(string(got), "OKF-0000000000AA") {
		t.Errorf("artifact missing inferred relationship section:\n%s", got)
	}
}

func TestProject_IDShapedButAbsentIsUnresolved(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nMentions OKF-ZZZZZZZZZZZZ which does not exist.\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if !sliceHas(r.UnresolvedRefs, "OKF-ZZZZZZZZZZZZ") {
		t.Errorf("expected unresolved ref OKF-ZZZZZZZZZZZZ, got %v", r.UnresolvedRefs)
	}
	if len(r.InferredEdges) != 0 {
		t.Errorf("unresolved ref must not become an edge, got %v", r.InferredEdges)
	}
	// The id may appear in copied source prose, but never under a relationship
	// section: an unresolved ref is not written as an edge.
	got, _ := os.ReadFile(r.ArtifactPath)
	if strings.Contains(string(got), "## Related") {
		t.Errorf("unresolved ref must not produce a relationship section:\n%s", got)
	}
}

func TestProject_DropsRefNotPermittedForType(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	// design→design via "related designs" is not in the design type's optional set.
	writeSrc(t, filepath.Join(store, "canon", "d1.md"), designDoc("OKF-1111111111BB"))
	src := filepath.Join(store, "specs", "feat", "design.md")
	writeSrc(t, src, "# Feat\n\n## Context\n\nBuilds on OKF-1111111111BB.\n\n## User Need\n\nN.\n\n## Design\n\nD.\n\n## Constraints\n\nC.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.InferredEdges) != 0 {
		t.Errorf("design→design edge is not permitted and must be dropped, got %v", r.InferredEdges)
	}
	got, _ := os.ReadFile(r.ArtifactPath)
	if strings.Contains(string(got), "Related Designs") {
		t.Errorf("must not emit a related-designs section for a design artifact:\n%s", got)
	}
}

func TestProject_LiteralOnlyNoFuzzyMatch(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	writeSrc(t, filepath.Join(store, "canon", "dec.md"), decisionDoc("OKF-0000000000AA"))
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	// 11-char suffix: a near-miss that is not a valid ID and must not match.
	writeSrc(t, src, "# Feat\n\n## Problem\n\nMentions OKF-0000000000A loosely.\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.InferredEdges) != 0 || len(r.UnresolvedRefs) != 0 {
		t.Errorf("near-miss must not match: edges=%v unresolved=%v", r.InferredEdges, r.UnresolvedRefs)
	}
}

func TestProject_NonEARSRequirementBlocks(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	// lowercase "shall" violates BCP-14 → an error-severity (blocking) issue.
	writeSrc(t, src, "# Feat\n\n## Problem\n\nP.\n\n## Requirements\n\n[REQ-001] the system shall use a lowercase keyword.\n")

	r, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.BlockingIssues) == 0 {
		t.Error("expected blocking issues for a lowercase-normative requirement")
	}
}

func TestProject_DryRunWritesNothing(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nP.\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	r, err := Project(cfg, src, Options{Store: store, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(r.ArtifactPath); !os.IsNotExist(statErr) {
		t.Errorf("dry-run must not create the artifact (stat err=%v)", statErr)
	}
	if !r.Changed {
		t.Error("a would-be-new artifact should report Changed even under dry-run")
	}
}

func TestProject_ChangedWithoutWriteLeavesFileIntact(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nfirst.\n\n## Requirements\n\n[REQ-001] The system SHALL do the first thing.\n")
	r1, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(r1.ArtifactPath)

	writeSrc(t, src, "# Feat\n\n## Problem\n\nSECOND changed.\n\n## Requirements\n\n[REQ-001] The system SHALL do the second thing.\n")
	r2, err := Project(cfg, src, Options{Store: store}) // no Write
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Changed {
		t.Error("expected Changed=true for a differing re-projection")
	}
	if r2.Diff == "" {
		t.Error("expected a non-empty diff for a changed artifact")
	}
	after, _ := os.ReadFile(r1.ArtifactPath)
	if string(before) != string(after) {
		t.Error("artifact must be byte-identical when re-projected without --write")
	}
}

func TestProject_WriteAppliesChange(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nfirst.\n\n## Requirements\n\n[REQ-001] The system SHALL do the first thing.\n")
	r1, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	writeSrc(t, src, "# Feat\n\n## Problem\n\nSECOND body text.\n\n## Requirements\n\n[REQ-001] The system SHALL do the second thing.\n")
	_, err = Project(cfg, src, Options{Store: store, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(r1.ArtifactPath)
	if !strings.Contains(string(after), "SECOND body text") {
		t.Errorf("--write must apply the change:\n%s", after)
	}
}

func TestProjectDir_SkipsTasks(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default()
	dir := filepath.Join(store, "specs", "feat")
	writeSrc(t, filepath.Join(dir, "requirements.md"), "# F\n\n## Problem\n\nP.\n\n## Requirements\n\n[REQ-001] The system SHALL x.\n")
	writeSrc(t, filepath.Join(dir, "design.md"), "# F\n\n## Context\n\nc.\n\n## User Need\n\nn.\n\n## Design\n\nd.\n\n## Constraints\n\nc.\n")
	writeSrc(t, filepath.Join(dir, "tasks.md"), "# Tasks\n\n- [ ] 1. do\n")

	results, err := ProjectDir(cfg, dir, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 projected docs (tasks.md skipped), got %d", len(results))
	}
	types := map[string]bool{}
	for _, r := range results {
		types[r.Type] = true
	}
	if !types["requirement"] || !types["design"] {
		t.Errorf("expected requirement+design, got %v", types)
	}
	if _, statErr := os.Stat(filepath.Join(store, "canon", "feat", "tasks.md")); !os.IsNotExist(statErr) {
		t.Error("tasks.md must not be projected to Canon")
	}
}

func TestProject_DetectsCrossRootCollision(t *testing.T) {
	store := t.TempDir()
	cfg := config.Default() // spec roots: specs, .kiro/specs
	body := "# F\n\n## Problem\n\nP.\n\n## Requirements\n\n[REQ-001] The system SHALL x.\n"
	writeSrc(t, filepath.Join(store, "specs", "dup", "requirements.md"), body)
	writeSrc(t, filepath.Join(store, ".kiro", "specs", "dup", "requirements.md"), body)

	_, err := Project(cfg, filepath.Join(store, "specs", "dup", "requirements.md"), Options{Store: store})
	if err == nil {
		t.Fatal("expected a collision error")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("expected collision error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(store, "canon", "dup", "requirements.md")); !os.IsNotExist(statErr) {
		t.Error("a collision must not write an artifact")
	}
}

func TestProject_NoSpuriousStemSelfEdge(t *testing.T) {
	// Re-projecting must not infer a self-edge just because the artifact's
	// filename stem ("requirements") appears as a common word in the prose.
	store := t.TempDir()
	cfg := config.Default()
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nThe requirements here are clear.\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	if _, err := Project(cfg, src, Options{Store: store}); err != nil {
		t.Fatal(err)
	}
	r2, err := Project(cfg, src, Options{Store: store, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.InferredEdges) != 0 {
		t.Errorf("filename stem must not produce a spurious edge, got %v", r2.InferredEdges)
	}
}

func TestProject_IdlessExistingTargetConverges(t *testing.T) {
	// An existing target with no usable id is treated as a create: the minted id
	// is written once and then reused, so re-projection converges.
	store := t.TempDir()
	cfg := config.Default()
	target := filepath.Join(store, "canon", "feat", "requirements.md")
	writeSrc(t, target, "# Stale\n\nno frontmatter id here\n")
	src := filepath.Join(store, "specs", "feat", "requirements.md")
	writeSrc(t, src, "# Feat\n\n## Problem\n\nP.\n\n## Requirements\n\n[REQ-001] The system SHALL do a thing.\n")

	r1, err := Project(cfg, src, Options{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if !r1.Created {
		t.Error("id-less existing target should be treated as a create")
	}
	if !identity.ValidID(r1.ID) {
		t.Fatalf("expected a minted id, got %q", r1.ID)
	}
	r2, err := Project(cfg, src, Options{Store: store, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if r2.ID != r1.ID {
		t.Errorf("id should converge after first write: %q != %q", r2.ID, r1.ID)
	}
}

func TestProject_CollisionOnlyForSpecRootSources(t *testing.T) {
	// A source outside any configured spec root is never a spec-root collision,
	// even if the same feature exists under the spec roots.
	store := t.TempDir()
	cfg := config.Default()
	body := "# F\n\n## Problem\n\nP.\n\n## Requirements\n\n[REQ-001] The system SHALL x.\n"
	writeSrc(t, filepath.Join(store, "specs", "dup", "requirements.md"), body)
	writeSrc(t, filepath.Join(store, ".kiro", "specs", "dup", "requirements.md"), body)
	docSrc := filepath.Join(store, "docs", "dup", "requirements.md")
	writeSrc(t, docSrc, body)

	if _, err := Project(cfg, docSrc, Options{Store: store}); err != nil {
		t.Errorf("source outside a spec root must not trigger a collision: %v", err)
	}
}
