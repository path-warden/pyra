package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// claudeInstaller manages the project-scoped Claude Code PostToolUse hook in
// <store>/.claude/settings.json. It JSON-merges a single pyra-managed entry,
// preserving every other setting and PostToolUse entry.
type claudeInstaller struct{}

func (claudeInstaller) Target() Target { return TargetClaude }

func claudePath(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".claude", "settings.json")
}

func (claudeInstaller) Detect(ctx Context) bool {
	fi, err := os.Stat(filepath.Join(ctx.StoreRoot, ".claude"))
	return err == nil && fi.IsDir()
}

// pyraClaudeEntry is the PostToolUse entry pyra installs: it runs the gate
// after file-writing tool calls. The marker in the command makes it identifiable.
func pyraClaudeEntry() map[string]any {
	return map[string]any{
		"matcher": "Write|Edit|MultiEdit",
		"hooks": []any{
			map[string]any{"type": "command", "command": "pyra gate  # " + ManagedMarker},
		},
	}
}

func (c claudeInstaller) Install(ctx Context) (Result, error) {
	path := claudePath(ctx)
	_, existed := readFileOK(path)

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
	ptu := filterOutPyraEntries(entries)
	ptu = append(ptu, pyraClaudeEntry())
	hooksObj["PostToolUse"] = ptu
	obj["hooks"] = hooksObj

	if err := writeJSONObject(path, obj); err != nil {
		return Result{}, err
	}
	action := ActionUpdated
	if !existed {
		action = ActionCreated
	}
	return Result{Target: TargetClaude, Action: action, Paths: []string{path}}, nil
}

func (c claudeInstaller) Uninstall(ctx Context) (Result, error) {
	path := claudePath(ctx)
	if _, ok := readFileOK(path); !ok {
		return Result{Target: TargetClaude, Action: ActionUnchanged}, nil
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
		return Result{Target: TargetClaude, Action: ActionUnchanged, Paths: []string{path}}, nil
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
	return Result{Target: TargetClaude, Action: ActionRemoved, Paths: []string{path}}, nil
}

func (c claudeInstaller) Status(ctx Context) (Result, error) {
	path := claudePath(ctx)
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
	for _, e := range entries {
		if isPyraEntry(e) {
			return Result{Target: TargetClaude, Action: ActionPresent, Paths: []string{path}}, nil
		}
	}
	return Result{Target: TargetClaude, Action: ActionAbsent}, nil
}

// filterOutPyraEntries returns the entries that are not pyra-managed.
func filterOutPyraEntries(entries []any) []any {
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		if !isPyraEntry(e) {
			out = append(out, e)
		}
	}
	return out
}

// isPyraEntry reports whether a PostToolUse entry was installed by pyra,
// detected by the managed marker in any of its hook commands.
func isPyraEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	for _, h := range asArray(m["hooks"]) {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, ManagedMarker) {
			return true
		}
	}
	return false
}

func asObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asArray(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}
