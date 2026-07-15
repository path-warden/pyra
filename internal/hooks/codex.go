package hooks

import (
	"fmt"
	"os"
	"path/filepath"
)

// codexInstaller manages a project-scoped Codex PostToolUse gate hook.
type codexInstaller struct{}

func (codexInstaller) Target() Target { return TargetCodex }

func codexPath(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".codex", "hooks.json")
}

func (codexInstaller) Detect(ctx Context) bool {
	fi, err := os.Stat(filepath.Join(ctx.StoreRoot, ".codex"))
	return err == nil && fi.IsDir()
}

func pyraCodexEntry() map[string]any {
	return map[string]any{
		"matcher": "Edit|Write",
		"hooks": []any{
			map[string]any{
				"type":          "command",
				"command":       "pyra gate .  # " + ManagedMarker,
				"statusMessage": "Checking Pyra Canon",
			},
		},
	}
}

func (codexInstaller) Install(ctx Context) (Result, error) {
	path := codexPath(ctx)
	_, existed := readFileOK(path)
	obj, err := readJSONObject(path)
	if err != nil {
		return Result{}, err
	}
	hooksObj, err := objectField(obj, "hooks")
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	existing, err := arrayField(hooksObj, "PostToolUse")
	if err != nil {
		return Result{}, fmt.Errorf("%s: hooks.%w", path, err)
	}
	entries := filterOutPyraEntries(existing)
	entries = append(entries, pyraCodexEntry())
	hooksObj["PostToolUse"] = entries
	obj["hooks"] = hooksObj
	if err := writeJSONObject(path, obj); err != nil {
		return Result{}, err
	}
	action := ActionUpdated
	if !existed {
		action = ActionCreated
	}
	return Result{Target: TargetCodex, Action: action, Paths: []string{path}}, nil
}

func (codexInstaller) Uninstall(ctx Context) (Result, error) {
	path := codexPath(ctx)
	if _, ok := readFileOK(path); !ok {
		return Result{Target: TargetCodex, Action: ActionUnchanged}, nil
	}
	obj, err := readJSONObject(path)
	if err != nil {
		return Result{}, err
	}
	hooksObj, err := objectField(obj, "hooks")
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	before, err := arrayField(hooksObj, "PostToolUse")
	if err != nil {
		return Result{}, fmt.Errorf("%s: hooks.%w", path, err)
	}
	after := filterOutPyraEntries(before)
	if len(after) == len(before) {
		return Result{Target: TargetCodex, Action: ActionUnchanged, Paths: []string{path}}, nil
	}
	if len(after) == 0 {
		delete(hooksObj, "PostToolUse")
	} else {
		hooksObj["PostToolUse"] = after
	}
	if len(hooksObj) == 0 {
		delete(obj, "hooks")
	} else {
		obj["hooks"] = hooksObj
	}
	if err := writeJSONObject(path, obj); err != nil {
		return Result{}, err
	}
	return Result{Target: TargetCodex, Action: ActionRemoved, Paths: []string{path}}, nil
}

func (codexInstaller) Status(ctx Context) (Result, error) {
	path := codexPath(ctx)
	obj, err := readJSONObject(path)
	if err != nil {
		return Result{}, err
	}
	hooksObj, err := objectField(obj, "hooks")
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	entries, err := arrayField(hooksObj, "PostToolUse")
	if err != nil {
		return Result{}, fmt.Errorf("%s: hooks.%w", path, err)
	}
	for _, entry := range entries {
		if isPyraEntry(entry) {
			return Result{Target: TargetCodex, Action: ActionPresent, Paths: []string{path}}, nil
		}
	}
	return Result{Target: TargetCodex, Action: ActionAbsent}, nil
}
