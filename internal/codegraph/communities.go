package codegraph

import "sort"

// lpMaxIter bounds label-propagation iterations (documented, not learned).
const lpMaxIter = 20

// Community is a set of symbol-ids grouped by label propagation.
type Community struct {
	ID      int      `json:"id"`
	Members []string `json:"members"`
}

// Communities partitions the symbol nodes into communities via deterministic
// label propagation on the undirected projection of the reference graph. Each
// node adopts the most frequent label among its neighbors (ties broken by the
// smallest label value, never map order); iteration is asynchronous in sorted
// Order and capped at lpMaxIter. Communities are renumbered by first appearance
// and their members are emitted in sorted order.
func (g *Graph) Communities() []Community {
	if len(g.Order) == 0 {
		return nil
	}

	adj := make(map[string][]string, len(g.Order))
	for _, id := range g.Order {
		set := map[string]bool{}
		for _, t := range g.Symbols[id].Out {
			if t != id {
				set[t] = true
			}
		}
		for _, s := range g.In[id] {
			if s != id {
				set[s] = true
			}
		}
		adj[id] = sortedSet(set)
	}

	label := make(map[string]string, len(g.Order))
	for _, id := range g.Order {
		label[id] = id
	}

	// Synchronous label propagation with self-inclusion: each node votes with its
	// own current label plus its neighbors', and all nodes update from the same
	// snapshot. This avoids the in-iteration cascade that collapses asynchronous
	// LPA into one monster community, while staying fully deterministic (a fixed
	// snapshot + smallest-label tiebreak).
	for iter := 0; iter < lpMaxIter; iter++ {
		next := make(map[string]string, len(g.Order))
		changed := false
		for _, id := range g.Order {
			counts := map[string]int{label[id]: 1} // self vote
			for _, nb := range adj[id] {
				counts[label[nb]]++
			}
			best, bestCount := "", -1
			for _, lbl := range sortedIntKeys(counts) {
				if counts[lbl] > bestCount { // sorted ⇒ ties resolve to smallest label
					bestCount, best = counts[lbl], lbl
				}
			}
			next[id] = best
			if best != label[id] {
				changed = true
			}
		}
		label = next
		if !changed {
			break
		}
	}

	renum := map[string]int{}
	var communities []Community
	for _, id := range g.Order {
		lbl := label[id]
		idx, ok := renum[lbl]
		if !ok {
			idx = len(communities)
			renum[lbl] = idx
			communities = append(communities, Community{ID: idx})
		}
		communities[idx].Members = append(communities[idx].Members, id)
	}
	return communities
}

// sortedIntKeys returns the keys of a string-int map in ascending key order.
func sortedIntKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
