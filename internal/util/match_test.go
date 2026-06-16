package util

import "testing"

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		value   string
		pattern string
		want    bool
	}{
		// Glob patterns
		{"foo.md", "*.md", true},
		{"foo.txt", "*.md", false},
		{"docs/foo.md", "docs/*.md", true},
		{"docs/sub/foo.md", "docs/**/*.md", true},
		{"foo.md", "foo.*", true},

		// Regex patterns
		{"https://example.com/api/v1", "/api/", true},
		{"https://example.com/docs", "/api/", false},
		{"/docs/getting-started", "/^/docs/", true},
		{"foo123bar", "/\\d+/", true},
		{"foobar", "/\\d+/", false},

		// Invalid patterns
		{"foo", "/[/", false}, // invalid regex
	}

	for _, tt := range tests {
		got := MatchesPattern(tt.value, tt.pattern)
		if got != tt.want {
			t.Errorf("MatchesPattern(%q, %q) = %v, want %v", tt.value, tt.pattern, got, tt.want)
		}
	}
}

func TestMatchesAnyPattern(t *testing.T) {
	tests := []struct {
		value    string
		patterns []string
		want     bool
	}{
		{"foo.md", []string{"*.md", "*.txt"}, true},
		{"foo.txt", []string{"*.md", "*.txt"}, true},
		{"foo.go", []string{"*.md", "*.txt"}, false},
		{"foo.md", []string{}, false},
		{"foo.md", nil, false},
		{"/api/users", []string{"/api/", "*.md"}, true},
	}

	for _, tt := range tests {
		got := MatchesAnyPattern(tt.value, tt.patterns)
		if got != tt.want {
			t.Errorf("MatchesAnyPattern(%q, %v) = %v, want %v", tt.value, tt.patterns, got, tt.want)
		}
	}
}
