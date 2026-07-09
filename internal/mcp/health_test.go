package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func healthBundleServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	writeGit(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	var b strings.Builder
	b.WriteString("package p\n\nfunc huge(a int) int {\n")
	for i := 0; i < 80; i++ {
		b.WriteString("\tif a>0 { if a>1 { if a>2 { if a>3 { if a>4 { a++ } } } } }\n")
	}
	b.WriteString("\treturn a\n}\n")
	writeGit(t, dir, "bad.go", b.String())
	writeGit(t, dir, "good.go", "package p\n\nfunc Ok() int { return 1 }\n")
	s, err := NewServer(ServerOptions{BundleDir: dir, Name: "t"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func TestMCPGetHealth(t *testing.T) {
	s := healthBundleServer(t)
	res, err := s.handleGetHealth(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, res)
	if m["available"] != true {
		t.Fatalf("available = %v, want true", m["available"])
	}
	lowest, _ := m["lowest"].([]any)
	if len(lowest) == 0 {
		t.Fatalf("expected lowest-scoring files, got %v", m["lowest"])
	}
	first, _ := lowest[0].(map[string]any)
	if first["path"] != "bad.go" {
		t.Errorf("worst file = %v, want bad.go", first["path"])
	}
	// Lazy build cached.
	if s.healthReport() == nil {
		t.Error("health report should be cached")
	}
	// Equivalent to the underlying report.
	if s.health.Files[0].Path != "bad.go" {
		t.Error("MCP result should match the report the CLI uses")
	}
}
