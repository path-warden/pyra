package reader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBundle(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create a concept file
	conceptContent := `---
type: "Guide"
title: "Test Concept"
description: "A test concept"
resource: "https://example.com/test"
tags:
  - test
  - example
timestamp: "2024-01-01T00:00:00.000Z"
---

# Test Concept

This is a test concept with a [link](./other.md).
`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(conceptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another concept
	otherContent := `---
type: "API Reference"
title: "Other Concept"
---

# Other Concept

Content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "other.md"), []byte(otherContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an index file (should be skipped)
	indexContent := `---
okf_version: "0.1"
---

# Index

- [Test](./test.md)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read the bundle
	concepts, err := ReadBundle(tmpDir)
	if err != nil {
		t.Fatalf("ReadBundle failed: %v", err)
	}

	// Should have 2 concepts (test and other), indexed by both ID and path
	// So 4 entries total
	if len(concepts) != 4 {
		t.Errorf("Expected 4 entries (2 concepts x 2 keys), got %d", len(concepts))
	}

	// Check test concept
	testConcept := concepts["test"]
	if testConcept == nil {
		t.Fatal("test concept not found")
	}
	if testConcept.Type != "Guide" {
		t.Errorf("Type = %q, want %q", testConcept.Type, "Guide")
	}
	if testConcept.Title != "Test Concept" {
		t.Errorf("Title = %q, want %q", testConcept.Title, "Test Concept")
	}
	if len(testConcept.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(testConcept.Tags))
	}

	// Check other concept
	otherConcept := concepts["other"]
	if otherConcept == nil {
		t.Fatal("other concept not found")
	}
	if otherConcept.Type != "API Reference" {
		t.Errorf("Type = %q, want %q", otherConcept.Type, "API Reference")
	}
}

func TestIsConceptMarkdownPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test.md", true},
		{"foo/bar.md", true},
		{"index.md", false},
		{"INDEX.MD", false},
		{"foo/index.md", false},
		{"log.md", false},
		{"test.txt", false},
		{"readme.md", true},
	}

	for _, tt := range tests {
		got := IsConceptMarkdownPath(tt.path)
		if got != tt.want {
			t.Errorf("IsConceptMarkdownPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
