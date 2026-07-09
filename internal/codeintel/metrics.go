package codeintel

import (
	"sort"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// FuncMetrics holds per-function AST metrics used by the code-health layer.
type FuncMetrics struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	File          string   `json:"file"`
	Class         string   `json:"class,omitempty"` // enclosing type; "" for a free function
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
	NLOC          int      `json:"nloc"`
	Cyclomatic    int      `json:"cyclomatic"`
	MaxNesting    int      `json:"max_nesting"`
	BoolOps       int      `json:"bool_ops"` // count of &&/||/and/or (for complex_conditional)
	ParamCount    int      `json:"param_count"`
	PrimitiveArgs int      `json:"primitive_args"`
	FieldAccess   []string `json:"field_access,omitempty"`
	ErrorPatterns []string `json:"error_patterns,omitempty"`
}

// FileMetrics is the per-file metrics result. Supported is false for languages
// without a metrics profile (their structural biomarkers then do not fire).
type FileMetrics struct {
	File      string        `json:"file"`
	Lang      string        `json:"lang"`
	Supported bool          `json:"supported"`
	Funcs     []FuncMetrics `json:"funcs,omitempty"`
}

// Metrics parses a file once and computes per-function AST metrics. It is
// deterministic and offline. Unsupported languages return Supported=false.
func (o *Ops) Metrics(file string) (FileMetrics, error) {
	openPath, err := o.confinedPath(file)
	if err != nil {
		return FileMetrics{File: file}, err
	}
	rel := o.rel(openPath)
	_, p, err := o.eng.Extract(openPath, rel)
	if err != nil {
		return FileMetrics{File: rel}, err
	}
	prof, ok := metricsProfiles[p.lang.Name]
	fm := FileMetrics{File: rel, Lang: p.lang.Name, Supported: ok}
	if !ok {
		return fm, nil
	}
	funcKinds := toSet(p.lang.Profile.FunctionKinds)
	walkForFunctions(p.tree.RootNode(), p, prof, funcKinds, &fm)
	return fm, nil
}

// walkForFunctions DFS-walks the tree, computing metrics for every function-kind
// node it finds (each independently — nested functions get their own record).
func walkForFunctions(n *gts.Node, p *parsed, prof metricsProfile, funcKinds map[string]bool, fm *FileMetrics) {
	if n == nil {
		return
	}
	if funcKinds[n.Type(p.lang.Grammar)] {
		fm.Funcs = append(fm.Funcs, computeFuncMetrics(n, p, prof))
	}
	for i := 0; i < n.ChildCount(); i++ {
		walkForFunctions(n.Child(i), p, prof, funcKinds, fm)
	}
}

// computeFuncMetrics walks a single function node's subtree (not descending into
// nested function nodes) accumulating complexity, nesting, params, field access,
// and error patterns.
func computeFuncMetrics(fn *gts.Node, p *parsed, prof metricsProfile) FuncMetrics {
	g := p.lang.Grammar
	pos := fn.StartPoint()
	name := ""
	if nn := fn.ChildByFieldName("name", g); nn != nil {
		name = nn.Text(p.src)
	}
	m := FuncMetrics{
		Name:       name,
		File:       p.rel,
		Class:      nearestContainer(fn, p.lang, p.src),
		StartLine:  int(pos.Row) + 1,
		EndLine:    int(fn.EndPoint().Row) + 1,
		NLOC:       nloc(p.src, int(fn.StartByte()), int(fn.EndByte())),
		Cyclomatic: 1,
	}
	if name != "" {
		m.ID = FormatID(p.lang.Name, p.rel, name, int(pos.Row)+1)
	}
	fields := map[string]bool{}
	errs := map[string]bool{}
	body := fn.ChildByFieldName("body", g)
	if body == nil {
		body = fn
	}
	accFunc := toSet(p.lang.Profile.FunctionKinds)
	walkFuncBody(body, p, prof, accFunc, 0, &m, fields, errs)
	m.FieldAccess = sortedSetKeys(fields)
	m.ErrorPatterns = sortedSetKeys(errs)
	m.ParamCount, m.PrimitiveArgs = paramStats(fn, p, prof)
	return m
}

// walkFuncBody accumulates metrics over one function body, stopping at nested
// function definitions (they are computed separately). depth tracks control-flow
// nesting.
func walkFuncBody(n *gts.Node, p *parsed, prof metricsProfile, funcKinds map[string]bool, depth int, m *FuncMetrics, fields, errs map[string]bool) {
	if n == nil {
		return
	}
	t := n.Type(p.lang.Grammar)
	if funcKinds[t] {
		return // a nested function is its own unit
	}
	if prof.control[t] || prof.caseKind[t] {
		m.Cyclomatic++
	}
	if prof.control[t] || prof.nestOnly[t] {
		depth++
		if depth > m.MaxNesting {
			m.MaxNesting = depth
		}
	}
	if prof.countBoolOp != nil && prof.countBoolOp(n, p) {
		m.Cyclomatic++
		m.BoolOps++
	}
	if prof.detectFields != nil {
		prof.detectFields(n, p, fields)
	}
	if prof.detectErrors != nil {
		prof.detectErrors(n, p, errs)
	}
	for i := 0; i < n.ChildCount(); i++ {
		walkFuncBody(n.Child(i), p, prof, funcKinds, depth, m, fields, errs)
	}
}

// paramStats counts parameters and primitive-typed parameters.
func paramStats(fn *gts.Node, p *parsed, prof metricsProfile) (count, primitive int) {
	if prof.paramsField == "" {
		return 0, 0
	}
	params := fn.ChildByFieldName(prof.paramsField, p.lang.Grammar)
	if params == nil {
		return 0, 0
	}
	for i := 0; i < params.NamedChildCount(); i++ {
		c := params.NamedChild(i)
		if c == nil || !prof.paramKind[c.Type(p.lang.Grammar)] {
			continue
		}
		count++
		if typeNode := c.ChildByFieldName("type", p.lang.Grammar); typeNode != nil {
			if prof.primitives[strings.TrimSpace(typeNode.Text(p.src))] {
				primitive++
			}
		}
	}
	return count, primitive
}

// nloc counts non-blank source lines in a byte span.
func nloc(src []byte, start, end int) int {
	if start < 0 || end > len(src) || start >= end {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(src[start:end]), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func sortedSetKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
