// Package codegraph builds a persistent, in-memory two-tier code graph from one
// codeintel walk of the code roots, and runs standard whole-graph analyses:
// PageRank centrality, label-propagation communities, Tarjan strongly-connected
// components (cycles), and entry-point reachability.
//
// It lives outside internal/canon/... (like internal/codeintel, on which it
// depends); the authority path may not import it (a boundary test enforces this).
// It is an independent implementation — standard algorithms, no external graph
// library, no learned constants — and every analysis iterates nodes and neighbors
// in sorted symbol-id order with fixed iteration caps, so identical repository
// state yields byte-identical output.
package codegraph

// SymbolNode is one definition in the graph, keyed by its stable symbol-id.
type SymbolNode struct {
	ID       string   `json:"id"` // "<lang>:<rel>#<name>@<line>"
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	File     string   `json:"file"` // repo-relative
	Lang     string   `json:"lang"`
	Parent   string   `json:"parent,omitempty"` // enclosing def name, "" if top-level
	Exported bool     `json:"exported"`
	Out      []string `json:"out,omitempty"` // reference edges → target symbol-ids (sorted, deduped)
}

// Options tunes a graph build.
type Options struct {
	Scope   string // optional subdirectory to restrict the graph
	NodeCap int    // 0 = no cap
}

// Graph is the built two-tier code graph.
type Graph struct {
	Symbols   map[string]*SymbolNode `json:"symbols"`
	Order     []string               `json:"order"`      // all symbol-ids, sorted (canonical iteration)
	In        map[string][]string    `json:"-"`          // reverse reference edges, sorted
	Files     map[string][]string    `json:"files"`      // file → symbol-ids it defines
	FileEdges map[string][]string    `json:"file_edges"` // file → files it depends on
	Truncated bool                   `json:"truncated"`
}

// NodeCount is the number of symbol nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.Order) }
