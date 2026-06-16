package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewServer(t *testing.T) {
	// Create a test bundle
	tmpDir, err := os.MkdirTemp("", "mcp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create index.md
	indexContent := `---
okf_version: "0.1"
---

# Test Bundle
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(indexContent), 0644); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	// Create a concept
	conceptContent := `---
type: "Guide"
title: "Test Concept"
description: "A test concept"
resource: "test"
tags:
  - test
timestamp: "2024-01-01T00:00:00.000Z"
---

# Test Concept

This is a test concept with some content.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(conceptContent), 0644); err != nil {
		t.Fatalf("failed to write concept: %v", err)
	}

	// Create server
	server, err := NewServer(ServerOptions{
		BundleDir:      tmpDir,
		Name:           "test-server",
		MaxResultChars: 1000,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Error("expected server to be non-nil")
	}

	if server.search == nil {
		t.Error("expected search to be initialized")
	}
}

func TestParseArgs(t *testing.T) {
	args := map[string]any{
		"query":  "test",
		"limit":  float64(10),
		"active": true,
	}

	// Test string extraction
	if v := getStringArg(args, "query"); v != "test" {
		t.Errorf("getStringArg(query) = %q, want %q", v, "test")
	}

	// Test missing key
	if v := getStringArg(args, "missing"); v != "" {
		t.Errorf("getStringArg(missing) = %q, want empty", v)
	}

	// Test float extraction
	if v := getFloatArg(args, "limit"); v != 10 {
		t.Errorf("getFloatArg(limit) = %f, want 10", v)
	}

	// Test wrong type
	if v := getStringArg(args, "limit"); v != "" {
		t.Errorf("getStringArg(limit) = %q, want empty", v)
	}
}

func getStringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getFloatArg(args map[string]any, key string) float64 {
	if v, ok := args[key].(float64); ok {
		return v
	}
	return 0
}
