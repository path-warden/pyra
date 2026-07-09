package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPGetDeadCode(t *testing.T) {
	dir := t.TempDir()
	writeGit(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	writeGit(t, dir, "app.go", "package p\n\nfunc Used() { helper() }\n\nfunc helper() {}\n\nfunc orphan() { return }\n")
	s, err := NewServer(ServerOptions{BundleDir: dir, Name: "t"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	res, err := s.handleGetDeadCode(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, res)
	if m["available"] != true {
		t.Fatalf("available = %v, want true", m["available"])
	}
	cands, _ := m["candidates"].([]any)
	found := false
	for _, c := range cands {
		if cm, _ := c.(map[string]any); cm["name"] == "orphan" {
			found = true
		}
	}
	if !found {
		t.Errorf("orphan should be reported dead via MCP, got %v", m["candidates"])
	}
}

func TestMCPGetDeadCode_TierFilter(t *testing.T) {
	dir := t.TempDir()
	writeGit(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	writeGit(t, dir, "app.go", "package p\n\nfunc Used() { helper() }\n\nfunc helper() {}\n\nfunc orphan() { return }\n")
	s, _ := NewServer(ServerOptions{BundleDir: dir, Name: "t"})

	res, _ := s.handleGetDeadCode(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"tier": "low"}}})
	m := decode(t, res)
	// orphan is high; a low filter yields no high candidates.
	for _, c := range asSlice(m["candidates"]) {
		if cm, _ := c.(map[string]any); cm["tier"] != "low" {
			t.Errorf("tier=low returned a %v candidate", cm["tier"])
		}
	}
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
