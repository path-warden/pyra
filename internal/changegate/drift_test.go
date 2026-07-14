package changegate

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/codeintel"
)

func TestDrift_UnresolvedSymbolReported(t *testing.T) {
	root := t.TempDir()
	// Real source defines Foo, not Bar.
	writeFile(t, root, "pkg/x.go", "package pkg\n\nfunc Foo() {}\n")
	// Artifact cites Bar (which does not exist) in the changed file.
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"Governs `go:pkg/x.go#Bar@1`.")
	s := loadStore(t, root)
	ops := codeintel.NewOps(nil, root)

	got := Evaluate(s, ops, []string{"pkg/x.go"})
	var drift, governed int
	for _, f := range got {
		switch f.Code {
		case CodeSymbolUnresolved:
			drift++
			if !containsID(f.Message, "OKF-000000000AAA") || !containsID(f.Message, "go:pkg/x.go#Bar@1") {
				t.Errorf("drift message missing artifact/symbol: %q", f.Message)
			}
		case CodeGovernedChange:
			governed++
		}
	}
	if drift != 1 {
		t.Errorf("drift findings = %d, want 1: %+v", drift, got)
	}
	// The file is still governed (by path) in addition to the drift finding.
	if governed != 1 {
		t.Errorf("governed findings = %d, want 1", governed)
	}
}

func TestDrift_ResolvedSymbolNoDrift(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/x.go", "package pkg\n\nfunc Foo() {}\n")
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"Governs `go:pkg/x.go#Foo@3`.")
	s := loadStore(t, root)
	ops := codeintel.NewOps(nil, root)

	got := Evaluate(s, ops, []string{"pkg/x.go"})
	for _, f := range got {
		if f.Code == CodeSymbolUnresolved {
			t.Errorf("Foo resolves; no drift expected, got %q", f.Message)
		}
	}
}

func TestDrift_NilOpsSkips(t *testing.T) {
	root := t.TempDir()
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"Governs `go:pkg/x.go#Bar@1`.")
	s := loadStore(t, root)

	got := Evaluate(s, nil, []string{"pkg/x.go"})
	for _, f := range got {
		if f.Code == CodeSymbolUnresolved {
			t.Errorf("nil ops must skip drift, got %q", f.Message)
		}
	}
}

func TestDrift_DeletedFileDoesNotFailRun(t *testing.T) {
	root := t.TempDir()
	// The cited file does not exist on disk at all (deleted).
	canonDecision(t, root, "canon/d.md", "OKF-000000000AAA",
		"Governs `go:pkg/gone.go#Thing@1`.")
	s := loadStore(t, root)
	ops := codeintel.NewOps(nil, root)

	got := Evaluate(s, ops, []string{"pkg/gone.go"})
	// Must not panic/fail; the missing file surfaces as an unresolved drift finding.
	var drift int
	for _, f := range got {
		if f.Code == CodeSymbolUnresolved {
			drift++
		}
	}
	if drift != 1 {
		t.Errorf("deleted-file citation should report 1 unresolved, got %d: %+v", drift, got)
	}
}
