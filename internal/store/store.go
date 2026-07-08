// Package store is the composition seam that unifies the Canon authority tier
// and the Reference (ingested) tier over one Markdown-in-Git substrate. It loads
// both tiers into a single Item representation and builds the derived indexes
// (full-text search, relationship graph) that serve queries.
//
// Derived indexes are rebuildable projections: Rebuild regenerates them entirely
// from the on-disk Markdown, so truth is never coupled to the index.
package store

import (
	"path/filepath"
	"strings"

	"github.com/chasedputnam/memphis/internal/canon"
	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/canon/relate"
	"github.com/chasedputnam/memphis/internal/config"
	"github.com/chasedputnam/memphis/internal/reader"
	"github.com/chasedputnam/memphis/internal/search"
	"github.com/chasedputnam/memphis/internal/types"
)

// Tier distinguishes authoritative Canon from supporting Reference knowledge.
type Tier int

const (
	TierReference Tier = iota
	TierCanon
)

func (t Tier) String() string {
	if t == TierCanon {
		return "canon"
	}
	return "reference"
}

// Item is the unified record for a piece of knowledge in either tier.
type Item struct {
	ID    string
	Title string
	Type  string
	Body  string
	Tags  []string
	Tier  Tier
	Path  string

	// Canon-only authority metadata (zero values for Reference).
	Status  string
	Edges   []relate.Edge
	Product *model.Product
}

// Store holds both tiers and the derived indexes built from them.
type Store struct {
	Root      string
	Cfg       config.Config
	Canon     []Item
	Reference []Item
	Graph     *relate.Graph

	index    *search.BundleSearch  // unified index over both tiers
	byID     map[string]*Item      // grounding lookup by artifact/concept ID
	rels     []relate.Relationship // resolved Canon relationships (rac-core model)
	entries  []relate.Entry        // Canon entries (for relationship summary)
	idByPath map[string]relate.Identity
	pathByID map[string]string
}

// Hit is a discovery result: an item and its relevance score.
type Hit struct {
	Item  *Item
	Score float64
}

// Load reads both tiers from storeRoot and builds the derived indexes.
func Load(storeRoot string, cfg config.Config) (*Store, error) {
	s := &Store{Root: storeRoot, Cfg: cfg}
	if err := s.build(); err != nil {
		return nil, err
	}
	return s, nil
}

// HasCanon reports whether any Canon artifacts exist. A store without Canon
// behaves exactly like the legacy memphis (no authority gate is imposed).
func (s *Store) HasCanon() bool { return len(s.Canon) > 0 }

// Rebuild regenerates all derived indexes from the on-disk Markdown.
func (s *Store) Rebuild() error {
	if s.index != nil {
		_ = s.index.Close()
		s.index = nil
	}
	s.Canon = nil
	s.Reference = nil
	s.Graph = nil
	s.byID = nil
	return s.build()
}

// Search runs full-text search over both tiers.
func (s *Store) Search(query string, opts search.SearchOptions) []types.SearchResult {
	if s.index == nil {
		return nil
	}
	return s.index.Search(query, opts)
}

// Discover runs full-text search over both tiers and resolves each hit to its
// store Item for grounding.
func (s *Store) Discover(query string, limit int) []Hit {
	if s.index == nil {
		return nil
	}
	results := s.index.Search(query, search.SearchOptions{Limit: limit})
	hits := make([]Hit, 0, len(results))
	for _, r := range results {
		if item := s.byID[r.ID]; item != nil {
			hits = append(hits, Hit{Item: item, Score: r.Score})
		}
	}
	return hits
}

// ByID returns the item with the given ID, or nil.
func (s *Store) ByID(id string) *Item { return s.byID[id] }

// Successor follows `supersedes` edges from a superseded artifact to the live
// artifact that supersedes it. Starting at id, it walks inbound supersedes edges
// while the current artifact's status is "superseded", guarding against cycles.
// It returns nil when id is unknown, the artifact is not superseded, or the
// chain cannot advance; otherwise it returns the terminal artifact reached.
//
// This is the single implementation of supersede resolution, shared by the
// retrieval loop and the change-aware gate.
func (s *Store) Successor(id string) *Item {
	item := s.byID[id]
	if item == nil || s.Graph == nil {
		return nil
	}
	visited := map[string]bool{}
	cur := item
	for cur != nil && cur.Status == "superseded" {
		if visited[cur.ID] {
			break
		}
		visited[cur.ID] = true
		var next *Item
		for _, e := range s.Graph.Inbound[cur.ID] {
			if e.Kind == "supersedes" {
				next = s.byID[e.From]
				break
			}
		}
		if next == nil {
			break
		}
		cur = next
	}
	if cur == item {
		return nil
	}
	return cur
}

// Relationships returns the resolved Canon relationship edges.
func (s *Store) Relationships() []relate.Relationship { return s.rels }

// Outgoing returns the references a Canon artifact declares, grouped by section.
func (s *Store) Outgoing(id string, limit int) relate.OutgoingReferences {
	return relate.Outgoing(s.rels, s.pathByID[id], limit)
}

// Incoming returns the artifacts whose references resolve to a Canon artifact.
func (s *Store) Incoming(id string, limit int) relate.IncomingReferences {
	return relate.Incoming(s.rels, s.idByPath, s.pathByID[id], limit)
}

// NeighborhoodByID returns the bounded multi-hop neighbourhood of a Canon
// artifact by ID.
func (s *Store) NeighborhoodByID(id string, depth int) relate.NeighborhoodResult {
	return relate.Neighborhood(s.rels, s.idByPath, s.pathByID[id], depth, 0, 0)
}

// RelationshipSummary aggregates Canon relationship health across the store.
func (s *Store) RelationshipSummary() relate.RelationshipSummary {
	return relate.Summarize(s.entries)
}

// Close releases the in-memory search index.
func (s *Store) Close() error {
	if s.index != nil {
		return s.index.Close()
	}
	return nil
}

func (s *Store) build() error {
	// Canon tier.
	arts, err := canon.LoadCorpus(s.Root, s.Cfg)
	if err != nil {
		return err
	}
	entries := make([]relate.Entry, 0, len(arts))
	for _, a := range arts {
		entries = append(entries, relate.Entry{
			ID: a.ID, Type: a.Type, Status: a.Status, Retired: a.Retired, Path: a.Path,
			Aliases: a.Aliases, Product: a.Product,
		})
	}
	graph, _ := relate.Build(entries, relate.DefaultSpecs())
	s.Graph = graph
	s.entries = entries
	s.rels = relate.Relationships(entries)
	s.idByPath = relate.IdentityByPath(entries)
	s.pathByID = map[string]string{}
	for path, id := range s.idByPath {
		s.pathByID[id.ID] = path
	}
	s.byID = map[string]*Item{}

	// Unified search index over both tiers, keyed by ID and path like ReadBundle.
	indexConcepts := map[string]*types.Concept{}

	for _, a := range arts {
		s.Canon = append(s.Canon, Item{
			ID:      a.ID,
			Title:   a.Product.Title,
			Type:    a.Type,
			Body:    productText(a.Product),
			Tags:    a.Frontmatter.Tags,
			Tier:    TierCanon,
			Path:    a.Path,
			Status:  a.Status,
			Edges:   graph.Edges[a.ID],
			Product: a.Product,
		})
	}
	for i := range s.Canon {
		it := &s.Canon[i]
		s.byID[it.ID] = it
		indexConcepts[it.ID] = canonConcept(it)
	}

	// Reference tier: everything in the bundle not under a Canon root.
	all, err := reader.ReadBundle(s.Root)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, c := range all {
		if s.isUnderCanon(c.Path) || seen[c.Path] {
			continue
		}
		seen[c.Path] = true
		s.Reference = append(s.Reference, Item{
			ID:    c.ID,
			Title: c.Title,
			Type:  c.Type,
			Body:  c.Body,
			Tags:  c.Tags,
			Tier:  TierReference,
			Path:  c.Path,
		})
	}
	for i := range s.Reference {
		it := &s.Reference[i]
		if _, exists := s.byID[it.ID]; !exists {
			s.byID[it.ID] = it
		}
		c := &types.Concept{
			ID: it.ID, Path: it.Path, Type: it.Type, Title: it.Title, Tags: it.Tags, Body: it.Body,
		}
		indexConcepts[it.ID] = c
		indexConcepts[it.Path] = c
	}

	idx, err := search.NewBundleSearchFromConcepts(indexConcepts)
	if err != nil {
		return err
	}
	s.index = idx
	return nil
}

func canonConcept(it *Item) *types.Concept {
	desc := ""
	if it.Product != nil {
		if body, ok := it.Product.Section("status"); ok {
			desc = body
		}
	}
	return &types.Concept{
		ID:          it.ID,
		Path:        it.Path,
		Type:        it.Type,
		Title:       it.Title,
		Description: desc,
		Tags:        it.Tags,
		Body:        it.Body,
	}
}

func (s *Store) isUnderCanon(path string) bool {
	p := filepath.ToSlash(path)
	for _, root := range s.Cfg.CanonRoots {
		r := strings.Trim(filepath.ToSlash(root), "/")
		if r == "" {
			continue
		}
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// productText concatenates a Canon product's title and section bodies for
// full-text indexing.
func productText(p *model.Product) string {
	var b strings.Builder
	b.WriteString(p.Title)
	b.WriteString("\n")
	for _, key := range p.Order {
		b.WriteString(p.Sections[key])
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
