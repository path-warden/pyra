package compress

import (
	"strings"
	"testing"
)

func TestCompress(t *testing.T) {
	content := `---
title: Test
---

# Main Title


Some content here.   

* Item one
* Item two

## Section Two

More content.
`

	tests := []struct {
		name        string
		level       Level
		budget      int
		wantSmaller bool
	}{
		{"none", LevelNone, 0, false},
		{"light", LevelLight, 0, true},
		{"medium no budget", LevelMedium, 0, true},
		{"medium with budget", LevelMedium, 20, true},
		{"aggressive with budget", LevelAggressive, 20, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Compress(content, Options{
				Level:               tt.level,
				TokenBudget:         tt.budget,
				PreserveFrontmatter: true,
			})

			if result.OriginalTokens == 0 {
				t.Error("OriginalTokens should not be 0")
			}

			if tt.wantSmaller && result.CompressedTokens > result.OriginalTokens {
				t.Errorf("Expected compression, got %d >= %d", result.CompressedTokens, result.OriginalTokens)
			}

			if tt.level == LevelNone && result.Content != content {
				t.Error("LevelNone should not modify content")
			}
		})
	}
}

func TestCompressStructuralChanges(t *testing.T) {
	content := "Line one   \n\n\n\nLine two\n* Item\n"

	result := Compress(content, Options{Level: LevelLight})

	// Should collapse blank lines
	if strings.Contains(result.Content, "\n\n\n") {
		t.Error("Should collapse multiple blank lines")
	}

	// Should remove trailing whitespace
	if strings.Contains(result.Content, "   \n") {
		t.Error("Should remove trailing whitespace")
	}

	// Should normalize list markers
	if strings.Contains(result.Content, "* ") {
		t.Error("Should normalize list markers to dash")
	}
}

func TestCompressTruncation(t *testing.T) {
	content := `# Title

## Section One

This is section one content.

## Section Two

This is section two content.

## Section Three

This is section three content.
`

	result := Compress(content, Options{
		Level:       LevelMedium,
		TokenBudget: 15,
	})

	if !result.Truncated {
		t.Error("Expected truncation with small budget")
	}

	// Content should be shorter
	if len(result.Content) >= len(content) {
		t.Error("Truncated content should be shorter")
	}
}

func TestCompressJSON(t *testing.T) {
	data := map[string]any{
		"title": "Test",
		"items": []string{"one", "two", "three"},
	}

	_, result := CompressJSON(data, Options{Level: LevelLight})

	if result.OriginalTokens == 0 {
		t.Error("Should count tokens in JSON")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"none", LevelNone},
		{"light", LevelLight},
		{"medium", LevelMedium},
		{"aggressive", LevelAggressive},
		{"invalid", LevelLight},
		{"", LevelLight},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Level != LevelLight {
		t.Errorf("Default level should be LevelLight, got %q", opts.Level)
	}
	if opts.TokenBudget != 0 {
		t.Errorf("Default budget should be 0, got %d", opts.TokenBudget)
	}
	if !opts.PreserveFrontmatter {
		t.Error("Default should preserve frontmatter")
	}
}
