package codegraph

import (
	"strings"
	"unicode"
)

// splitLang extracts the language prefix of a symbol-id ("<lang>:...").
func splitLang(id string) string {
	if i := strings.IndexByte(id, ':'); i >= 0 {
		return id[:i]
	}
	return ""
}

// isExported reports whether a symbol is exported/public, per a documented,
// deterministic per-language rule. Where a language's visibility is not encoded
// in the name (java, rust, and unknown languages), a top-level definition is
// treated as public — deliberately conservative so a library's reachable set
// stays sane for the dead-code consumer.
func isExported(lang, name, parent string) bool {
	if name == "" {
		return false
	}
	switch lang {
	case "go":
		return unicode.IsUpper([]rune(name)[0])
	case "python", "javascript", "typescript", "tsx":
		return !strings.HasPrefix(name, "_")
	default: // java, rust, and any other language
		return parent == ""
	}
}

// isEntryPoint reports whether a node is a reachability root: a program entry
// (named "main") or an exported/public symbol.
func isEntryPoint(n *SymbolNode) bool {
	return n.Name == "main" || n.Exported
}
