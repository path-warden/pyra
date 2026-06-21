package mcp

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// CompressionStats tracks token savings across the session.
type CompressionStats struct {
	mu              sync.Mutex
	TotalOriginal   int64
	TotalCompressed int64
	RequestCount    int64
	ByTool          map[string]*ToolStats
}

// ToolStats tracks stats for a specific tool.
type ToolStats struct {
	Original   int64 `json:"original"`
	Compressed int64 `json:"compressed"`
	Calls      int64 `json:"calls"`
}

// NewCompressionStats creates a new stats tracker.
func NewCompressionStats() *CompressionStats {
	return &CompressionStats{
		ByTool: make(map[string]*ToolStats),
	}
}

// Record records compression for a tool call.
func (cs *CompressionStats) Record(tool string, original, compressed int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.TotalOriginal += int64(original)
	cs.TotalCompressed += int64(compressed)
	cs.RequestCount++

	if cs.ByTool[tool] == nil {
		cs.ByTool[tool] = &ToolStats{}
	}
	cs.ByTool[tool].Original += int64(original)
	cs.ByTool[tool].Compressed += int64(compressed)
	cs.ByTool[tool].Calls++
}

// Summary returns current statistics.
func (cs *CompressionStats) Summary() map[string]any {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	savings := float64(0)
	if cs.TotalOriginal > 0 {
		savings = float64(cs.TotalOriginal-cs.TotalCompressed) / float64(cs.TotalOriginal) * 100
	}

	byTool := make(map[string]any)
	for name, stats := range cs.ByTool {
		toolSavings := float64(0)
		if stats.Original > 0 {
			toolSavings = float64(stats.Original-stats.Compressed) / float64(stats.Original) * 100
		}
		byTool[name] = map[string]any{
			"original":        stats.Original,
			"compressed":      stats.Compressed,
			"calls":           stats.Calls,
			"savings_percent": toolSavings,
		}
	}

	return map[string]any{
		"total_original":   cs.TotalOriginal,
		"total_compressed": cs.TotalCompressed,
		"total_calls":      cs.RequestCount,
		"savings_percent":  savings,
		"by_tool":          byTool,
	}
}

func (s *Server) handleCompressionStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.jsonResult(s.stats.Summary())
}
