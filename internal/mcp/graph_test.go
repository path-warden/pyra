package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func graphBundleServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	writeGit(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	writeGit(t, dir, "store/store.go", "package store\n\nfunc Put() {}\n")
	writeGit(t, dir, "app/app.go", "package app\n\nfunc Use() { Put() }\n")
	writeGit(t, dir, "util/util.go", "package util\n\nfunc helper() {}\n")
	s, err := NewServer(ServerOptions{BundleDir: dir, Name: "t"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func TestMCPGraphCentrality(t *testing.T) {
	s := graphBundleServer(t)
	res, err := s.handleGraphCentrality(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, res)
	if m["available"] != true {
		t.Fatalf("available = %v, want true", m["available"])
	}
	cent, _ := m["centrality"].([]any)
	if len(cent) == 0 {
		t.Fatalf("expected centrality rows, got %v", m["centrality"])
	}
	// Equivalent to the underlying graph (what the CLI renders).
	if s.graphIndex().NodeCount() == 0 {
		t.Error("graph should have nodes")
	}
	// Lazy build happened once and is cached.
	if s.graph == nil {
		t.Error("graph should be cached after first use")
	}
}

func TestMCPGraphCommunitiesAndCycles(t *testing.T) {
	s := graphBundleServer(t)

	rc, _ := s.handleCommunities(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if decode(t, rc)["available"] != true {
		t.Error("communities should be available")
	}

	ry, _ := s.handleCycles(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	m := decode(t, ry)
	if m["available"] != true {
		t.Error("cycles should be available")
	}
	// This acyclic fixture has no cycles.
	if cy, _ := m["cycles"].([]any); len(cy) != 0 {
		t.Errorf("acyclic fixture should report no cycles, got %v", cy)
	}
}
