package writer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/types"
)

func TestGenerateFrontmatter(t *testing.T) {
	doc := types.NormalizedDocument{
		Type:     "Guide",
		Title:    "Getting Started",
		Markdown: "# Getting Started\n\nThis is a guide.",
		Resource: "https://example.com/getting-started",
		Tags:     []string{"intro", "tutorial"},
	}

	fm := GenerateFrontmatter(doc, "2024-01-15T10:30:00.000Z")

	if !strings.Contains(fm, `type: "Guide"`) {
		t.Errorf("frontmatter missing type")
	}
	if !strings.Contains(fm, `title: "Getting Started"`) {
		t.Errorf("frontmatter missing title")
	}
	if !strings.Contains(fm, `resource: "https://example.com/getting-started"`) {
		t.Errorf("frontmatter missing resource")
	}
	if !strings.Contains(fm, `- "intro"`) {
		t.Errorf("frontmatter missing tag")
	}
	if !strings.Contains(fm, `timestamp: "2024-01-15T10:30:00.000Z"`) {
		t.Errorf("frontmatter missing timestamp")
	}
}

func TestSafeConceptOutputPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"getting-started.md", "getting-started.md"},
		{"index.md", "home.md"},
		{"log.md", "change-log.md"},
		{"docs/index.md", "docs/overview.md"},
		{"docs/log.md", "docs/change-log.md"},
		{"INDEX.MD", "home.md"},
	}

	for _, tt := range tests {
		got := safeConceptOutputPath(tt.input)
		if got != tt.want {
			t.Errorf("safeConceptOutputPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteOKFBundle(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "bundle")

	docs := []types.NormalizedDocument{
		{
			SourceID:   "intro",
			Title:      "Introduction",
			Markdown:   "# Introduction\n\nWelcome to the docs.",
			SourcePath: "intro.md",
			Tags:       []string{"intro"},
			Type:       "Guide",
		},
		{
			SourceID:   "api",
			Title:      "API Reference",
			Markdown:   "# API Reference\n\nSee [Introduction](./intro.md).",
			SourcePath: "api.md",
			Tags:       []string{"api"},
			Type:       "API Reference",
		},
	}

	opts := WriteOptions{
		OutDir:    outDir,
		Title:     "Test Bundle",
		Timestamp: "2024-01-01T00:00:00.000Z",
	}

	written, err := WriteOKFBundle(docs, opts)
	if err != nil {
		t.Fatalf("WriteOKFBundle failed: %v", err)
	}

	if len(written) != 2 {
		t.Errorf("wrote %d files, want 2", len(written))
	}

	// Check intro.md exists
	introContent, err := os.ReadFile(filepath.Join(outDir, "intro.md"))
	if err != nil {
		t.Fatalf("failed to read intro.md: %v", err)
	}
	if !strings.Contains(string(introContent), "---") {
		t.Errorf("intro.md missing frontmatter")
	}
	if !strings.Contains(string(introContent), `type: "Guide"`) {
		t.Errorf("intro.md missing type in frontmatter")
	}

	// Check index.md exists
	indexContent, err := os.ReadFile(filepath.Join(outDir, "index.md"))
	if err != nil {
		t.Fatalf("failed to read index.md: %v", err)
	}
	if !strings.Contains(string(indexContent), "okf_version") {
		t.Errorf("index.md missing okf_version")
	}
	if !strings.Contains(string(indexContent), "Test Bundle") {
		t.Errorf("index.md missing title")
	}
}

func TestWriteOKFBundleForceRequired(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "existing")

	// Create existing directory with content
	_ = os.MkdirAll(outDir, 0755)
	_ = os.WriteFile(filepath.Join(outDir, "existing.txt"), []byte("test"), 0644)

	docs := []types.NormalizedDocument{
		{
			SourceID:   "test",
			Title:      "Test",
			Markdown:   "# Test",
			SourcePath: "test.md",
			Type:       "Concept",
		},
	}

	opts := WriteOptions{
		OutDir: outDir,
		Force:  false,
	}

	_, err := WriteOKFBundle(docs, opts)
	if err == nil {
		t.Error("expected error when directory not empty and --force not set")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWithTitle(t *testing.T) {
	tests := []struct {
		title    string
		markdown string
		want     string
	}{
		{"Test", "# Existing Title\n\nContent", "# Existing Title\n\nContent"},
		{"Test", "Content without heading", "# Test\n\nContent without heading"},
		{"Test", "  # Spaced Title\n\nContent", "# Spaced Title\n\nContent"},
	}

	for _, tt := range tests {
		got := WithTitle(tt.title, tt.markdown)
		if got != tt.want {
			t.Errorf("WithTitle(%q, %q) = %q, want %q", tt.title, tt.markdown, got, tt.want)
		}
	}
}
