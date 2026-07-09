package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/memphis/internal/codegraph"
)

func writeGraphFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func graphRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeGraphFile(t, root, "store/store.go", "package store\n\nfunc Put() {}\n")
	writeGraphFile(t, root, "app/app.go", "package app\n\nfunc Use() { Put() }\n")
	writeGraphFile(t, root, "util/util.go", "package util\n\nfunc helper() {}\n")
	return root
}

func TestRunGraph_CentralityJSON(t *testing.T) {
	root := graphRepo(t)
	out := captureStdout(t, func() {
		graphCmd.Flags().Set("json", "true")
		if err := runGraph(graphCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		graphCmd.Flags().Set("json", "false")
	})
	var payload struct {
		Total      int                    `json:"total"`
		Centrality []codegraph.Centrality `json:"centrality"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("centrality JSON did not parse: %v\n%s", err, out)
	}
	if payload.Total == 0 || len(payload.Centrality) == 0 {
		t.Errorf("expected centrality rows, got %+v", payload)
	}
}

func TestRunGraph_ReachabilityText(t *testing.T) {
	root := graphRepo(t)
	out := captureStdout(t, func() {
		graphCmd.Flags().Set("reachability", "true")
		if err := runGraph(graphCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		graphCmd.Flags().Set("reachability", "false")
	})
	// helper is private + unreferenced → reported unreachable.
	if !strings.Contains(out, "unreachable") || !strings.Contains(out, "helper") {
		t.Errorf("reachability text should list helper unreachable:\n%s", out)
	}
}

func TestRunGraph_NodeCapTruncationSignalled(t *testing.T) {
	root := graphRepo(t)
	out := captureStdout(t, func() {
		graphCmd.Flags().Set("node-cap", "1")
		if err := runGraph(graphCmd, []string{root}); err != nil {
			t.Fatal(err)
		}
		graphCmd.Flags().Set("node-cap", "0")
	})
	if !strings.Contains(out, "truncated") {
		t.Errorf("node-cap truncation should be signalled:\n%s", out)
	}
}
