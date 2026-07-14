package summarize

import (
	"fmt"
	"strings"
)

// Mode identifies the summarization strategy.
type Mode string

const (
	// ModeExtractive uses an embedded extractive algorithm (LSA, LexRank, etc.).
	ModeExtractive Mode = "extractive"
	// ModeLLM uses an external or local LLM to generate the summary.
	ModeLLM Mode = "llm"
)

// DefaultMode is the summarization mode used when none is supplied.
const DefaultMode = ModeExtractive

// DefaultAlgorithm is the extractive algorithm used when none is supplied.
const DefaultAlgorithm = "lsa"

// DefaultLanguage is the language used when none is supplied.
const DefaultLanguage = "english"

// Config configures a Summarizer.
type Config struct {
	Mode      Mode
	Algorithm string
	Language  string
	// BundlePath is consulted by LLM mode to locate llm.config inside the
	// bundle, and by Edmundson to locate edmundson.config.
	BundlePath string
	// EdmundsonConfigPath optionally overrides the standard search for
	// edmundson.config (bundle dir → ~/.config/pyra/).
	EdmundsonConfigPath string
}

// Summarizer generates a summary for a document's content.
type Summarizer interface {
	// Summarize returns a Summary for the given content and (optional) title.
	Summarize(content, title string) (Summary, error)
	// Name returns the summarizer's source identifier (e.g. "lsa", "llm").
	Name() string
}

// NewSummarizer constructs a Summarizer for the given configuration. For
// llm mode, an extractive fallback summarizer is also created and used when
// LLM generation fails.
func NewSummarizer(cfg Config) (Summarizer, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = DefaultMode
	}
	algorithm := cfg.Algorithm
	if algorithm == "" {
		algorithm = DefaultAlgorithm
	}
	language := cfg.Language
	if language == "" {
		language = DefaultLanguage
	}

	// Propagate resolved defaults so downstream callers see the same values.
	cfg.Mode = mode
	cfg.Algorithm = algorithm
	cfg.Language = language

	switch mode {
	case ModeExtractive:
		return NewExtractiveWithConfig(cfg)
	case ModeLLM:
		return newLLMOrFallback(cfg, algorithm, language)
	default:
		return nil, fmt.Errorf("unknown summarization mode %q (supported: %s, %s)", mode, ModeExtractive, ModeLLM)
	}
}

// ParseMode parses a mode string, returning DefaultMode for empty input.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return DefaultMode, nil
	case string(ModeExtractive):
		return ModeExtractive, nil
	case string(ModeLLM):
		return ModeLLM, nil
	default:
		return "", fmt.Errorf("unknown summarization mode %q", s)
	}
}

// newLLMOrFallback is implemented in llm_factory.go so we can keep the LLM
// dependencies isolated to that file.
