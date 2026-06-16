package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestContentTypeConstants(t *testing.T) {
	tests := []struct {
		ct   ContentType
		want string
	}{
		{ContentTypeHTML, "html"},
		{ContentTypeMarkdown, "markdown"},
		{ContentTypeMDX, "mdx"},
		{ContentTypeText, "text"},
	}
	for _, tt := range tests {
		if string(tt.ct) != tt.want {
			t.Errorf("ContentType = %q, want %q", tt.ct, tt.want)
		}
	}
}

func TestValidationReportJSON(t *testing.T) {
	report := ValidationReport{
		Valid: false,
		Issues: []ValidationIssue{
			{Severity: "error", Code: "missing_type", Message: "type required", Path: "foo.md"},
		},
		ConceptCount:      10,
		ReservedFileCount: 2,
		WarningCount:      1,
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ValidationReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Valid != report.Valid {
		t.Errorf("Valid = %v, want %v", decoded.Valid, report.Valid)
	}
	if decoded.ConceptCount != report.ConceptCount {
		t.Errorf("ConceptCount = %d, want %d", decoded.ConceptCount, report.ConceptCount)
	}
	if len(decoded.Issues) != 1 {
		t.Fatalf("len(Issues) = %d, want 1", len(decoded.Issues))
	}
	if decoded.Issues[0].Code != "missing_type" {
		t.Errorf("Issues[0].Code = %q, want %q", decoded.Issues[0].Code, "missing_type")
	}
}

func TestBundleStatsJSON(t *testing.T) {
	stats := BundleStats{
		Title:            "Test Bundle",
		ConceptCount:     5,
		LinkCount:        10,
		BrokenLinks:      2,
		OrphanConcepts:   []string{"orphan1"},
		TypeDistribution: map[string]int{"Guide": 3, "API Reference": 2},
		TagDistribution:  map[string]int{"api": 5},
		TopLinkedConcepts: []LinkedConcept{
			{ID: "intro", Title: "Introduction", Count: 5},
		},
		SourceDomains: map[string]int{"docs.example.com": 5},
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded BundleStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Title != stats.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, stats.Title)
	}
	if decoded.LinkCount != stats.LinkCount {
		t.Errorf("LinkCount = %d, want %d", decoded.LinkCount, stats.LinkCount)
	}
	if decoded.TypeDistribution["Guide"] != 3 {
		t.Errorf("TypeDistribution[Guide] = %d, want 3", decoded.TypeDistribution["Guide"])
	}
}

func TestSearchResultJSON(t *testing.T) {
	result := SearchResult{
		ID:          "getting-started",
		Title:       "Getting Started",
		Type:        "Guide",
		Description: "Learn how to get started",
		Tags:        []string{"intro", "tutorial"},
		Resource:    "https://docs.example.com/getting-started",
		Snippet:     "This guide helps you...",
		Score:       0.95,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded SearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != result.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, result.ID)
	}
	if decoded.Score != result.Score {
		t.Errorf("Score = %f, want %f", decoded.Score, result.Score)
	}
	if len(decoded.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(decoded.Tags))
	}
}

func TestRawDocumentFields(t *testing.T) {
	now := time.Now()
	doc := RawDocument{
		SourceID:     "test-doc",
		URL:          "https://example.com/doc",
		FilePath:     "",
		ContentType:  ContentTypeHTML,
		Raw:          "<html><body>Test</body></html>",
		DiscoveredAt: now,
	}

	if doc.SourceID != "test-doc" {
		t.Errorf("SourceID = %q, want %q", doc.SourceID, "test-doc")
	}
	if doc.ContentType != ContentTypeHTML {
		t.Errorf("ContentType = %q, want %q", doc.ContentType, ContentTypeHTML)
	}
	if doc.DiscoveredAt != now {
		t.Errorf("DiscoveredAt mismatch")
	}
}

func TestNormalizedDocumentFields(t *testing.T) {
	doc := NormalizedDocument{
		SourceID:   "test",
		Title:      "Test Title",
		Markdown:   "# Test\n\nContent here",
		Resource:   "https://example.com/test",
		SourcePath: "docs/test.md",
		OutputPath: "test.md",
		Headings:   []Heading{{Depth: 1, Text: "Test", Slug: "test"}},
		Links:      []Link{{Href: "./other.md", Text: "Other"}},
		Tags:       []string{"test", "example"},
		Type:       "Guide",
	}

	if doc.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Test Title")
	}
	if len(doc.Headings) != 1 {
		t.Errorf("len(Headings) = %d, want 1", len(doc.Headings))
	}
	if doc.Headings[0].Depth != 1 {
		t.Errorf("Headings[0].Depth = %d, want 1", doc.Headings[0].Depth)
	}
}

func TestConceptFields(t *testing.T) {
	concept := Concept{
		ID:   "intro",
		Path: "intro.md",
		Frontmatter: map[string]any{
			"type":  "Guide",
			"title": "Introduction",
		},
		Type:        "Guide",
		Title:       "Introduction",
		Description: "An introduction",
		Resource:    "https://example.com/intro",
		Tags:        []string{"intro"},
		Body:        "# Introduction\n\nWelcome!",
	}

	if concept.ID != "intro" {
		t.Errorf("ID = %q, want %q", concept.ID, "intro")
	}
	if concept.Frontmatter["type"] != "Guide" {
		t.Errorf("Frontmatter[type] = %v, want %q", concept.Frontmatter["type"], "Guide")
	}
}

func TestKnowledgeGraphFields(t *testing.T) {
	concept1 := &Concept{ID: "a", Path: "a.md"}
	concept2 := &Concept{ID: "b", Path: "b.md"}

	graph := KnowledgeGraph{
		Concepts:  map[string]*Concept{"a": concept1, "b": concept2},
		Outbound:  map[string][]string{"a": {"b"}},
		Backlinks: map[string][]string{"b": {"a"}},
	}

	if len(graph.Concepts) != 2 {
		t.Errorf("len(Concepts) = %d, want 2", len(graph.Concepts))
	}
	if len(graph.Outbound["a"]) != 1 || graph.Outbound["a"][0] != "b" {
		t.Errorf("Outbound[a] = %v, want [b]", graph.Outbound["a"])
	}
	if len(graph.Backlinks["b"]) != 1 || graph.Backlinks["b"][0] != "a" {
		t.Errorf("Backlinks[b] = %v, want [a]", graph.Backlinks["b"])
	}
}
