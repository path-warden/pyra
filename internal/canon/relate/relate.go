// Package relate builds and validates the typed relationship graph over Canon
// artifacts, ported from rac-core's services/relationships.py and
// core/relationship_types.py.
//
// References are resolved against an alias index (canonical identifier plus
// legacy aliases). Each reference is checked for referential integrity
// (not-found / ambiguous / self-reference), each edge for legality (the section
// must be declared in the source type's spec.optional), range (the resolved
// target's type must be in the edge's range), status-consistency (a live source
// must not reference a retired target except via supersedes), and acyclicity
// (no cycle in a directional acyclic edge kind). Issue codes and severities match
// rac-core's JSON contract.
package relate

import (
	"regexp"
	"sort"
	"strings"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/model"
)

// Stable issue codes (part of the JSON contract, ported from rac-core).
const (
	CodeDuplicateIdentifier = "duplicate-artifact-identifier"
	CodeTargetNotFound      = "relationship-target-not-found"
	CodeTargetAmbiguous     = "relationship-target-ambiguous"
	CodeSelfReference       = "relationship-self-reference"
	CodeEdgeUnsupported     = "relationship-edge-unsupported"
	CodeTargetSuperseded    = "relationship-target-superseded"
	CodeTargetTypeMismatch  = "relationship-target-type-mismatch"
	CodeRelationshipCycle   = "relationship-cycle"
)

// relationshipSeverity is the intrinsic severity per finding (rac-core
// RELATIONSHIP_SEVERITY).
var relationshipSeverity = map[string]string{
	CodeTargetNotFound:      model.SeverityError,
	CodeTargetAmbiguous:     model.SeverityError,
	CodeTargetTypeMismatch:  model.SeverityError,
	CodeRelationshipCycle:   model.SeverityError,
	CodeDuplicateIdentifier: model.SeverityError,
	CodeTargetSuperseded:    model.SeverityWarning,
	CodeSelfReference:       model.SeverityWarning,
	CodeEdgeUnsupported:     model.SeverityWarning,
}

// relationshipSections is the canonical relationship-section vocabulary and order
// (rac-core RELATIONSHIP_SECTIONS), normalized space-form.
var relationshipSections = []string{
	"related requirements", "related decisions", "related roadmaps",
	"related prompts", "related designs", "supersedes", "related tickets",
}

// EdgeSpec is the graph schema of one relationship kind (rac-core EdgeSpec).
type EdgeSpec struct {
	Section             string   // normalized space-form heading
	Kind                string   // snake_case edge key (issue "relationship" field)
	Range               []string // legal target artifact types (empty = none/external)
	Acyclic             bool
	ForbidsTargetStatus bool
	External            bool
}

// Edge is a resolved relationship between artifact IDs.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Entry is a Canon artifact participating in the graph.
type Entry struct {
	ID      string
	Type    string
	Status  string
	Retired bool
	Path    string
	Aliases []string
	Product *model.Product
}

// Graph holds the resolved relationship edges (keyed by artifact ID).
type Graph struct {
	Edges   map[string][]Edge
	Inbound map[string][]Edge
}

// DefaultSpecs returns the built-in edge specs (rac-core REGISTRY).
func DefaultSpecs() []EdgeSpec {
	rel := func(t string) EdgeSpec {
		return EdgeSpec{Section: "related " + t + "s", Kind: "related_" + t + "s", Range: []string{t}, ForbidsTargetStatus: true}
	}
	return []EdgeSpec{
		rel("requirement"), rel("decision"), rel("roadmap"), rel("prompt"), rel("design"),
		{Section: "supersedes", Kind: "supersedes", Range: []string{"decision"}, Acyclic: true, ForbidsTargetStatus: false},
		{Section: "related tickets", Kind: "related_tickets", External: true},
	}
}

var listMarkerRe = regexp.MustCompile(`^(?:[-*+]|\d+\.)\s+`)

// parseReferences splits a relationship section body into reference strings: one
// per non-empty line, with a single leading list marker stripped, otherwise
// verbatim (rac-core parse_references — the whole line is the reference).
func parseReferences(body string) []string {
	var refs []string
	for _, line := range strings.Split(body, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		refs = append(refs, strings.TrimSpace(listMarkerRe.ReplaceAllString(s, "")))
	}
	return refs
}

// Build resolves edges and returns the graph and integrity issues, in rac-core's
// deterministic order: duplicate identifiers, edge-unsupported, range,
// status-consistency, cycles, then referential integrity.
func Build(entries []Entry, specs []EdgeSpec) (*Graph, []model.Issue) {
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	specBySection := map[string]EdgeSpec{}
	for _, s := range specs {
		specBySection[s.Section] = s
	}
	reg := artifacts.Default()
	byPath := map[string]Entry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}

	g := &Graph{Edges: map[string][]Edge{}, Inbound: map[string][]Edge{}}

	// Canonical-identifier index (one entry per file) for duplicate detection.
	canonIdx := map[string][]Entry{}
	for _, e := range entries {
		k := strings.ToLower(e.ID)
		canonIdx[k] = append(canonIdx[k], e)
	}
	// Resolution index: canonical + alias identifiers -> unique paths.
	resIdx := map[string][]string{}
	addRes := func(ident, path string) {
		if ident == "" {
			return
		}
		k := strings.ToLower(ident)
		for _, p := range resIdx[k] {
			if p == path {
				return
			}
		}
		resIdx[k] = append(resIdx[k], path)
	}
	for _, e := range entries {
		addRes(e.ID, e.Path)
		for _, a := range e.Aliases {
			addRes(a, e.Path)
		}
	}

	resolveUnique := func(ref, srcPath string) (string, string) {
		ps := resIdx[strings.ToLower(strings.TrimSpace(ref))]
		switch {
		case len(ps) == 0:
			return "", CodeTargetNotFound
		case len(ps) > 1:
			return "", CodeTargetAmbiguous
		case ps[0] == srcPath:
			return "", CodeSelfReference
		default:
			return ps[0], ""
		}
	}

	var issues []model.Issue

	// (a) Duplicate identifiers, sorted by identifier.
	var dups []Entry
	for _, group := range canonIdx {
		if len(group) > 1 {
			best := group[0]
			for _, e := range group {
				if e.Path < best.Path {
					best = e
				}
			}
			dups = append(dups, best)
		}
	}
	sort.Slice(dups, func(i, j int) bool { return strings.ToLower(dups[i].ID) < strings.ToLower(dups[j].ID) })
	for _, d := range dups {
		var paths []string
		for _, e := range canonIdx[strings.ToLower(d.ID)] {
			paths = append(paths, e.Path)
		}
		sort.Strings(paths)
		issues = append(issues, relIssue(CodeDuplicateIdentifier,
			"artifact identifier '"+d.ID+"' is used by multiple files: "+strings.Join(paths, ", "), paths[0]))
	}

	// (b) Edge-legality: a relationship section present+populated but not declared
	// in the source type's spec.optional.
	for _, e := range entries {
		if e.Type == artifacts.TypeUnknown {
			continue
		}
		optional := reg[e.Type].Optional
		for _, section := range relationshipSections {
			if contains(optional, section) {
				continue
			}
			if body, ok := e.Product.Section(section); ok && len(parseReferences(body)) > 0 {
				issues = append(issues, relIssue(CodeEdgeUnsupported,
					"relationship section '"+section+"' is not supported by type "+e.Type, e.Path))
			}
		}
	}

	// Iterate edges declared in spec.optional (the legal ones) for the graph rules.
	var legal []declaredEdge
	for _, e := range entries {
		if e.Type == artifacts.TypeUnknown {
			continue
		}
		optional := reg[e.Type].Optional
		for _, section := range relationshipSections {
			if !contains(optional, section) {
				continue
			}
			spec, ok := specBySection[section]
			if !ok {
				continue
			}
			body, ok := e.Product.Section(section)
			if !ok {
				continue
			}
			refs := parseReferences(body)
			if len(refs) > 0 {
				legal = append(legal, declaredEdge{entry: e, spec: spec, refs: refs})
			}
		}
	}

	// (c) Range: a uniquely-resolved target whose type is not in the edge range.
	for _, d := range legal {
		if d.spec.External {
			continue
		}
		for _, ref := range d.refs {
			target, code := resolveUnique(ref, d.entry.Path)
			if code != "" {
				continue // owned by referential integrity
			}
			tgt := byPath[target]
			if tgt.Type == artifacts.TypeUnknown {
				continue // untyped target is not a range violation
			}
			if !contains(d.spec.Range, tgt.Type) {
				issues = append(issues, relIssue(CodeTargetTypeMismatch,
					"relationship '"+d.spec.Kind+"' cannot target type "+tgt.Type+" ("+ref+")", d.entry.Path))
			}
		}
	}

	// (d) Status-consistency: live source -> retired target (non-supersedes).
	for _, d := range legal {
		if d.spec.External || !d.spec.ForbidsTargetStatus || d.entry.Retired {
			continue
		}
		for _, ref := range d.refs {
			target, code := resolveUnique(ref, d.entry.Path)
			if code != "" {
				continue
			}
			if byPath[target].Retired {
				issues = append(issues, relIssue(CodeTargetSuperseded,
					"live artifact references retired target "+ref, d.entry.Path))
			}
		}
	}

	// (e) Acyclicity: cycles in directional acyclic edge kinds (supersedes).
	issues = append(issues, cycleIssues(entries, legal, resolveUnique)...)

	// (f) Referential integrity + build the resolved graph.
	for _, d := range legal {
		if d.spec.External {
			continue
		}
		for _, ref := range d.refs {
			target, code := resolveUnique(ref, d.entry.Path)
			if code != "" {
				issues = append(issues, relIssue(code, referentialMessage(code, ref), d.entry.Path))
				continue
			}
			edge := Edge{From: d.entry.ID, To: byPath[target].ID, Kind: d.spec.Kind}
			g.Edges[d.entry.ID] = append(g.Edges[d.entry.ID], edge)
			g.Inbound[byPath[target].ID] = append(g.Inbound[byPath[target].ID], edge)
		}
	}

	return g, issues
}

// declaredEdge is one artifact's populated, legal relationship section.
type declaredEdge struct {
	entry Entry
	spec  EdgeSpec
	refs  []string
}

func cycleIssues(entries []Entry, legal []declaredEdge, resolve func(string, string) (string, string)) []model.Issue {
	// Build adjacency (by path) per acyclic kind from uniquely-resolved non-self edges.
	kinds := map[string]bool{}
	adjByKind := map[string]map[string][]string{}
	for _, d := range legal {
		if !d.spec.Acyclic {
			continue
		}
		kinds[d.spec.Kind] = true
		if adjByKind[d.spec.Kind] == nil {
			adjByKind[d.spec.Kind] = map[string][]string{}
		}
		set := map[string]bool{}
		for _, ref := range d.refs {
			target, code := resolve(ref, d.entry.Path)
			if code == "" && !set[target] {
				set[target] = true
				adjByKind[d.spec.Kind][d.entry.Path] = append(adjByKind[d.spec.Kind][d.entry.Path], target)
			}
		}
	}
	var sortedKinds []string
	for k := range kinds {
		sortedKinds = append(sortedKinds, k)
	}
	sort.Strings(sortedKinds)

	var issues []model.Issue
	for _, kind := range sortedKinds {
		for _, comp := range cyclicComponents(adjByKind[kind]) {
			issues = append(issues, model.Issue{
				Severity: relationshipSeverity[CodeRelationshipCycle], Code: CodeRelationshipCycle,
				Message: "relationship cycle in '" + kind + "': " + strings.Join(comp, ", "),
				Path:    comp[0],
			})
		}
	}
	return issues
}

// cyclicComponents returns strongly-connected components of size > 1 (Tarjan),
// each a sorted node list; components sorted by first node.
func cyclicComponents(adjacency map[string][]string) [][]string {
	nodeset := map[string]bool{}
	for v, ts := range adjacency {
		nodeset[v] = true
		for _, t := range ts {
			nodeset[t] = true
		}
	}
	var nodes []string
	for n := range nodeset {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	indices := map[string]int{}
	lowlink := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	counter := 0
	var components [][]string

	var strongconnect func(string)
	strongconnect = func(v string) {
		indices[v] = counter
		lowlink[v] = counter
		counter++
		stack = append(stack, v)
		onStack[v] = true
		neighbors := append([]string(nil), adjacency[v]...)
		sort.Strings(neighbors)
		for _, w := range neighbors {
			if _, seen := indices[w]; !seen {
				strongconnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}
		if lowlink[v] == indices[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			if len(comp) > 1 {
				sort.Strings(comp)
				components = append(components, comp)
			}
		}
	}
	for _, n := range nodes {
		if _, seen := indices[n]; !seen {
			strongconnect(n)
		}
	}
	sort.Slice(components, func(i, j int) bool { return components[i][0] < components[j][0] })
	return components
}

// Neighborhood returns edges reachable from an artifact ID within depth hops
// (both directions), de-duplicated.
func (g *Graph) Neighborhood(id string, depth int) []Edge {
	var out []Edge
	seen := map[string]bool{}
	frontier := []string{id}
	for d := 0; d < depth; d++ {
		var next []string
		for _, node := range frontier {
			for _, e := range g.Edges[node] {
				key := e.From + "\x00" + e.To + "\x00" + e.Kind
				if !seen[key] {
					seen[key] = true
					out = append(out, e)
					next = append(next, e.To)
				}
			}
			for _, e := range g.Inbound[node] {
				key := e.From + "\x00" + e.To + "\x00" + e.Kind
				if !seen[key] {
					seen[key] = true
					out = append(out, e)
					next = append(next, e.From)
				}
			}
		}
		frontier = next
	}
	return out
}

// InboundCounts returns the number of resolved inbound edges per artifact ID.
func (g *Graph) InboundCounts() map[string]int {
	counts := map[string]int{}
	for id, edges := range g.Inbound {
		counts[id] = len(edges)
	}
	return counts
}

func referentialMessage(code, ref string) string {
	switch code {
	case CodeTargetNotFound:
		return "relationship target not found: " + ref
	case CodeTargetAmbiguous:
		return "relationship target is ambiguous: " + ref
	case CodeSelfReference:
		return "artifact references itself: " + ref
	default:
		return ref
	}
}

func contains(items []string, want string) bool {
	for _, i := range items {
		if i == want {
			return true
		}
	}
	return false
}

func relIssue(code, msg, path string) model.Issue {
	return model.Issue{Severity: relationshipSeverity[code], Code: code, Message: msg, Path: path}
}
