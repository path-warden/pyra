package gate

import (
	"testing"

	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/config"
)

func TestApplyPolicy_IntrinsicSeverity(t *testing.T) {
	raw := []model.Issue{
		{Severity: model.SeverityError, Code: "some-error", Message: "e"},
		{Severity: model.SeverityWarning, Code: "some-warn", Message: "w"},
	}
	res := ApplyPolicy(config.Default(), raw)
	if res.Blocking != 1 {
		t.Errorf("Blocking = %d, want 1", res.Blocking)
	}
	if res.Advisory != 1 {
		t.Errorf("Advisory = %d, want 1", res.Advisory)
	}
	if len(res.Issues) != 2 {
		t.Fatalf("Issues = %d, want 2", len(res.Issues))
	}
}

func TestApplyPolicy_DisabledDropped(t *testing.T) {
	raw := []model.Issue{{Severity: model.SeverityError, Code: "noisy", Message: "x"}}
	cfg := config.Default()
	cfg.Enforcement.Disabled = []string{"noisy"}
	res := ApplyPolicy(cfg, raw)
	if res.Blocking != 0 || res.Advisory != 0 || len(res.Issues) != 0 {
		t.Errorf("disabled code should be dropped, got %+v", res)
	}
}

func TestApplyPolicy_ForceBlocking(t *testing.T) {
	// A warning-severity code forced to blocking by policy becomes blocking.
	raw := []model.Issue{{Severity: model.SeverityWarning, Code: "governed", Message: "x"}}
	cfg := config.Default()
	cfg.Enforcement.Blocking = []string{"governed"}
	res := ApplyPolicy(cfg, raw)
	if res.Blocking != 1 {
		t.Errorf("Blocking = %d, want 1", res.Blocking)
	}
	if len(res.Issues) != 1 || res.Issues[0].Severity != model.SeverityError {
		t.Errorf("forced-blocking issue should carry error severity, got %+v", res.Issues)
	}
}

func TestApplyPolicy_ForceAdvisory(t *testing.T) {
	// An error-severity code forced to advisory becomes advisory.
	raw := []model.Issue{{Severity: model.SeverityError, Code: "demote", Message: "x"}}
	cfg := config.Default()
	cfg.Enforcement.Advisory = []string{"demote"}
	res := ApplyPolicy(cfg, raw)
	if res.Advisory != 1 || res.Blocking != 0 {
		t.Errorf("forced-advisory should demote, got %+v", res)
	}
	if res.Issues[0].Severity != model.SeverityWarning {
		t.Errorf("demoted issue should carry warning severity, got %s", res.Issues[0].Severity)
	}
}

func TestResultMerge_SumsAndConcats(t *testing.T) {
	a := Result{
		Issues:        []model.Issue{{Code: "a"}},
		Blocking:      1,
		Advisory:      2,
		ArtifactCount: 5,
	}
	b := Result{
		Issues:   []model.Issue{{Code: "b"}, {Code: "c"}},
		Blocking: 3,
		Advisory: 0,
	}
	got := a.Merge(b)
	if got.Blocking != 4 {
		t.Errorf("Blocking = %d, want 4", got.Blocking)
	}
	if got.Advisory != 2 {
		t.Errorf("Advisory = %d, want 2", got.Advisory)
	}
	if got.ArtifactCount != 5 {
		t.Errorf("ArtifactCount = %d, want 5", got.ArtifactCount)
	}
	if len(got.Issues) != 3 {
		t.Errorf("Issues = %d, want 3", len(got.Issues))
	}
	if got.Passed() {
		t.Error("merged result with blocking findings should not pass")
	}
}
