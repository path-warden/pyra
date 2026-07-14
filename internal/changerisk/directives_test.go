package changerisk

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/store"
)

func set(items ...string) map[string]bool {
	m := map[string]bool{}
	for _, i := range items {
		m[i] = true
	}
	return m
}

// commit writes files and commits them (test helper).
func commit(t *testing.T, root, msg string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		writeF(t, root, rel, content)
	}
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-m", msg)
}

func TestMissingTests_ConventionMap(t *testing.T) {
	cases := []struct {
		file   string
		inDiff []string
		wantMT bool
	}{
		{"internal/cache/store.go", nil, true},                                       // no test in diff
		{"internal/cache/store.go", []string{"internal/cache/store_test.go"}, false}, // test present
		{"pkg/mod.py", []string{"pkg/test_mod.py"}, false},
		{"pkg/mod.py", nil, true},
		{"src/a.ts", []string{"src/a.test.ts"}, false},
		{"src/a.ts", []string{"src/a.spec.ts"}, false},
		{"lib/Foo.java", []string{"lib/FooTest.java"}, false},
		{"lib/thing.rs", nil, false},                 // Rust: in-file tests, no directive
		{"internal/cache/store_test.go", nil, false}, // a test file is not a source
		{"README.md", nil, false},                    // unknown language
	}
	for _, c := range cases {
		changeSet := set(append([]string{c.file}, c.inDiff...)...)
		got := missingTestsDirectives([]string{c.file}, changeSet)
		has := len(got) == 1 && got[0].Code == CodeMissingTests
		if has != c.wantMT {
			t.Errorf("missingTests(%s, diff=%v) = %v, want %v", c.file, c.inDiff, has, c.wantMT)
		}
	}
}

func TestWillBreak_And_ImportLinkedCoChange(t *testing.T) {
	root := t.TempDir()
	// store.go defines Put; user.go references Put (dependent + import-linked).
	writeF(t, root, "cache/store.go", "package cache\n\nfunc Put() {}\n")
	writeF(t, root, "app/user.go", "package app\n\nimport \"x/cache\"\n\nfunc Use() { cache.Put() }\n")
	ops := codeintel.NewOps(nil, root)
	g := buildCodeGraph(ops, root)
	if g == nil {
		t.Fatal("codeGraph should build")
	}

	// will_break: changing store.go → user.go references Put, is a dependent.
	wb := willBreakDirectives(g, []string{"cache/store.go"}, set("cache/store.go"))
	if len(wb) != 1 || wb[0].Code != CodeWillBreak {
		t.Fatalf("will_break = %+v, want one finding", wb)
	}
	if !contains(wb[0].Message, "app/user.go") {
		t.Errorf("will_break should name app/user.go: %q", wb[0].Message)
	}

	// When the dependent is already in the change set, no will_break.
	wb2 := willBreakDirectives(g, []string{"cache/store.go"}, set("cache/store.go", "app/user.go"))
	if len(wb2) != 0 {
		t.Errorf("dependent in change set should suppress will_break: %+v", wb2)
	}

	// import-linked: store.go and user.go are structurally linked.
	if !g.importLinked("cache/store.go", "app/user.go") {
		t.Error("store.go and user.go should be import-linked")
	}
	if g.importLinked("cache/store.go", "cache/store.go") {
		// a file trivially "links" to itself via defs∩refs? defs=Put, refs of store.go empty → false.
	}
}

func TestGovernanceRisk_ReusesChangegate(t *testing.T) {
	root := t.TempDir()
	writeF(t, root, "canon/d.md", `---
schema_version: 1
id: OKF-000000000AAA
type: decision
---

# Cache Decision

## Status

Accepted

## Context

The file internal/cache/store.go must cache in memory.

## Decision

We SHALL cache in memory.

## Consequences

Fast.
`)
	st, err := store.Load(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	got := governanceDirectives(st, nil, []string{"internal/cache/store.go"})
	if len(got) != 1 || got[0].Code != CodeGovernanceRisk {
		t.Fatalf("governance directive = %+v, want one risk-governance", got)
	}
	if !contains(got[0].Message, "OKF-000000000AAA") {
		t.Errorf("governance message should cite the artifact: %q", got[0].Message)
	}
	// Ungoverned file → no directive.
	if d := governanceDirectives(st, nil, []string{"unrelated/file.go"}); len(d) != 0 {
		t.Errorf("ungoverned file should yield no governance directive: %+v", d)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestMissingCoChanges_MinusImportEdges(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	// hidden.go co-changes with store.go but has NO structural link → surfaced.
	// user.go co-changes with store.go AND references it → excluded (import-linked).
	commit(t, root, "c1", map[string]string{
		"store.go":  "package p\nfunc Put(){}\n",
		"hidden.go": "package p\nvar A = 1\n",
		"user.go":   "package p\nfunc U(){ Put() }\n",
	})
	commit(t, root, "c2", map[string]string{
		"store.go":  "package p\nfunc Put(){ _ = 1 }\n",
		"hidden.go": "package p\nvar A = 2\n",
		"user.go":   "package p\nfunc U(){ Put(); _ = 1 }\n",
	})
	h, ok := gitint.New(root, 100)
	if !ok {
		t.Skip("git unavailable")
	}
	ops := codeintel.NewOps(nil, root)
	g := buildCodeGraph(ops, root)

	// Change only store.go; hidden.go (coupled, no import) should surface,
	// user.go (import-linked) should not.
	got := missingCoChangeDirectives(h, g, []string{"store.go"}, set("store.go"))
	if len(got) != 1 {
		t.Fatalf("want one co-change directive, got %+v", got)
	}
	msg := got[0].Message
	if !contains(msg, "hidden.go") {
		t.Errorf("should surface hidden.go (hidden coupling): %q", msg)
	}
	if contains(msg, "user.go") {
		t.Errorf("should exclude import-linked user.go: %q", msg)
	}
}
