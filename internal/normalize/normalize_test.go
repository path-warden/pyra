package normalize

import (
	"testing"
	"time"

	"github.com/chasedputnam/pyra/internal/types"
)

func TestHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "simple html",
			html:      "<html><body><h1>Hello</h1><p>World</p></body></html>",
			wantTitle: "Hello",
			wantBody:  "Hello",
		},
		{
			name:      "extracts from main",
			html:      "<html><body><nav>Nav</nav><main><h1>Main Content</h1><p>Body</p></main><footer>Footer</footer></body></html>",
			wantTitle: "Main Content",
			wantBody:  "Main Content",
		},
		{
			name:      "removes scripts",
			html:      "<html><body><script>alert('x')</script><h1>Title</h1></body></html>",
			wantTitle: "Title",
			wantBody:  "Title",
		},
		{
			name:      "title from title tag",
			html:      "<html><head><title>Page Title</title></head><body><p>Content</p></body></html>",
			wantTitle: "Page Title",
			wantBody:  "Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markdown, title := HTMLToMarkdown(tt.html)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if len(markdown) == 0 {
				t.Errorf("markdown is empty")
			}
			if tt.wantBody != "" && !contains(markdown, tt.wantBody) {
				t.Errorf("markdown = %q, want to contain %q", markdown, tt.wantBody)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExtractHeadings(t *testing.T) {
	markdown := `# Title
Some content
## Section One
More content
### Subsection
#### Deep
`
	headings := ExtractHeadings(markdown)
	if len(headings) != 4 {
		t.Fatalf("len(headings) = %d, want 4", len(headings))
	}
	if headings[0].Depth != 1 || headings[0].Text != "Title" {
		t.Errorf("headings[0] = %+v, want depth=1, text=Title", headings[0])
	}
	if headings[1].Depth != 2 || headings[1].Text != "Section One" {
		t.Errorf("headings[1] = %+v, want depth=2, text=Section One", headings[1])
	}
}

func TestExtractMarkdownLinks(t *testing.T) {
	markdown := `Check [this link](./other.md) and [external](https://example.com).
Also [titled link](./page.md "Title").`

	links := ExtractMarkdownLinks(markdown)
	if len(links) != 3 {
		t.Fatalf("len(links) = %d, want 3", len(links))
	}
	if links[0].Text != "this link" || links[0].Href != "./other.md" {
		t.Errorf("links[0] = %+v", links[0])
	}
	if links[1].Text != "external" || links[1].Href != "https://example.com" {
		t.Errorf("links[1] = %+v", links[1])
	}
}

func TestInferType(t *testing.T) {
	tests := []struct {
		title    string
		sourceID string
		markdown string
		want     string
	}{
		{"README", "README.md", "# README", "README"},
		{"API Reference", "api.md", "endpoint parameter request", "API Reference"},
		{"Getting Started", "start.md", "quickstart guide tutorial", "Guide"},
		{"Docs", "docs.md", "documentation page", "Documentation Page"},
		{"Something", "file.md", "random content", "Concept"},
	}

	for _, tt := range tests {
		got := InferType(tt.title, tt.sourceID, tt.markdown)
		if got != tt.want {
			t.Errorf("InferType(%q, %q, ...) = %q, want %q", tt.title, tt.sourceID, got, tt.want)
		}
	}
}

func TestInferTags(t *testing.T) {
	headings := []types.Heading{
		{Text: "Authentication"},
		{Text: "OAuth Setup"},
	}
	tags := InferTags("Getting Started", "auth/setup.md", headings)

	if len(tags) == 0 {
		t.Error("expected tags, got none")
	}
	if len(tags) > 6 {
		t.Errorf("len(tags) = %d, want <= 6", len(tags))
	}
}

func TestDescriptionFromMarkdown(t *testing.T) {
	markdown := `---
type: Guide
---

# Title

This is the first paragraph of content that should be extracted for the description.

## Section

More content here.`

	desc := DescriptionFromMarkdown(markdown)
	if desc == "" {
		t.Error("description is empty")
	}
	if len(desc) > 180 {
		t.Errorf("description too long: %d chars", len(desc))
	}
	if !contains(desc, "first paragraph") {
		t.Errorf("description = %q, want to contain 'first paragraph'", desc)
	}
}

func TestNormalizeDocument(t *testing.T) {
	raw := types.RawDocument{
		SourceID:     "test",
		URL:          "https://example.com/docs/getting-started",
		ContentType:  types.ContentTypeHTML,
		Raw:          "<html><body><h1>Getting Started</h1><p>Welcome to our guide.</p></body></html>",
		DiscoveredAt: time.Now(),
	}

	doc := NormalizeDocument(raw)

	if doc.Title != "Getting Started" {
		t.Errorf("Title = %q, want 'Getting Started'", doc.Title)
	}
	if doc.Type == "" {
		t.Error("Type is empty")
	}
	if len(doc.Markdown) == 0 {
		t.Error("Markdown is empty")
	}
	if doc.Resource != raw.URL {
		t.Errorf("Resource = %q, want %q", doc.Resource, raw.URL)
	}
}

func TestNormalizeDocumentMarkdown(t *testing.T) {
	raw := types.RawDocument{
		SourceID:     "readme",
		FilePath:     "docs/README.md",
		ContentType:  types.ContentTypeMarkdown,
		Raw:          "# Project README\n\nThis is the readme file.\n\n## Installation\n\nRun npm install.",
		DiscoveredAt: time.Now(),
	}

	doc := NormalizeDocument(raw)

	if doc.Title != "Project README" {
		t.Errorf("Title = %q, want 'Project README'", doc.Title)
	}
	if doc.Type != "README" {
		t.Errorf("Type = %q, want 'README'", doc.Type)
	}
	if len(doc.Headings) != 2 {
		t.Errorf("len(Headings) = %d, want 2", len(doc.Headings))
	}
}

func TestNormalizeDocumentText(t *testing.T) {
	raw := types.RawDocument{
		SourceID:     "config",
		FilePath:     "config.txt",
		ContentType:  types.ContentTypeText,
		Raw:          "key=value\nother=setting",
		DiscoveredAt: time.Now(),
	}

	doc := NormalizeDocument(raw)

	if !contains(doc.Markdown, "```text") {
		t.Error("text content should be wrapped in code block")
	}
}
