// Package tokens provides token estimation for LLM context management.
package tokens

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// Estimator provides token count estimation for text.
type Estimator struct {
	enc  *tiktoken.Tiktoken
	once sync.Once
	err  error
}

// encodingName is the tiktoken encoding used (cl100k_base for GPT-4/Claude).
const encodingName = "cl100k_base"

// fallbackRatio is chars per token when tiktoken unavailable.
const fallbackRatio = 4

// NewEstimator creates a token estimator.
func NewEstimator() *Estimator {
	return &Estimator{}
}

// init lazily initializes the tiktoken encoder.
func (e *Estimator) init() {
	e.once.Do(func() {
		e.enc, e.err = tiktoken.GetEncoding(encodingName)
	})
}

// Count returns estimated token count for text.
func (e *Estimator) Count(text string) int {
	e.init()
	if e.enc == nil {
		return e.fallbackCount(text)
	}
	tokens := e.enc.Encode(text, nil, nil)
	return len(tokens)
}

// CountJSON returns estimated tokens for JSON-serialized data.
func (e *Estimator) CountJSON(v any) int {
	data, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return e.Count(string(data))
}

// Truncate truncates text to fit within token budget.
// Returns truncated text and whether truncation occurred.
func (e *Estimator) Truncate(text string, budget int) (string, bool) {
	if budget <= 0 {
		return text, false
	}

	e.init()
	if e.enc == nil {
		return e.fallbackTruncate(text, budget)
	}

	tokens := e.enc.Encode(text, nil, nil)
	if len(tokens) <= budget {
		return text, false
	}

	// Truncate tokens and decode back
	truncated := tokens[:budget]
	result := e.enc.Decode(truncated)
	return result, true
}

// TruncateToSection truncates text at section boundaries to fit budget.
// Prefers truncating at ## or ### headers for cleaner breaks.
func (e *Estimator) TruncateToSection(text string, budget int) (string, bool) {
	if budget <= 0 {
		return text, false
	}

	currentTokens := e.Count(text)
	if currentTokens <= budget {
		return text, false
	}

	// Find section boundaries (## headers)
	lines := strings.Split(text, "\n")
	var result strings.Builder
	var lastGoodBreak int

	for i, line := range lines {
		testText := result.String() + line + "\n"
		tokens := e.Count(testText)

		if tokens > budget {
			// Use last good break point if we have one
			if lastGoodBreak > 0 {
				breakLines := lines[:lastGoodBreak]
				return strings.Join(breakLines, "\n"), true
			}
			// Otherwise truncate at current position
			if result.Len() > 0 {
				return strings.TrimRight(result.String(), "\n"), true
			}
			// Fall back to token-level truncation
			return e.Truncate(text, budget)
		}

		result.WriteString(line)
		result.WriteString("\n")

		// Track section boundaries as good break points
		if strings.HasPrefix(strings.TrimSpace(line), "##") {
			lastGoodBreak = i
		}
	}

	return text, false
}

// fallbackCount estimates tokens using character ratio.
func (e *Estimator) fallbackCount(text string) int {
	return (len(text) + fallbackRatio - 1) / fallbackRatio
}

// fallbackTruncate truncates using character ratio.
func (e *Estimator) fallbackTruncate(text string, budget int) (string, bool) {
	maxChars := budget * fallbackRatio
	if len(text) <= maxChars {
		return text, false
	}
	return text[:maxChars], true
}

// Available returns true if tiktoken encoding is available.
func (e *Estimator) Available() bool {
	e.init()
	return e.enc != nil
}
