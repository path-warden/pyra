package mcp

import (
	"context"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/config"
)

// graphIndex lazily builds and caches the code dependency graph over the bundle's
// code roots. Building can be the most expensive operation, so it is deferred to
// first use rather than paid at serve startup.
func (s *Server) graphIndex() *codegraph.Graph {
	s.graphOnce.Do(func() {
		cfg, _ := config.Load(s.bundleDir)
		var roots []string
		for _, r := range cfg.CodeRoots {
			roots = append(roots, filepath.Join(s.bundleDir, r))
		}
		g, err := codegraph.Build(s.code, roots, codegraph.Options{})
		if err == nil {
			s.graph = g
		}
	})
	return s.graph
}

// registerGraphTools registers the read-only code-graph tools.
func (s *Server) registerGraphTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_graph_centrality",
		mcp.WithDescription("Rank symbols by PageRank over the code dependency graph — the codebase's hubs. Deterministic and offline."),
		mcp.WithNumber("limit", mcp.Description("Maximum symbols to return (default 20)")),
	), s.handleGraphCentrality)

	s.mcpServer.AddTool(mcp.NewTool("get_communities",
		mcp.WithDescription("Partition the code dependency graph into communities (logical modules) via deterministic label propagation."),
	), s.handleCommunities)

	s.mcpServer.AddTool(mcp.NewTool("get_cycles",
		mcp.WithDescription("Report dependency cycles (strongly-connected components) in the code dependency graph."),
	), s.handleCycles)
}

func (s *Server) handleGraphCentrality(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	g := s.graphIndex()
	if g == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "no code graph"})
	}
	args, _ := r.Params.Arguments.(map[string]any)
	limit := int(getArgFloat(args, "limit"))
	if _, ok := args["limit"]; !ok {
		limit = 20
	}
	return s.jsonResult(map[string]any{
		"available":  true,
		"total":      g.NodeCount(),
		"truncated":  g.Truncated,
		"centrality": g.TopCentral(limit),
	})
}

func (s *Server) handleCommunities(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	g := s.graphIndex()
	if g == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "no code graph"})
	}
	return s.jsonResult(map[string]any{"available": true, "total": g.NodeCount(), "communities": g.Communities()})
}

func (s *Server) handleCycles(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	g := s.graphIndex()
	if g == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "no code graph"})
	}
	return s.jsonResult(map[string]any{"available": true, "cycles": g.Cycles()})
}
