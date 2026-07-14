package differ

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chasedputnam/pyra/internal/types"
)

func TestDiffBundlesAddedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing bundle with one file
	existingContent := `---
type: Concept
title: Existing
---
# Existing

Content here.
`
	_ = os.WriteFile(filepath.Join(tmpDir, "existing.md"), []byte(existingContent), 0644)

	// New docs include existing and a new file
	newDocs := []types.NormalizedDocument{
		{OutputPath: "existing.md", Markdown: "# Existing\n\nContent here."},
		{OutputPath: "new-file.md", Markdown: "# New File\n\nNew content."},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Errorf("Expected 1 added file, got %d", len(result.Added))
	}
	if len(result.Added) > 0 && result.Added[0].Path != "new-file.md" {
		t.Errorf("Expected added file 'new-file.md', got '%s'", result.Added[0].Path)
	}
	if len(result.Modified) != 0 {
		t.Errorf("Expected 0 modified files, got %d", len(result.Modified))
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Expected 0 deleted files, got %d", len(result.Deleted))
	}
}

func TestDiffBundlesModifiedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing bundle
	existingContent := `---
type: Concept
title: Test
---
# Test

Old content.
`
	_ = os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(existingContent), 0644)

	// New doc has different content
	newDocs := []types.NormalizedDocument{
		{OutputPath: "test.md", Markdown: "# Test\n\nNew content that is different."},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	if len(result.Added) != 0 {
		t.Errorf("Expected 0 added files, got %d", len(result.Added))
	}
	if len(result.Modified) != 1 {
		t.Errorf("Expected 1 modified file, got %d", len(result.Modified))
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Expected 0 deleted files, got %d", len(result.Deleted))
	}
}

func TestDiffBundlesDeletedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing bundle with two files
	_ = os.WriteFile(filepath.Join(tmpDir, "keep.md"), []byte("---\ntype: Concept\n---\n# Keep"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "delete.md"), []byte("---\ntype: Concept\n---\n# Delete"), 0644)

	// New docs only have one file
	newDocs := []types.NormalizedDocument{
		{OutputPath: "keep.md", Markdown: "# Keep"},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	if len(result.Added) != 0 {
		t.Errorf("Expected 0 added files, got %d", len(result.Added))
	}
	if len(result.Modified) != 0 {
		t.Errorf("Expected 0 modified files, got %d", len(result.Modified))
	}
	if len(result.Deleted) != 1 {
		t.Errorf("Expected 1 deleted file, got %d", len(result.Deleted))
	}
	if len(result.Deleted) > 0 && result.Deleted[0].Path != "delete.md" {
		t.Errorf("Expected deleted file 'delete.md', got '%s'", result.Deleted[0].Path)
	}
}

func TestDiffBundlesIgnoreTimestampChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing bundle with timestamp
	existingContent := `---
type: Concept
title: Test
timestamp: "2024-01-01T00:00:00Z"
---
# Test

Same content.
`
	_ = os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(existingContent), 0644)

	// New doc has same body content but different frontmatter would be generated
	newDocs := []types.NormalizedDocument{
		{OutputPath: "test.md", Markdown: "# Test\n\nSame content."},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	// Should not be marked as modified since body content is the same
	if len(result.Modified) != 0 {
		t.Errorf("Expected 0 modified files (timestamp-only change should be ignored), got %d", len(result.Modified))
	}
}

func TestDiffBundlesSkipsReservedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing bundle with reserved files
	_ = os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("# Index"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "log.md"), []byte("# Log"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "concept.md"), []byte("---\ntype: Concept\n---\n# Concept"), 0644)

	// New docs don't include reserved files
	newDocs := []types.NormalizedDocument{
		{OutputPath: "concept.md", Markdown: "# Concept"},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	// Reserved files should not appear as deleted
	if len(result.Deleted) != 0 {
		t.Errorf("Expected 0 deleted files (reserved files should be skipped), got %d", len(result.Deleted))
		for _, d := range result.Deleted {
			t.Logf("Deleted: %s", d.Path)
		}
	}
}

func TestDiffBundlesSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing bundle with subdirectory
	_ = os.MkdirAll(filepath.Join(tmpDir, "guides"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "guides", "intro.md"), []byte("---\ntype: Guide\n---\n# Intro"), 0644)

	// New docs include file in subdirectory
	newDocs := []types.NormalizedDocument{
		{OutputPath: "guides/intro.md", Markdown: "# Intro"},
		{OutputPath: "guides/advanced.md", Markdown: "# Advanced"},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Errorf("Expected 1 added file, got %d", len(result.Added))
	}
	if len(result.Added) > 0 && result.Added[0].Path != "guides/advanced.md" {
		t.Errorf("Expected added file 'guides/advanced.md', got '%s'", result.Added[0].Path)
	}
}

func TestHasChanges(t *testing.T) {
	result := &DiffResult{}
	if result.HasChanges() {
		t.Error("Empty result should not have changes")
	}

	result.Added = append(result.Added, FileChange{Path: "test.md"})
	if !result.HasChanges() {
		t.Error("Result with added file should have changes")
	}
}

func TestSummary(t *testing.T) {
	result := &DiffResult{}
	if result.Summary() != "No changes" {
		t.Errorf("Expected 'No changes', got '%s'", result.Summary())
	}

	result.Added = append(result.Added, FileChange{Path: "a.md"})
	result.Modified = append(result.Modified, FileChange{Path: "b.md"}, FileChange{Path: "c.md"})
	result.Deleted = append(result.Deleted, FileChange{Path: "d.md"})

	summary := result.Summary()
	if summary != "1 added, 2 modified, 1 deleted" {
		t.Errorf("Unexpected summary: '%s'", summary)
	}
}

// TestDiffBundlesIgnoresInjectedSummaryCallout pins the differ contract:
// the bundle stores files with an injected `> [!summary]` callout, while
// freshly-fetched source markdown does not. When the underlying body is
// unchanged the diff must report 0 modified files. Before the fix, the
// callout caused every file to appear modified on every update.
func TestDiffBundlesIgnoresInjectedSummaryCallout(t *testing.T) {
	tmpDir := t.TempDir()

	bundleContent := `---
type: Concept
title: Test
---
# Test

> [!summary]
> Old summary text generated at import time.

Body content that has not changed.
`
	_ = os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(bundleContent), 0644)

	// Fresh source has no callout; body is byte-identical to bundle body.
	newDocs := []types.NormalizedDocument{
		{OutputPath: "test.md", Markdown: "# Test\n\nBody content that has not changed.\n"},
	}

	result, err := DiffBundles(tmpDir, newDocs)
	if err != nil {
		t.Fatalf("DiffBundles failed: %v", err)
	}

	if len(result.Modified) != 0 {
		t.Errorf("expected 0 modified (callout should be stripped before compare), got %d: %+v",
			len(result.Modified), result.Modified)
	}
	if len(result.Added) != 0 || len(result.Deleted) != 0 {
		t.Errorf("expected 0 added/deleted, got added=%d deleted=%d", len(result.Added), len(result.Deleted))
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"# No frontmatter", "# No frontmatter"},
		{"---\ntype: Test\n---\n# With frontmatter", "# With frontmatter"},
		{"---\ntype: Test\ntitle: Hello\n---\n\n# Content", "\n# Content"},
		{"---incomplete", "---incomplete"},
	}

	for _, tt := range tests {
		result := stripFrontmatter(tt.input)
		if result != tt.expected {
			t.Errorf("stripFrontmatter(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
