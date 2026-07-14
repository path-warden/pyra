// Package llm implements LLM-backed summarization with external API, local
// platform-native, and Ollama providers.
package llm

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/chasedputnam/pyra/internal/tokens"
)

// DefaultTimeout is the per-file timeout for LLM summarization.
const DefaultTimeout = 30 * time.Second

// DefaultPromptTemplate is used when llm.config does not supply one.
const DefaultPromptTemplate = `Summarize the following document in one concise sentence (max 200 characters).
Focus on the main topic and key takeaway.

Title: {{.Title}}

Content:
{{.Content}}`

// MaxContentTokens is the token budget for rendered document content sent to
// the LLM. Matches the spec's 8000-token cap. The remaining budget (~200
// tokens) is reserved for the prompt template and title.
const MaxContentTokens = 8000

// Provider abstracts an LLM source.
type Provider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
	Available() bool
}

// Engine orchestrates the configured providers.
type Engine struct {
	config    *Config
	provider  Provider
	tmpl      *template.Template
	timeout   time.Duration
	estimator *tokens.Estimator
}

// NewEngine constructs an Engine, loading config from bundlePath (or user
// config directory). If no config can be loaded, the defaults are used.
func NewEngine(bundlePath string) (*Engine, error) {
	cfg, err := LoadConfig(bundlePath)
	if err != nil {
		return nil, err
	}

	prov, err := selectProvider(cfg)
	if err != nil {
		return nil, err
	}

	templateText := strings.TrimSpace(cfg.PromptTemplate)
	if templateText == "" {
		templateText = DefaultPromptTemplate
	}
	tmpl, err := template.New("prompt").Parse(templateText)
	if err != nil {
		return nil, fmt.Errorf("invalid prompt_template: %w", err)
	}

	return &Engine{
		config:    cfg,
		provider:  prov,
		tmpl:      tmpl,
		timeout:   DefaultTimeout,
		estimator: tokens.NewEstimator(),
	}, nil
}

// ProviderName returns the active provider's name (e.g., "api", "ollama").
func (e *Engine) ProviderName() string {
	if e.provider == nil {
		return ""
	}
	return e.provider.Name()
}

// Summarize calls the configured provider with the rendered prompt and a
// per-file timeout.
func (e *Engine) Summarize(content, title string) (string, error) {
	if e.provider == nil {
		return "", ErrNoProvider
	}

	content = intelligentTruncate(content, MaxContentTokens, e.estimator)

	var buf strings.Builder
	err := e.tmpl.Execute(&buf, struct {
		Title   string
		Content string
	}{Title: title, Content: content})
	if err != nil {
		return "", fmt.Errorf("rendering prompt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	out, err := e.provider.Generate(ctx, buf.String())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// intelligentTruncate shortens content to fit within budget tokens while
// preserving the parts most useful for summarization. The strategy, in order:
//
//  1. If the document already fits, return it unchanged.
//  2. Split into sections by markdown heading (any of #, ##, ###, etc.).
//     Build a reduced version that includes each heading plus its first
//     paragraph. Stop adding sections when the budget is reached.
//  3. If even the heading-only reduction overflows, fall back to a hard
//     head-of-document token truncation with a marker.
//
// Headings and first paragraphs are preferred because they tend to encode
// the document's structure and main claims — the cues a summarizer needs.
func intelligentTruncate(content string, budget int, est *tokens.Estimator) string {
	if budget <= 0 {
		return content
	}
	if est == nil {
		est = tokens.NewEstimator()
	}
	if est.Count(content) <= budget {
		return content
	}

	sections := splitSections(content)
	if reduced, ok := assembleReduction(sections, budget, est); ok {
		return reduced
	}

	// Last resort: hard token-level truncation with marker.
	truncated, _ := est.Truncate(content, budget)
	return strings.TrimRight(truncated, "\n") + "\n\n[... content truncated ...]"
}

// markdownSection is a heading plus its body paragraphs (in encounter order).
type markdownSection struct {
	heading    string   // full heading line including "# " prefix; empty for the preamble
	paragraphs []string // each entry is one blank-line-separated block as it appeared
}

// splitSections parses markdown into sequential sections. Anything before
// the first heading becomes a section with an empty heading.
func splitSections(content string) []markdownSection {
	lines := strings.Split(content, "\n")
	var sections []markdownSection
	current := markdownSection{}
	var paraBuf []string
	flushPara := func() {
		if len(paraBuf) > 0 {
			current.paragraphs = append(current.paragraphs, strings.Join(paraBuf, "\n"))
			paraBuf = nil
		}
	}
	flushSection := func() {
		flushPara()
		if current.heading != "" || len(current.paragraphs) > 0 {
			sections = append(sections, current)
		}
		current = markdownSection{}
	}

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if isMarkdownHeading(trimmed) {
			flushSection()
			current.heading = line
			continue
		}
		if strings.TrimSpace(line) == "" {
			flushPara()
			continue
		}
		paraBuf = append(paraBuf, line)
	}
	flushSection()
	return sections
}

// isMarkdownHeading returns true for ATX-style headings (#…#### prefix).
func isMarkdownHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	// Count leading '#' chars; must be 1..6 followed by space.
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i < 1 || i > 6 {
		return false
	}
	return i < len(line) && (line[i] == ' ' || line[i] == '\t')
}

// assembleReduction builds a token-bounded reduction that keeps each
// section's heading + first paragraph. Returns the reduced text and true if
// the result fits within budget. Returns ("", false) if even the trimmed
// headings exceed the budget (caller should fall back to hard truncation).
func assembleReduction(sections []markdownSection, budget int, est *tokens.Estimator) (string, bool) {
	if len(sections) == 0 {
		return "", false
	}
	var parts []string
	for _, sec := range sections {
		if sec.heading != "" {
			parts = append(parts, sec.heading)
		}
		if len(sec.paragraphs) > 0 {
			parts = append(parts, sec.paragraphs[0])
		}
	}
	if len(parts) == 0 {
		return "", false
	}

	// Greedily add parts (heading or first paragraph) in order until the next
	// would push us over budget. Always include the first part — if even that
	// alone exceeds the budget the caller falls back to hard truncation.
	var kept []string
	for _, part := range parts {
		candidate := joinParts(append(kept, part))
		if est.Count(candidate) > budget {
			if len(kept) == 0 {
				return "", false
			}
			break
		}
		kept = append(kept, part)
	}
	if len(kept) == 0 {
		return "", false
	}
	out := joinParts(kept)
	if len(kept) < len(parts) {
		out += "\n\n[... content truncated ...]"
	}
	return out, true
}

func joinParts(parts []string) string {
	return strings.Join(parts, "\n\n")
}
