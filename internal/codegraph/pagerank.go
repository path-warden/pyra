package codegraph

import (
	"math"
	"sort"
)

// PageRank parameters (documented, not learned).
const (
	prDamping = 0.85
	prMaxIter = 100
	prTol     = 1e-6
)

// Centrality is one symbol's PageRank score.
type Centrality struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

// PageRank computes a PageRank score for every symbol node via power iteration,
// distributing each node's rank across its reference edges. Dangling nodes (no
// out-edges) redistribute their rank uniformly. Deterministic: nodes are visited
// in sorted Order, so the floating-point summation order is fixed. Returns nodes
// ranked by score descending, tie-broken by symbol-id.
func (g *Graph) PageRank() []Centrality {
	n := len(g.Order)
	if n == 0 {
		return nil
	}
	nf := float64(n)
	rank := make(map[string]float64, n)
	for _, id := range g.Order {
		rank[id] = 1.0 / nf
	}

	for iter := 0; iter < prMaxIter; iter++ {
		var dangling float64
		for _, id := range g.Order {
			if len(g.Symbols[id].Out) == 0 {
				dangling += rank[id]
			}
		}
		base := (1-prDamping)/nf + prDamping*dangling/nf
		next := make(map[string]float64, n)
		for _, id := range g.Order {
			next[id] = base
		}
		for _, id := range g.Order {
			out := g.Symbols[id].Out
			if len(out) == 0 {
				continue
			}
			share := prDamping * rank[id] / float64(len(out))
			for _, t := range out {
				next[t] += share
			}
		}
		var delta float64
		for _, id := range g.Order {
			delta += math.Abs(next[id] - rank[id])
		}
		rank = next
		if delta < prTol {
			break
		}
	}

	res := make([]Centrality, 0, n)
	for _, id := range g.Order {
		res = append(res, Centrality{ID: id, Score: rank[id]})
	}
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].Score != res[j].Score {
			return res[i].Score > res[j].Score
		}
		return res[i].ID < res[j].ID
	})
	return res
}

// TopCentral returns the top-N most central symbols (all when limit <= 0).
func (g *Graph) TopCentral(limit int) []Centrality {
	c := g.PageRank()
	if limit > 0 && len(c) > limit {
		c = c[:limit]
	}
	return c
}
