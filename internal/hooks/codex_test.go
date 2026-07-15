package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func codexStore(t *testing.T) Context {
	t.Helper()
	return Context{StoreRoot: t.TempDir(), Config: config.Default()}
}

func codexHooksPath(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".codex", "hooks.json")
}

func TestCodexInstallPreservesExistingAndIsIdempotent(t *testing.T) {
	ctx := codexStore(t)
	path := codexHooksPath(ctx)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo other"}]}]},"other":true}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	installer := codexInstaller{}
	if _, err := installer.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(ctx); err != nil {
		t.Fatal(err)
	}
	obj, err := readJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	if obj["other"] != true {
		t.Fatal("unrelated setting lost")
	}
	entries := asArray(asObject(obj["hooks"])["PostToolUse"])
	if len(entries) != 2 {
		t.Fatalf("expected existing plus one Pyra hook, got %#v", entries)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "Edit|Write") || strings.Count(string(body), ManagedMarker) != 1 {
		t.Fatalf("bad managed hook:\n%s", body)
	}
	status, err := installer.Status(ctx)
	if err != nil || status.Action != ActionPresent {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestCodexUninstallRemovesOnlyManagedEntry(t *testing.T) {
	ctx := codexStore(t)
	installer := codexInstaller{}
	if _, err := installer.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	obj, err := readJSONObject(codexHooksPath(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if len(asObject(obj["hooks"])) != 0 {
		t.Fatalf("managed hook remains: %#v", obj)
	}
}

func TestCodexStatusRejectsIncompatibleHookShape(t *testing.T) {
	ctx := codexStore(t)
	path := codexHooksPath(ctx)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"hooks":{"PostToolUse":"keep"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (codexInstaller{}).Status(ctx); err == nil || !strings.Contains(err.Error(), "PostToolUse must be an array") {
		t.Fatalf("expected incompatible hook shape error, got %v", err)
	}
}
