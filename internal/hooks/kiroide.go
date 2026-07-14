package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// kiroIDEInstaller manages the Kiro IDE Agent Hook as a standalone JSON file at
// <store>/.kiro/hooks/pyra-gate.json. It owns only that file and never
// touches other hook files in the directory.
type kiroIDEInstaller struct{}

func (kiroIDEInstaller) Target() Target { return TargetKiroIDE }

func kiroIDEPath(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".kiro", "hooks", "pyra-gate.json")
}

func (kiroIDEInstaller) Detect(ctx Context) bool {
	fi, err := os.Stat(filepath.Join(ctx.StoreRoot, ".kiro"))
	return err == nil && fi.IsDir()
}

// pathMatcher builds a regular expression matching Markdown files under the
// store's canon and spec roots, used to scope the on-save hook.
func pathMatcher(ctx Context) string {
	var parts []string
	seen := map[string]bool{}
	for _, r := range append(append([]string{}, ctx.Config.CanonRoots...), ctx.Config.SpecRoots...) {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		parts = append(parts, regexp.QuoteMeta(r))
	}
	if len(parts) == 0 {
		return `.*\.md$`
	}
	return `(^|/)(` + strings.Join(parts, "|") + `)/.*\.md$`
}

func (k kiroIDEInstaller) hookObject(ctx Context) map[string]any {
	return map[string]any{
		"name":        "pyra-gate",
		"description": "Gate Canon on save (" + ManagedMarker + ")",
		"trigger":     "PostFileSave",
		"matcher":     pathMatcher(ctx),
		"action":      map[string]any{"type": "command", "command": "pyra gate"},
		"enabled":     true,
	}
}

func (k kiroIDEInstaller) Install(ctx Context) (Result, error) {
	path := kiroIDEPath(ctx)
	_, existed := readFileOK(path)
	if err := writeJSONObject(path, k.hookObject(ctx)); err != nil {
		return Result{}, err
	}
	action := ActionCreated
	if existed {
		action = ActionUpdated
	}
	return Result{Target: TargetKiroIDE, Action: action, Paths: []string{path}}, nil
}

func (k kiroIDEInstaller) Uninstall(ctx Context) (Result, error) {
	path := kiroIDEPath(ctx)
	if _, ok := readFileOK(path); !ok {
		return Result{Target: TargetKiroIDE, Action: ActionUnchanged}, nil
	}
	if err := os.Remove(path); err != nil {
		return Result{}, err
	}
	return Result{Target: TargetKiroIDE, Action: ActionRemoved, Paths: []string{path}}, nil
}

func (k kiroIDEInstaller) Status(ctx Context) (Result, error) {
	path := kiroIDEPath(ctx)
	if _, ok := readFileOK(path); ok {
		return Result{Target: TargetKiroIDE, Action: ActionPresent, Paths: []string{path}}, nil
	}
	return Result{Target: TargetKiroIDE, Action: ActionAbsent}, nil
}
