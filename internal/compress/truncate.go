package compress

import (
	"strings"

	"github.com/okfy/okf-mcp/internal/tokens"
)

// TruncateOptions configures truncation behavior.
type TruncateOptions struct {
	TokenBudget         int
	PreserveFrontmatter bool
	AddIndicator        bool
}

// TruncateResult contains truncation results.
type TruncateResult struct {
	Content     string
	Truncated   bool
	TruncatedAt string   // Section name where truncation occurred
	Sections    []string // Outline of sections in truncated content
}

// Truncate truncates markdown content to fit within a token budget.
// It attempts to truncate at section boundaries for cleaner breaks.
func Truncate(content string, opts TruncateOptions) TruncateResult {
	if opts.TokenBudget <= 0 {
		return TruncateResult{Content: content, Truncated: false}
	}

	est := tokens.NewEstimator()
	currentTokens := est.Count(content)
	if currentTokens <= opts.TokenBudget {
		return TruncateResult{Content: content, Truncated: false}
	}

	// Split frontmatter from body
	frontmatter, body := splitFrontmatter(content)
	frontmatterTokens := 0
	if frontmatter != "" && opts.PreserveFrontmatter {
		frontmatterTokens = est.Count(frontmatter)
	}

	// Calculate available budget for body
	bodyBudget := opts.TokenBudget - frontmatterTokens
	if bodyBudget <= 0 {
		// Frontmatter alone exceeds budget
		if opts.PreserveFrontmatter {
			return TruncateResult{
				Content:   frontmatter,
				Truncated: true,
			}
		}
		bodyBudget = opts.TokenBudget
		frontmatter = ""
	}

	// Parse sections
	sections := parseSections(body)
	
	// Build result by adding sections until budget exceeded
	var result strings.Builder
	var includedSections []string
	var truncatedAt string
	truncatedTokens := 0

	for _, section := range sections {
		sectionTokens := est.Count(section.Content)
		if truncatedTokens+sectionTokens > bodyBudget {
			truncatedAt = section.Title
			break
		}
		result.WriteString(section.Content)
		truncatedTokens += sectionTokens
		if section.Title != "" {
			includedSections = append(includedSections, section.Title)
		}
	}

	// Build final content
	var final strings.Builder
	if frontmatter != "" && opts.PreserveFrontmatter {
		final.WriteString(frontmatter)
	}
	final.WriteString(result.String())

	// Add truncation indicator
	if opts.AddIndicator && truncatedAt != "" {
		final.WriteString("\n\n---\n*[Content truncated. ")
		final.WriteString("Use `read_concept` with higher token budget for full content.]*\n")
	}

	return TruncateResult{
		Content:     strings.TrimRight(final.String(), "\n") + "\n",
		Truncated:   true,
		TruncatedAt: truncatedAt,
		Sections:    includedSections,
	}
}

// section represents a markdown section.
type section struct {
	Title   string
	Level   int
	Content string
}

// parseSections splits markdown into sections based on headers.
func parseSections(content string) []section {
	lines := strings.Split(content, "\n")
	var sections []section
	var current strings.Builder
	currentTitle := ""
	currentLevel := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check for header
		if strings.HasPrefix(trimmed, "#") {
			// Save previous section if any
			if current.Len() > 0 {
				sections = append(sections, section{
					Title:   currentTitle,
					Level:   currentLevel,
					Content: current.String(),
				})
				current.Reset()
			}

			// Parse header level and title
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			currentTitle = strings.TrimSpace(trimmed[level:])
			currentLevel = level
		}

		current.WriteString(line)
		current.WriteString("\n")
	}

	// Add final section
	if current.Len() > 0 {
		sections = append(sections, section{
			Title:   currentTitle,
			Level:   currentLevel,
			Content: current.String(),
		})
	}

	return sections
}

// splitFrontmatter separates YAML frontmatter from markdown body.
func splitFrontmatter(content string) (frontmatter, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}

	// Find closing ---
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return "", content
	}

	// Include the closing --- and newline
	frontmatter = content[:3+idx+4]
	body = strings.TrimPrefix(content[3+idx+4:], "\n")
	return frontmatter, body
}

// GenerateSectionOutline creates a brief outline of sections.
func GenerateSectionOutline(content string) []string {
	_, body := splitFrontmatter(content)
	sections := parseSections(body)
	
	var outline []string
	for _, s := range sections {
		if s.Title != "" {
			prefix := strings.Repeat("  ", s.Level-1)
			outline = append(outline, prefix+s.Title)
		}
	}
	return outline
}
