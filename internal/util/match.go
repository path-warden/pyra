package util

import (
	"github.com/gobwas/glob"
	"regexp"
	"strings"
)

// MatchesPattern checks if a value matches a pattern.
// Patterns starting and ending with / are treated as regex.
// Otherwise, the pattern is treated as a glob.
func MatchesPattern(value, pattern string) bool {
	// Check if it's a regex pattern
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) > 2 {
		regexStr := pattern[1 : len(pattern)-1]
		re, err := regexp.Compile(regexStr)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	}

	// Treat as glob pattern
	g, err := glob.Compile(pattern, '/')
	if err != nil {
		return false
	}
	return g.Match(value)
}

// MatchesAnyPattern checks if a value matches any of the given patterns.
func MatchesAnyPattern(value string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if MatchesPattern(value, pattern) {
			return true
		}
	}
	return false
}
