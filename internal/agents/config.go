package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

const (
	serverName   = "pyra"
	piMCPPackage = "npm:pi-mcp-adapter"
	codexBegin   = "# >>> pyra managed MCP >>>"
	codexEnd     = "# <<< pyra managed MCP <<<"
)

// Action describes whether applying a planned file creates or updates it.
type Action string

const (
	Create Action = "create"
	Update Action = "update"
)

// FileChange is one fully rendered repository-local setup file.
type FileChange struct {
	Path   string
	Bytes  []byte
	Action Action
}

// Plan contains all agent setup changes, sorted by path.
type Plan struct {
	Agents  []ID
	Changes []FileChange
}

// ApplyResult identifies completed and pending files after applying a plan.
type ApplyResult struct {
	Completed []string
	Pending   []string
}

// BuildPlan parses and renders every selected integration before any writes.
func BuildPlan(storeRoot string, selected []ID) (Plan, error) {
	absRoot, err := filepath.Abs(storeRoot)
	if err != nil {
		return Plan{}, err
	}
	absRoot = filepath.Clean(absRoot)
	ids, err := normalizeIDs(selected)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Agents: ids}

	if err := addRendered(&plan, filepath.Join(absRoot, "AGENTS.md"), func(existing []byte) ([]byte, error) {
		next, rerr := renderAgents(string(existing))
		return []byte(next), rerr
	}); err != nil {
		return Plan{}, err
	}

	if hasID(ids, Claude) || hasID(ids, Pi) {
		path := filepath.Join(absRoot, ".mcp.json")
		if err := addRendered(&plan, path, func(existing []byte) ([]byte, error) {
			return renderMCPJSON(path, existing, absRoot)
		}); err != nil {
			return Plan{}, err
		}
	}
	if hasID(ids, Codex) {
		path := filepath.Join(absRoot, ".codex", "config.toml")
		if err := addRendered(&plan, path, func(existing []byte) ([]byte, error) {
			return renderCodexTOML(path, existing, absRoot)
		}); err != nil {
			return Plan{}, err
		}
	}
	if hasID(ids, OpenCode) {
		path, perr := openCodePath(absRoot)
		if perr != nil {
			return Plan{}, perr
		}
		if err := addRendered(&plan, path, func(existing []byte) ([]byte, error) {
			return renderOpenCode(path, existing, absRoot)
		}); err != nil {
			return Plan{}, err
		}
	}
	if hasID(ids, Pi) {
		path := filepath.Join(absRoot, ".pi", "settings.json")
		if err := addRendered(&plan, path, func(existing []byte) ([]byte, error) {
			return renderPiSettings(path, existing)
		}); err != nil {
			return Plan{}, err
		}
	}
	if hasID(ids, Kiro) {
		path := filepath.Join(absRoot, ".kiro", "settings", "mcp.json")
		if err := addRendered(&plan, path, func(existing []byte) ([]byte, error) {
			return renderMCPJSON(path, existing, absRoot)
		}); err != nil {
			return Plan{}, err
		}
	}

	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].Path < plan.Changes[j].Path })
	return plan, nil
}

func normalizeIDs(selected []ID) ([]ID, error) {
	values := make([]string, len(selected))
	for i, id := range selected {
		values[i] = string(id)
	}
	return ParseIDs(values)
}

func hasID(ids []ID, want ID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func addRendered(plan *Plan, path string, render func([]byte) ([]byte, error)) error {
	existing, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	next, err := render(existing)
	if err != nil {
		return fmt.Errorf("configure %s: %w", path, err)
	}
	action := Create
	if existed {
		action = Update
	}
	plan.Changes = append(plan.Changes, FileChange{Path: path, Bytes: next, Action: action})
	return nil
}

func pyraCommand(root string) map[string]any {
	return map[string]any{
		"command": "pyra",
		"args":    []any{"serve", root, "--mcp"},
	}
}

func parseJSONObject(raw []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("root must be a JSON object")
	}
	return obj, nil
}

func parseJSONCObject(raw []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	standard, err := hujson.Standardize(raw)
	if err != nil {
		return nil, err
	}
	return parseJSONObject(standard)
}

func marshalObject(obj map[string]any) ([]byte, error) {
	raw, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func objectAt(obj map[string]any, key string) (map[string]any, error) {
	v, ok := obj[key]
	if !ok || v == nil {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return m, nil
}

func ownedStrictEntry(v any) bool {
	m, ok := v.(map[string]any)
	return ok && m["command"] == "pyra"
}

func renderMCPJSON(path string, existing []byte, root string) ([]byte, error) {
	obj, err := parseJSONObject(existing)
	if err != nil {
		return nil, err
	}
	servers, err := objectAt(obj, "mcpServers")
	if err != nil {
		return nil, err
	}
	if current, ok := servers[serverName]; ok && !ownedStrictEntry(current) {
		return nil, fmt.Errorf("%s contains a non-Pyra-owned %q server", path, serverName)
	}
	servers[serverName] = pyraCommand(root)
	obj["mcpServers"] = servers
	return marshalObject(obj)
}

func openCodePath(root string) (string, error) {
	jsonPath := filepath.Join(root, "opencode.json")
	jsoncPath := filepath.Join(root, "opencode.jsonc")
	_, jsonErr := os.Stat(jsonPath)
	_, jsoncErr := os.Stat(jsoncPath)
	jsonExists := jsonErr == nil
	jsoncExists := jsoncErr == nil
	if jsonErr != nil && !os.IsNotExist(jsonErr) {
		return "", jsonErr
	}
	if jsoncErr != nil && !os.IsNotExist(jsoncErr) {
		return "", jsoncErr
	}
	if jsonExists && jsoncExists {
		return "", fmt.Errorf("both %s and %s exist; consolidate OpenCode configuration before initialization", jsonPath, jsoncPath)
	}
	if jsoncExists {
		return jsoncPath, nil
	}
	return jsonPath, nil
}

func ownedOpenCodeEntry(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	argv, ok := m["command"].([]any)
	return ok && len(argv) > 0 && argv[0] == "pyra"
}

func renderOpenCode(path string, existing []byte, root string) ([]byte, error) {
	obj, err := parseJSONCObject(existing)
	if err != nil {
		return nil, err
	}
	servers, err := objectAt(obj, "mcp")
	if err != nil {
		return nil, err
	}
	if current, ok := servers[serverName]; ok && !ownedOpenCodeEntry(current) {
		return nil, fmt.Errorf("%s contains a non-Pyra-owned %q server", path, serverName)
	}
	servers[serverName] = map[string]any{
		"type":    "local",
		"command": []any{"pyra", "serve", root, "--mcp"},
		"cwd":     root,
		"enabled": true,
	}
	obj["mcp"] = servers
	return marshalObject(obj)
}

func renderPiSettings(path string, existing []byte) ([]byte, error) {
	obj, err := parseJSONObject(existing)
	if err != nil {
		return nil, err
	}
	var packages []any
	if v, ok := obj["packages"]; ok {
		var listOK bool
		packages, listOK = v.([]any)
		if !listOK {
			return nil, fmt.Errorf("packages must be an array")
		}
	}
	out := make([]any, 0, len(packages)+1)
	found := false
	for _, item := range packages {
		switch v := item.(type) {
		case string:
			if isPiMCPPackage(v) {
				if !found {
					out = append(out, piMCPPackage)
					found = true
				}
				continue
			}
		case map[string]any:
			if source, _ := v["source"].(string); isPiMCPPackage(source) {
				return nil, fmt.Errorf("%s contains an object-form pi-mcp-adapter entry that Pyra cannot safely replace", path)
			}
		}
		out = append(out, item)
	}
	if !found {
		out = append(out, piMCPPackage)
	}
	obj["packages"] = out
	return marshalObject(obj)
}

func isPiMCPPackage(value string) bool {
	return value == piMCPPackage || strings.HasPrefix(value, piMCPPackage+"@")
}

func renderCodexTOML(path string, existing []byte, root string) ([]byte, error) {
	if len(strings.TrimSpace(string(existing))) > 0 {
		var parsed map[string]any
		if err := toml.Unmarshal(existing, &parsed); err != nil {
			return nil, err
		}
	}
	text := string(existing)
	starts := strings.Count(text, codexBegin)
	ends := strings.Count(text, codexEnd)
	if starts > 1 || ends > 1 || starts != ends {
		return nil, fmt.Errorf("%s contains malformed or duplicate Pyra MCP markers", path)
	}
	withoutManaged := text
	managedStart, managedEnd := -1, -1
	if starts == 1 {
		managedStart = strings.Index(text, codexBegin)
		endRel := strings.Index(text[managedStart:], codexEnd)
		if endRel < 0 {
			return nil, fmt.Errorf("%s contains reversed Pyra MCP markers", path)
		}
		managedEnd = managedStart + endRel + len(codexEnd)
		withoutManaged = text[:managedStart] + strings.TrimPrefix(text[managedEnd:], "\n")
	}
	if strings.Contains(withoutManaged, "[mcp_servers.pyra]") {
		return nil, fmt.Errorf("%s contains a non-Pyra-owned [mcp_servers.pyra] table", path)
	}
	rootLiteral, err := toml.Marshal(map[string]string{"value": root})
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(string(rootLiteral))
	quotedRoot := strings.TrimSpace(strings.TrimPrefix(line, "value ="))
	block := codexBegin + "\n[mcp_servers.pyra]\ncommand = \"pyra\"\nargs = [\"serve\", " + quotedRoot + ", \"--mcp\"]\nrequired = true\n" + codexEnd + "\n"
	var next string
	if starts == 1 {
		rest := strings.TrimPrefix(text[managedEnd:], "\n")
		next = text[:managedStart] + block + rest
	} else if text == "" {
		next = block
	} else {
		sep := "\n\n"
		if strings.HasSuffix(text, "\n\n") {
			sep = ""
		} else if strings.HasSuffix(text, "\n") {
			sep = "\n"
		}
		next = text + sep + block
	}
	var parsed map[string]any
	if err := toml.Unmarshal([]byte(next), &parsed); err != nil {
		return nil, fmt.Errorf("rendered TOML is invalid: %w", err)
	}
	return []byte(next), nil
}

// ApplyPlan writes each rendered file atomically in stable plan order.
func ApplyPlan(plan Plan) (ApplyResult, error) {
	result := ApplyResult{}
	for i, change := range plan.Changes {
		if err := atomicWrite(change.Path, change.Bytes); err != nil {
			for _, pending := range plan.Changes[i:] {
				result.Pending = append(result.Pending, pending.Path)
			}
			return result, fmt.Errorf("write %s: %w", change.Path, err)
		}
		result.Completed = append(result.Completed, change.Path)
	}
	return result, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".pyra-init-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}
