package compress

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	content := `---
title: Test Doc
type: Guide
---

# Introduction

This is the introduction section with some content.

## Section One

Content for section one with details about the topic.

## Section Two

Content for section two with more information.

## Section Three

Final section with concluding remarks.
`

	tests := []struct {
		name        string
		budget      int
		wantTrunc   bool
		wantSection string // expected truncatedAt
	}{
		{"large budget no truncation", 1000, false, ""},
		{"small budget truncates", 30, true, ""},
		{"zero budget no change", 0, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(content, TruncateOptions{
				TokenBudget:         tt.budget,
				PreserveFrontmatter: true,
			})
			if result.Truncated != tt.wantTrunc {
				t.Errorf("Truncate() truncated = %v, want %v", result.Truncated, tt.wantTrunc)
			}
		})
	}
}

func TestTruncatePreservesFrontmatter(t *testing.T) {
	content := `---
title: Important Doc
type: Reference
---

# Main Content

Lots of content here that might get truncated.
`

	result := Truncate(content, TruncateOptions{
		TokenBudget:         20,
		PreserveFrontmatter: true,
	})

	if !strings.HasPrefix(result.Content, "---") {
		t.Error("Expected frontmatter to be preserved")
	}
	if !strings.Contains(result.Content, "title: Important Doc") {
		t.Error("Expected frontmatter content to be preserved")
	}
}

func TestTruncateWithIndicator(t *testing.T) {
	content := `# Title

## Section 1
Content one.

## Section 2
Content two.

## Section 3
Content three.
`

	result := Truncate(content, TruncateOptions{
		TokenBudget:  15,
		AddIndicator: true,
	})

	if result.Truncated && !strings.Contains(result.Content, "[Content truncated") {
		t.Error("Expected truncation indicator when AddIndicator=true")
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		wantFrontmatter string
		wantBody        string
	}{
		{
			"with frontmatter",
			"---\ntitle: Test\n---\n# Body",
			"---\ntitle: Test\n---",
			"# Body",
		},
		{
			"no frontmatter",
			"# Just Body",
			"",
			"# Just Body",
		},
		{
			"unclosed frontmatter",
			"---\ntitle: Test\n# Body",
			"",
			"---\ntitle: Test\n# Body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := splitFrontmatter(tt.content)
			if fm != tt.wantFrontmatter {
				t.Errorf("frontmatter = %q, want %q", fm, tt.wantFrontmatter)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestParseSections(t *testing.T) {
	content := `# Title

Introduction.

## Section One

Content one.

## Section Two

Content two.
`

	sections := parseSections(content)
	
	if len(sections) != 3 {
		t.Errorf("Expected 3 sections, got %d", len(sections))
	}

	if sections[0].Title != "Title" {
		t.Errorf("First section title = %q, want 'Title'", sections[0].Title)
	}
	if sections[0].Level != 1 {
		t.Errorf("First section level = %d, want 1", sections[0].Level)
	}

	if sections[1].Title != "Section One" {
		t.Errorf("Second section title = %q, want 'Section One'", sections[1].Title)
	}
	if sections[1].Level != 2 {
		t.Errorf("Second section level = %d, want 2", sections[1].Level)
	}
}

func TestGenerateSectionOutline(t *testing.T) {
	content := `---
title: Doc
---

# Main Title

Intro.

## Overview

Overview content.

### Details

Details content.

## Conclusion

Final words.
`

	outline := GenerateSectionOutline(content)
	
	if len(outline) != 4 {
		t.Errorf("Expected 4 outline items, got %d: %v", len(outline), outline)
	}

	expected := []string{"Main Title", "  Overview", "    Details", "  Conclusion"}
	for i, want := range expected {
		if i >= len(outline) {
			break
		}
		if outline[i] != want {
			t.Errorf("outline[%d] = %q, want %q", i, outline[i], want)
		}
	}
}
