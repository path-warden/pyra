package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// codeStore builds a store root with a Go source file and a Canon decision that
// references the file's Widget symbol by symbol-id (for grounding tests).
func codeStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "pkg/sample.go", "package sample\n\nfunc Widget() int { return 1 }\n")
	// Widget is on line 3 -> id go:pkg/sample.go#Widget@3
	writeFile(t, root, "canon/adr-001.md", `---
schema_version: 1
id: OKF-0123456789AB
type: decision
---

# Widget Decision

## Status

Accepted

## Context

The Widget helper `+"`go:pkg/sample.go#Widget@3`"+` is load-bearing.

## Decision

We SHALL keep Widget pure.

## Consequences

Simplicity.
`)
	writeFile(t, root, "guides/x.md", "---\ntype: Guide\ntitle: X\n---\n\n# X\n\nbody\n")
	return root
}

func TestCodeIntel_SymbolsAndOutline(t *testing.T) {
	srv := createTestServer(t, codeStore(t))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"dir": "pkg", "kind": "function"}
	res, err := srv.handleSymbols(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Widget") || !strings.Contains(text, "go:pkg/sample.go#Widget@3") {
		t.Errorf("symbols missing Widget/id: %s", text)
	}
}

func TestCodeIntel_ToolFailureDoesNotCrash(t *testing.T) {
	srv := createTestServer(t, codeStore(t))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"file": "pkg/does-not-exist.go"}
	res, err := srv.handleOutline(context.Background(), req)
	if err != nil {
		t.Fatalf("handler should not return a Go error: %v", err)
	}
	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "error") {
		t.Errorf("expected an encoded error payload, got %s", text)
	}
}

func TestCodeIntel_CodeForArtifact_Grounding(t *testing.T) {
	srv := createTestServer(t, codeStore(t))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"id": "OKF-0123456789AB"}
	res, err := srv.handleCodeForArtifact(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	resolved, ok := m["resolved"].([]any)
	if !ok || len(resolved) == 0 {
		t.Fatalf("expected resolved symbols, got %v", m["resolved"])
	}
	first := resolved[0].(map[string]any)
	if !strings.Contains(first["source"].(string), "func Widget()") {
		t.Errorf("expected Widget source, got %v", first["source"])
	}
}

func TestCodeIntel_ArtifactsForSymbol_Grounding(t *testing.T) {
	srv := createTestServer(t, codeStore(t))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"id": "go:pkg/sample.go#Widget@3"}
	res, err := srv.handleArtifactsForSymbol(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	arts, ok := m["artifacts"].([]any)
	if !ok || len(arts) == 0 {
		t.Fatalf("expected artifacts referencing the symbol, got %v", m["artifacts"])
	}
	if arts[0].(map[string]any)["id"] != "OKF-0123456789AB" {
		t.Errorf("expected the Widget decision, got %v", arts[0])
	}
}
