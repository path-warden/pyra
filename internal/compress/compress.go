// Package compress provides content compression strategies for reducing token usage.
package compress

import (
	"encoding/json"

	"github.com/chasedputnam/pyra/internal/tokens"
)

// Level defines compression aggressiveness.
type Level string

const (
	LevelNone       Level = "none"
	LevelLight      Level = "light"      // Structural only
	LevelMedium     Level = "medium"     // Structural + truncation
	LevelAggressive Level = "aggressive" // All strategies
)

// Options configures compression behavior.
type Options struct {
	Level               Level
	TokenBudget         int // 0 = no limit
	PreserveFrontmatter bool
}

// Result contains compressed content and metadata.
type Result struct {
	Content          string
	OriginalTokens   int
	CompressedTokens int
	Truncated        bool
	TruncatedAt      string // Section where truncation occurred
}

// DefaultOptions returns sensible default compression options.
func DefaultOptions() Options {
	return Options{
		Level:               LevelLight,
		TokenBudget:         0,
		PreserveFrontmatter: true,
	}
}

// estimator is the shared token estimator.
var estimator = tokens.NewEstimator()

// Compress applies compression strategies to markdown content.
func Compress(content string, opts Options) Result {
	originalTokens := estimator.Count(content)

	if opts.Level == LevelNone {
		return Result{
			Content:          content,
			OriginalTokens:   originalTokens,
			CompressedTokens: originalTokens,
			Truncated:        false,
		}
	}

	result := content

	// Light: Apply structural compression
	if opts.Level == LevelLight || opts.Level == LevelMedium || opts.Level == LevelAggressive {
		result = Structural(result)
	}

	// Medium/Aggressive: Apply truncation if budget specified
	truncated := false
	truncatedAt := ""
	if (opts.Level == LevelMedium || opts.Level == LevelAggressive) && opts.TokenBudget > 0 {
		truncResult := Truncate(result, TruncateOptions{
			TokenBudget:         opts.TokenBudget,
			PreserveFrontmatter: opts.PreserveFrontmatter,
			AddIndicator:        opts.Level == LevelAggressive,
		})
		result = truncResult.Content
		truncated = truncResult.Truncated
		truncatedAt = truncResult.TruncatedAt
	}

	compressedTokens := estimator.Count(result)

	return Result{
		Content:          result,
		OriginalTokens:   originalTokens,
		CompressedTokens: compressedTokens,
		Truncated:        truncated,
		TruncatedAt:      truncatedAt,
	}
}

// CompressJSON compresses JSON tool responses by removing whitespace.
func CompressJSON(v any, opts Options) (any, Result) {
	// Serialize to compact JSON
	data, err := json.Marshal(v)
	if err != nil {
		return v, Result{Content: "", OriginalTokens: 0, CompressedTokens: 0}
	}

	originalTokens := estimator.Count(string(data))

	if opts.Level == LevelNone {
		return v, Result{
			Content:          string(data),
			OriginalTokens:   originalTokens,
			CompressedTokens: originalTokens,
		}
	}

	// For JSON, we can't easily apply markdown compression
	// Just return the compact JSON representation
	compressedTokens := originalTokens

	return v, Result{
		Content:          string(data),
		OriginalTokens:   originalTokens,
		CompressedTokens: compressedTokens,
	}
}

// ParseLevel parses a compression level string.
func ParseLevel(s string) Level {
	switch s {
	case "none":
		return LevelNone
	case "light":
		return LevelLight
	case "medium":
		return LevelMedium
	case "aggressive":
		return LevelAggressive
	default:
		return LevelLight
	}
}
