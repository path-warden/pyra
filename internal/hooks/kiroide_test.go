package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func kiroStore(t *testing.T, withDir bool) Context {
	t.Helper()
	store := t.TempDir()
	if withDir {
		if err := os.MkdirAll(filepath.Join(store, ".kiro"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return Context{StoreRoot: store, Config: config.Default()}
}

func TestKiroIDE_Detect(t *testing.T) {
	if !(kiroIDEInstaller{}).Detect(kiroStore(t, true)) {
		t.Error("expected Detect true when .kiro exists")
	}
	if (kiroIDEInstaller{}).Detect(kiroStore(t, false)) {
		t.Error("expected Detect false without .kiro")
	}
}

func TestKiroIDE_CreatesValidHook(t *testing.T) {
	ctx := kiroStore(t, true)
	if _, err := (kiroIDEInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ctx.StoreRoot, ".kiro", "hooks", "pyra-gate.json")
	obj, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("hook file not valid JSON: %v", err)
	}
	if obj["name"] != "pyra-gate" {
		t.Errorf("name=%v", obj["name"])
	}
	if obj["trigger"] != "PostFileSave" {
		t.Errorf("trigger=%v", obj["trigger"])
	}
	if obj["enabled"] != true {
		t.Errorf("enabled=%v", obj["enabled"])
	}
	action, _ := obj["action"].(map[string]any)
	cmd, _ := action["command"].(string)
	if cmd == "" || action["type"] != "command" {
		t.Errorf("action=%v", obj["action"])
	}
}

func TestKiroIDE_SiblingUntouched(t *testing.T) {
	ctx := kiroStore(t, true)
	other := filepath.Join(ctx.StoreRoot, ".kiro", "hooks", "other.json")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := `{"name":"other"}`
	if err := os.WriteFile(other, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (kiroIDEInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(other)
	if string(got) != orig {
		t.Errorf("sibling hook modified: %s", got)
	}
}

func TestKiroIDE_Idempotent(t *testing.T) {
	ctx := kiroStore(t, true)
	g := kiroIDEInstaller{}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ctx.StoreRoot, ".kiro", "hooks", "pyra-gate.json")
	first, _ := os.ReadFile(path)
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Error("re-install changed the hook file")
	}
}

func TestKiroIDE_Uninstall(t *testing.T) {
	ctx := kiroStore(t, true)
	other := filepath.Join(ctx.StoreRoot, ".kiro", "hooks", "other.json")
	_ = os.MkdirAll(filepath.Dir(other), 0o755)
	_ = os.WriteFile(other, []byte(`{"name":"other"}`), 0o644)

	g := kiroIDEInstaller{}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ctx.StoreRoot, ".kiro", "hooks", "pyra-gate.json")); !os.IsNotExist(err) {
		t.Error("pyra hook file should be removed")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("sibling hook file must remain")
	}
}
