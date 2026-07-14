package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// upsertBlock inserts or replaces the pyra-managed block (delimited by
// BlockBegin/BlockEnd) carrying body, preserving all surrounding content. It is
// idempotent: applying it with the same body yields identical output.
func upsertBlock(existing, body string) string {
	block := BlockBegin + "\n" + body + "\n" + BlockEnd + "\n"

	if i := strings.Index(existing, BlockBegin); i >= 0 {
		if j := strings.Index(existing[i:], BlockEnd); j >= 0 {
			endIdx := i + j + len(BlockEnd)
			rest := strings.TrimPrefix(existing[endIdx:], "\n")
			return existing[:i] + block + rest
		}
	}

	if existing == "" {
		return block
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + block
}

// removeBlock strips the pyra-managed block, preserving surrounding content.
func removeBlock(existing string) string {
	i := strings.Index(existing, BlockBegin)
	if i < 0 {
		return existing
	}
	j := strings.Index(existing[i:], BlockEnd)
	if j < 0 {
		return existing
	}
	endIdx := i + j + len(BlockEnd)
	rest := strings.TrimPrefix(existing[endIdx:], "\n")
	before := existing[:i]
	return before + rest
}

// hasBlock reports whether a pyra-managed block is present.
func hasBlock(existing string) bool {
	return strings.Contains(existing, BlockBegin) && strings.Contains(existing, BlockEnd)
}

// readJSONObject reads a JSON object file into a map. A missing file yields an
// empty (non-nil) map and no error, so callers can merge unconditionally.
func readJSONObject(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		obj = map[string]any{}
	}
	return obj, nil
}

// writeJSONObject writes obj as indented JSON, creating parent directories.
func writeJSONObject(path string, obj map[string]any) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
