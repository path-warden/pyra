package hooks

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// kiroCLIInstaller manages the Kiro CLI hook, which lives inside an agent config
// at <store>/.kiro/agents/<agent>.json under hooks.postToolUse. Because this
// edits a user-owned file, agent selection is conservative (Requirement 7.5):
// create a pyra-owned agent when none exists, target the sole agent when there
// is exactly one, and refuse (ambiguous) when several exist without --kiro-agent.
type kiroCLIInstaller struct{}

func (kiroCLIInstaller) Target() Target { return TargetKiroCLI }

func (kiroCLIInstaller) agentsDir(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".kiro", "agents")
}

func (k kiroCLIInstaller) listAgents(ctx Context) []string {
	matches, _ := filepath.Glob(filepath.Join(k.agentsDir(ctx), "*.json"))
	sort.Strings(matches)
	return matches
}

func (k kiroCLIInstaller) Detect(ctx Context) bool {
	return len(k.listAgents(ctx)) > 0
}

// pyraCLIEntry is the postToolUse entry pyra installs: it runs the gate
// after file-writing tool calls. The marker makes it identifiable.
func pyraCLIEntry() map[string]any {
	return map[string]any{
		"matcher": "fs_write",
		"command": "pyra gate  # " + ManagedMarker,
	}
}

// selectAgent resolves which agent config to edit, or reports ambiguity.
func (k kiroCLIInstaller) selectAgent(ctx Context) (path string, ambiguous bool, err error) {
	dir := k.agentsDir(ctx)
	if strings.TrimSpace(ctx.KiroAgent) != "" {
		name := strings.TrimSuffix(strings.TrimSpace(ctx.KiroAgent), ".json")
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return "", false, fmt.Errorf("invalid Kiro agent name %q", ctx.KiroAgent)
		}
		return filepath.Join(dir, name+".json"), false, nil
	}
	agents := k.listAgents(ctx)
	switch len(agents) {
	case 0:
		return filepath.Join(dir, "pyra.json"), false, nil
	case 1:
		return agents[0], false, nil
	default:
		return "", true, nil
	}
}

func (k kiroCLIInstaller) Install(ctx Context) (Result, error) {
	path, ambiguous, err := k.selectAgent(ctx)
	if err != nil {
		return Result{}, err
	}
	if ambiguous {
		return Result{
			Target: TargetKiroCLI, Action: ActionAmbiguous,
			Detail: "multiple agent configs in .kiro/agents; select one with --kiro-agent <name>",
		}, nil
	}

	_, existed := readFileOK(path)
	obj, err := readJSONObject(path)
	if err != nil {
		return Result{}, err
	}
	hooksObj, err := objectField(obj, "hooks")
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	entries, err := arrayField(hooksObj, "postToolUse")
	if err != nil {
		return Result{}, fmt.Errorf("%s: hooks.%w", path, err)
	}
	ptu := filterOutPyraCLI(entries)
	ptu = append(ptu, pyraCLIEntry())
	hooksObj["postToolUse"] = ptu
	obj["hooks"] = hooksObj
	// A freshly created agent config needs a name (the filename stem).
	if _, ok := obj["name"]; !ok {
		obj["name"] = strings.TrimSuffix(filepath.Base(path), ".json")
	}

	if err := writeJSONObject(path, obj); err != nil {
		return Result{}, err
	}
	action := ActionUpdated
	if !existed {
		action = ActionCreated
	}
	return Result{Target: TargetKiroCLI, Action: action, Paths: []string{path}}, nil
}

func (k kiroCLIInstaller) Uninstall(ctx Context) (Result, error) {
	res := Result{Target: TargetKiroCLI, Action: ActionUnchanged}
	for _, path := range k.listAgents(ctx) {
		obj, err := readJSONObject(path)
		if err != nil {
			return Result{}, err
		}
		hooksObj, err := objectField(obj, "hooks")
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", path, err)
		}
		before, err := arrayField(hooksObj, "postToolUse")
		if err != nil {
			return Result{}, fmt.Errorf("%s: hooks.%w", path, err)
		}
		after := filterOutPyraCLI(before)
		if len(after) == len(before) {
			continue
		}
		if len(after) == 0 {
			delete(hooksObj, "postToolUse")
		} else {
			hooksObj["postToolUse"] = after
		}
		if len(hooksObj) == 0 {
			delete(obj, "hooks")
		} else {
			obj["hooks"] = hooksObj
		}
		if err := writeJSONObject(path, obj); err != nil {
			return Result{}, err
		}
		res.Paths = append(res.Paths, path)
		res.Action = ActionRemoved
	}
	return res, nil
}

func (k kiroCLIInstaller) Status(ctx Context) (Result, error) {
	path, ambiguous, err := k.selectAgent(ctx)
	if err != nil {
		return Result{}, err
	}
	if ambiguous {
		return Result{Target: TargetKiroCLI, Action: ActionAmbiguous,
			Detail: "multiple agent configs in .kiro/agents; select one with --kiro-agent <name>"}, nil
	}
	obj, err := readJSONObject(path)
	if err != nil {
		return Result{}, err
	}
	hooksObj, err := objectField(obj, "hooks")
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	entries, err := arrayField(hooksObj, "postToolUse")
	if err != nil {
		return Result{}, fmt.Errorf("%s: hooks.%w", path, err)
	}
	for _, entry := range entries {
		if isPyraCLIEntry(entry) {
			return Result{Target: TargetKiroCLI, Action: ActionPresent, Paths: []string{path}}, nil
		}
	}
	return Result{Target: TargetKiroCLI, Action: ActionAbsent}, nil
}

func filterOutPyraCLI(entries []any) []any {
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		if !isPyraCLIEntry(e) {
			out = append(out, e)
		}
	}
	return out
}

func isPyraCLIEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	cmd, ok := m["command"].(string)
	return ok && strings.Contains(cmd, ManagedMarker)
}
