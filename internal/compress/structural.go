package compress

import (
	"regexp"
	"strings"
)

// Structural applies structural compression to markdown content.
// This is the lightest form of compression, focusing on whitespace normalization.
func Structural(content string) string {
	result := content

	// Collapse multiple blank lines to single blank line
	result = collapseBlankLines(result)

	// Remove trailing whitespace from lines
	result = removeTrailingWhitespace(result)

	// Normalize list markers to consistent dash
	result = normalizeListMarkers(result)

	// Remove excessive indentation in non-code blocks
	result = normalizeIndentation(result)

	return result
}

// collapseBlankLines reduces multiple consecutive blank lines to a single blank line.
func collapseBlankLines(content string) string {
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllString(content, "\n\n")
}

// removeTrailingWhitespace removes trailing spaces/tabs from each line.
func removeTrailingWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// normalizeListMarkers converts * and + list markers to -.
func normalizeListMarkers(content string) string {
	lines := strings.Split(content, "\n")
	inCodeBlock := false

	for i, line := range lines {
		// Track code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Normalize list markers outside code blocks
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			indent := line[:len(line)-len(trimmed)]
			lines[i] = indent + "-" + trimmed[1:]
		}
	}

	return strings.Join(lines, "\n")
}

// normalizeIndentation removes excessive leading whitespace while preserving structure.
func normalizeIndentation(content string) string {
	lines := strings.Split(content, "\n")
	inCodeBlock := false

	for i, line := range lines {
		// Track code blocks - don't modify indentation inside them
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Convert tabs to spaces (2 spaces per tab) for consistency
		lines[i] = strings.ReplaceAll(line, "\t", "  ")
	}

	return strings.Join(lines, "\n")
}
