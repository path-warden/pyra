package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okfy/okf-mcp/internal/changelog"
	"github.com/okfy/okf-mcp/internal/differ"
)

func TestIsURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"https://example.com/path/to/docs", true},
		{"/local/path", false},
		{"./relative/path", false},
		{"file:///path", false},
		{"ftp://example.com", false},
	}

	for _, tt := range tests {
		got := isURL(tt.input)
		if got != tt.want {
			t.Errorf("isURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestUpdateBundleNotFound(t *testing.T) {
	_, err := Update(context.Background(), UpdateOptions{
		BundlePath: "/nonexistent/bundle",
	})
	if err == nil {
		t.Error("expected error for nonexistent bundle")
	}
	if !strings.Contains(err.Error(), "bundle not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateNoSource(t *testing.T) {
	// Create a bundle without changelog
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty bundle dir
	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err = Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
	})
	if err == nil {
		t.Error("expected error when no source specified")
	}
	if !strings.Contains(err.Error(), "no source specified") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateDryRun(t *testing.T) {
	// Create a bundle with changelog and one file
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source directory with a file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc.md"), []byte("# New Doc\n\nContent"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun to be true")
	}
	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}

	// Verify no files were actually created
	files, _ := filepath.Glob(filepath.Join(bundleDir, "*.md"))
	if len(files) > 0 {
		t.Errorf("dry run should not create files, found: %v", files)
	}
}

func TestUpdateForceMode(t *testing.T) {
	// Create a bundle with an existing file
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing file in bundle
	existingContent := `---
type: "Guide"
title: "Existing"
description: "Old content"
resource: "existing.md"
tags: []
timestamp: "2024-01-01T00:00:00Z"
---
# Existing

Old content here`
	if err := os.WriteFile(filepath.Join(bundleDir, "existing.md"), []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create source directory with modified file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "existing.md"), []byte("# Existing\n\nNew content here"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	// Track if prompt was called
	promptCalled := false
	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		Force:      true,
		OnPrompt: func(changeType differ.ChangeType, files []differ.FileChange) (bool, bool, bool) {
			promptCalled = true
			return true, false, false
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if promptCalled {
		t.Error("Force mode should not call OnPrompt")
	}
	if result.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", result.Modified)
	}

	// Verify file was updated
	content, _ := os.ReadFile(filepath.Join(bundleDir, "existing.md"))
	if !strings.Contains(string(content), "New content here") {
		t.Error("file should have been updated with new content")
	}
}

func TestUpdateWithPromptSkip(t *testing.T) {
	// Create a bundle with an existing file
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing file in bundle
	existingContent := `---
type: "Guide"
title: "Existing"
description: "Old content"
resource: "existing.md"
tags: []
timestamp: "2024-01-01T00:00:00Z"
---
# Existing

Old content here`
	if err := os.WriteFile(filepath.Join(bundleDir, "existing.md"), []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create source directory with modified file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "existing.md"), []byte("# Existing\n\nNew content here"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	// Decline changes via prompt
	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		OnPrompt: func(changeType differ.ChangeType, files []differ.FileChange) (bool, bool, bool) {
			return false, false, false // decline change
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
	if result.Modified != 0 {
		t.Errorf("expected 0 modified, got %d", result.Modified)
	}

	// Verify file was NOT updated
	content, _ := os.ReadFile(filepath.Join(bundleDir, "existing.md"))
	if strings.Contains(string(content), "New content here") {
		t.Error("file should not have been updated")
	}
}

func TestUpdateWithPromptCancel(t *testing.T) {
	// Create a bundle with an existing file
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing file in bundle
	existingContent := `---
type: "Guide"
title: "Existing"
description: "Old content"
resource: "existing.md"
tags: []
timestamp: "2024-01-01T00:00:00Z"
---
# Existing

Old content here`
	if err := os.WriteFile(filepath.Join(bundleDir, "existing.md"), []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create source directory with modified file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "existing.md"), []byte("# Existing\n\nNew content here"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	// Cancel update via prompt
	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		OnPrompt: func(changeType differ.ChangeType, files []differ.FileChange) (bool, bool, bool) {
			return false, false, true // cancel
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Modified != 0 {
		t.Errorf("expected 0 modified after cancel, got %d", result.Modified)
	}
}

func TestUpdateAdditions(t *testing.T) {
	// Create empty bundle
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source directory with new files
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc1.md"), []byte("# Doc 1\n\nContent 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc2.md"), []byte("# Doc 2\n\nContent 2"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Added != 2 {
		t.Errorf("expected 2 added, got %d", result.Added)
	}

	// Verify files were created
	if _, err := os.Stat(filepath.Join(bundleDir, "doc1.md")); os.IsNotExist(err) {
		t.Error("doc1.md should have been created")
	}
	if _, err := os.Stat(filepath.Join(bundleDir, "doc2.md")); os.IsNotExist(err) {
		t.Error("doc2.md should have been created")
	}
}

func TestUpdateDeletions(t *testing.T) {
	// Create bundle with files
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing files in bundle
	existingContent := `---
type: "Guide"
title: "To Delete"
description: "Will be deleted"
resource: "to-delete.md"
tags: []
timestamp: "2024-01-01T00:00:00Z"
---
# To Delete

Content`
	if err := os.WriteFile(filepath.Join(bundleDir, "to-delete.md"), []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create source directory with a different file (so to-delete.md becomes a deletion)
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Add a file to source so it's not empty
	keepContent := "# Keep This\n\nThis file stays."
	if err := os.WriteFile(filepath.Join(sourceDir, "keep.md"), []byte(keepContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Also add keep.md to bundle so it's not seen as an addition
	keepBundleContent := `---
type: "Guide"
title: "Keep This"
description: "This file stays"
resource: "keep.md"
tags: []
timestamp: "2024-01-01T00:00:00Z"
---
# Keep This

This file stays.`
	if err := os.WriteFile(filepath.Join(bundleDir, "keep.md"), []byte(keepBundleContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", result.Deleted)
	}

	// Verify file was deleted
	if _, err := os.Stat(filepath.Join(bundleDir, "to-delete.md")); !os.IsNotExist(err) {
		t.Error("to-delete.md should have been deleted")
	}

	// Verify keep.md still exists
	if _, err := os.Stat(filepath.Join(bundleDir, "keep.md")); err != nil {
		t.Error("keep.md should still exist")
	}
}

func TestUpdateNoChanges(t *testing.T) {
	// Create bundle with a file
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	existingContent := `---
type: "Guide"
title: "Same"
description: "Same content"
resource: "same.md"
tags: []
timestamp: "2024-01-01T00:00:00Z"
---
# Same

Same content`
	if err := os.WriteFile(filepath.Join(bundleDir, "same.md"), []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create source directory with identical content (ignoring frontmatter)
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "same.md"), []byte("# Same\n\nSame content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Added != 0 || result.Modified != 0 || result.Deleted != 0 {
		t.Errorf("expected no changes, got added=%d, modified=%d, deleted=%d", 
			result.Added, result.Modified, result.Deleted)
	}
}

func TestUpdateSourceOverride(t *testing.T) {
	// Create bundle with changelog pointing to wrong source
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to nonexistent source
	if err := changelog.CreateChangelog(bundleDir, "/nonexistent/source", 1); err != nil {
		t.Fatal(err)
	}

	// Create actual source directory
	sourceDir := filepath.Join(tmpDir, "actual-source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc.md"), []byte("# Doc\n\nContent"), 0644); err != nil {
		t.Fatal(err)
	}

	// Override source
	result, err := Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
}

func TestUpdateProgressCallback(t *testing.T) {
	// Create bundle
	tmpDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source directory with a file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc.md"), []byte("# Doc\n\nContent"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create changelog pointing to source
	if err := changelog.CreateChangelog(bundleDir, sourceDir, 1); err != nil {
		t.Fatal(err)
	}

	// Track progress phases
	phases := []string{}
	_, err = Update(context.Background(), UpdateOptions{
		BundlePath: bundleDir,
		OnProgress: func(phase string, message string) {
			phases = append(phases, phase)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have progress for fetching, diffing, and applying
	if len(phases) < 2 {
		t.Errorf("expected at least 2 progress phases, got %d: %v", len(phases), phases)
	}
}
