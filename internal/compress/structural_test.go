package compress

import (
	"testing"
)

func TestStructural(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "collapse blank lines",
			input:    "line1\n\n\n\nline2",
			expected: "line1\n\nline2",
		},
		{
			name:     "remove trailing whitespace",
			input:    "line1   \nline2\t\t",
			expected: "line1\nline2",
		},
		{
			name:     "normalize asterisk list markers",
			input:    "* item1\n* item2",
			expected: "- item1\n- item2",
		},
		{
			name:     "normalize plus list markers",
			input:    "+ item1\n+ item2",
			expected: "- item1\n- item2",
		},
		{
			name:     "preserve dash list markers",
			input:    "- item1\n- item2",
			expected: "- item1\n- item2",
		},
		{
			name:     "preserve indented lists",
			input:    "  * nested\n    * deep",
			expected: "  - nested\n    - deep",
		},
		{
			name:     "preserve code blocks",
			input:    "```\n* not a list\n+ also not\n```",
			expected: "```\n* not a list\n+ also not\n```",
		},
		{
			name:     "convert tabs to spaces",
			input:    "\tindented",
			expected: "  indented",
		},
		{
			name: "preserve code block indentation",
			input: "```go\n\tindented code\n```",
			expected: "```go\n\tindented code\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Structural(tt.input)
			if result != tt.expected {
				t.Errorf("Structural() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCollapseBlankLines(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a\n\nb", "a\n\nb"},           // two newlines = one blank line, keep
		{"a\n\n\nb", "a\n\nb"},         // three newlines, reduce
		{"a\n\n\n\n\nb", "a\n\nb"},     // many newlines, reduce
		{"no blanks", "no blanks"},
	}

	for _, tt := range tests {
		result := collapseBlankLines(tt.input)
		if result != tt.expected {
			t.Errorf("collapseBlankLines(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRemoveTrailingWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello   ", "hello"},
		{"line1  \nline2\t", "line1\nline2"},
		{"  leading", "  leading"}, // preserve leading
		{"no trailing", "no trailing"},
	}

	for _, tt := range tests {
		result := removeTrailingWhitespace(tt.input)
		if result != tt.expected {
			t.Errorf("removeTrailingWhitespace(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeListMarkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "asterisk to dash",
			input:    "* item",
			expected: "- item",
		},
		{
			name:     "plus to dash",
			input:    "+ item",
			expected: "- item",
		},
		{
			name:     "preserve in code",
			input:    "```\n* code\n```",
			expected: "```\n* code\n```",
		},
		{
			name:     "mixed content",
			input:    "text\n* list\n```\n* code\n```\n+ more",
			expected: "text\n- list\n```\n* code\n```\n- more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeListMarkers(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeListMarkers() = %q, want %q", result, tt.expected)
			}
		})
	}
}
