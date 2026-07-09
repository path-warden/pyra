package codegraph

import "sort"

// Cycles returns the strongly-connected components of the directed reference
// graph that contain a cycle — components of size greater than one, or a single
// node with a self-edge. Computed with an iterative Tarjan (no recursion depth
// limit). Deterministic: roots are visited in sorted Order, out-edges are sorted,
// members are sorted, and components are ordered by their smallest member.
func (g *Graph) Cycles() [][]string {
	var (
		index      int
		indices    = map[string]int{}
		lowlink    = map[string]int{}
		onStack    = map[string]bool{}
		tjStack    []string
		components [][]string
	)

	type frame struct {
		node     string
		childIdx int
	}

	for _, root := range g.Order {
		if _, seen := indices[root]; seen {
			continue
		}
		call := []*frame{{node: root}}
		for len(call) > 0 {
			fr := call[len(call)-1]
			v := fr.node
			if fr.childIdx == 0 {
				indices[v] = index
				lowlink[v] = index
				index++
				tjStack = append(tjStack, v)
				onStack[v] = true
			}
			out := g.Symbols[v].Out
			if fr.childIdx < len(out) {
				w := out[fr.childIdx]
				fr.childIdx++
				if _, seen := indices[w]; !seen {
					call = append(call, &frame{node: w})
				} else if onStack[w] && indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
				continue
			}
			// v's children are exhausted: it may root an SCC.
			if lowlink[v] == indices[v] {
				var comp []string
				for {
					w := tjStack[len(tjStack)-1]
					tjStack = tjStack[:len(tjStack)-1]
					onStack[w] = false
					comp = append(comp, w)
					if w == v {
						break
					}
				}
				sort.Strings(comp)
				components = append(components, comp)
			}
			call = call[:len(call)-1]
			if len(call) > 0 {
				parent := call[len(call)-1].node
				if lowlink[v] < lowlink[parent] {
					lowlink[parent] = lowlink[v]
				}
			}
		}
	}

	var cycles [][]string
	for _, comp := range components {
		if len(comp) > 1 {
			cycles = append(cycles, comp)
			continue
		}
		v := comp[0] // single-node component: report only if self-referential
		for _, w := range g.Symbols[v].Out {
			if w == v {
				cycles = append(cycles, comp)
				break
			}
		}
	}
	sort.Slice(cycles, func(i, j int) bool { return cycles[i][0] < cycles[j][0] })
	return cycles
}
