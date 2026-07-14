package mcp

import (
	"context"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chasedputnam/pyra/internal/codegraph"
	"github.com/chasedputnam/pyra/internal/codehealth"
	"github.com/chasedputnam/pyra/internal/config"
	"github.com/chasedputnam/pyra/internal/gitint"
)

// healthReport lazily builds and caches the code-health report over the bundle.
// Building composes several layers, so it is deferred to first use.
func (s *Server) healthReport() *codehealth.Report {
	s.healthOnce.Do(func() {
		cfg, _ := config.Load(s.bundleDir)
		var roots []string
		for _, r := range cfg.CodeRoots {
			roots = append(roots, filepath.Join(s.bundleDir, r))
		}
		in := codehealth.Inputs{Ops: s.code, Roots: roots, Root: s.bundleDir, Store: s.store}
		if h, ok := gitint.New(s.bundleDir, gitint.DefaultWindow); ok {
			in.History = h
		}
		if g, err := codegraph.Build(s.code, roots, codegraph.Options{}); err == nil {
			in.Graph = g
		}
		if rep, err := codehealth.Analyze(in); err == nil {
			s.health = &rep
		}
	})
	return s.health
}

func (s *Server) registerHealthTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_health",
		mcp.WithDescription("Score files for code health across three signals (defect / maintainability / performance) from deterministic biomarkers. Returns repo KPIs and the lowest-scoring files with their top marker and refactoring suggestion. Deterministic and offline."),
		mcp.WithNumber("limit", mcp.Description("Maximum lowest-scoring files to return (default 15)")),
	), s.handleGetHealth)
}

func (s *Server) handleGetHealth(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rep := s.healthReport()
	if rep == nil {
		return s.jsonResult(map[string]any{"available": false, "reason": "no code to analyze"})
	}
	args, _ := r.Params.Arguments.(map[string]any)
	limit := int(getArgFloat(args, "limit"))
	if _, ok := args["limit"]; !ok {
		limit = 15
	}
	var lowest []codehealth.FileHealth
	for _, f := range rep.Files {
		if f.Defect >= 10.0 {
			continue
		}
		if limit > 0 && len(lowest) >= limit {
			break
		}
		lowest = append(lowest, f)
	}
	return s.jsonResult(map[string]any{
		"available":               true,
		"average_health":          rep.AverageHealth,
		"hotspot_health":          rep.HotspotHealth,
		"file_count":              rep.FileCount,
		"worst":                   rep.Worst,
		"lowest":                  lowest,
		"contradictory_decisions": rep.Contradictions,
	})
}
