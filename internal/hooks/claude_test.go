package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func claudeStore(t *testing.T, withDir bool) Context {
	t.Helper()
	store := t.TempDir()
	if withDir {
		if err := os.MkdirAll(filepath.Join(store, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return Context{StoreRoot: store, Config: config.Default()}
}

func claudeSettings(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".claude", "settings.json")
}

func postToolUse(t *testing.T, path string) []any {
	t.Helper()
	obj, err := readJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	h, ok := obj["hooks"].(map[string]any)
	if !ok {
		return nil
	}
	a, _ := h["PostToolUse"].([]any)
	return a
}

func TestClaude_Detect(t *testing.T) {
	if !(claudeInstaller{}).Detect(claudeStore(t, true)) {
		t.Error("expected Detect true when .claude exists")
	}
	if (claudeInstaller{}).Detect(claudeStore(t, false)) {
		t.Error("expected Detect false without .claude")
	}
}

func TestClaude_MergePreservesExisting(t *testing.T) {
	ctx := claudeStore(t, true)
	pre := `{
  "model": "opus",
  "hooks": { "PostToolUse": [ { "matcher": "Bash", "hooks": [ { "type": "command", "command": "echo other" } ] } ] }
}`
	if err := os.WriteFile(claudeSettings(ctx), []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	obj, _ := readJSONObject(claudeSettings(ctx))
	if obj["model"] != "opus" {
		t.Errorf("unrelated key lost: %v", obj["model"])
	}
	if n := len(postToolUse(t, claudeSettings(ctx))); n != 2 {
		t.Errorf("expected 2 PostToolUse entries (other + pyra), got %d", n)
	}
	body, _ := os.ReadFile(claudeSettings(ctx))
	if !strings.Contains(string(body), ManagedMarker) {
		t.Error("pyra entry missing marker")
	}
	if !strings.Contains(string(body), "echo other") {
		t.Error("unrelated PostToolUse entry lost")
	}
}

func TestClaude_Idempotent(t *testing.T) {
	ctx := claudeStore(t, true)
	if _, err := (claudeInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(postToolUse(t, claudeSettings(ctx))); n != 1 {
		t.Errorf("re-install duplicated the entry: %d", n)
	}
}

func TestClaude_AbsentFileCreated(t *testing.T) {
	ctx := claudeStore(t, false)
	if _, err := (claudeInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claudeSettings(ctx)); err != nil {
		t.Errorf("settings.json not created: %v", err)
	}
}

func TestClaude_UninstallRemovesOnlyPyra(t *testing.T) {
	ctx := claudeStore(t, true)
	pre := `{ "hooks": { "PostToolUse": [ { "matcher": "Bash", "hooks": [ { "type": "command", "command": "echo other" } ] } ] } }`
	if err := os.WriteFile(claudeSettings(ctx), []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeInstaller{}).Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(postToolUse(t, claudeSettings(ctx))); n != 1 {
		t.Errorf("expected only the non-pyra entry to remain, got %d", n)
	}
	body, _ := os.ReadFile(claudeSettings(ctx))
	if strings.Contains(string(body), ManagedMarker) {
		t.Error("pyra entry not removed")
	}
	if !strings.Contains(string(body), "echo other") {
		t.Error("non-pyra entry was removed")
	}
}
