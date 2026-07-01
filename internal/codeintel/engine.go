package codeintel

import (
	"os"
	"strings"
	"sync"

	gts "github.com/odvcencio/gotreesitter"
)

// Engine parses source and extracts structural information. It caches compiled
// per-language queries; parsing itself is on-demand (no persistent index), so
// identical repository state yields identical results.
type Engine struct {
	reg *Registry

	mu      sync.Mutex
	queries map[string]*compiledQueries // keyed by language name
}

type compiledQueries struct {
	tags    *gts.Query
	locals  *gts.Query // nil if absent or refused
	imports *gts.Query // nil if absent or refused
}

// NewEngine builds an engine over the given registry (nil = DefaultRegistry).
func NewEngine(reg *Registry) *Engine {
	if reg == nil {
		reg = DefaultRegistry()
	}
	return &Engine{reg: reg, queries: map[string]*compiledQueries{}}
}

// parsed holds a single parse of a file plus its resolved language.
type parsed struct {
	lang *Language
	tree *gts.Tree
	src  []byte
	rel  string
	cq   *compiledQueries
}

// parseFile reads and parses a file once. rel is the path used in symbol-ids.
func (e *Engine) parseFile(path, rel string) (*parsed, error) {
	lang, err := e.reg.ForFile(path)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := gts.NewParser(lang.Grammar)
	tree, err := p.Parse(src)
	if err != nil {
		return nil, err
	}
	cq, err := e.compiled(lang)
	if err != nil {
		return nil, err
	}
	return &parsed{lang: lang, tree: tree, src: src, rel: rel, cq: cq}, nil
}

// compiled returns (and memoizes) the compiled queries for a language. Optional
// locals/imports queries degrade to nil on compile error or supertype syntax.
func (e *Engine) compiled(lang *Language) (*compiledQueries, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cq, ok := e.queries[lang.Name]; ok {
		return cq, nil
	}
	tags, err := gts.NewQuery(lang.Tags, lang.Grammar)
	if err != nil {
		return nil, err
	}
	cq := &compiledQueries{tags: tags}
	cq.locals = compileOptional(lang.Locals, lang.Grammar)
	cq.imports = compileOptional(lang.Imports, lang.Grammar)
	e.queries[lang.Name] = cq
	return cq, nil
}

// compileOptional compiles an optional query, returning nil when the text is
// empty, fails to compile, or contains supertype syntax "(a/b)" (grove refuses
// these as a defensive measure; we mirror the capability semantics).
func compileOptional(text string, lang *gts.Language) *gts.Query {
	if strings.TrimSpace(text) == "" || hasSupertypePattern(text) {
		return nil
	}
	q, err := gts.NewQuery(text, lang)
	if err != nil {
		return nil
	}
	return q
}

// hasSupertypePattern reports a "(a/b)" supertype token outside of any string.
func hasSupertypePattern(text string) bool {
	inString := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '/' && i > 0 && i+1 < len(text) {
			p, n := text[i-1], text[i+1]
			if isIdentByte(p) && isIdentByte(n) {
				return true
			}
		}
	}
	return false
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Extract parses a file and returns its symbols (definitions and references).
// It also returns the parse for callers that need the tree/source.
func (e *Engine) Extract(path, rel string) ([]Symbol, *parsed, error) {
	p, err := e.parseFile(path, rel)
	if err != nil {
		return nil, nil, err
	}
	syms := e.extractFrom(p)
	return syms, p, nil
}

// extractFrom runs the tags query and maps captures to Symbols, mirroring
// grove: @definition.<kind> sets kind+is_definition, @reference.<kind> a
// reference, @name the identifier. Function/method spans widen to the body and
// parent is the nearest container. Overlapping matches dedupe by
// (start,end,is_definition), first kept.
func (e *Engine) extractFrom(p *parsed) []Symbol {
	var out []Symbol
	src := p.src
	lang := p.lang
	for _, m := range p.cq.tags.Execute(p.tree) {
		var anchor *gts.Node
		var nameNode *gts.Node
		var kind string
		isDef := false
		for _, c := range m.Captures {
			switch {
			case c.Name == "name":
				nameNode = c.Node
			case strings.HasPrefix(c.Name, "definition."):
				anchor = c.Node
				kind = c.Name[len("definition."):]
				isDef = true
			case strings.HasPrefix(c.Name, "reference."):
				anchor = c.Node
				kind = c.Name[len("reference."):]
				isDef = false
			}
		}
		if anchor == nil {
			continue
		}
		nn := nameNode
		if nn == nil {
			nn = anchor
		}
		pos := nn.StartPoint()
		name := nn.Text(src)
		// The anchor is the @definition/@reference node, which in our tags
		// queries is the whole construct (e.g. the entire function_declaration,
		// body included), so its byte span already covers the full symbol — no
		// separate widening pass is needed the way grove's engine does it.
		startByte := int(anchor.StartByte())
		endByte := int(anchor.EndByte())
		sym := Symbol{
			ID:           FormatID(lang.Name, p.rel, name, int(pos.Row)+1),
			Name:         name,
			Kind:         kind,
			IsDefinition: isDef,
			File:         p.rel,
			Line:         int(pos.Row) + 1,
			Col:          int(pos.Column) + 1,
			StartByte:    startByte,
			EndByte:      endByte,
			Signature:    lineText(src, int(nn.StartByte())),
		}
		if isDef {
			if parent := nearestContainer(anchor, lang, src); parent != "" {
				sym.Parent = &parent
			}
		}
		out = append(out, sym)
	}
	return dedupeSymbols(out)
}

// dedupeSymbols drops overlapping tag matches by (start,end,is_definition),
// keeping the first (query pattern order decides which kind survives).
func dedupeSymbols(syms []Symbol) []Symbol {
	type key struct {
		s, e int
		d    bool
	}
	seen := map[key]bool{}
	out := syms[:0]
	for _, s := range syms {
		k := key{s.StartByte, s.EndByte, s.IsDefinition}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

// nearestContainer walks up from a definition node to the nearest container
// node (per profile.containers) and returns its name, truncated at '<' to strip
// generics, mirroring grove.
func nearestContainer(node *gts.Node, lang *Language, src []byte) string {
	for n := node.Parent(); n != nil; n = n.Parent() {
		t := n.Type(lang.Grammar)
		for _, c := range lang.Profile.Containers {
			if c.Kind == t {
				field := n.ChildByFieldName(c.Field, lang.Grammar)
				if field != nil {
					name := field.Text(src)
					if i := strings.IndexByte(name, '<'); i >= 0 {
						name = name[:i]
					}
					return name
				}
			}
		}
	}
	return ""
}

// Check parses a file and collects ERROR/MISSING nodes.
func (e *Engine) Check(path string) ([]Defect, error) {
	p, err := e.parseFile(path, path)
	if err != nil {
		return nil, err
	}
	var defects []Defect
	collectDefects(p.tree.RootNode(), p.src, &defects)
	return defects, nil
}

func collectDefects(n *gts.Node, src []byte, out *[]Defect) {
	if n == nil {
		return
	}
	if n.IsError() || n.IsMissing() {
		pos := n.StartPoint()
		kind := "error"
		if n.IsMissing() {
			kind = "missing"
		}
		txt := n.Text(src)
		if len(txt) > 60 {
			txt = txt[:60]
		}
		*out = append(*out, Defect{
			Kind:      kind,
			Line:      int(pos.Row) + 1,
			Col:       int(pos.Column) + 1,
			StartByte: int(n.StartByte()),
			EndByte:   int(n.EndByte()),
			Text:      txt,
		})
	}
	for i := 0; i < n.ChildCount(); i++ {
		collectDefects(n.Child(i), src, out)
	}
}

// Slice returns the source text of a symbol's byte span.
func Slice(src []byte, s Symbol) string {
	if s.StartByte < 0 || s.EndByte > len(src) || s.StartByte > s.EndByte {
		return ""
	}
	return string(src[s.StartByte:s.EndByte])
}

// lineText returns the trimmed source line containing byteOffset.
func lineText(src []byte, byteOffset int) string {
	if byteOffset < 0 || byteOffset > len(src) {
		return ""
	}
	start := byteOffset
	for start > 0 && src[start-1] != '\n' {
		start--
	}
	end := byteOffset
	for end < len(src) && src[end] != '\n' {
		end++
	}
	return strings.TrimSpace(string(src[start:end]))
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
