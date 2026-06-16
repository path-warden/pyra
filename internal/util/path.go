// Package util provides utility functions for URL, path, and pattern matching.
package util

import (
	"path"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ToPosixPath converts a path to use forward slashes.
func ToPosixPath(input string) string {
	return strings.ReplaceAll(input, "\\", "/")
}

// StripMdExtension removes the .md extension from a path.
func StripMdExtension(input string) string {
	if strings.HasSuffix(strings.ToLower(input), ".md") {
		return input[:len(input)-3]
	}
	return input
}

// SafeSegment converts a string into a safe path segment.
func SafeSegment(input string) string {
	// Try to decode URL encoding
	decoded := input
	// Simple percent decoding for common cases
	decoded = strings.ReplaceAll(decoded, "%20", " ")
	decoded = strings.ReplaceAll(decoded, "%2F", "/")
	decoded = strings.ReplaceAll(decoded, "%3A", ":")

	// Normalize Unicode
	decoded = norm.NFKD.String(decoded)

	// Replace non-word characters with hyphens
	var result strings.Builder
	lastHyphen := false
	for _, r := range decoded {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '~' {
			result.WriteRune(unicode.ToLower(r))
			lastHyphen = false
		} else if !lastHyphen {
			result.WriteRune('-')
			lastHyphen = true
		}
	}

	// Trim leading/trailing hyphens and collapse multiple hyphens
	cleaned := strings.Trim(result.String(), "-")
	cleaned = regexp.MustCompile(`-{2,}`).ReplaceAllString(cleaned, "-")

	if cleaned == "" {
		return "index"
	}
	return cleaned
}

// EnsureMarkdownPath ensures a path ends with .md extension.
func EnsureMarkdownPath(input string) string {
	if input == "" || input == "/" {
		return "index.md"
	}

	trimmed := strings.TrimPrefix(input, "/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "" {
		return "index.md"
	}

	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		parts[i] = SafeSegment(part)
	}

	last := parts[len(parts)-1]
	lower := strings.ToLower(last)

	// Check if already has a supported extension
	if strings.HasSuffix(lower, ".md") {
		// Already markdown
	} else if strings.HasSuffix(lower, ".mdx") || strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") || strings.HasSuffix(lower, ".txt") {
		// Replace extension with .md
		ext := path.Ext(last)
		parts[len(parts)-1] = last[:len(last)-len(ext)] + ".md"
	} else {
		// Add .md extension
		parts[len(parts)-1] = last + ".md"
	}

	return strings.Join(parts, "/")
}

// URLToOutputPath converts a URL to an output file path.
func URLToOutputPath(urlStr string) string {
	// Extract path from URL
	idx := strings.Index(urlStr, "://")
	if idx != -1 {
		urlStr = urlStr[idx+3:]
		// Remove host
		idx = strings.Index(urlStr, "/")
		if idx == -1 {
			return "index.md"
		}
		urlStr = urlStr[idx:]
	}

	pathname := urlStr
	if pathname == "/" || pathname == "" {
		return "index.md"
	}

	// Handle trailing slash
	if strings.HasSuffix(pathname, "/") {
		trimmed := strings.Trim(pathname, "/")
		parts := strings.Split(trimmed, "/")
		for i, part := range parts {
			parts[i] = SafeSegment(part)
		}
		return strings.Join(parts, "/") + "/index.md"
	}

	return EnsureMarkdownPath(pathname)
}

// RelativeMarkdownLink creates a relative link from one path to another.
func RelativeMarkdownLink(fromPath, toPath string) string {
	fromDir := path.Dir(ToPosixPath(fromPath))
	to := ToPosixPath(toPath)

	rel, err := relPath(fromDir, to)
	if err != nil {
		return to
	}

	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}

// relPath computes a relative path from base to target.
func relPath(base, target string) (string, error) {
	// Normalize paths
	base = path.Clean(base)
	target = path.Clean(target)

	if base == "." {
		return target, nil
	}

	baseParts := strings.Split(base, "/")
	targetParts := strings.Split(target, "/")

	// Find common prefix
	common := 0
	for i := 0; i < len(baseParts) && i < len(targetParts); i++ {
		if baseParts[i] == targetParts[i] {
			common++
		} else {
			break
		}
	}

	// Build relative path
	var result []string

	// Add ".." for each remaining base directory
	for i := common; i < len(baseParts); i++ {
		if baseParts[i] != "" && baseParts[i] != "." {
			result = append(result, "..")
		}
	}

	// Add remaining target parts
	for i := common; i < len(targetParts); i++ {
		result = append(result, targetParts[i])
	}

	if len(result) == 0 {
		return ".", nil
	}

	return strings.Join(result, "/"), nil
}
