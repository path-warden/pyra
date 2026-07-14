// Package search provides full-text search over OKF bundles.
package search

import (
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/chasedputnam/pyra/internal/graph"
	"github.com/chasedputnam/pyra/internal/reader"
	"github.com/chasedputnam/pyra/internal/types"
)

// SearchOptions configures search behavior.
type SearchOptions struct {
	Type  string
	Tags  []string
	Limit int
}

// searchDoc is the document indexed by Bleve.
type searchDoc struct {
	ID          string
	Title       string
	Type        string
	Description string
	Tags        string
	Body        string
}

// BundleSearch provides search over a bundle.
type BundleSearch struct {
	Graph *types.KnowledgeGraph
	index bleve.Index
}

// NewBundleSearch creates a new search index from a bundle directory.
func NewBundleSearch(bundleDir string) (*BundleSearch, error) {
	concepts, err := reader.ReadBundle(bundleDir)
	if err != nil {
		return nil, err
	}
	return NewBundleSearchFromConcepts(concepts)
}

// NewBundleSearchFromConcepts creates a search index from concepts.
func NewBundleSearchFromConcepts(concepts map[string]*types.Concept) (*BundleSearch, error) {
	kg := graph.BuildGraph(concepts)

	// Create index mapping
	indexMapping := buildIndexMapping()

	// Create in-memory index
	index, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return nil, err
	}

	// Index all concepts
	for _, concept := range kg.Concepts {
		doc := searchDoc{
			ID:          concept.ID,
			Title:       concept.Title,
			Type:        concept.Type,
			Description: concept.Description,
			Tags:        strings.Join(concept.Tags, " "),
			Body:        concept.Body,
		}
		if err := index.Index(concept.ID, doc); err != nil {
			_ = index.Close()
			return nil, err
		}
	}

	return &BundleSearch{
		Graph: kg,
		index: index,
	}, nil
}

// buildIndexMapping creates the Bleve index mapping with field boosts.
func buildIndexMapping() mapping.IndexMapping {
	// Create document mapping
	docMapping := bleve.NewDocumentMapping()

	// Title field with boost
	titleFieldMapping := bleve.NewTextFieldMapping()
	titleFieldMapping.Analyzer = standard.Name
	titleFieldMapping.Store = false
	docMapping.AddFieldMappingsAt("Title", titleFieldMapping)

	// Tags field with boost
	tagsFieldMapping := bleve.NewTextFieldMapping()
	tagsFieldMapping.Analyzer = standard.Name
	tagsFieldMapping.Store = false
	docMapping.AddFieldMappingsAt("Tags", tagsFieldMapping)

	// Type field with boost
	typeFieldMapping := bleve.NewTextFieldMapping()
	typeFieldMapping.Analyzer = standard.Name
	typeFieldMapping.Store = false
	docMapping.AddFieldMappingsAt("Type", typeFieldMapping)

	// Description field
	descFieldMapping := bleve.NewTextFieldMapping()
	descFieldMapping.Analyzer = standard.Name
	descFieldMapping.Store = false
	docMapping.AddFieldMappingsAt("Description", descFieldMapping)

	// Body field
	bodyFieldMapping := bleve.NewTextFieldMapping()
	bodyFieldMapping.Analyzer = standard.Name
	bodyFieldMapping.Store = false
	docMapping.AddFieldMappingsAt("Body", bodyFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = standard.Name

	return indexMapping
}

// Search searches concepts by query.
func (s *BundleSearch) Search(queryStr string, opts SearchOptions) []types.SearchResult {
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Build query
	var q query.Query
	if queryStr == "" || queryStr == "*" {
		q = bleve.NewMatchAllQuery()
	} else {
		// Use a disjunction of boosted field queries
		titleQuery := bleve.NewMatchQuery(queryStr)
		titleQuery.SetField("Title")
		titleQuery.SetBoost(4.0)
		titleQuery.SetFuzziness(1)

		tagsQuery := bleve.NewMatchQuery(queryStr)
		tagsQuery.SetField("Tags")
		tagsQuery.SetBoost(3.0)
		tagsQuery.SetFuzziness(1)

		typeQuery := bleve.NewMatchQuery(queryStr)
		typeQuery.SetField("Type")
		typeQuery.SetBoost(2.0)
		typeQuery.SetFuzziness(1)

		descQuery := bleve.NewMatchQuery(queryStr)
		descQuery.SetField("Description")
		descQuery.SetBoost(2.0)
		descQuery.SetFuzziness(1)

		bodyQuery := bleve.NewMatchQuery(queryStr)
		bodyQuery.SetField("Body")
		bodyQuery.SetFuzziness(1)

		q = bleve.NewDisjunctionQuery(titleQuery, tagsQuery, typeQuery, descQuery, bodyQuery)
	}

	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.Size = 100 // Get more results for filtering

	searchResult, err := s.index.Search(searchRequest)
	if err != nil {
		return nil
	}

	// Filter and convert results
	results := make([]types.SearchResult, 0, limit)
	tagFilter := make(map[string]bool)
	for _, tag := range opts.Tags {
		tagFilter[tag] = true
	}

	for _, hit := range searchResult.Hits {
		concept, ok := s.Graph.Concepts[hit.ID]
		if !ok {
			continue
		}

		// Filter by type
		if opts.Type != "" && concept.Type != opts.Type {
			continue
		}

		// Filter by tags
		if len(tagFilter) > 0 {
			hasTag := false
			for _, tag := range concept.Tags {
				if tagFilter[tag] {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		results = append(results, types.SearchResult{
			ID:          concept.ID,
			Title:       concept.Title,
			Type:        concept.Type,
			Description: concept.Description,
			Tags:        concept.Tags,
			Resource:    concept.Resource,
			Snippet:     snippet(concept, queryStr, 240),
			Score:       hit.Score,
		})

		if len(results) >= limit {
			break
		}
	}

	return results
}

// GetConcept retrieves a concept by ID or path.
func (s *BundleSearch) GetConcept(idOrPath string) *types.Concept {
	// Try as-is
	if concept, ok := s.Graph.Concepts[idOrPath]; ok {
		return concept
	}

	// Try without .md extension
	id := strings.TrimSuffix(strings.ToLower(idOrPath), ".md")
	if concept, ok := s.Graph.Concepts[id]; ok {
		return concept
	}

	// Search through concepts by path
	for _, concept := range s.Graph.Concepts {
		if concept.Path == idOrPath {
			return concept
		}
	}

	return nil
}

// Close closes the search index.
func (s *BundleSearch) Close() error {
	return s.index.Close()
}

// snippet extracts a snippet around the first matching term.
func snippet(concept *types.Concept, queryStr string, maxLen int) string {
	text := concept.Description + " " + concept.Body
	text = strings.Join(strings.Fields(text), " ")

	lower := strings.ToLower(text)
	terms := strings.Fields(strings.ToLower(queryStr))

	idx := -1
	for _, term := range terms {
		if term != "" {
			idx = strings.Index(lower, term)
			if idx >= 0 {
				break
			}
		}
	}

	start := 0
	if idx > 80 {
		start = idx - 80
	}

	end := start + maxLen
	if end > len(text) {
		end = len(text)
	}

	return text[start:end]
}
