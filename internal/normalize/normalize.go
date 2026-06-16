// Package normalize handles document normalization and HTML to Markdown conversion.
package normalize

import (
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/okfy/okf-mcp/internal/types"
	"github.com/okfy/okf-mcp/internal/util"
)

// elementsToRemove are HTML elements that should be stripped before conversion.
var elementsToRemove = []string{
	"script", "style", "noscript", "svg", "header", "footer", "nav", "aside",
}

// contentSelectors are selectors to try for main content extraction.
var contentSelectors = []string{
	"main",
	"article",
	"[role='main']",
	".markdown-body",
	".docs-content",
}

// converter is the shared HTML to Markdown converter.
var converter *md.Converter

func init() {
	converter = md.NewConverter("", true, &md.Options{
		HeadingStyle:    "atx",
		CodeBlockStyle:  "fenced",
		BulletListMarker: "-",
	})
	// Keep tables as HTML (they'll be preserved)
	converter.Keep("table", "thead", "tbody", "tr", "th", "td")
}

// HTMLToMarkdown converts HTML content to Markdown.
func HTMLToMarkdown(html string) (string, string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html, ""
	}

	// Remove unwanted elements
	for _, selector := range elementsToRemove {
		doc.Find(selector).Remove()
	}

	// Extract title
	title := strings.TrimSpace(doc.Find("h1").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}

	// Try to find main content
	var content *goquery.Selection
	for _, selector := range contentSelectors {
		sel := doc.Find(selector).First()
		if sel.Length() > 0 {
			content = sel
			break
		}
	}
	if content == nil {
		content = doc.Find("body")
	}

	contentHTML, _ := content.Html()
	if contentHTML == "" {
		contentHTML = html
	}

	markdown, err := converter.ConvertString(contentHTML)
	if err != nil {
		return html, title
	}

	return strings.TrimSpace(markdown), title
}

// ExtractHeadings extracts headings from Markdown content.
func ExtractHeadings(markdown string) []types.Heading {
	re := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := re.FindAllStringSubmatch(markdown, -1)

	headings := make([]types.Heading, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 3 {
			text := strings.TrimSpace(match[2])
			headings = append(headings, types.Heading{
				Depth: len(match[1]),
				Text:  text,
				Slug:  util.SafeSegment(text),
			})
		}
	}
	return headings
}

// ExtractMarkdownLinks extracts links from Markdown content.
func ExtractMarkdownLinks(markdown string) []types.Link {
	re := regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	matches := re.FindAllStringSubmatch(markdown, -1)

	links := make([]types.Link, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 3 {
			links = append(links, types.Link{
				Text: match[1],
				Href: match[2],
			})
		}
	}
	return links
}

// InferType infers the document type from title, sourceID, and content.
func InferType(title, sourceID, markdown string) string {
	haystack := strings.ToLower(title + " " + sourceID + " " + markdown[:min(2000, len(markdown))])

	if regexp.MustCompile(`\breadme\b`).MatchString(haystack) {
		return "README"
	}
	if regexp.MustCompile(`\b(api|reference|sdk|endpoint|parameter|request|response)\b`).MatchString(haystack) {
		return "API Reference"
	}
	if regexp.MustCompile(`\b(quickstart|guide|tutorial|walkthrough|get started)\b`).MatchString(haystack) {
		return "Guide"
	}
	if regexp.MustCompile(`\bdocs?\b`).MatchString(haystack) {
		return "Documentation Page"
	}
	return "Concept"
}

// InferTags infers tags from sourceID, title, and headings.
func InferTags(title, sourceID string, headings []types.Heading) []string {
	// Build raw text from sourceID, title, and first 3 headings
	var parts []string
	parts = append(parts, sourceID, title)
	for i, h := range headings {
		if i >= 3 {
			break
		}
		parts = append(parts, h.Text)
	}
	raw := strings.Join(parts, " ")

	// Remove URLs
	raw = regexp.MustCompile(`https?://[^/]+`).ReplaceAllString(raw, "")
	raw = strings.ToLower(raw)

	// Extract words
	wordRe := regexp.MustCompile(`[a-z0-9]+`)
	words := wordRe.FindAllString(raw, -1)

	// Filter words
	stopWords := map[string]bool{
		"html": true, "markdown": true, "index": true,
		"docs": true, "page": true, "guide": true,
	}

	seen := make(map[string]bool)
	tags := make([]string, 0)
	for _, word := range words {
		if len(word) < 3 || len(word) > 24 {
			continue
		}
		if stopWords[word] {
			continue
		}
		if seen[word] {
			continue
		}
		seen[word] = true
		tags = append(tags, word)
		if len(tags) >= 6 {
			break
		}
	}
	return tags
}

// DescriptionFromMarkdown extracts a description from Markdown content.
func DescriptionFromMarkdown(markdown string) string {
	text := markdown

	// Remove frontmatter
	text = regexp.MustCompile(`(?s)^---.*?---\s*`).ReplaceAllString(text, "")

	// Remove headings
	text = regexp.MustCompile(`(?m)^#{1,6}\s+.+$`).ReplaceAllString(text, "")

	// Remove code blocks
	text = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(text, "")

	// Remove links but keep text
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	// Remove special characters
	text = regexp.MustCompile("[`*_>#-]").ReplaceAllString(text, "")

	// Collapse whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	if len(text) > 180 {
		text = text[:180]
	}
	if text == "" {
		return "Generated OKF concept."
	}
	return text
}

// fallbackTitle creates a title from sourceID.
func fallbackTitle(sourceID string) string {
	// Get the last path segment
	parts := regexp.MustCompile(`[/?#]`).Split(sourceID, -1)
	var leaf string
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			leaf = parts[i]
			break
		}
	}
	if leaf == "" {
		return "Index"
	}

	// Remove extension
	leaf = regexp.MustCompile(`\.[a-z0-9]+$`).ReplaceAllString(leaf, "")

	// Split on separators and title case
	words := regexp.MustCompile(`[-_\s]+`).Split(leaf, -1)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// titleFromMarkdown extracts the title from markdown content.
func titleFromMarkdown(markdown, fallback string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	match := re.FindStringSubmatch(markdown)
	if len(match) >= 2 {
		return plainTitle(strings.TrimSpace(match[1]))
	}
	return fallback
}

// plainTitle removes markdown formatting from a title.
func plainTitle(title string) string {
	// Remove links
	title = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(title, "$1")
	// Remove formatting
	title = regexp.MustCompile("[`*_#]").ReplaceAllString(title, "")
	// Collapse whitespace
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	return strings.TrimSpace(title)
}

// NormalizeDocument normalizes a raw document into a normalized document.
func NormalizeDocument(raw types.RawDocument) types.NormalizedDocument {
	var markdown string
	title := fallbackTitle(raw.URL)
	if title == "" {
		title = fallbackTitle(raw.FilePath)
	}
	if title == "" {
		title = fallbackTitle(raw.SourceID)
	}

	switch raw.ContentType {
	case types.ContentTypeHTML:
		var extractedTitle string
		markdown, extractedTitle = HTMLToMarkdown(raw.Raw)
		if extractedTitle != "" {
			title = extractedTitle
		}
	case types.ContentTypeText:
		markdown = "# " + title + "\n\n```text\n" + strings.TrimSpace(raw.Raw) + "\n```"
	default:
		markdown = raw.Raw
	}

	// Normalize line endings
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.TrimSpace(markdown)

	// Extract title from markdown if present
	title = titleFromMarkdown(markdown, plainTitle(title))
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)

	headings := ExtractHeadings(markdown)
	links := ExtractMarkdownLinks(markdown)

	sourceID := raw.URL
	if sourceID == "" {
		sourceID = raw.FilePath
	}
	if sourceID == "" {
		sourceID = raw.SourceID
	}

	return types.NormalizedDocument{
		SourceID:   sourceID,
		Title:      title,
		Markdown:   markdown,
		Resource:   raw.URL,
		SourcePath: raw.FilePath,
		Headings:   headings,
		Links:      links,
		Tags:       InferTags(title, sourceID, headings),
		Type:       InferType(title, sourceID, markdown),
	}
}
