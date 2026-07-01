package codeintel

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// enclosingFunction returns the name of the function/method whose body contains
// byteOff, qualified as "Container::name" when the definition sits inside a
// container. Returns nil when none is found.
func enclosingFunction(p *parsed, byteOff int) *string {
	lang := p.lang
	node := p.tree.RootNode().NamedDescendantForByteRange(uint32(byteOff), uint32(byteOff))
	for n := node; n != nil; n = n.Parent() {
		if !contains(lang.Profile.FunctionKinds, n.Type(lang.Grammar)) {
			continue
		}
		name := functionName(n, lang, p.src)
		if name == "" {
			return nil
		}
		if container := nearestContainer(n, lang, p.src); container != "" {
			qualified := container + "::" + name
			return &qualified
		}
		return &name
	}
	return nil
}

// functionName extracts a function/method node's name via its name field, or
// the first identifier-kind child as a fallback.
func functionName(n *gts.Node, lang *Language, src []byte) string {
	if field := n.ChildByFieldName("name", lang.Grammar); field != nil {
		return field.Text(src)
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if contains(lang.Profile.IdentifierKinds, c.Type(lang.Grammar)) {
			return c.Text(src)
		}
	}
	return ""
}

// wholeWordCol returns the 0-based column of the first whole-word occurrence of
// word in line (boundaries are non-[A-Za-z0-9_]), matching grep -w semantics.
func wholeWordCol(line, word string) (int, bool) {
	if word == "" {
		return 0, false
	}
	from := 0
	for {
		idx := strings.Index(line[from:], word)
		if idx < 0 {
			return 0, false
		}
		start := from + idx
		end := start + len(word)
		leftOK := start == 0 || !isIdentByte(line[start-1])
		rightOK := end == len(line) || !isIdentByte(line[end])
		if leftOK && rightOK {
			return start, true
		}
		from = start + 1
		if from >= len(line) {
			return 0, false
		}
	}
}

// lineStartByte returns the byte offset where line index i (0-based) begins.
func lineStartByte(src []byte, i int) int {
	if i <= 0 {
		return 0
	}
	count := 0
	for off := 0; off < len(src); off++ {
		if src[off] == '\n' {
			count++
			if count == i {
				return off + 1
			}
		}
	}
	return len(src)
}
