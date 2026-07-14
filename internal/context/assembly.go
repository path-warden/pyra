// Package context provides intelligent context assembly for MCP tools.
package context

import (
	"sort"

	"github.com/chasedputnam/pyra/internal/compress"
	"github.com/chasedputnam/pyra/internal/search"
	"github.com/chasedputnam/pyra/internal/tokens"
)

// AssemblyOptions configures context assembly.
type AssemblyOptions struct {
	Query            string
	TokenBudget      int
	IncludeNeighbors bool
	Depth            int
	Compression      compress.Level
}

// ConceptSnippet is a concept with potentially compressed content.
type ConceptSnippet struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Score       float64  `json:"score"`
	TokenCount  int      `json:"token_count"`
	Truncated   bool     `json:"truncated,omitempty"`
}

// AssembledContext is the result of intelligent context gathering.
type AssembledContext struct {
	Concepts          []ConceptSnippet `json:"concepts"`
	TotalTokens       int              `json:"total_tokens"`
	Truncated         bool             `json:"truncated"`
	SuggestedFollowup []string         `json:"suggested_followup,omitempty"`
}

// Assemble gathers relevant context within token budget.
func Assemble(bundleSearch *search.BundleSearch, opts AssemblyOptions) AssembledContext {
	est := tokens.NewEstimator()

	// Default budget
	budget := opts.TokenBudget
	if budget <= 0 {
		budget = 4000
	}

	// Default depth
	depth := opts.Depth
	if depth <= 0 {
		depth = 1
	}
	if depth > 2 {
		depth = 2
	}

	// Search for relevant concepts
	searchResults := bundleSearch.Search(opts.Query, search.SearchOptions{
		Limit: 20,
	})

	// Collect candidate concepts with scores
	candidates := make(map[string]float64)
	for _, r := range searchResults {
		candidates[r.ID] = r.Score
	}

	// Add neighbors if requested
	if opts.IncludeNeighbors && len(searchResults) > 0 {
		for i, r := range searchResults {
			if i >= 3 { // Only get neighbors for top 3
				break
			}
			neighbors := getNeighbors(bundleSearch, r.ID, depth)
			for _, nID := range neighbors {
				if _, exists := candidates[nID]; !exists {
					// Score neighbors lower than direct matches
					candidates[nID] = r.Score * 0.5
				}
			}
		}
	}

	// Sort candidates by score
	type scored struct {
		id    string
		score float64
	}
	var sorted []scored
	for id, score := range candidates {
		sorted = append(sorted, scored{id, score})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	// Assemble context within budget
	var snippets []ConceptSnippet
	var suggested []string
	totalTokens := 0

	for _, s := range sorted {
		concept := bundleSearch.Graph.Concepts[s.id]
		if concept == nil {
			continue
		}

		// Build snippet
		snippet := ConceptSnippet{
			ID:          concept.ID,
			Title:       concept.Title,
			Type:        concept.Type,
			Description: concept.Description,
			Tags:        concept.Tags,
			Score:       s.score,
		}

		// Estimate base tokens (without content)
		baseTokens := est.CountJSON(snippet)

		// Calculate remaining budget for content
		contentBudget := budget - totalTokens - baseTokens - 50 // Reserve 50 for overhead

		if contentBudget <= 0 {
			// No room for this concept, suggest for followup
			suggested = append(suggested, concept.ID)
			continue
		}

		// Compress/truncate content to fit
		content := concept.Body
		truncated := false

		if opts.Compression != compress.LevelNone {
			compResult := compress.Compress(content, compress.Options{
				Level:               opts.Compression,
				TokenBudget:         contentBudget,
				PreserveFrontmatter: false,
			})
			content = compResult.Content
			truncated = compResult.Truncated
		} else {
			// Just truncate if needed
			truncResult := compress.Truncate(content, compress.TruncateOptions{
				TokenBudget:  contentBudget,
				AddIndicator: false,
			})
			content = truncResult.Content
			truncated = truncResult.Truncated
		}

		snippet.Content = content
		snippet.Truncated = truncated
		snippet.TokenCount = est.Count(content)

		// Check if we can fit this snippet
		snippetTokens := est.CountJSON(snippet)
		if totalTokens+snippetTokens > budget {
			suggested = append(suggested, concept.ID)
			continue
		}

		snippets = append(snippets, snippet)
		totalTokens += snippetTokens
	}

	return AssembledContext{
		Concepts:          snippets,
		TotalTokens:       totalTokens,
		Truncated:         len(suggested) > 0,
		SuggestedFollowup: suggested,
	}
}

// getNeighbors returns IDs of concepts linked to/from the given concept.
func getNeighbors(bundleSearch *search.BundleSearch, id string, depth int) []string {
	seen := make(map[string]bool)
	frontier := []string{id}
	seen[id] = true

	for d := 0; d < depth; d++ {
		var next []string
		for _, nodeID := range frontier {
			// Outbound links
			for _, to := range bundleSearch.Graph.Outbound[nodeID] {
				if !seen[to] {
					seen[to] = true
					next = append(next, to)
				}
			}
			// Backlinks
			for _, from := range bundleSearch.Graph.Backlinks[nodeID] {
				if !seen[from] {
					seen[from] = true
					next = append(next, from)
				}
			}
		}
		frontier = next
	}

	// Return all seen except the original
	var neighbors []string
	for nID := range seen {
		if nID != id {
			neighbors = append(neighbors, nID)
		}
	}
	return neighbors
}

// DefaultOptions returns sensible default assembly options.
func DefaultOptions() AssemblyOptions {
	return AssemblyOptions{
		TokenBudget:      4000,
		IncludeNeighbors: true,
		Depth:            1,
		Compression:      compress.LevelLight,
	}
}
