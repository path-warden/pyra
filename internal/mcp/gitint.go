package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerGitIntelTools registers the read-only git-intelligence tools. They
// function even when no git index is available (they report "unavailable").
func (s *Server) registerGitIntelTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_hotspots",
		mcp.WithDescription("Rank files by git churn: the repository's hotspots (top-quartile temporally-decayed churn that also clears activity floors), each with churn percentile, commit counts, primary owner, and bus factor. Deterministic and offline."),
		mcp.WithNumber("limit", mcp.Description("Maximum hotspots to return (default 20)")),
	), s.handleGetHotspots)

	s.mcpServer.AddTool(mcp.NewTool("get_ownership",
		mcp.WithDescription("Git ownership for a file (primary owner + commit share, recent owner, contributor count, bus factor) or, for a directory or empty path, the top-level module rollups. Deterministic and offline."),
		mcp.WithString("path", mcp.Description("A file or directory path; empty returns the module rollups")),
	), s.handleGetOwnership)
}

func (s *Server) handleGetHotspots(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.git == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "not a git repository or no history"})
	}
	args, _ := r.Params.Arguments.(map[string]any)
	limit := int(getArgFloat(args, "limit"))
	if _, ok := args["limit"]; !ok {
		limit = 20
	}
	hot := s.git.Hotspots()
	if limit > 0 && len(hot) > limit {
		hot = hot[:limit]
	}
	return s.jsonResult(map[string]any{"available": true, "hotspots": hot})
}

func (s *Server) handleGetOwnership(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.git == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "not a git repository or no history"})
	}
	args, _ := r.Params.Arguments.(map[string]any)
	path := getArgString(args, "path")
	if path == "" {
		return s.jsonResult(map[string]any{"available": true, "modules": s.git.Modules()})
	}
	return s.jsonResult(map[string]any{"available": true, "ownership": s.git.OwnershipAt(path)})
}
