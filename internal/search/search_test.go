package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okfy/okf-mcp/internal/types"
)

func TestBundleSearch(t *testing.T) {
	// Create a test bundle
	tmpDir, err := os.MkdirTemp("", "search-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create concept files
	concept1 := `---
type: "Guide"
title: "Getting Started"
description: "Learn how to get started"
resource: "https://example.com/getting-started"
tags:
  - tutorial
  - beginner
timestamp: "2024-01-01T00:00:00.000Z"
---

# Getting Started

This is a tutorial for beginners to learn the basics.
`

	concept2 := `---
type: "API Reference"
title: "API Endpoints"
description: "API endpoint documentation"
resource: "https://example.com/api"
tags:
  - api
  - reference
timestamp: "2024-01-01T00:00:00.000Z"
---

# API Endpoints

Documentation for the REST API endpoints.
`

	if err := os.WriteFile(filepath.Join(tmpDir, "getting-started.md"), []byte(concept1), 0644); err != nil {
		t.Fatalf("failed to write concept1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "api.md"), []byte(concept2), 0644); err != nil {
		t.Fatalf("failed to write concept2: %v", err)
	}

	// Create index.md
	indexContent := `---
okf_version: "0.1"
---

# Test Bundle

- [Getting Started](getting-started.md)
- [API Endpoints](api.md)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(indexContent), 0644); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	// Create search index
	search, err := NewBundleSearch(tmpDir)
	if err != nil {
		t.Fatalf("failed to create search: %v", err)
	}

	// Test search for "tutorial"
	results := search.Search("tutorial", SearchOptions{Limit: 10})
	if len(results) == 0 {
		t.Error("expected results for 'tutorial', got none")
	}
	if len(results) > 0 && results[0].ID != "getting-started" {
		t.Errorf("expected first result to be 'getting-started', got %q", results[0].ID)
	}

	// Test search for "API"
	results = search.Search("API", SearchOptions{Limit: 10})
	if len(results) == 0 {
		t.Error("expected results for 'API', got none")
	}

	// Test search with type filter
	results = search.Search("documentation", SearchOptions{Type: "API Reference", Limit: 10})
	for _, r := range results {
		if r.Type != "API Reference" {
			t.Errorf("expected type 'API Reference', got %q", r.Type)
		}
	}

	// Test GetConcept
	concept := search.GetConcept("getting-started")
	if concept == nil {
		t.Error("expected to find concept 'getting-started'")
	}
	if concept != nil && concept.Title != "Getting Started" {
		t.Errorf("expected title 'Getting Started', got %q", concept.Title)
	}

	// Test GetConcept by path
	concept = search.GetConcept("api.md")
	if concept == nil {
		t.Error("expected to find concept 'api.md'")
	}
}

func TestSnippet(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		query   string
		wantLen int
	}{
		{
			name:    "short text",
			body:    "Hello world",
			query:   "hello",
			wantLen: 11,
		},
		{
			name:    "long text with match",
			body:    "The quick brown fox jumps over the lazy dog. This is a test sentence that contains the word tutorial in the middle somewhere. More text follows after that.",
			query:   "tutorial",
			wantLen: 240,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			concept := &types.Concept{Body: tt.body}
			result := snippet(concept, tt.query, 240)
			if len(result) > tt.wantLen {
				t.Errorf("snippet too long: got %d, want max %d", len(result), tt.wantLen)
			}
		})
	}
}
