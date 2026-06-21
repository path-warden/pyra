package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func createTestBundle(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "mcp-updates-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

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

	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

func createTestServer(t *testing.T, bundleDir string) *Server {
	t.Helper()
	server, err := NewServer(ServerOptions{
		BundleDir:      bundleDir,
		Name:           "test-server",
		MaxResultChars: 1000,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	return server
}

func TestHandleCheckUpdates_NoSource(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	server := createTestServer(t, bundleDir)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := server.handleCheckUpdates(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCheckUpdates failed: %v", err)
	}

	// Should return error since no changelog with source
	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["error"] == nil {
		t.Error("expected error response for missing source")
	}

	errMap := response["error"].(map[string]any)
	if errMap["code"] != "no_source" {
		t.Errorf("expected code=no_source, got %v", errMap["code"])
	}
}

func TestHandleCheckUpdates_WithChangelog(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	// Create changelog with source (first line is raw source)
	changelogContent := bundleDir + `
2024-01-01T00:00:00Z - Initial creation
`
	if err := os.WriteFile(filepath.Join(bundleDir, "changelog.txt"), []byte(changelogContent), 0644); err != nil {
		t.Fatalf("failed to write changelog: %v", err)
	}

	server := createTestServer(t, bundleDir)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"timeout_seconds": float64(10),
	}

	result, err := server.handleCheckUpdates(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCheckUpdates failed: %v", err)
	}

	var response CheckUpdatesResult
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify response has expected fields
	if response.SourceURL != bundleDir {
		t.Errorf("expected source_url=%s, got %s", bundleDir, response.SourceURL)
	}
}

func TestHandleApplyUpdates_RequiresConfirm(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	server := createTestServer(t, bundleDir)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := server.handleApplyUpdates(context.Background(), req)
	if err != nil {
		t.Fatalf("handleApplyUpdates failed: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["error"] == nil {
		t.Error("expected error response for missing confirmation")
	}

	errMap := response["error"].(map[string]any)
	if errMap["code"] != "confirmation_required" {
		t.Errorf("expected code=confirmation_required, got %v", errMap["code"])
	}
}

func TestHandleApplyUpdates_DryRun(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	// Create changelog with source (first line is raw source)
	changelogContent := bundleDir + `
2024-01-01T00:00:00Z - Initial creation
`
	if err := os.WriteFile(filepath.Join(bundleDir, "changelog.txt"), []byte(changelogContent), 0644); err != nil {
		t.Fatalf("failed to write changelog: %v", err)
	}

	server := createTestServer(t, bundleDir)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"dry_run": true,
	}

	result, err := server.handleApplyUpdates(context.Background(), req)
	if err != nil {
		t.Fatalf("handleApplyUpdates failed: %v", err)
	}

	var response ApplyUpdatesResult
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("expected success=true for dry run")
	}
}

func TestHandleBundleHealth_Valid(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	server := createTestServer(t, bundleDir)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"check_source": false,
	}

	result, err := server.handleBundleHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBundleHealth failed: %v", err)
	}

	var response BundleHealthResult
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !response.Valid {
		t.Errorf("expected valid=true, got false with warnings: %v", response.Warnings)
	}

	if response.ConceptCount < 1 {
		t.Errorf("expected at least 1 concept, got %d", response.ConceptCount)
	}
}

func TestHandleBundleHealth_WithSource(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	// Create changelog with source (first line is raw source)
	changelogContent := `https://example.com/docs
2024-01-01T00:00:00Z - Initial creation
`
	if err := os.WriteFile(filepath.Join(bundleDir, "changelog.txt"), []byte(changelogContent), 0644); err != nil {
		t.Fatalf("failed to write changelog: %v", err)
	}

	server := createTestServer(t, bundleDir)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"check_source": true,
	}

	result, err := server.handleBundleHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBundleHealth failed: %v", err)
	}

	var response BundleHealthResult
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.SourceURL != "https://example.com/docs" {
		t.Errorf("expected source_url=https://example.com/docs, got %s", response.SourceURL)
	}

	// source_reachable should be set (false since example.com won't have /docs)
	if response.SourceReachable == nil {
		t.Error("expected source_reachable to be set")
	}
}

func TestReloadSearchIndex(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	server := createTestServer(t, bundleDir)

	// Add a new concept file
	newConcept := `---
type: "Guide"
title: "New Concept"
description: "A new concept"
resource: "new"
tags:
  - new
timestamp: "2024-01-02T00:00:00.000Z"
---

# New Concept

This is a new concept.
`
	if err := os.WriteFile(filepath.Join(bundleDir, "new.md"), []byte(newConcept), 0644); err != nil {
		t.Fatalf("failed to write new concept: %v", err)
	}

	// Reload index
	if err := server.reloadSearchIndex(); err != nil {
		t.Fatalf("reloadSearchIndex failed: %v", err)
	}

	// Verify new concept is searchable
	srch := server.getSearch()
	concept := srch.GetConcept("new")
	if concept == nil {
		t.Error("expected new concept to be found after reload")
	}
}

func TestReloadSearchIndex_KeepsOldOnFailure(t *testing.T) {
	bundleDir, cleanup := createTestBundle(t)
	defer cleanup()

	server := createTestServer(t, bundleDir)

	// Get reference to old search
	oldSearch := server.getSearch()

	// Corrupt the bundle by removing all files
	os.RemoveAll(bundleDir)

	// Reload should fail since directory is gone
	err := server.reloadSearchIndex()
	if err == nil {
		t.Error("expected reloadSearchIndex to fail with missing bundle")
	}

	// Old search should still be valid
	currentSearch := server.getSearch()
	if currentSearch != oldSearch {
		t.Error("expected old search to be preserved on reload failure")
	}
}
