package mcp

import (
	"context"

	"github.com/chasedputnam/pyra/internal/compress"
	ctxpkg "github.com/chasedputnam/pyra/internal/context"
	"github.com/mark3labs/mcp-go/mcp"
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

	// When a Canon store is present, use the authority-aware
	// discover -> ground -> assemble loop so Canon is ranked first and grounded
	// with citations. Reference-only stores keep the legacy assembly.
	if s.store != nil && s.store.HasCanon() {
		return s.contextUnified(query, tokenBudget, compressionStr)
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
