// Package retrieval implements the unified agent retrieval loop:
// discover (fuzzy full-text over both tiers) -> ground (resolve Canon hits to
// authoritative, status-checked artifacts, following supersedes to the
// successor) -> assemble (pack into a token budget, Canon-first).
//
// Reference content is compressed freely to fit the budget. A normative
// requirement statement ([REQ-NNN] line) is never compressed below fidelity: it
// is emitted verbatim or the artifact is deferred to SuggestedFollowup.
package retrieval

import (
	"regexp"
	"sort"
	"strings"

	"github.com/chasedputnam/memphis/internal/compress"
	"github.com/chasedputnam/memphis/internal/store"
	"github.com/chasedputnam/memphis/internal/tokens"
)

const (
	defaultBudget = 4000
	defaultLimit  = 20
)

var reqLineRe = regexp.MustCompile(`\[REQ-\d+\]`)

// Options configures assembly.
type Options struct {
	TokenBudget int
	Limit       int
	Compression compress.Level
}

func (o Options) withDefaults() Options {
	if o.TokenBudget <= 0 {
		o.TokenBudget = defaultBudget
	}
	if o.Limit <= 0 {
		o.Limit = defaultLimit
	}
	if o.Compression == "" {
		o.Compression = compress.LevelLight
	}
	return o
}

// Citation is the verifiable grounding for a Canon item.
type Citation struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Type   string `json:"type"`
	Status string `json:"status,omitempty"`
}

// Packed is one assembled, budgeted item.
type Packed struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Type     string    `json:"type"`
	Tier     string    `json:"tier"`
	Score    float64   `json:"score"`
	Body     string    `json:"body"`
	Tokens   int       `json:"tokens"`
	Citation *Citation `json:"citation,omitempty"`
}

// Result is the assembled context.
type Result struct {
	Query             string   `json:"query"`
	Items             []Packed `json:"items"`
	SuggestedFollowup []string `json:"suggested_followup,omitempty"`
	TotalTokens       int      `json:"total_tokens"`
}

// Assemble runs discover -> ground -> assemble over the store.
func Assemble(s *store.Store, query string, opts Options) Result {
	opts = opts.withDefaults()
	est := tokens.NewEstimator()

	hits := s.Discover(query, opts.Limit)
	hits = ground(s, hits)
	rank(hits)

	res := Result{Query: query}
	remaining := opts.TokenBudget

	for _, h := range hits {
		body, toks, ok := fit(h.Item, remaining, est, opts.Compression)
		if !ok {
			res.SuggestedFollowup = append(res.SuggestedFollowup, h.Item.ID)
			continue
		}
		p := Packed{
			ID: h.Item.ID, Title: h.Item.Title, Type: h.Item.Type,
			Tier: h.Item.Tier.String(), Score: h.Score, Body: body, Tokens: toks,
		}
		if h.Item.Tier == store.TierCanon {
			p.Citation = &Citation{ID: h.Item.ID, Path: h.Item.Path, Type: h.Item.Type, Status: h.Item.Status}
		}
		res.Items = append(res.Items, p)
		remaining -= toks
		res.TotalTokens += toks
	}
	return res
}

// ground resolves superseded Canon hits to their successor and de-duplicates.
func ground(s *store.Store, hits []store.Hit) []store.Hit {
	seen := map[string]bool{}
	out := make([]store.Hit, 0, len(hits))
	for _, h := range hits {
		item := h.Item
		if item.Tier == store.TierCanon {
			if succ := s.Successor(item.ID); succ != nil {
				item = succ
			}
		}
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, store.Hit{Item: item, Score: h.Score})
	}
	return out
}

// rank applies authority-first ordering: Canon outranks Reference whenever both
// match (tier is the primary key), and within a tier the more relevant item
// wins (score descending). This reflects the store's authority-first primacy —
// the agent sees authoritative artifacts before supporting reference material.
// The sort is stable, preserving discovery order on full ties.
func rank(hits []store.Hit) {
	sort.SliceStable(hits, func(i, j int) bool {
		ri, rj := tierRank(hits[i].Item.Tier), tierRank(hits[j].Item.Tier)
		if ri != rj {
			return ri < rj
		}
		return hits[i].Score > hits[j].Score
	})
}

func tierRank(t store.Tier) int {
	if t == store.TierCanon {
		return 0
	}
	return 1
}

// fit produces a budgeted body for an item. Requirement statements are never
// truncated; if they do not fit, the item is deferred (ok=false).
func fit(item *store.Item, remaining int, est *tokens.Estimator, level compress.Level) (string, int, bool) {
	if remaining <= 0 {
		return "", 0, false
	}
	full := item.Body
	if est.Count(full) <= remaining {
		return full, est.Count(full), true
	}

	reqLines := extractReqLines(full)
	if item.Tier == store.TierCanon && len(reqLines) > 0 {
		// Preserve requirement statements verbatim; drop surrounding prose if
		// necessary. Never truncate the requirement text itself.
		joined := strings.Join(reqLines, "\n")
		if c := est.Count(joined); c <= remaining {
			return joined, c, true
		}
		return "", 0, false
	}

	// Reference (or non-requirement Canon prose): compress, then hard-truncate.
	res := compress.Compress(full, compress.Options{Level: level, TokenBudget: remaining})
	body := res.Content
	if c := est.Count(body); c <= remaining {
		return body, c, true
	}
	truncated, _ := est.Truncate(body, remaining)
	if c := est.Count(truncated); c <= remaining && strings.TrimSpace(truncated) != "" {
		return truncated, c, true
	}
	return "", 0, false
}

func extractReqLines(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if reqLineRe.MatchString(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}
