package summarize

import (
	"github.com/chasedputnam/pyra/internal/summarize/llm"
)

// newLLMOrFallback constructs an LLM-backed summarizer with an extractive
// fallback. The extractive fallback is invoked if LLM generation fails for a
// given document (e.g., provider error, timeout, content too long).
func newLLMOrFallback(cfg Config, algorithm, language string) (Summarizer, error) {
	fallback, err := NewExtractive(algorithm, language)
	if err != nil {
		return nil, err
	}
	engine, err := llm.NewEngine(cfg.BundlePath)
	if err != nil {
		// Engine construction failure means we can't build any LLM at all —
		// return an LLM summarizer that always falls back so callers get
		// extractive output and a warning.
		return &llmSummarizer{engine: nil, fallback: fallback, initErr: err}, nil
	}
	return &llmSummarizer{engine: engine, fallback: fallback}, nil
}

// llmSummarizer adapts an LLM engine to the Summarizer interface, falling
// back to an extractive summarizer when the LLM cannot produce a summary.
type llmSummarizer struct {
	engine   *llm.Engine
	fallback *ExtractiveAdapter
	initErr  error
}

// Name returns "llm" when the engine is functional, otherwise the fallback's name.
func (s *llmSummarizer) Name() string {
	if s.engine == nil {
		return s.fallback.Name()
	}
	return "llm"
}

// Summarize runs the LLM engine and falls back to extractive on failure.
func (s *llmSummarizer) Summarize(content, title string) (Summary, error) {
	if s.engine == nil {
		return s.fallback.Summarize(content, title)
	}
	text, err := s.engine.Summarize(content, title)
	if err != nil {
		return s.fallback.Summarize(content, title)
	}
	text = TruncateAtWord(text, MaxSummaryLength)
	return Summary{Text: text, Source: "llm"}, nil
}
