package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/config"
)

func kiroCLIStore(t *testing.T) (Context, string) {
	t.Helper()
	store := t.TempDir()
	agentsDir := filepath.Join(store, ".kiro", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return Context{StoreRoot: store, Config: config.Default()}, agentsDir
}

func writeAgent(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name+".json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func cliPostToolUse(t *testing.T, path string) []any {
	t.Helper()
	obj, err := readJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	h, _ := obj["hooks"].(map[string]any)
	a, _ := h["postToolUse"].([]any)
	return a
}

func TestKiroCLI_Detect(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	if (kiroCLIInstaller{}).Detect(ctx) {
		t.Error("Detect should be false with no agent configs")
	}
	writeAgent(t, dir, "dev", `{"name":"dev"}`)
	if !(kiroCLIInstaller{}).Detect(ctx) {
		t.Error("Detect should be true once an agent config exists")
	}
}

func TestKiroCLI_NoneCreatesPyraAgent(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	res, err := (kiroCLIInstaller{}).Install(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action == ActionAmbiguous {
		t.Fatal("should not be ambiguous when no agents exist")
	}
	path := filepath.Join(dir, "pyra.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pyra.json not created: %v", err)
	}
	ptu := cliPostToolUse(t, path)
	if len(ptu) != 1 {
		t.Fatalf("expected one postToolUse entry, got %d", len(ptu))
	}
	entry := ptu[0].(map[string]any)
	if entry["matcher"] != "fs_write" || !strings.Contains(entry["command"].(string), ManagedMarker) {
		t.Errorf("unexpected entry: %v", entry)
	}
}

func TestKiroCLI_OneExistingMerged(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	path := writeAgent(t, dir, "dev", `{"name":"dev","model":"x","hooks":{"postToolUse":[{"matcher":"fs_read","command":"echo r"}]}}`)
	if _, err := (kiroCLIInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	obj, _ := readJSONObject(path)
	if obj["model"] != "x" {
		t.Errorf("unrelated key lost: %v", obj["model"])
	}
	ptu := cliPostToolUse(t, path)
	if len(ptu) != 2 {
		t.Fatalf("expected 2 entries (existing + pyra), got %d", len(ptu))
	}
	if _, err := os.Stat(filepath.Join(dir, "pyra.json")); !os.IsNotExist(err) {
		t.Error("should not create pyra.json when one agent already exists")
	}
}

func TestKiroCLI_MultipleAmbiguous(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	a := writeAgent(t, dir, "a", `{"name":"a"}`)
	b := writeAgent(t, dir, "b", `{"name":"b"}`)
	beforeA, _ := os.ReadFile(a)
	beforeB, _ := os.ReadFile(b)

	res, err := (kiroCLIInstaller{}).Install(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionAmbiguous {
		t.Errorf("expected ActionAmbiguous, got %v", res.Action)
	}
	if _, err := os.Stat(filepath.Join(dir, "pyra.json")); !os.IsNotExist(err) {
		t.Error("ambiguous install must not create pyra.json")
	}
	afterA, _ := os.ReadFile(a)
	afterB, _ := os.ReadFile(b)
	if string(beforeA) != string(afterA) || string(beforeB) != string(afterB) {
		t.Error("ambiguous install must not modify any agent")
	}
}

func TestKiroCLI_KiroAgentSelects(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	a := writeAgent(t, dir, "a", `{"name":"a"}`)
	b := writeAgent(t, dir, "b", `{"name":"b"}`)
	beforeB, _ := os.ReadFile(b)

	ctx.KiroAgent = "a"
	if _, err := (kiroCLIInstaller{}).Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(cliPostToolUse(t, a)) != 1 {
		t.Error("selected agent a should get the pyra entry")
	}
	afterB, _ := os.ReadFile(b)
	if string(beforeB) != string(afterB) {
		t.Error("non-selected agent b must be untouched")
	}
}

func TestKiroCLI_Idempotent(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	g := kiroCLIInstaller{}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(cliPostToolUse(t, filepath.Join(dir, "pyra.json"))); n != 1 {
		t.Errorf("re-install duplicated entry: %d", n)
	}
}

func TestKiroCLI_UninstallRemovesOnlyPyra(t *testing.T) {
	ctx, dir := kiroCLIStore(t)
	path := writeAgent(t, dir, "dev", `{"name":"dev","hooks":{"postToolUse":[{"matcher":"fs_read","command":"echo r"}]}}`)
	g := kiroCLIInstaller{}
	if _, err := g.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	ptu := cliPostToolUse(t, path)
	if len(ptu) != 1 {
		t.Fatalf("expected only the non-pyra entry, got %d", len(ptu))
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), ManagedMarker) {
		t.Error("pyra entry not removed")
	}
	if !strings.Contains(string(body), "echo r") {
		t.Error("non-pyra entry removed")
	}
}
