package codeintel

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// identifierAt returns the identifier text at a 0-based row/col, if the node
// there is one of the language's identifier kinds. ok is false otherwise.
func identifierAt(p *parsed, row, col int) (name string, node *gts.Node, ok bool) {
	pt := gts.Point{Row: uint32(row), Column: uint32(col)}
	n := p.tree.RootNode().NamedDescendantForPointRange(pt, pt)
	if n == nil {
		return "", nil, false
	}
	if !contains(p.lang.Profile.IdentifierKinds, n.Type(p.lang.Grammar)) {
		return "", nil, false
	}
	return n.Text(p.src), n, true
}

// resolveLocalAt attempts scope-aware resolution of the identifier at (row,col)
// using the locals query. It returns a synthetic local Symbol when an enclosing
// binding is found (shadowing resolves innermost-first), else ok=false so the
// caller falls back to name/import resolution. Never worse than name lookup.
func (e *Engine) resolveLocalAt(p *parsed, name string, ref *gts.Node) (Symbol, bool) {
	if p.cq.locals == nil || ref == nil {
		return Symbol{}, false
	}
	var scopes []*gts.Node
	var defs []*gts.Node
	for _, m := range p.cq.locals.Execute(p.tree) {
		for _, c := range m.Captures {
			switch {
			case strings.HasPrefix(c.Name, "local.scope"):
				scopes = append(scopes, c.Node)
			case strings.HasPrefix(c.Name, "local.definition"):
				defs = append(defs, c.Node)
			}
		}
	}
	refStart := ref.StartByte()
	refEnd := ref.EndByte()

	// Enclosing scopes, innermost (smallest span) first.
	type scoped struct {
		node *gts.Node
		span uint32
	}
	var enclosing []scoped
	for _, s := range scopes {
		if s.StartByte() <= refStart && s.EndByte() >= refEnd {
			enclosing = append(enclosing, scoped{s, s.EndByte() - s.StartByte()})
		}
	}
	for i := 1; i < len(enclosing); i++ {
		for j := i; j > 0 && enclosing[j-1].span > enclosing[j].span; j-- {
			enclosing[j-1], enclosing[j] = enclosing[j], enclosing[j-1]
		}
	}

	for _, sc := range enclosing {
		for _, d := range defs {
			if d.StartByte() >= sc.node.StartByte() && d.EndByte() <= sc.node.EndByte() &&
				d.Text(p.src) == name {
				return localSymbol(p, name, d), true
			}
		}
	}
	return Symbol{}, false
}

// localSymbol builds a Symbol for a local binding, widening the span to the
// binding's enclosing statement where possible.
func localSymbol(p *parsed, name string, def *gts.Node) Symbol {
	pos := def.StartPoint()
	start := int(def.StartByte())
	end := int(def.EndByte())
	if parent := def.Parent(); parent != nil {
		start = int(parent.StartByte())
		end = int(parent.EndByte())
	}
	return Symbol{
		ID:           FormatID(p.lang.Name, p.rel, name, int(pos.Row)+1),
		Name:         name,
		Kind:         "local",
		IsDefinition: true,
		File:         p.rel,
		Line:         int(pos.Row) + 1,
		Col:          int(pos.Column) + 1,
		StartByte:    start,
		EndByte:      end,
		Signature:    lineText(p.src, int(def.StartByte())),
	}
}
