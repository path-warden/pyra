// Package frontmatter performs strict parsing and hardening of a Canon
// artifact's YAML frontmatter envelope.
//
// gopkg.in/yaml.v3 does not replicate PyYAML's protections against alias/anchor
// bombs or deeply nested documents, so this package adds explicit guards (byte
// size, nesting depth, alias rejection) before decoding, and decodes strictly so
// unknown or duplicate keys are rejected.
package frontmatter

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

const (
	maxBytes = 64 * 1024 // generous ceiling for a metadata envelope
	maxDepth = 32        // nesting guard against depth bombs
)

// CurrentSchemaVersion is the schema version new artifacts are minted with.
const CurrentSchemaVersion = 1

// SupportedSchemaVersions lists schema versions this build can read.
var SupportedSchemaVersions = []int{1}

// Issue codes emitted by this package.
const (
	CodeTooLarge      = "frontmatter_too_large"
	CodeBomb          = "frontmatter_bomb"
	CodeMalformed     = "malformed_frontmatter"
	CodeUnknownField  = "frontmatter_unknown_field"
	CodeDuplicateKey  = "frontmatter_duplicate_key"
	CodeUnsupportedSV = "unsupported_schema_version"
)

// Parse hardens and decodes a frontmatter block. It returns the decoded envelope
// (with Present=true on success) and any issues found. A non-nil error is
// returned only for unexpected internal failures; expected problems are reported
// as issues so callers can aggregate them uniformly.
func Parse(raw []byte) (model.Frontmatter, []model.Issue) {
	var fm model.Frontmatter
	var issues []model.Issue

	if len(bytes.TrimSpace(raw)) == 0 {
		return fm, issues // empty envelope; absence handled by the caller
	}
	if len(raw) > maxBytes {
		return fm, append(issues, issue(CodeTooLarge, "frontmatter exceeds maximum size"))
	}

	// Structural inspection: reject aliases/anchors and excessive depth before
	// decoding into the typed envelope.
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return fm, append(issues, issue(CodeMalformed, "frontmatter is not valid YAML: "+err.Error()))
	}
	if hasAlias(&node) {
		return fm, append(issues, issue(CodeBomb, "frontmatter uses YAML aliases/anchors, which are not allowed"))
	}
	if depth(&node) > maxDepth {
		return fm, append(issues, issue(CodeBomb, "frontmatter nesting is too deep"))
	}

	// Strict decode: unknown and duplicate keys are errors.
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "already defined") || strings.Contains(msg, "duplicate"):
			return fm, append(issues, issue(CodeDuplicateKey, "frontmatter has a duplicate key: "+msg))
		case strings.Contains(msg, "not found in type") || strings.Contains(msg, "field"):
			return fm, append(issues, issue(CodeUnknownField, "frontmatter has an unknown field: "+msg))
		default:
			return fm, append(issues, issue(CodeMalformed, "frontmatter could not be decoded: "+msg))
		}
	}
	fm.Present = true

	if fm.SchemaVersion != 0 && !supported(fm.SchemaVersion) {
		issues = append(issues, issue(CodeUnsupportedSV, "unsupported schema_version"))
	}
	return fm, issues
}

// Migrate brings an envelope up to CurrentSchemaVersion deterministically. It
// returns the migrated envelope and whether anything changed.
func Migrate(fm model.Frontmatter) (model.Frontmatter, bool) {
	if fm.SchemaVersion >= CurrentSchemaVersion {
		return fm, false
	}
	fm.SchemaVersion = CurrentSchemaVersion
	return fm, true
}

func supported(v int) bool {
	for _, s := range SupportedSchemaVersions {
		if s == v {
			return true
		}
	}
	return false
}

func hasAlias(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return true
	}
	for _, c := range n.Content {
		if hasAlias(c) {
			return true
		}
	}
	return false
}

func depth(n *yaml.Node) int {
	if n == nil || len(n.Content) == 0 {
		return 1
	}
	max := 0
	for _, c := range n.Content {
		if d := depth(c); d > max {
			max = d
		}
	}
	return max + 1
}

func issue(code, msg string) model.Issue {
	return model.Issue{Severity: model.SeverityError, Code: code, Message: msg}
}
