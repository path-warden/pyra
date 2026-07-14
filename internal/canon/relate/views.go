package relate

import (
	"sort"
	"strings"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
)

// Traversal/edge caps, ported from rac-core core/limits.py.
const (
	MaxRelatedEdges      = 1000
	MaxTraversalDepth    = 5
	MaxTraversalFrontier = 1000
	MaxTraversalWork     = 10000
)

// relationshipOrder ranks a snake_case relationship section in the canonical
// RELATIONSHIP_SECTIONS order (rac-core _RELATIONSHIP_ORDER).
var relationshipOrder = func() map[string]int {
	m := map[string]int{}
	for i, s := range relationshipSections {
		m[snake(s)] = i
	}
	return m
}()

func snake(section string) string { return strings.ReplaceAll(section, " ", "_") }

func relationshipRank(section string) int {
	if r, ok := relationshipOrder[section]; ok {
		return r
	}
	return len(relationshipOrder)
}

// Identity is an artifact's display identity (rac-core identity_by_path tuple).
type Identity struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

// IdentityByPath maps each entry's path to its display identity.
func IdentityByPath(entries []Entry) map[string]Identity {
	out := make(map[string]Identity, len(entries))
	for _, e := range entries {
		title := ""
		if e.Product != nil {
			title = e.Product.Title
		}
		out[e.Path] = Identity{ID: e.ID, Type: e.Type, Title: title}
	}
	return out
}

// resolutionIndex maps casefold(identifier) -> unique paths (canonical + aliases).
func resolutionIndex(entries []Entry) map[string][]string {
	idx := map[string][]string{}
	add := func(ident, path string) {
		if ident == "" {
			return
		}
		k := strings.ToLower(ident)
		for _, p := range idx[k] {
			if p == path {
				return
			}
		}
		idx[k] = append(idx[k], path)
	}
	for _, e := range entries {
		add(e.ID, e.Path)
		for _, a := range e.Aliases {
			add(a, e.Path)
		}
	}
	return idx
}

// Relationship is one declared cross-artifact reference with its resolution
// outcome (rac-core Relationship).
type Relationship struct {
	SourcePath   string `json:"source_path"`
	Relationship string `json:"relationship"` // snake_case section
	Target       string `json:"target"`       // raw reference text
	ResolvedPath string `json:"resolved_path,omitempty"`
	Issue        string `json:"issue,omitempty"` // stable code, or ""
	External     bool   `json:"external,omitempty"`
}

// Relationships returns every declared reference in the corpus with its
// resolution outcome, in deterministic order (sorted source path, the source
// type's spec.optional section order, reference declaration order).
func Relationships(entries []Entry) []Relationship {
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	reg := artifacts.Default()
	specBySection := map[string]EdgeSpec{}
	for _, s := range DefaultSpecs() {
		specBySection[s.Section] = s
	}
	relSet := map[string]bool{}
	for _, s := range relationshipSections {
		relSet[s] = true
	}
	idx := resolutionIndex(entries)

	var out []Relationship
	for _, e := range entries {
		if e.Type == artifacts.TypeUnknown {
			continue
		}
		for _, section := range reg[e.Type].Optional { // artifact's own schema order
			if !relSet[section] {
				continue
			}
			body, ok := e.Product.Section(section)
			if !ok {
				continue
			}
			edge := specBySection[section]
			for _, ref := range parseReferences(body) {
				rel := Relationship{SourcePath: e.Path, Relationship: snake(section), Target: ref, External: edge.External}
				if edge.External {
					out = append(out, rel) // external: never resolved, never "broken"
					continue
				}
				ps := idx[strings.ToLower(strings.TrimSpace(ref))]
				switch {
				case len(ps) == 0:
					rel.Issue = CodeTargetNotFound
				case len(ps) > 1:
					rel.Issue = CodeTargetAmbiguous
				case ps[0] == e.Path:
					rel.Issue = CodeSelfReference
				default:
					rel.ResolvedPath = ps[0]
				}
				out = append(out, rel)
			}
		}
	}
	return out
}

// InboundCounts returns {path -> count of resolved edges pointing at it}
// (rac-core inbound_counts_from_corpus): resolved, unique, non-self only.
func InboundCounts(entries []Entry) map[string]int {
	counts := map[string]int{}
	for _, rel := range Relationships(entries) {
		if rel.ResolvedPath != "" {
			counts[rel.ResolvedPath]++
		}
	}
	return counts
}

// OutgoingReferences groups a source's references by section, capped (rac-core).
type OutgoingReferences struct {
	BySection map[string][]string `json:"by_section"`
	Total     int                 `json:"total"`
}

// Outgoing returns the references sourcePath declares, grouped by snake_case
// section, capped at limit (<=0 uses MaxRelatedEdges).
func Outgoing(rels []Relationship, sourcePath string, limit int) OutgoingReferences {
	if limit <= 0 {
		limit = MaxRelatedEdges
	}
	out := OutgoingReferences{BySection: map[string][]string{}}
	kept := 0
	for _, rel := range rels {
		if rel.SourcePath != sourcePath {
			continue
		}
		out.Total++
		if kept < limit {
			out.BySection[rel.Relationship] = append(out.BySection[rel.Relationship], rel.Target)
			kept++
		}
	}
	return out
}

// IncomingReference is one artifact whose reference resolves to a target.
type IncomingReference struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title,omitempty"`
	Path    string `json:"path"`
	Section string `json:"section"`
	Target  string `json:"target"`
}

// IncomingReferences is the capped, ordered incoming edges plus the full count.
type IncomingReferences struct {
	Items []IncomingReference `json:"items"`
	Total int                 `json:"total"`
}

// Incoming returns artifacts whose references resolve uniquely to targetPath,
// ordered by relationship rank then id then path, capped at limit.
func Incoming(rels []Relationship, idByPath map[string]Identity, targetPath string, limit int) IncomingReferences {
	if limit <= 0 {
		limit = MaxRelatedEdges
	}
	var items []IncomingReference
	total := 0
	for _, rel := range rels {
		if rel.ResolvedPath != targetPath || rel.SourcePath == targetPath {
			continue
		}
		id, ok := idByPath[rel.SourcePath]
		if !ok {
			continue
		}
		total++
		if len(items) < limit {
			items = append(items, IncomingReference{
				ID: id.ID, Type: id.Type, Title: id.Title,
				Path: rel.SourcePath, Section: rel.Relationship, Target: rel.Target,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		ri, rj := relationshipRank(items[i].Section), relationshipRank(items[j].Section)
		if ri != rj {
			return ri < rj
		}
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		return items[i].Path < items[j].Path
	})
	return IncomingReferences{Items: items, Total: total}
}

// NeighborhoodNode is one artifact reachable from the origin, with hop distance.
type NeighborhoodNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
	Path  string `json:"path"`
	Hops  int    `json:"hops"`
}

// NeighborhoodResult is the bounded multi-hop neighbourhood of an origin.
type NeighborhoodResult struct {
	Nodes     []NeighborhoodNode `json:"nodes"`
	Truncated bool               `json:"truncated"`
}

// Neighborhood walks resolved edges (both directions) up to depth hops, bounded
// by the traversal caps (rac-core neighborhood). The origin is excluded; nodes
// are ordered by (hops, type, id).
func Neighborhood(rels []Relationship, idByPath map[string]Identity, originPath string, depth, maxFrontier, workBudget int) NeighborhoodResult {
	if maxFrontier <= 0 {
		maxFrontier = MaxTraversalFrontier
	}
	if workBudget <= 0 {
		workBudget = MaxTraversalWork
	}
	if depth < 0 {
		depth = 0
	}
	if depth > MaxTraversalDepth {
		depth = MaxTraversalDepth
	}

	// Undirected adjacency over resolved, non-self edges, with relationship rank.
	adjacency := map[string][]adjEdge{}
	for _, rel := range rels {
		if rel.ResolvedPath == "" || rel.SourcePath == rel.ResolvedPath {
			continue
		}
		rank := relationshipRank(rel.Relationship)
		adjacency[rel.SourcePath] = append(adjacency[rel.SourcePath], adjEdge{rel.ResolvedPath, rank})
		adjacency[rel.ResolvedPath] = append(adjacency[rel.ResolvedPath], adjEdge{rel.SourcePath, rank})
	}

	visited := map[string]bool{originPath: true}
	type disc struct {
		hops, rank int
		id, path   string
	}
	var discovered []disc
	frontier := []string{originPath}
	work := 0
	truncated := false

	for d := 1; d <= depth; d++ {
		var next []string
		for _, path := range sortedStrings(frontier) {
			for _, nb := range dedupSortedAdj(adjacency[path]) {
				work++
				if work > workBudget {
					truncated = true
					break
				}
				if visited[nb.path] {
					continue
				}
				visited[nb.path] = true
				id, ok := idByPath[nb.path]
				if !ok {
					continue
				}
				discovered = append(discovered, disc{hops: d, rank: nb.rank, id: id.ID, path: nb.path})
				if len(next) >= maxFrontier {
					truncated = true
				} else {
					next = append(next, nb.path)
				}
			}
			if truncated && work > workBudget {
				break
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}

	nodes := make([]NeighborhoodNode, 0, len(discovered))
	for _, d := range discovered {
		id := idByPath[d.path]
		nodes = append(nodes, NeighborhoodNode{ID: id.ID, Type: id.Type, Title: id.Title, Path: d.path, Hops: d.hops})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Hops != nodes[j].Hops {
			return nodes[i].Hops < nodes[j].Hops
		}
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return nodes[i].ID < nodes[j].ID
	})
	return NeighborhoodResult{Nodes: nodes, Truncated: truncated}
}

// adjEdge is one undirected adjacency entry (neighbor path + relationship rank).
type adjEdge struct {
	path string
	rank int
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func dedupSortedAdj(in []adjEdge) []adjEdge {
	seen := map[string]bool{}
	var out []adjEdge
	for _, a := range in {
		if !seen[a.path] {
			seen[a.path] = true
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return out[i].rank < out[j].rank
	})
	return out
}

// RelationshipSummary is repository-level relationship health (rac-core).
type RelationshipSummary struct {
	Total    int     `json:"total"`
	Valid    int     `json:"valid"`
	Broken   int     `json:"broken"`
	Orphaned int     `json:"orphaned"`
	Coverage float64 `json:"coverage"` // 0.0 – 1.0
}

// Summarize aggregates relationship health across the corpus (rac-core
// _summarize): broken counts unresolved references (not-found/ambiguous/self);
// orphaned counts known artifacts that are never a resolved target; coverage is
// the fraction of known artifacts declaring at least one relationship.
func Summarize(entries []Entry) RelationshipSummary {
	known := map[string]bool{}
	for _, e := range entries {
		if e.Type != artifacts.TypeUnknown {
			known[e.Path] = true
		}
	}
	if len(known) == 0 {
		return RelationshipSummary{Coverage: 1.0}
	}

	rels := Relationships(entries)
	checked, broken := 0, 0
	resolvedTargets := map[string]bool{}
	withRels := map[string]bool{}
	for _, rel := range rels {
		withRels[rel.SourcePath] = true
		if rel.External {
			continue // external refs are not resolution-checked
		}
		checked++
		if rel.Issue != "" {
			broken++
		} else if rel.ResolvedPath != "" {
			resolvedTargets[rel.ResolvedPath] = true
		}
	}

	orphaned := 0
	for p := range known {
		if !resolvedTargets[p] {
			orphaned++
		}
	}
	withCount := 0
	for p := range withRels {
		if known[p] {
			withCount++
		}
	}
	coverage := float64(withCount) / float64(len(known))

	return RelationshipSummary{
		Total:    checked,
		Valid:    checked - broken,
		Broken:   broken,
		Orphaned: orphaned,
		Coverage: round4(coverage),
	}
}

func round4(f float64) float64 {
	return float64(int(f*10000+0.5)) / 10000
}
