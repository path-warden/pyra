package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/okfy/okf-mcp/internal/compress"
	ctxpkg "github.com/okfy/okf-mcp/internal/context"
)

func (s *Server) handleGetContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	query := getArgString(args, "query")
	tokenBudgetF := getArgFloat(args, "token_budget")
	includeNeighbors := getArgBool(args, "include_neighbors", true)
	depthF := getArgFloat(args, "depth")
	compressionStr := getArgString(args, "compression")

	tokenBudget := int(tokenBudgetF)
	if tokenBudget <= 0 {
		tokenBudget = 4000
	}

	depth := int(depthF)
	if depth <= 0 {
		depth = 1
	}
	if depth > 2 {
		depth = 2
	}

	compressionLevel := compress.LevelLight
	if compressionStr != "" {
		compressionLevel = compress.ParseLevel(compressionStr)
	}

	result := ctxpkg.Assemble(s.search, ctxpkg.AssemblyOptions{
		Query:            query,
		TokenBudget:      tokenBudget,
		IncludeNeighbors: includeNeighbors,
		Depth:            depth,
		Compression:      compressionLevel,
	})

	return s.jsonResult(result)
}

func getArgBool(args map[string]any, key string, defaultVal bool) bool {
	if args == nil {
		return defaultVal
	}
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}
