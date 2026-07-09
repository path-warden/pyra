package codegraph

import "sort"

// graphFromEdges builds a Graph directly from a directed adjacency map, for
// precise testing of the graph algorithms without going through codeintel. Node
// names double as symbol-ids. In-edges and Order are derived deterministically.
func graphFromEdges(edges map[string][]string) *Graph {
	g := &Graph{
		Symbols:   map[string]*SymbolNode{},
		In:        map[string][]string{},
		Files:     map[string][]string{},
		FileEdges: map[string][]string{},
	}
	ensure := func(id string) {
		if g.Symbols[id] == nil {
			g.Symbols[id] = &SymbolNode{ID: id, Name: id, File: id + ".go"}
		}
	}
	for src, outs := range edges {
		ensure(src)
		set := map[string]bool{}
		for _, t := range outs {
			ensure(t)
			set[t] = true
		}
		g.Symbols[src].Out = sortedSet(set)
		for _, t := range g.Symbols[src].Out {
			g.In[t] = append(g.In[t], src)
		}
	}
	for id := range g.Symbols {
		g.Order = append(g.Order, id)
	}
	sort.Strings(g.Order)
	for t := range g.In {
		sort.Strings(g.In[t])
	}
	return g
}

// communityOf returns the community id that contains member.
func communityOf(cs []Community, member string) int {
	for _, c := range cs {
		for _, m := range c.Members {
			if m == member {
				return c.ID
			}
		}
	}
	return -1
}
