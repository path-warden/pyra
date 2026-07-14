// Package mcp implements the MCP (Model Context Protocol) server.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/codehealth"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/compress"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/scale"
	"github.com/chasedputnam/pyra/internal/search"
	"github.com/chasedputnam/pyra/internal/store"
	"github.com/chasedputnam/pyra/internal/tokens"
	"github.com/chasedputnam/pyra/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServerOptions configures the MCP server.
type ServerOptions struct {
	BundleDir      string
	Name           string
	MaxResultChars int
}

// Server is an MCP server for OKF bundles.
type Server struct {
	bundleDir      string
	name           string
	maxResultChars int
	search         *search.BundleSearch
	searchMu       sync.RWMutex
	mcpServer      *server.MCPServer
	stats          *CompressionStats

	store *store.Store    // unified Canon + Reference store for authority-aware tools
	code  *codeintel.Ops  // code-intelligence operations, rooted at the bundle dir
	git   *gitint.History // git-intelligence index, rooted at the bundle dir (may be nil)

	graph     *codegraph.Graph // code dependency graph, built lazily on first use
	graphOnce sync.Once

	health     *codehealth.Report // code-health report, built lazily on first use
	healthOnce sync.Once
}

// NewServer creates a new MCP server.
func NewServer(opts ServerOptions) (*Server, error) {
	bundleSearch, err := search.NewBundleSearch(opts.BundleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bundle: %w", err)
	}

	name := opts.Name
	if name == "" {
		name = "pyra"
	}

	maxResultChars := opts.MaxResultChars
	if maxResultChars <= 0 {
		maxResultChars = 12000
	}

	s := &Server{
		bundleDir:      opts.BundleDir,
		name:           name,
		maxResultChars: maxResultChars,
		search:         bundleSearch,
		stats:          NewCompressionStats(),
	}

	// Load the unified store for authority-aware (Canon) tools. A failure here is
	// non-fatal: the server still serves the Reference tier.
	if cfg, cerr := config.Load(opts.BundleDir); cerr == nil {
		if st, serr := store.Load(opts.BundleDir, cfg); serr == nil {
			s.store = st
		}
	}

	s.code = codeintel.NewOps(codeintel.NewEngine(nil), opts.BundleDir)

	// Git-intelligence index, best-effort like the store: nil when the bundle is
	// not a git repository (the tools then report "unavailable").
	if gi, ok := gitint.New(opts.BundleDir, gitint.DefaultWindow); ok {
		s.git = gi
	}

	s.mcpServer = server.NewMCPServer(name, "0.1.0")
	s.registerTools()
	s.registerCanonTools()
	s.registerCodeIntelTools()
	s.registerGitIntelTools()
	s.registerGraphTools()
	s.registerHealthTools()
	s.registerDeadCodeTools()

	return s, nil
}

// Serve starts the MCP server on stdio.
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcpServer)
}

// Close closes the server and its resources.
func (s *Server) Close() error {
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	if s.store != nil {
		_ = s.store.Close()
	}
	return s.search.Close()
}

// reloadSearchIndex reloads the search index after bundle changes.
func (s *Server) reloadSearchIndex() error {
	newSearch, err := search.NewBundleSearch(s.bundleDir)
	if err != nil {
		return err
	}
	s.searchMu.Lock()
	oldSearch := s.search
	s.search = newSearch
	s.searchMu.Unlock()
	return oldSearch.Close()
}

// getSearch returns the current search index with read lock.
func (s *Server) getSearch() *search.BundleSearch {
	s.searchMu.RLock()
	defer s.searchMu.RUnlock()
	return s.search
}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("search_concepts",
		mcp.WithDescription("Search OKF concepts by query, type, and tags."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("type", mcp.Description("Filter by concept type")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags to filter by")),
		mcp.WithNumber("limit", mcp.Description("Maximum results (default 10, max 50)")),
		mcp.WithNumber("token_budget", mcp.Description("Maximum tokens for all results combined")),
		mcp.WithString("compression", mcp.Description("Compression level: none, light, medium, aggressive (default: light)")),
		mcp.WithNumber("detail_level", mcp.Description("Detail level 0-3 (default: 1)")),
	), s.handleSearchConcepts)

	s.mcpServer.AddTool(mcp.NewTool("read_concept",
		mcp.WithDescription("Read one OKF concept by id or path."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Concept ID or path")),
		mcp.WithNumber("max_chars", mcp.Description("Maximum characters to return")),
		mcp.WithNumber("token_budget", mcp.Description("Maximum tokens to return")),
		mcp.WithString("compression", mcp.Description("Compression level: none, light, medium, aggressive")),
		mcp.WithNumber("detail_level", mcp.Description("Detail level 0-3")),
	), s.handleReadConcept)

	s.mcpServer.AddTool(mcp.NewTool("get_neighbors",
		mcp.WithDescription("Return outbound links and backlinks for a concept."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Concept ID or path")),
		mcp.WithNumber("depth", mcp.Description("Traversal depth (1 or 2, default 1)")),
	), s.handleGetNeighbors)

	s.mcpServer.AddTool(mcp.NewTool("list_types",
		mcp.WithDescription("List concept types and counts."),
	), s.handleListTypes)

	s.mcpServer.AddTool(mcp.NewTool("list_tags",
		mcp.WithDescription("List concept tags and counts."),
	), s.handleListTags)

	s.mcpServer.AddTool(mcp.NewTool("bundle_summary",
		mcp.WithDescription("Return bundle stats and validation status."),
	), s.handleBundleSummary)

	s.mcpServer.AddTool(mcp.NewTool("get_context",
		mcp.WithDescription("Assemble relevant context for a query within a token budget."),
		mcp.WithString("query", mcp.Required(), mcp.Description("The query to find relevant context for")),
		mcp.WithNumber("token_budget", mcp.Description("Maximum tokens to return (default: 4000)")),
		mcp.WithBoolean("include_neighbors", mcp.Description("Include linked concepts (default: true)")),
		mcp.WithNumber("depth", mcp.Description("Neighbor traversal depth (default: 1, max: 2)")),
		mcp.WithString("compression", mcp.Description("Compression level: none, light, medium, aggressive (default: light)")),
	), s.handleGetContext)

	s.mcpServer.AddTool(mcp.NewTool("compression_stats",
		mcp.WithDescription("Return token compression statistics for this session."),
	), s.handleCompressionStats)

	s.mcpServer.AddTool(mcp.NewTool("check_updates",
		mcp.WithDescription("Check if the bundle source has updates available."),
		mcp.WithNumber("timeout_seconds", mcp.Description("Timeout for fetching source (default: 30)")),
	), s.handleCheckUpdates)

	s.mcpServer.AddTool(mcp.NewTool("apply_updates",
		mcp.WithDescription("Apply pending updates to the bundle from its source."),
		mcp.WithBoolean("confirm", mcp.Description("Must be true to apply changes")),
		mcp.WithBoolean("dry_run", mcp.Description("Preview changes without applying")),
	), s.handleApplyUpdates)

	s.mcpServer.AddTool(mcp.NewTool("bundle_health",
		mcp.WithDescription("Check bundle health: validation status, staleness, and source reachability."),
		mcp.WithBoolean("check_source", mcp.Description("Check if source URL is reachable (default: true)")),
	), s.handleBundleHealth)
}

func getArgString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getArgFloat(args map[string]any, key string) float64 {
	if args == nil {
		return 0
	}
	if v, ok := args[key].(float64); ok {
		return v
	}
	return 0
}

func (s *Server) handleSearchConcepts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	query := getArgString(args, "query")
	typeFilter := getArgString(args, "type")
	tagsStr := getArgString(args, "tags")
	limitF := getArgFloat(args, "limit")
	tokenBudgetF := getArgFloat(args, "token_budget")
	compressionStr := getArgString(args, "compression")
	detailLevelF := getArgFloat(args, "detail_level")

	limit := int(limitF)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	tokenBudget := int(tokenBudgetF)
	detailLevel := int(detailLevelF)
	if detailLevel < 0 {
		detailLevel = 1
	}
	if detailLevel > 3 {
		detailLevel = 3
	}

	compressionLevel := compress.LevelLight
	if compressionStr != "" {
		compressionLevel = compress.ParseLevel(compressionStr)
	}

	var tags []string
	if tagsStr != "" {
		tags = append(tags, splitTags(tagsStr)...)
	}

	srch := s.getSearch()
	results := srch.Search(query, search.SearchOptions{
		Type:  typeFilter,
		Tags:  tags,
		Limit: limit,
	})

	// Apply token budget and compression if specified
	if tokenBudget > 0 || compressionLevel != compress.LevelNone {
		est := tokens.NewEstimator()
		totalTokens := 0
		enhancedResults := make([]map[string]any, 0, len(results))

		for _, r := range results {
			// Calculate full token count for concept body
			concept := srch.GetConcept(r.ID)
			fullTokens := 0
			if concept != nil {
				fullTokens = est.Count(concept.Body)
			}

			// Build result based on detail level
			result := map[string]any{
				"id":    r.ID,
				"type":  r.Type,
				"score": r.Score,
			}

			switch detailLevel {
			case 0:
				result["title"] = r.Title
			case 1:
				result["title"] = r.Title
				result["description"] = r.Description
				result["tags"] = r.Tags
			case 2:
				result["title"] = r.Title
				result["description"] = r.Description
				result["tags"] = r.Tags
				result["snippet"] = r.Snippet
			default:
				result["title"] = r.Title
				result["description"] = r.Description
				result["tags"] = r.Tags
				result["snippet"] = r.Snippet
				result["resource"] = r.Resource
			}

			// Add expansion hints
			result["expandable"] = fullTokens > 100
			result["full_tokens"] = fullTokens

			// Count tokens for this result
			resultTokens := est.CountJSON(result)
			result["token_count"] = resultTokens

			// Check budget
			if tokenBudget > 0 && totalTokens+resultTokens > tokenBudget {
				break
			}
			totalTokens += resultTokens

			enhancedResults = append(enhancedResults, result)
		}

		return s.jsonResult(map[string]any{
			"results":      enhancedResults,
			"total_tokens": totalTokens,
			"truncated":    len(enhancedResults) < len(results),
		})
	}

	return s.jsonResult(results)
}

func (s *Server) handleReadConcept(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	maxCharsF := getArgFloat(args, "max_chars")
	tokenBudgetF := getArgFloat(args, "token_budget")
	compressionStr := getArgString(args, "compression")
	detailLevelF := getArgFloat(args, "detail_level")

	maxChars := int(maxCharsF)
	if maxChars <= 0 {
		maxChars = s.maxResultChars
	}

	tokenBudget := int(tokenBudgetF)
	detailLevel := int(detailLevelF)
	if detailLevel < 0 {
		detailLevel = 3 // Default to full detail for read
	}
	if detailLevel > 3 {
		detailLevel = 3
	}

	compressionLevel := compress.LevelNone
	if compressionStr != "" {
		compressionLevel = compress.ParseLevel(compressionStr)
	}

	srch := s.getSearch()
	concept := srch.GetConcept(id)
	if concept == nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "unknown_concept",
				"message": fmt.Sprintf("No concept found for %s", id),
			},
		})
	}

	est := tokens.NewEstimator()
	body := concept.Body

	// Apply compression if specified
	if compressionLevel != compress.LevelNone {
		compResult := compress.Compress(body, compress.Options{
			Level:               compressionLevel,
			TokenBudget:         tokenBudget,
			PreserveFrontmatter: false, // Body doesn't have frontmatter
		})
		body = compResult.Content
	} else if tokenBudget > 0 {
		// Apply token budget truncation
		truncResult := compress.Truncate(body, compress.TruncateOptions{
			TokenBudget:  tokenBudget,
			AddIndicator: true,
		})
		body = truncResult.Content
	} else if len(body) > maxChars {
		body = body[:maxChars]
	}

	// Build response based on detail level
	result := map[string]any{
		"id":   concept.ID,
		"type": concept.Type,
	}

	switch detailLevel {
	case 0:
		result["title"] = concept.Title
	case 1:
		result["title"] = concept.Title
		result["description"] = concept.Description
		result["tags"] = concept.Tags
	case 2:
		result["title"] = concept.Title
		result["description"] = concept.Description
		result["tags"] = concept.Tags
		result["frontmatter"] = concept.Frontmatter
	default:
		result["title"] = concept.Title
		result["description"] = concept.Description
		result["tags"] = concept.Tags
		result["frontmatter"] = concept.Frontmatter
		result["markdown_body"] = body
		result["outbound_links"] = srch.Graph.Outbound[concept.ID]
		result["backlinks"] = srch.Graph.Backlinks[concept.ID]
		result["source_resource"] = concept.Resource
	}

	result["token_count"] = est.Count(body)

	return s.jsonResult(result)
}

func (s *Server) handleGetNeighbors(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	depthF := getArgFloat(args, "depth")

	depth := int(depthF)
	if depth <= 0 {
		depth = 1
	}
	if depth > 2 {
		depth = 2
	}

	srch := s.getSearch()
	root := srch.GetConcept(id)
	if root == nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "unknown_concept",
				"message": fmt.Sprintf("No concept found for %s", id),
			},
		})
	}

	seen := map[string]bool{root.ID: true}
	frontier := []string{root.ID}
	var edges []map[string]any

	for level := 0; level < depth; level++ {
		var next []string
		for _, nodeID := range frontier {
			for _, to := range srch.Graph.Outbound[nodeID] {
				edges = append(edges, map[string]any{
					"from":              nodeID,
					"to":                to,
					"direction":         "outbound",
					"relationship_text": "Markdown link",
				})
				if !seen[to] {
					next = append(next, to)
				}
				seen[to] = true
			}
			for _, from := range srch.Graph.Backlinks[nodeID] {
				edges = append(edges, map[string]any{
					"from":              from,
					"to":                nodeID,
					"direction":         "backlink",
					"relationship_text": "Backlink",
				})
				if !seen[from] {
					next = append(next, from)
				}
				seen[from] = true
			}
		}
		frontier = next
	}

	var concepts []map[string]any
	for nodeID := range seen {
		concept := srch.Graph.Concepts[nodeID]
		if concept != nil {
			concepts = append(concepts, map[string]any{
				"id":       concept.ID,
				"title":    concept.Title,
				"type":     concept.Type,
				"resource": concept.Resource,
			})
		}
	}

	return s.jsonResult(map[string]any{
		"root":     root.ID,
		"concepts": concepts,
		"edges":    edges,
	})
}

func (s *Server) handleListTypes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats, err := validate.InspectBundle(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{"error": err.Error()})
	}
	return s.jsonResult(stats.TypeDistribution)
}

func (s *Server) handleListTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats, err := validate.InspectBundle(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{"error": err.Error()})
	}
	return s.jsonResult(stats.TagDistribution)
}

func (s *Server) handleBundleSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats, err := validate.InspectBundle(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{"error": err.Error()})
	}

	report, err := validate.ValidateBundle(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{"error": err.Error()})
	}

	// Get scale metrics
	metrics, ceiling, _ := scale.Analyze(s.bundleDir)

	// Read index content for navigation
	var indexContent string
	indexPath := filepath.Join(s.bundleDir, "index.md")
	if data, err := os.ReadFile(indexPath); err == nil {
		indexContent = string(data)
	}

	result := map[string]any{
		"title":             stats.Title,
		"conceptCount":      stats.ConceptCount,
		"linkCount":         stats.LinkCount,
		"brokenLinks":       stats.BrokenLinks,
		"orphanConcepts":    stats.OrphanConcepts,
		"typeDistribution":  stats.TypeDistribution,
		"tagDistribution":   stats.TagDistribution,
		"topLinkedConcepts": stats.TopLinkedConcepts,
		"sourceDomains":     stats.SourceDomains,
		"validationStatus":  boolToStatus(report.Valid),
		"validationIssues":  report.Issues,
	}

	// Add scale metrics if available
	if metrics != nil {
		result["totalTokens"] = metrics.TotalTokens
		result["indexTokens"] = metrics.IndexTokens
		result["avgTokensPerConcept"] = metrics.AvgTokensPerConcept
		result["scaleStatus"] = string(ceiling.Status)
		result["scaleMessage"] = ceiling.Message
	}

	// Add index content for summary-first navigation
	if indexContent != "" {
		result["indexContent"] = indexContent
	}

	return s.jsonResult(result)
}

func boolToStatus(valid bool) string {
	if valid {
		return "valid"
	}
	return "invalid"
}

func (s *Server) jsonResult(value any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}

	text := string(data)
	if len(text) > s.maxResultChars {
		text = text[:s.maxResultChars] + "\n...truncated"
	}

	return mcp.NewToolResultText(text), nil
}

func splitTags(s string) []string {
	var tags []string
	for t := range strings.SplitSeq(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
