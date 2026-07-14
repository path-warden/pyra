package frontmatter

import (
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

func TestParse_Valid(t *testing.T) {
	fm, issues := Parse([]byte("schema_version: 1\nid: OKF-0123456789AB\ntype: decision\ntags: [a, b]\n"))
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if !fm.Present {
		t.Error("expected Present=true")
	}
	if fm.ID != "OKF-0123456789AB" || fm.Type != "decision" {
		t.Errorf("decoded wrong: %+v", fm)
	}
	if len(fm.Tags) != 2 {
		t.Errorf("tags: %+v", fm.Tags)
	}
}

func TestParse_UnknownField(t *testing.T) {
	_, issues := Parse([]byte("id: OKF-0123456789AB\nbogus_field: nope\n"))
	if !hasCode(issues, CodeUnknownField) {
		t.Errorf("expected %s, got %+v", CodeUnknownField, issues)
	}
}

func TestParse_DuplicateKey(t *testing.T) {
	_, issues := Parse([]byte("id: OKF-0123456789AB\nid: OKF-OTHER123456\n"))
	if !hasCode(issues, CodeDuplicateKey) {
		t.Errorf("expected %s, got %+v", CodeDuplicateKey, issues)
	}
}

func TestParse_AliasBombRejected(t *testing.T) {
	bomb := "a: &anchor [x]\nb: *anchor\ntags: [z]\n"
	_, issues := Parse([]byte(bomb))
	if !hasCode(issues, CodeBomb) {
		t.Errorf("expected %s, got %+v", CodeBomb, issues)
	}
}

func TestParse_DepthBombRejected(t *testing.T) {
	var sb strings.Builder
	// Build deeply nested flow mappings exceeding maxDepth.
	for i := 0; i < maxDepth+5; i++ {
		sb.WriteString("{a: ")
	}
	sb.WriteString("1")
	for i := 0; i < maxDepth+5; i++ {
		sb.WriteString("}")
	}
	_, issues := Parse([]byte("tags: " + sb.String() + "\n"))
	if !hasCode(issues, CodeBomb) && !hasCode(issues, CodeUnknownField) {
		// Depth guard or unknown-field guard must catch it; it must not pass clean.
		t.Errorf("expected bomb/unknown rejection, got %+v", issues)
	}
}

func TestParse_TooLarge(t *testing.T) {
	big := "tags:\n" + strings.Repeat("  - item\n", maxBytes/4)
	_, issues := Parse([]byte(big))
	if !hasCode(issues, CodeTooLarge) {
		t.Errorf("expected %s, got %d issues", CodeTooLarge, len(issues))
	}
}

func TestParse_UnsupportedSchemaVersion(t *testing.T) {
	_, issues := Parse([]byte("schema_version: 99\nid: OKF-0123456789AB\n"))
	if !hasCode(issues, CodeUnsupportedSV) {
		t.Errorf("expected %s, got %+v", CodeUnsupportedSV, issues)
	}
}

func TestParse_EmptyIsClean(t *testing.T) {
	fm, issues := Parse([]byte("   \n"))
	if len(issues) != 0 {
		t.Errorf("expected no issues for empty, got %+v", issues)
	}
	if fm.Present {
		t.Error("expected Present=false for empty frontmatter")
	}
}

func TestMigrate(t *testing.T) {
	got, changed := Migrate(model.Frontmatter{SchemaVersion: 0})
	if !changed || got.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected migration to v%d, got v%d changed=%v", CurrentSchemaVersion, got.SchemaVersion, changed)
	}
	got2, changed2 := Migrate(model.Frontmatter{SchemaVersion: CurrentSchemaVersion})
	if changed2 || got2.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected no-op for current version, changed=%v", changed2)
	}
}

func hasCode(issues []model.Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
