package relate

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/parse"
)

func entryAlias(id, typ, status, body string, aliases ...string) Entry {
	return Entry{
		ID: id, Type: typ, Status: status,
		Retired: artifacts.Default().IsRetired(typ, status),
		Path:    id + ".md", Aliases: aliases, Product: parse.Parse([]byte(body)),
	}
}

func entry(id, typ, status, body string) Entry {
	// Default alias: the id itself (filename stem == id here).
	return entryAlias(id, typ, status, body, id)
}

func hasCode(issues []model.Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func severityOf(issues []model.Issue, code string) string {
	for _, i := range issues {
		if i.Code == code {
			return i.Severity
		}
	}
	return ""
}

func TestBuild_ValidEdges(t *testing.T) {
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- DEC-2\n"),
		entry("DEC-2", "decision", "accepted", "# B\n"),
	}
	g, issues := Build(entries, DefaultSpecs())
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if len(g.Edges["DEC-1"]) != 1 || g.Edges["DEC-1"][0].To != "DEC-2" {
		t.Errorf("edge not built: %+v", g.Edges)
	}
	if g.InboundCounts()["DEC-2"] != 1 {
		t.Errorf("inbound count wrong: %v", g.InboundCounts())
	}
}

func TestBuild_TargetNotFound(t *testing.T) {
	entries := []Entry{entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- GHOST\n")}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeTargetNotFound) != model.SeverityError {
		t.Errorf("expected relationship-target-not-found error, got %+v", issues)
	}
}

func TestBuild_TargetAmbiguous(t *testing.T) {
	// Two artifacts both answer to the alias "shared".
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- shared\n"),
		entryAlias("DEC-2", "decision", "accepted", "# B\n", "DEC-2", "shared"),
		entryAlias("DEC-3", "decision", "accepted", "# C\n", "DEC-3", "shared"),
	}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeTargetAmbiguous) != model.SeverityError {
		t.Errorf("expected relationship-target-ambiguous error, got %+v", issues)
	}
}

func TestBuild_SelfReference(t *testing.T) {
	entries := []Entry{entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- DEC-1\n")}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeSelfReference) != model.SeverityWarning {
		t.Errorf("expected relationship-self-reference warning, got %+v", issues)
	}
}

func TestBuild_DuplicateIdentifier(t *testing.T) {
	entries := []Entry{
		{ID: "DEC-1", Type: "decision", Status: "accepted", Path: "a.md", Aliases: []string{"DEC-1"}, Product: parse.Parse([]byte("# A\n"))},
		{ID: "DEC-1", Type: "decision", Status: "accepted", Path: "b.md", Aliases: []string{"DEC-1"}, Product: parse.Parse([]byte("# B\n"))},
	}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeDuplicateIdentifier) != model.SeverityError {
		t.Errorf("expected duplicate-artifact-identifier error, got %+v", issues)
	}
}

func TestBuild_RangeMismatch(t *testing.T) {
	// related decisions (range decision) targeting a requirement.
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- REQ-2\n"),
		entry("REQ-2", "requirement", "accepted", "# B\n"),
	}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeTargetTypeMismatch) != model.SeverityError {
		t.Errorf("expected relationship-target-type-mismatch error, got %+v", issues)
	}
}

func TestBuild_EdgeUnsupportedIsWarning(t *testing.T) {
	// supersedes is only declared in decision.optional; a requirement using it is
	// an unsupported edge (warning), not an error.
	entries := []Entry{
		entry("REQ-1", "requirement", "accepted", "# A\n\n## Supersedes\n\n- DEC-2\n"),
		entry("DEC-2", "decision", "accepted", "# B\n"),
	}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeEdgeUnsupported) != model.SeverityWarning {
		t.Errorf("expected relationship-edge-unsupported warning, got %+v", issues)
	}
}

func TestBuild_StatusConsistency(t *testing.T) {
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- DEC-2\n"),
		entry("DEC-2", "decision", "superseded", "# B\n"),
	}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeTargetSuperseded) != model.SeverityWarning {
		t.Errorf("expected relationship-target-superseded warning, got %+v", issues)
	}
}

func TestBuild_SupersedesToRetiredAllowed(t *testing.T) {
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Supersedes\n\n- DEC-2\n"),
		entry("DEC-2", "decision", "superseded", "# B\n"),
	}
	_, issues := Build(entries, DefaultSpecs())
	if hasCode(issues, CodeTargetSuperseded) {
		t.Errorf("supersedes to retired target must be allowed, got %+v", issues)
	}
}

func TestBuild_Cycle(t *testing.T) {
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Supersedes\n\n- DEC-2\n"),
		entry("DEC-2", "decision", "accepted", "# B\n\n## Supersedes\n\n- DEC-1\n"),
	}
	_, issues := Build(entries, DefaultSpecs())
	if severityOf(issues, CodeRelationshipCycle) != model.SeverityError {
		t.Errorf("expected relationship-cycle error, got %+v", issues)
	}
}

func TestBuild_RelatedTicketsAreNotGraphEdges(t *testing.T) {
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Related Tickets\n\n- owner/repo#1\n"),
	}
	g, issues := Build(entries, DefaultSpecs())
	if len(issues) != 0 {
		t.Errorf("related tickets should not produce relationship issues, got %+v", issues)
	}
	if len(g.Edges["DEC-1"]) != 0 {
		t.Errorf("related tickets should not be graph edges, got %+v", g.Edges)
	}
}

func TestBuild_AliasResolution(t *testing.T) {
	// A reference by filename-prefix alias resolves to the canonical id.
	entries := []Entry{
		entry("RAC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- adr-002\n"),
		entryAlias("RAC-2", "decision", "accepted", "# B\n", "RAC-2", "adr-002"),
	}
	g, issues := Build(entries, DefaultSpecs())
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if len(g.Edges["RAC-1"]) != 1 || g.Edges["RAC-1"][0].To != "RAC-2" {
		t.Errorf("alias did not resolve to canonical id: %+v", g.Edges)
	}
}

func TestNeighborhood(t *testing.T) {
	entries := []Entry{
		entry("DEC-1", "decision", "accepted", "# A\n\n## Related Decisions\n\n- DEC-2\n"),
		entry("DEC-2", "decision", "accepted", "# B\n\n## Related Decisions\n\n- DEC-3\n"),
		entry("DEC-3", "decision", "accepted", "# C\n"),
	}
	g, _ := Build(entries, DefaultSpecs())
	if n := g.Neighborhood("DEC-1", 1); len(n) != 1 {
		t.Errorf("depth 1: got %d edges", len(n))
	}
	if n := g.Neighborhood("DEC-1", 2); len(n) != 2 {
		t.Errorf("depth 2: got %d edges (%+v)", len(n), n)
	}
}
