package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/compress"
	"github.com/chasedputnam/pyra/internal/retrieval"
	"github.com/chasedputnam/pyra/internal/store"
)

// registerCanonTools adds the read-only authority (Canon) tools. All Canon
// mutation happens via CLI/PR review, never here (Requirement 8a).
func (s *Server) registerCanonTools() {
	s.mcpServer.AddTool(mcp.NewTool("get_artifact",
		mcp.WithDescription("Read one authoritative Canon artifact by id, with its type, status, relationships, and citation."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Canon artifact ID")),
	), s.handleGetArtifact)

	s.mcpServer.AddTool(mcp.NewTool("find_decisions",
		mcp.WithDescription("Find Canon decisions related to a topic."),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic to search decisions for")),
		mcp.WithNumber("limit", mcp.Description("Maximum results (default 10)")),
	), s.handleFindDecisions)

	s.mcpServer.AddTool(mcp.NewTool("get_related",
		mcp.WithDescription("Return typed relationships for a Canon artifact, traversed to a depth."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Canon artifact ID")),
		mcp.WithNumber("depth", mcp.Description("Traversal depth (default 1)")),
	), s.handleGetRelated)

	s.mcpServer.AddTool(mcp.NewTool("get_summary",
		mcp.WithDescription("Summarize the Canon corpus: counts by type and lifecycle status."),
	), s.handleGetSummary)
}

func (s *Server) handleGetArtifact(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	if s.store == nil {
		return mcp.NewToolResultText(`{"error":"no canon store loaded"}`), nil
	}
	item := s.store.ByID(id)
	if item == nil || item.Tier != store.TierCanon {
		return mcp.NewToolResultText(`{"error":"canon artifact not found: ` + jsonEscape(id) + `"}`), nil
	}
	return s.jsonResult(map[string]any{
		"id":            item.ID,
		"title":         item.Title,
		"type":          item.Type,
		"status":        item.Status,
		"path":          item.Path,
		"body":          item.Body,
		"relationships": item.Edges,
		"citation":      map[string]string{"id": item.ID, "path": item.Path, "type": item.Type, "status": item.Status},
		"authoritative": true,
		"derived":       false,
	})
}

func (s *Server) handleFindDecisions(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	topic := getArgString(args, "topic")
	limit := int(getArgFloat(args, "limit"))
	if limit <= 0 {
		limit = 10
	}
	if s.store == nil {
		return mcp.NewToolResultText(`{"decisions":[]}`), nil
	}
	hits := s.store.Discover(topic, limit*3)
	decisions := []map[string]any{}
	for _, h := range hits {
		if h.Item.Tier != store.TierCanon || h.Item.Type != artifacts.TypeDecision {
			continue
		}
		decisions = append(decisions, map[string]any{
			"id": h.Item.ID, "title": h.Item.Title, "status": h.Item.Status,
			"path": h.Item.Path, "score": h.Score,
		})
		if len(decisions) >= limit {
			break
		}
	}
	return s.jsonResult(map[string]any{"decisions": decisions})
}

func (s *Server) handleGetRelated(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	depth := int(getArgFloat(args, "depth"))
	if depth <= 0 {
		depth = 1
	}
	if s.store == nil {
		return mcp.NewToolResultText(`{"outgoing":{},"incoming":[],"neighborhood":[]}`), nil
	}
	outgoing := s.store.Outgoing(id, 0)
	incoming := s.store.Incoming(id, 0)
	nb := s.store.NeighborhoodByID(id, depth)
	return s.jsonResult(map[string]any{
		"id":    id,
		"depth": depth,
		"outgoing": map[string]any{
			"by_section": outgoing.BySection,
			"total":      outgoing.Total,
		},
		"incoming": map[string]any{
			"items": incoming.Items,
			"total": incoming.Total,
		},
		"neighborhood": map[string]any{
			"nodes":     nb.Nodes,
			"truncated": nb.Truncated,
		},
	})
}

func (s *Server) handleGetSummary(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.store == nil {
		return mcp.NewToolResultText(`{"canon_count":0}`), nil
	}
	byType := map[string]int{}
	byStatus := map[string]int{}
	for _, it := range s.store.Canon {
		byType[it.Type]++
		if it.Status != "" {
			byStatus[it.Status]++
		}
	}
	return s.jsonResult(map[string]any{
		"canon_count":     len(s.store.Canon),
		"reference_count": len(s.store.Reference),
		"by_type":         byType,
		"by_status":       byStatus,
	})
}

// contextUnified runs the authority-aware discover->ground->assemble loop over
// both tiers. Reference items are marked non-authoritative/derived to signal the
// trust boundary.
func (s *Server) contextUnified(query string, tokenBudget int, compression string) (*mcp.CallToolResult, error) {
	opts := retrieval.Options{TokenBudget: tokenBudget}
	if compression != "" {
		opts.Compression = compress.ParseLevel(compression)
	}
	res := retrieval.Assemble(s.store, query, opts)

	items := make([]map[string]any, 0, len(res.Items))
	for _, it := range res.Items {
		m := map[string]any{
			"id": it.ID, "title": it.Title, "type": it.Type, "tier": it.Tier,
			"score": it.Score, "body": it.Body, "tokens": it.Tokens,
			"authoritative": it.Tier == "canon",
			"derived":       it.Tier != "canon",
		}
		if it.Citation != nil {
			m["citation"] = it.Citation
		}
		items = append(items, m)
	}
	return s.jsonResult(map[string]any{
		"query":              res.Query,
		"items":              items,
		"total_tokens":       res.TotalTokens,
		"suggested_followup": res.SuggestedFollowup,
	})
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return strings.Trim(string(b), `"`)
}
