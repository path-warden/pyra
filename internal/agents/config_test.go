package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func changeAt(t *testing.T, plan Plan, suffix string) FileChange {
	t.Helper()
	for _, c := range plan.Changes {
		if strings.HasSuffix(c.Path, suffix) {
			return c
		}
	}
	t.Fatalf("plan has no change ending %q: %+v", suffix, plan.Changes)
	return FileChange{}
}

func decodeObjectT(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestBuildPlanStrictJSONAndMultiAgent(t *testing.T) {
	root := t.TempDir()
	existing := `{"other":{"command":"keep"},"mcpServers":{"existing":{"command":"other"}}}`
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(root, []ID{Pi, Claude, Kiro})
	if err != nil {
		t.Fatal(err)
	}
	shared := changeAt(t, plan, ".mcp.json")
	obj := decodeObjectT(t, shared.Bytes)
	if _, ok := obj["other"]; !ok {
		t.Fatal("unrelated root setting was lost")
	}
	servers := obj["mcpServers"].(map[string]any)
	if len(servers) != 2 || servers["pyra"] == nil || servers["existing"] == nil {
		t.Fatalf("unexpected shared servers: %#v", servers)
	}
	entry := servers["pyra"].(map[string]any)
	args := entry["args"].([]any)
	if entry["command"] != "pyra" || args[0] != "serve" || args[1] != root || args[2] != "--mcp" {
		t.Fatalf("bad pyra command: %#v", entry)
	}

	pi := decodeObjectT(t, changeAt(t, plan, filepath.Join(".pi", "settings.json")).Bytes)
	packages := pi["packages"].([]any)
	if len(packages) != 1 || packages[0] != piMCPPackage {
		t.Fatalf("bad Pi packages: %#v", packages)
	}
	kiro := decodeObjectT(t, changeAt(t, plan, filepath.Join(".kiro", "settings", "mcp.json")).Bytes)
	if kiro["mcpServers"].(map[string]any)["pyra"] == nil {
		t.Fatal("Kiro Pyra server missing")
	}
	if strings.Count(string(shared.Bytes), `"pyra": {`) != 1 {
		t.Fatalf("shared config contains duplicate pyra entry:\n%s", shared.Bytes)
	}
}

func TestBuildPlanPiPreservesSimilarlyPrefixedPackages(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"packages":["npm:pi-mcp-adapter-helper",{"source":"npm:pi-mcp-adapter-tools"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(root, []ID{Pi})
	if err != nil {
		t.Fatal(err)
	}
	settings := decodeObjectT(t, changeAt(t, plan, filepath.Join(".pi", "settings.json")).Bytes)
	packages := settings["packages"].([]any)
	if len(packages) != 3 || packages[0] != "npm:pi-mcp-adapter-helper" {
		t.Fatalf("unrelated prefixed packages were not preserved: %#v", packages)
	}
}

func TestBuildPlanRejectsNonOwnedPyraCollision(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"pyra":{"command":"not-pyra"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildPlan(root, []ID{Claude}); err == nil || !strings.Contains(err.Error(), ".mcp.json") {
		t.Fatalf("expected path-specific collision error, got %v", err)
	}
}

func TestBuildPlanOpenCodeJSONC(t *testing.T) {
	root := t.TempDir()
	raw := []byte("{\n  // keep this setting\n  \"model\": \"provider/model\",\n}\n")
	if err := os.WriteFile(filepath.Join(root, "opencode.jsonc"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(root, []ID{OpenCode})
	if err != nil {
		t.Fatal(err)
	}
	change := changeAt(t, plan, "opencode.jsonc")
	obj, err := parseJSONCObject(change.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if obj["model"] != "provider/model" {
		t.Fatal("OpenCode setting lost")
	}
	entry := obj["mcp"].(map[string]any)["pyra"].(map[string]any)
	if entry["type"] != "local" || entry["enabled"] != true || entry["cwd"] != root {
		t.Fatalf("bad OpenCode entry: %#v", entry)
	}
}

func TestBuildPlanOpenCodeAmbiguousAndMalformed(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := BuildPlan(root, []ID{OpenCode}); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
	if err := os.Remove(filepath.Join(root, "opencode.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "opencode.jsonc"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildPlan(root, []ID{OpenCode}); err == nil || !strings.Contains(err.Error(), "opencode.jsonc") {
		t.Fatalf("expected malformed JSONC error, got %v", err)
	}
}

func TestBuildPlanCodexTOMLPreservesCommentsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# preserve me\nmodel = \"gpt\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(root, []ID{Codex})
	if err != nil {
		t.Fatal(err)
	}
	first := changeAt(t, plan, filepath.Join(".codex", "config.toml"))
	if !strings.HasPrefix(string(first.Bytes), "# preserve me\nmodel = \"gpt\"\n") || !strings.Contains(string(first.Bytes), codexBegin) || !strings.Contains(string(first.Bytes), "required = true") {
		t.Fatalf("bad Codex render:\n%s", first.Bytes)
	}
	var parsed map[string]any
	if err := toml.Unmarshal(first.Bytes, &parsed); err != nil {
		t.Fatalf("rendered TOML invalid: %v", err)
	}
	if err := os.WriteFile(path, first.Bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	secondPlan, err := BuildPlan(root, []ID{Codex})
	if err != nil {
		t.Fatal(err)
	}
	second := changeAt(t, secondPlan, filepath.Join(".codex", "config.toml"))
	if string(first.Bytes) != string(second.Bytes) {
		t.Fatalf("Codex render not idempotent\nfirst:\n%s\nsecond:\n%s", first.Bytes, second.Bytes)
	}
}

func TestBuildPlanCodexRejectsInvalidOrUnowned(t *testing.T) {
	for name, body := range map[string]string{
		"invalid":  "[broken\n",
		"unowned":  "[mcp_servers.pyra]\ncommand = \"other\"\n",
		"markers":  codexBegin + "\nmissing end\n",
		"reversed": codexEnd + "\n" + codexBegin + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".codex", "config.toml")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := BuildPlan(root, []ID{Codex}); err == nil || !strings.Contains(err.Error(), "config.toml") {
				t.Fatalf("expected path-specific error, got %v", err)
			}
		})
	}
}

func TestBuildPlanStableOrderAndApply(t *testing.T) {
	root := t.TempDir()
	plan, err := BuildPlan(root, []ID{Kiro, Pi, OpenCode, Codex, Claude})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(plan.Changes); i++ {
		if plan.Changes[i-1].Path > plan.Changes[i].Path {
			t.Fatalf("changes not sorted: %+v", plan.Changes)
		}
	}
	result, err := ApplyPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Completed) != len(plan.Changes) || len(result.Pending) != 0 {
		t.Fatalf("bad apply result: %+v", result)
	}
	for _, c := range plan.Changes {
		got, err := os.ReadFile(c.Path)
		if err != nil || string(got) != string(c.Bytes) {
			t.Errorf("change not applied to %s: err=%v", c.Path, err)
		}
	}
}
