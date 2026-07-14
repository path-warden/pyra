package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertBlock_InsertAndPreserve(t *testing.T) {
	existing := "#!/bin/sh\necho hi\n"
	out := upsertBlock(existing, "pyra gate")
	if !strings.Contains(out, "echo hi") {
		t.Error("surrounding content must be preserved")
	}
	for _, want := range []string{BlockBegin, "pyra gate", BlockEnd} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestUpsertBlock_Idempotent(t *testing.T) {
	a := upsertBlock("#!/bin/sh\n", "pyra gate")
	b := upsertBlock(a, "pyra gate")
	if a != b {
		t.Errorf("upsert not idempotent:\n--a--\n%q\n--b--\n%q", a, b)
	}
}

func TestUpsertBlock_ReplaceUpdatesBody(t *testing.T) {
	a := upsertBlock("x\n", "OLD")
	b := upsertBlock(a, "NEW")
	if strings.Contains(b, "OLD") {
		t.Error("old block body should have been replaced")
	}
	if !strings.Contains(b, "NEW") {
		t.Error("new block body missing")
	}
	if n := strings.Count(b, BlockBegin); n != 1 {
		t.Errorf("expected exactly one managed block, got %d", n)
	}
}

func TestRemoveBlock(t *testing.T) {
	a := upsertBlock("#!/bin/sh\necho hi\n", "pyra gate")
	b := removeBlock(a)
	if strings.Contains(b, BlockBegin) || strings.Contains(b, "pyra gate") {
		t.Errorf("block not removed:\n%s", b)
	}
	if !strings.Contains(b, "echo hi") {
		t.Error("surrounding content must be preserved after removal")
	}
}

func TestHasBlock(t *testing.T) {
	if hasBlock("nothing here") {
		t.Error("hasBlock false positive")
	}
	if !hasBlock(upsertBlock("", "x")) {
		t.Error("hasBlock should detect an inserted block")
	}
}

func TestReadWriteJSONObject(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "x.json")

	m, err := readJSONObject(p)
	if err != nil {
		t.Fatalf("readJSONObject(missing): %v", err)
	}
	if len(m) != 0 {
		t.Errorf("missing file should yield empty map, got %v", m)
	}

	m["k"] = "v"
	if err := writeJSONObject(p, m); err != nil {
		t.Fatalf("writeJSONObject: %v", err)
	}
	m2, err := readJSONObject(p)
	if err != nil {
		t.Fatal(err)
	}
	if m2["k"] != "v" {
		t.Errorf("round-trip lost data: %v", m2)
	}
}
