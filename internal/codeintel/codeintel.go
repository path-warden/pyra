// Package codeintel provides structural code search and navigation — a native
// Go port of grove's operations — over a pure-Go tree-sitter runtime
// (github.com/odvcencio/gotreesitter). It is read-only: operations only read
// source files and never mutate the repository.
//
// The package is deliberately placed outside internal/canon so that the Canon
// authority path stays a pure, offline, deterministic function of repo state
// (see internal/canon/archcheck_test.go). Grammars are embedded at build time,
// so no operation performs any network access.
//
// Output shapes and the symbol-id scheme mirror grove for parity: a symbol-id
// is "<lang>:<relpath>#<name>@<line>" with a 1-based line, and it round-trips
// through the seven operations (outline, symbols, source, check, callers, map,
// definition).
package codeintel

// Symbol is one definition or reference extracted from source. Field names and
// JSON keys match grove's Symbol for output parity.
type Symbol struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	IsDefinition bool    `json:"is_definition"`
	File         string  `json:"file"`
	Line         int     `json:"line"` // 1-based
	Col          int     `json:"col"`  // 1-based
	StartByte    int     `json:"start_byte"`
	EndByte      int     `json:"end_byte"`
	Signature    string  `json:"signature"`
	Parent       *string `json:"parent,omitempty"`
}

// Defect is a syntax error (an ERROR or MISSING node) reported by Check.
type Defect struct {
	Kind      string `json:"kind"` // "missing" | "error"
	Line      int    `json:"line"` // 1-based
	Col       int    `json:"col"`  // 1-based
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Text      string `json:"text"` // first 60 chars of the node's source
}

// SourceResult is the payload of the source operation.
type SourceResult struct {
	ID              string   `json:"id"`
	Source          string   `json:"source"`
	OtherCandidates []string `json:"other_candidates,omitempty"`
}

// CallSite is one reference returned by Callers. Source is "structural"
// (tree-sitter resolved) or "textual" (whole-word grep).
type CallSite struct {
	File       string  `json:"file"`
	Line       int     `json:"line"` // 1-based
	Col        int     `json:"col"`  // 1-based
	InFunction *string `json:"in_function,omitempty"`
	Text       string  `json:"text"`
	Source     string  `json:"source"`
}

// FileMap groups a file's definitions with their outgoing references (Map).
type FileMap struct {
	File    string     `json:"file"`
	Entries []MapEntry `json:"entries"`
}

// MapEntry is one definition and the names it references. Row carries the
// 1-based line (grove names this field "row"); preserved for output parity.
type MapEntry struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Parent     *string  `json:"parent,omitempty"`
	Row        int      `json:"row"`
	Signature  string   `json:"signature"`
	References []string `json:"references,omitempty"`
}

// DefinitionResult is the unified shape returned by Definition for both name
// and position (--at) modes. grove diverges here between its CLI and MCP faces;
// memphis standardizes on this single shape so CLI and MCP are equivalent.
type DefinitionResult struct {
	Resolved    string   `json:"resolved"`
	Definitions []Symbol `json:"definitions"`
}
