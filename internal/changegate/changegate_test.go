package changegate

import (
	"strings"
	"testing"

	"github.com/chasedputnam/memphis/internal/canon/model"
)

func TestCodesAreStable(t *testing.T) {
	if CodeGovernedChange != "canon-governed-change" {
		t.Errorf("CodeGovernedChange = %q", CodeGovernedChange)
	}
	if CodeSymbolUnresolved != "governed-symbol-unresolved" {
		t.Errorf("CodeSymbolUnresolved = %q", CodeSymbolUnresolved)
	}
}

func TestToIssue_GovernedChange(t *testing.T) {
	f := Finding{
		Code:     CodeGovernedChange,
		File:     "internal/cache/store.go",
		Artifact: "OKF-ABC123",
		Type:     "decision",
		Status:   "accepted",
		Title:    "Cache in memory",
	}
	iss := f.toIssue()
	if iss.Severity != model.SeverityWarning {
		t.Errorf("severity = %q, want warning", iss.Severity)
	}
	if iss.Code != CodeGovernedChange {
		t.Errorf("code = %q", iss.Code)
	}
	if iss.Path != "internal/cache/store.go" {
		t.Errorf("Path = %q, want the changed file", iss.Path)
	}
	for _, want := range []string{"OKF-ABC123", "internal/cache/store.go", "Cache in memory"} {
		if !strings.Contains(iss.Message, want) {
			t.Errorf("message %q missing %q", iss.Message, want)
		}
	}
}

func TestToIssue_SymbolUnresolved(t *testing.T) {
	f := Finding{
		Code:     CodeSymbolUnresolved,
		File:     "internal/cache/store.go",
		Artifact: "OKF-ABC123",
		Symbol:   "go:internal/cache/store.go#Put@42",
	}
	iss := f.toIssue()
	if iss.Path != "internal/cache/store.go" {
		t.Errorf("Path = %q, want the changed file", iss.Path)
	}
	for _, want := range []string{"OKF-ABC123", "go:internal/cache/store.go#Put@42", "no longer resolves"} {
		if !strings.Contains(iss.Message, want) {
			t.Errorf("message %q missing %q", iss.Message, want)
		}
	}
}
