package codegraph

// Reachability is the split of symbol nodes into those reachable from the entry
// points and the unreachable remainder (which a later dead-code capability
// consumes), plus the entry-point set itself.
type Reachability struct {
	EntryPoints []string `json:"entry_points"`
	Reachable   []string `json:"reachable"`
	Unreachable []string `json:"unreachable"`
}

// Reachability computes the set of symbols reachable via reference edges from the
// entry points (symbols named "main" or exported/public). Deterministic: entry
// points and result lists are built in sorted Order. With no entry points the
// reachable set is empty and every node is unreachable.
func (g *Graph) Reachability() Reachability {
	var roots []string
	reached := map[string]bool{}
	for _, id := range g.Order {
		if isEntryPoint(g.Symbols[id]) {
			roots = append(roots, id)
			reached[id] = true
		}
	}

	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, w := range g.Symbols[v].Out {
			if !reached[w] {
				reached[w] = true
				queue = append(queue, w)
			}
		}
	}

	res := Reachability{EntryPoints: roots}
	for _, id := range g.Order {
		if reached[id] {
			res.Reachable = append(res.Reachable, id)
		} else {
			res.Unreachable = append(res.Unreachable, id)
		}
	}
	return res
}
