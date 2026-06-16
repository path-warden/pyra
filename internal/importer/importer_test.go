package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImport(t *testing.T) {
	// Create test input directory
	inputDir, err := os.MkdirTemp("", "import-input-*")
	if err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}
	defer os.RemoveAll(inputDir)

	// Create test output directory
	outputDir, err := os.MkdirTemp("", "import-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create test files
	testFiles := map[string]string{
		"readme.md": "# README\n\nThis is a readme file.",
		"docs/guide.md": `# Getting Started

This is a guide for getting started.

## Installation

Run the install command.
`,
		"docs/api.md": "# API Reference\n\nAPI documentation here.",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(inputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	// Run import
	result, err := Import(ImportOptions{
		InputPath:  inputDir,
		OutDir:     outputDir,
		SourceName: "Test Docs",
		Force:      true,
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify results
	if len(result.Documents) != 3 {
		t.Errorf("got %d documents, want 3", len(result.Documents))
	}

	// Check that output files exist
	for _, written := range result.Written {
		fullPath := filepath.Join(outputDir, written)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", written)
		}
	}

	// Check index.md exists
	indexPath := filepath.Join(outputDir, "index.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index.md: %v", err)
	}
	if !strings.Contains(string(content), "okf_version") {
		t.Error("index.md should contain okf_version")
	}
}

func TestImportWithExclude(t *testing.T) {
	// Create test input directory
	inputDir, err := os.MkdirTemp("", "import-exclude-*")
	if err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}
	defer os.RemoveAll(inputDir)

	// Create test output directory
	outputDir, err := os.MkdirTemp("", "import-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create test files
	testFiles := map[string]string{
		"readme.md":     "# README",
		"internal.md":   "# Internal",
		"docs/guide.md": "# Guide",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(inputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	// Run import with exclude pattern
	result, err := Import(ImportOptions{
		InputPath: inputDir,
		OutDir:    outputDir,
		Exclude:   []string{"internal.md"},
		Force:     true,
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Should have 2 documents (internal.md excluded)
	if len(result.Documents) != 2 {
		t.Errorf("got %d documents, want 2", len(result.Documents))
	}
}

func TestImportNoFiles(t *testing.T) {
	// Create empty input directory
	inputDir, err := os.MkdirTemp("", "import-empty-*")
	if err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}
	defer os.RemoveAll(inputDir)

	outputDir, err := os.MkdirTemp("", "import-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Run import - should fail
	_, err = Import(ImportOptions{
		InputPath: inputDir,
		OutDir:    outputDir,
	})
	if err == nil {
		t.Error("expected error for empty directory")
	}
}
