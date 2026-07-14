package hooks

import (
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
func (k kiroCLIInstaller) selectAgent(ctx Context) (path string, ambiguous bool) {
	dir := k.agentsDir(ctx)
	if strings.TrimSpace(ctx.KiroAgent) != "" {
		name := strings.TrimSuffix(ctx.KiroAgent, ".json")
		return filepath.Join(dir, name+".json"), false
	}
	agents := k.listAgents(ctx)
	switch len(agents) {
	case 0:
		return filepath.Join(dir, "pyra.json"), false
	case 1:
		return agents[0], false
	default:
		return "", true
	}
}

func (k kiroCLIInstaller) Install(ctx Context) (Result, error) {
	path, ambiguous := k.selectAgent(ctx)
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
	hooksObj := asObject(obj["hooks"])
	ptu := filterOutPyraCLI(asArray(hooksObj["postToolUse"]))
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
		hooksObj := asObject(obj["hooks"])
		before := asArray(hooksObj["postToolUse"])
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
	res := Result{Target: TargetKiroCLI, Action: ActionAbsent}
	for _, path := range k.listAgents(ctx) {
		obj, err := readJSONObject(path)
		if err != nil {
			return Result{}, err
		}
		hooksObj := asObject(obj["hooks"])
		for _, e := range asArray(hooksObj["postToolUse"]) {
			if isPyraCLIEntry(e) {
				res.Paths = append(res.Paths, path)
				res.Action = ActionPresent
			}
		}
	}
	return res, nil
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
