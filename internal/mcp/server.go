// Package mcp implements the MCP (Model Context Protocol) server.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/okfy/okf-mcp/internal/search"
	"github.com/okfy/okf-mcp/internal/validate"
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
	mcpServer      *server.MCPServer
}

// NewServer creates a new MCP server.
func NewServer(opts ServerOptions) (*Server, error) {
	bundleSearch, err := search.NewBundleSearch(opts.BundleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load bundle: %w", err)
	}

	name := opts.Name
	if name == "" {
		name = "okf-cli"
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
	}

	s.mcpServer = server.NewMCPServer(name, "0.1.0")
	s.registerTools()

	return s, nil
}

// Serve starts the MCP server on stdio.
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcpServer)
}

// Close closes the server and its resources.
func (s *Server) Close() error {
	return s.search.Close()
}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("search_concepts",
		mcp.WithDescription("Search OKF concepts by query, type, and tags."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("type", mcp.Description("Filter by concept type")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags to filter by")),
		mcp.WithNumber("limit", mcp.Description("Maximum results (default 10, max 50)")),
	), s.handleSearchConcepts)

	s.mcpServer.AddTool(mcp.NewTool("read_concept",
		mcp.WithDescription("Read one OKF concept by id or path."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Concept ID or path")),
		mcp.WithNumber("max_chars", mcp.Description("Maximum characters to return")),
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

	limit := int(limitF)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	var tags []string
	if tagsStr != "" {
		for _, tag := range splitTags(tagsStr) {
			tags = append(tags, tag)
		}
	}

	results := s.search.Search(query, search.SearchOptions{
		Type:  typeFilter,
		Tags:  tags,
		Limit: limit,
	})

	return s.jsonResult(results)
}

func (s *Server) handleReadConcept(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	maxCharsF := getArgFloat(args, "max_chars")

	maxChars := int(maxCharsF)
	if maxChars <= 0 {
		maxChars = s.maxResultChars
	}

	concept := s.search.GetConcept(id)
	if concept == nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "unknown_concept",
				"message": fmt.Sprintf("No concept found for %s", id),
			},
		})
	}

	body := concept.Body
	if len(body) > maxChars {
		body = body[:maxChars]
	}

	return s.jsonResult(map[string]any{
		"frontmatter":    concept.Frontmatter,
		"markdown_body":  body,
		"outbound_links": s.search.Graph.Outbound[concept.ID],
		"backlinks":      s.search.Graph.Backlinks[concept.ID],
		"source_resource": concept.Resource,
	})
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

	root := s.search.GetConcept(id)
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
			for _, to := range s.search.Graph.Outbound[nodeID] {
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
			for _, from := range s.search.Graph.Backlinks[nodeID] {
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
		concept := s.search.Graph.Concepts[nodeID]
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

	return s.jsonResult(map[string]any{
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
	})
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
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
