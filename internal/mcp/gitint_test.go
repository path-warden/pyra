package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func gitAt(t *testing.T, root string, ts int64, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	date := "@" + strconv.FormatInt(ts, 10) + " +0000"
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@e",
		"GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@e",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeGit(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitBundleServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	writeGit(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	gitAt(t, dir, 1, "init")
	for _, f := range []string{"internal/cool.go", "internal/util.go", "cmd/main.go", "docs/readme.md"} {
		writeGit(t, dir, f, "package p\n")
	}
	gitAt(t, dir, 1000, "add", ".")
	gitAt(t, dir, 1000, "commit", "-m", "seed")
	for i := 0; i < 8; i++ {
		writeGit(t, dir, "internal/hot.go", "package p\n//"+strconv.Itoa(i)+"\n")
		gitAt(t, dir, int64(2000+i), "add", ".")
		gitAt(t, dir, int64(2000+i), "commit", "-m", "hot")
	}
	s, err := NewServer(ServerOptions{BundleDir: dir, Name: "t"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s
}

func decode(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m
}

func TestMCPGetHotspots(t *testing.T) {
	s := gitBundleServer(t)
	if s.git == nil {
		t.Fatal("server should have built a git index over the git bundle")
	}
	res, err := s.handleGetHotspots(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, res)
	if m["available"] != true {
		t.Fatalf("available = %v, want true", m["available"])
	}
	hot, _ := m["hotspots"].([]any)
	if len(hot) == 0 {
		t.Fatalf("expected hotspots, got %v", m["hotspots"])
	}
	first, _ := hot[0].(map[string]any)
	if first["path"] != "internal/hot.go" {
		t.Errorf("top hotspot = %v, want internal/hot.go", first["path"])
	}

	// Equivalence with the underlying index (same as the CLI would render).
	if s.git.Hotspots()[0].Path != "internal/hot.go" {
		t.Error("MCP result should match the index the CLI uses")
	}
}

func TestMCPGetOwnership_FileAndModules(t *testing.T) {
	s := gitBundleServer(t)

	// File ownership.
	res, _ := s.handleGetOwnership(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": "internal/hot.go"}}})
	m := decode(t, res)
	own, _ := m["ownership"].(map[string]any)
	if own["primary_owner"] != "Ann" {
		t.Errorf("owner = %v, want Ann", own["primary_owner"])
	}

	// No path → module rollups.
	res2, _ := s.handleGetOwnership(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	m2 := decode(t, res2)
	if mods, _ := m2["modules"].([]any); len(mods) == 0 {
		t.Error("expected module rollups for empty path")
	}
}

func TestMCPGitIntel_UnavailableWhenNoGit(t *testing.T) {
	// A bundle that is not a git repo → git index nil → tools report unavailable.
	dir := t.TempDir()
	writeGit(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	s, err := NewServer(ServerOptions{BundleDir: dir, Name: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if s.git != nil {
		t.Fatal("non-git bundle should leave git index nil")
	}
	res, err := s.handleGetHotspots(context.Background(),
		mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if decode(t, res)["available"] != false {
		t.Error("nil git index should report available=false, not crash")
	}
}
