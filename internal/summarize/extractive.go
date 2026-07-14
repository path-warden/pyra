package summarize

import (
	"fmt"
	"strings"

	"github.com/chasedputnam/sumer"
	"github.com/chasedputnam/sumer/nlp"
	"github.com/chasedputnam/sumer/summarizers"
)

// ExtractiveAdapter adapts a sumer.Summarizer to the high-level Summarizer
// interface used elsewhere in pyra.
type ExtractiveAdapter struct {
	algorithm       string
	language        string
	tokenizer       *nlp.Tokenizer
	parser          *nlp.Parser
	inner           sumer.Summarizer
	edmundsonConfig *EdmundsonConfig // optional; populated when algorithm == "edmundson"
}

// NewExtractive builds an ExtractiveAdapter for the given algorithm and
// language. It is a convenience wrapper around NewExtractiveWithConfig.
func NewExtractive(algorithm, language string) (*ExtractiveAdapter, error) {
	return NewExtractiveWithConfig(Config{
		Mode:      ModeExtractive,
		Algorithm: algorithm,
		Language:  language,
	})
}

// NewExtractiveWithConfig builds an ExtractiveAdapter from a full Config,
// loading algorithm-specific configuration (currently only Edmundson) from
// disk when needed.
func NewExtractiveWithConfig(cfg Config) (*ExtractiveAdapter, error) {
	canonical := sumer.CanonicalAlgorithm(cfg.Algorithm)
	if canonical == "" {
		canonical = DefaultAlgorithm
	}
	language := cfg.Language
	if language == "" {
		language = DefaultLanguage
	}
	s, err := sumer.NewSummarizer(canonical, language)
	if err != nil {
		return nil, err
	}
	tok := nlp.NewTokenizer(language)
	adapter := &ExtractiveAdapter{
		algorithm: canonical,
		language:  language,
		tokenizer: tok,
		parser:    nlp.NewParser(tok),
		inner:     s,
	}

	if canonical == "edmundson" {
		ec, err := LoadEdmundsonConfig(cfg.BundlePath, cfg.EdmundsonConfigPath)
		if err != nil {
			return nil, fmt.Errorf("loading edmundson config: %w", err)
		}
		adapter.edmundsonConfig = ec
	}

	return adapter, nil
}

// Name returns the algorithm identifier (e.g. "lsa").
func (e *ExtractiveAdapter) Name() string { return e.algorithm }

// Summarize parses content, runs the configured extractive algorithm, picks
// the single highest-scoring sentence, and truncates to MaxSummaryLength.
func (e *ExtractiveAdapter) Summarize(content, title string) (Summary, error) {
	if strings.TrimSpace(content) == "" {
		return Summary{Source: SourceNone}, nil
	}
	doc := e.parser.Parse(content)

	// Configure algorithm-specific runtime state.
	if ed, ok := e.inner.(*summarizers.EdmundsonSummarizer); ok {
		if strings.TrimSpace(title) != "" {
			ed.SetTitle(title)
		}
		if e.edmundsonConfig != nil {
			ed.BonusWords = stringSet(e.edmundsonConfig.Bonus)
			ed.StigmaWords = stringSet(e.edmundsonConfig.Stigma)
			ed.NullWords = stringSet(e.edmundsonConfig.Null)
		}
	}

	sentences := e.inner.Summarize(doc, 1)
	if len(sentences) == 0 {
		return Summary{Source: SourceNone}, nil
	}

	text := strings.TrimSpace(sentences[0].Text())
	if text == "" {
		return Summary{Source: SourceNone}, nil
	}

	text = TruncateAtWord(text, MaxSummaryLength)
	return Summary{
		Text:   text,
		Source: e.algorithm,
	}, nil
}

// stringSet converts a slice of words to a set for fast lookup.
func stringSet(words []string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, w := range words {
		out[w] = true
	}
	return out
}

// TruncateAtWord truncates text to maxLen, breaking at a word boundary if
// reasonable. Appends "..." when truncation occurs.
func TruncateAtWord(text string, maxLen int) string {
	return truncateAtWord(text, maxLen)
}
