package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chasedputnam/pyra/internal/deadcode"
)

func (s *Server) registerDeadCodeTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_dead_code",
		mcp.WithDescription("Report likely-unreachable definitions (no path from entry points) ranked by cleanup impact, each with a confidence tier (high/medium/low) and a governed flag (Canon still cites it). Deterministic and offline."),
		mcp.WithString("tier", mcp.Description("Only this tier: high, medium, or low")),
		mcp.WithNumber("limit", mcp.Description("Maximum candidates to return (0 = all)")),
	), s.handleGetDeadCode)
}

func (s *Server) handleGetDeadCode(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	g := s.graphIndex()
	if g == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "no code graph"})
	}
	var canonBodies []string
	if s.store != nil {
		for _, it := range s.store.Canon {
			canonBodies = append(canonBodies, it.Body)
		}
	}
	rep := deadcode.Analyze(g, s.code, s.bundleDir, canonBodies)

	args, _ := r.Params.Arguments.(map[string]any)
	tier := getArgString(args, "tier")
	limit := int(getArgFloat(args, "limit"))

	var cands []deadcode.Candidate
	for _, c := range rep.Candidates {
		if tier != "" && c.Tier != tier {
			continue
		}
		if limit > 0 && len(cands) >= limit {
			break
		}
		cands = append(cands, c)
	}
	return s.jsonResult(map[string]any{
		"available":    true,
		"candidates":   cands,
		"total_impact": rep.TotalImpact,
	})
}
