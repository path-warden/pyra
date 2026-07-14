// Package writer handles writing OKF bundles to disk.
package writer

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/chasedputnam/pyra/internal/changelog"
	"github.com/chasedputnam/pyra/internal/normalize"
	"github.com/chasedputnam/pyra/internal/summarize"
	"github.com/chasedputnam/pyra/internal/tokens"
	"github.com/chasedputnam/pyra/internal/types"
	"github.com/chasedputnam/pyra/internal/util"
)

// WriteOptions configures bundle writing.
type WriteOptions struct {
	OutDir                       string
	Title                        string
	SourceName                   string
	Force                        bool
	InputPath                    string
	DangerouslyAllowUnsafeOutput bool
	Timestamp                    string
	Source                       string // URL or file path for changelog
	SummarizeMode                string // "extractive" | "llm"
	SummarizeAlgorithm           string // lsa | lexrank | textrank | ...
	Language                     string // language for summarization
	EdmundsonConfigPath          string // optional path to edmundson.config
	// OnProgress is invoked once per document with (index, total, source).
	OnProgress func(index, total int, source string)
	// OnSummarizeWarning is invoked when a summary falls back to the legacy
	// heuristic or fails entirely. Callers can route this to stderr.
	OnSummarizeWarning func(path, message string)
}

// SummaryStats tracks the outcome of summarization across a bundle.
type SummaryStats struct {
	Total     int
	BySource  map[string]int
	Fallbacks int
	Failed    int
}

// NewSummaryStats creates an empty SummaryStats.
func NewSummaryStats() *SummaryStats {
	return &SummaryStats{BySource: map[string]int{}}
}

// reservedFilenames are OKF reserved filenames that cannot be used for concepts.
var reservedFilenames = map[string]bool{
	"index.md": true,
	"log.md":   true,
}

// writtenConcept tracks a written concept for index generation.
type writtenConcept struct {
	relPath     string
	title       string
	description string
	summary     string
	docType     string
	tags        []string
	tokenCount  int
}

// isReservedOKFPath checks if a path is a reserved OKF filename.
func isReservedOKFPath(p string) bool {
	base := strings.ToLower(path.Base(util.ToPosixPath(p)))
	return reservedFilenames[base]
}

// safeConceptOutputPath ensures the output path doesn't conflict with reserved names.
func safeConceptOutputPath(candidate string) string {
	if !isReservedOKFPath(candidate) {
		return candidate
	}
	dir := path.Dir(candidate)
	base := path.Base(candidate)
	name := strings.ToLower(strings.TrimSuffix(base, ".md"))

	var safeName string
	if name == "log" {
		safeName = "change-log"
	} else if dir != "." && dir != "" {
		safeName = "overview"
	} else {
		safeName = "home"
	}
	return path.Join(dir, safeName+".md")
}

// sourceKey returns the canonical key for a document.
func sourceKey(doc types.NormalizedDocument) string {
	if doc.Resource != "" {
		canonical, err := util.CanonicalizeURL(doc.Resource)
		if err == nil {
			return canonical
		}
	}
	return util.ToPosixPath(doc.SourcePath)
}

// assignOutputPaths assigns unique output paths to all documents.
func assignOutputPaths(docs []types.NormalizedDocument) map[string]string {
	used := make(map[string]bool)
	result := make(map[string]string)

	for i := range docs {
		doc := &docs[i]
		var base string
		if doc.Resource != "" {
			base = util.URLToOutputPath(doc.Resource)
		} else {
			base = util.EnsureMarkdownPath(doc.SourcePath)
		}
		base = safeConceptOutputPath(base)

		candidate := base
		index := 2
		for used[candidate] {
			ext := path.Ext(base)
			name := strings.TrimSuffix(base, ext)
			candidate = fmt.Sprintf("%s-%d%s", name, index, ext)
			index++
		}
		used[candidate] = true
		result[sourceKey(*doc)] = candidate
		doc.OutputPath = candidate
	}
	return result
}

// rewriteLinks rewrites internal links to relative markdown paths.
func rewriteLinks(doc types.NormalizedDocument, sourceToOutput map[string]string) string {
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)([^)]*)\)`)

	return linkRe.ReplaceAllStringFunc(doc.Markdown, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		text, href, suffix := parts[1], parts[2], parts[3]

		// External URL
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
			canonical, err := util.CanonicalizeURL(href)
			if err == nil {
				if target, ok := sourceToOutput[canonical]; ok && doc.OutputPath != "" {
					return fmt.Sprintf("[%s](%s%s)", text, util.RelativeMarkdownLink(doc.OutputPath, target), suffix)
				}
			}
			return match
		}

		// Anchor only
		if strings.HasPrefix(href, "#") {
			return match
		}

		// Relative link with resource context
		if doc.Resource != "" {
			canonical, err := util.CanonicalizeURL(href, doc.Resource)
			if err == nil {
				if target, ok := sourceToOutput[canonical]; ok && doc.OutputPath != "" {
					return fmt.Sprintf("[%s](%s%s)", text, util.RelativeMarkdownLink(doc.OutputPath, target), suffix)
				}
				return fmt.Sprintf("[%s](%s%s)", text, canonical, suffix)
			}
		}

		// Relative link with source path context
		if doc.SourcePath != "" {
			dir := path.Dir(util.ToPosixPath(doc.SourcePath))
			abs := path.Clean(path.Join(dir, href))
			noHash := strings.Split(abs, "#")[0]
			if target, ok := sourceToOutput[noHash]; ok && doc.OutputPath != "" {
				return fmt.Sprintf("[%s](%s%s)", text, util.RelativeMarkdownLink(doc.OutputPath, target), suffix)
			}
		}

		return match
	})
}

// WithTitle ensures markdown has a title heading.
func WithTitle(title, markdown string) string {
	trimmed := strings.TrimSpace(markdown)
	if regexp.MustCompile(`^#\s+`).MatchString(trimmed) {
		return trimmed
	}
	return "# " + title + "\n\n" + trimmed
}

// yamlScalar encodes a string as a YAML scalar.
func yamlScalar(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

// GenerateFrontmatter generates YAML frontmatter for a document.
func GenerateFrontmatter(doc types.NormalizedDocument, timestamp string) string {
	return GenerateFrontmatterWithBacklinks(doc, timestamp, nil)
}

// GenerateFrontmatterWithBacklinks generates YAML frontmatter with optional backlinks.
func GenerateFrontmatterWithBacklinks(doc types.NormalizedDocument, timestamp string, backlinks []string) string {
	resource := doc.Resource
	if resource == "" {
		resource = doc.SourcePath
	}
	if resource == "" {
		resource = doc.SourceID
	}

	var lines []string
	lines = append(lines, "---")
	lines = append(lines, fmt.Sprintf("type: %s", yamlScalar(doc.Type)))
	lines = append(lines, fmt.Sprintf("title: %s", yamlScalar(doc.Title)))
	lines = append(lines, fmt.Sprintf("description: %s", yamlScalar(normalize.DescriptionFromMarkdown(doc.Markdown))))
	lines = append(lines, fmt.Sprintf("resource: %s", yamlScalar(resource)))
	lines = append(lines, "tags:")
	if len(doc.Tags) > 0 {
		for _, tag := range doc.Tags {
			lines = append(lines, fmt.Sprintf("  - %s", yamlScalar(tag)))
		}
	} else {
		lines = append(lines, "  []")
	}
	lines = append(lines, fmt.Sprintf("timestamp: %s", yamlScalar(timestamp)))

	// Add backlinks if present
	if len(backlinks) > 0 {
		lines = append(lines, "backlinks:")
		for _, bl := range backlinks {
			lines = append(lines, fmt.Sprintf("  - %s", yamlScalar(bl)))
		}
	}

	lines = append(lines, "---")
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// injectSummaryCallout inserts a summary callout after the first heading.
func injectSummaryCallout(markdown string, sum summarize.Summary) string {
	if sum.Text == "" {
		return markdown
	}

	callout := summarize.FormatCallout(sum)
	lines := strings.Split(markdown, "\n")

	// Find the first heading and insert callout after it
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			// Insert callout after heading with blank line
			before := strings.Join(lines[:i+1], "\n")
			after := strings.Join(lines[i+1:], "\n")
			return before + "\n\n" + callout + "\n" + after
		}
	}

	// No heading found, prepend callout
	return callout + "\n\n" + markdown
}

// computeBacklinks builds a map of output path -> list of paths that link to it.
func computeBacklinks(docs []types.NormalizedDocument, sourceToOutput map[string]string) map[string][]string {
	backlinks := make(map[string][]string)
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)`)

	for _, doc := range docs {
		if doc.OutputPath == "" {
			continue
		}

		// Find all links in this document
		matches := linkRe.FindAllStringSubmatch(doc.Markdown, -1)
		seen := make(map[string]bool)

		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			href := match[2]

			// Skip external links and anchors
			if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
				strings.HasPrefix(href, "//") || strings.HasPrefix(href, "#") {
				continue
			}

			// Resolve target output path
			var targetOutput string

			if doc.Resource != "" {
				// Try resolving as URL
				canonical, err := util.CanonicalizeURL(href, doc.Resource)
				if err == nil {
					targetOutput = sourceToOutput[canonical]
				}
			}

			if targetOutput == "" && doc.SourcePath != "" {
				// Try resolving as relative path
				dir := path.Dir(util.ToPosixPath(doc.SourcePath))
				abs := path.Clean(path.Join(dir, href))
				noHash := strings.Split(abs, "#")[0]
				targetOutput = sourceToOutput[noHash]
			}

			// Add backlink if we found a valid target
			if targetOutput != "" && targetOutput != doc.OutputPath && !seen[targetOutput] {
				seen[targetOutput] = true
				// Store the source path (without .md extension for cleaner display)
				sourcePath := strings.TrimSuffix(doc.OutputPath, ".md")
				backlinks[targetOutput] = append(backlinks[targetOutput], sourcePath)
			}
		}
	}

	// Sort backlinks for deterministic output
	for target := range backlinks {
		sort.Strings(backlinks[target])
	}

	return backlinks
}

// WriteOKFBundle writes documents as an OKF bundle.
func WriteOKFBundle(docs []types.NormalizedDocument, opts WriteOptions) ([]string, error) {
	res, err := WriteOKFBundleWithStats(docs, opts)
	if err != nil {
		return nil, err
	}
	return res.Written, nil
}

// WriteResult bundles the written paths with summarization statistics.
type WriteResult struct {
	Written []string
	Stats   *SummaryStats
}

// WriteOKFBundleWithStats writes documents as an OKF bundle and returns
// summarization statistics.
func WriteOKFBundleWithStats(docs []types.NormalizedDocument, opts WriteOptions) (*WriteResult, error) {
	if err := ensureCleanOutDir(opts); err != nil {
		return nil, err
	}

	sourceToOutput := assignOutputPaths(docs)
	timestamp := opts.Timestamp
	if timestamp == "" {
		timestamp = "2024-01-01T00:00:00.000Z"
	}

	// Compute backlinks from all outbound links
	backlinks := computeBacklinks(docs, sourceToOutput)

	// Build a summarizer based on options.
	summarizer, summarizerErr := buildSummarizer(opts)

	written := make([]string, 0, len(docs))
	conceptsByDir := make(map[string][]writtenConcept)
	estimator := tokens.NewEstimator()
	stats := NewSummaryStats()
	stats.Total = len(docs)

	for i := range docs {
		doc := &docs[i]

		if opts.OnProgress != nil {
			opts.OnProgress(i+1, len(docs), doc.OutputPath)
		}

		// Rewrite links
		markdown := rewriteLinks(*doc, sourceToOutput)
		markdown = WithTitle(doc.Title, markdown)

		// Generate summary using the configured summarizer.
		description := normalize.DescriptionFromMarkdown(doc.Markdown)
		sum, source := generateSummary(summarizer, summarizerErr, description, doc, stats, opts.OnSummarizeWarning, opts.SummarizeMode)

		// Inject summary callout at top of body (after title)
		markdown = injectSummaryCallout(markdown, sum)

		// Get backlinks for this document
		docBacklinks := backlinks[doc.OutputPath]

		// Generate frontmatter with backlinks
		fm := GenerateFrontmatterWithBacklinks(*doc, timestamp, docBacklinks)

		// Write file
		outPath := filepath.Join(opts.OutDir, doc.OutputPath)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		content := fm + markdown
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		written = append(written, doc.OutputPath)

		// Update stats with the summary source actually used.
		if source != "" {
			stats.BySource[source]++
		}

		// Track for index generation
		dir := path.Dir(doc.OutputPath)
		if dir == "" {
			dir = "."
		}
		conceptsByDir[dir] = append(conceptsByDir[dir], writtenConcept{
			relPath:     doc.OutputPath,
			title:       doc.Title,
			description: description,
			summary:     sum.Text,
			docType:     doc.Type,
			tags:        doc.Tags,
			tokenCount:  estimator.Count(content),
		})
	}

	// Generate index files with summaries
	if err := writeIndexFiles(opts, conceptsByDir); err != nil {
		return nil, err
	}

	// Create changelog with summarization metadata if source is provided.
	if opts.Source != "" {
		mode := opts.SummarizeMode
		algo := opts.SummarizeAlgorithm
		lang := opts.Language
		if err := changelog.CreateChangelogWithMetadata(opts.OutDir, opts.Source, len(docs), mode, algo, lang); err != nil {
			return nil, fmt.Errorf("failed to create changelog: %w", err)
		}
	}

	return &WriteResult{Written: written, Stats: stats}, nil
}

// buildSummarizer constructs the configured Summarizer or returns a
// descriptive error. Callers fall back to the legacy heuristic on error.
func buildSummarizer(opts WriteOptions) (summarize.Summarizer, error) {
	mode := opts.SummarizeMode
	if mode == "" {
		mode = string(summarize.DefaultMode)
	}
	algo := opts.SummarizeAlgorithm
	if algo == "" {
		algo = summarize.DefaultAlgorithm
	}
	lang := opts.Language
	if lang == "" {
		lang = summarize.DefaultLanguage
	}
	return summarize.NewSummarizer(summarize.Config{
		Mode:                summarize.Mode(mode),
		Algorithm:           algo,
		Language:            lang,
		BundlePath:          opts.OutDir,
		EdmundsonConfigPath: opts.EdmundsonConfigPath,
	})
}

// generateSummary runs the configured summarizer, falling back to the legacy
// heuristic extractor if anything fails. The returned source identifies which
// path actually produced the summary, used by the caller to update stats.
// requestedMode is the mode the user asked for; when the returned source is
// inconsistent with the request (e.g., mode=llm but source="lsa"), that
// counts as a fallback.
func generateSummary(
	s summarize.Summarizer,
	sErr error,
	description string,
	doc *types.NormalizedDocument,
	stats *SummaryStats,
	onWarn func(path, message string),
	requestedMode string,
) (summarize.Summary, string) {
	if s == nil || sErr != nil {
		sum := summarize.Extract(description, doc.Markdown, doc.Title)
		stats.Fallbacks++
		if sum.Source == summarize.SourceNone {
			stats.Failed++
			if onWarn != nil {
				onWarn(doc.OutputPath, "no summary could be generated")
			}
		} else if onWarn != nil && sErr != nil {
			onWarn(doc.OutputPath, fmt.Sprintf("summarizer init failed, used heuristic: %v", sErr))
		}
		return sum, sum.Source
	}

	sum, err := s.Summarize(doc.Markdown, doc.Title)
	if err != nil || sum.Text == "" {
		fallback := summarize.Extract(description, doc.Markdown, doc.Title)
		stats.Fallbacks++
		if fallback.Source == summarize.SourceNone {
			stats.Failed++
			if onWarn != nil {
				if err != nil {
					onWarn(doc.OutputPath, fmt.Sprintf("summarization failed: %v", err))
				} else {
					onWarn(doc.OutputPath, "no summary could be generated")
				}
			}
			return fallback, summarize.SourceNone
		}
		if onWarn != nil && err != nil {
			onWarn(doc.OutputPath, fmt.Sprintf("fell back to heuristic: %v", err))
		}
		return fallback, fallback.Source
	}

	// If the user asked for LLM mode but a non-LLM source came back, the
	// internal LLM summarizer fell back to its extractive helper.
	if requestedMode == string(summarize.ModeLLM) && sum.Source != "llm" {
		stats.Fallbacks++
		if onWarn != nil {
			onWarn(doc.OutputPath, fmt.Sprintf("LLM unavailable, fell back to %s", sum.Source))
		}
	}
	return sum, sum.Source
}

// writeIndexFiles generates index.md files for each directory with summaries.
func writeIndexFiles(opts WriteOptions, conceptsByDir map[string][]writtenConcept) error {
	// Get all directories
	dirs := make([]string, 0, len(conceptsByDir))
	for dir := range conceptsByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	// Count total concepts and tokens for root index
	totalConcepts := 0
	totalTokens := 0
	for _, concepts := range conceptsByDir {
		totalConcepts += len(concepts)
		for _, c := range concepts {
			totalTokens += c.tokenCount
		}
	}

	for _, dir := range dirs {
		concepts := conceptsByDir[dir]
		sort.Slice(concepts, func(i, j int) bool {
			return concepts[i].title < concepts[j].title
		})

		title := indexTitle(dir, opts)
		var content strings.Builder

		// Root index has enhanced frontmatter
		if dir == "." {
			content.WriteString("---\n")
			content.WriteString("okf_version: \"0.1\"\n")
			_, _ = fmt.Fprintf(&content, "total_concepts: %d\n", totalConcepts)
			_, _ = fmt.Fprintf(&content, "total_tokens: %d\n", totalTokens)
			_, _ = fmt.Fprintf(&content, "generated: %s\n", opts.Timestamp)
			content.WriteString("---\n\n")
		}

		content.WriteString("# " + title + "\n\n")
		_, _ = fmt.Fprintf(&content, "## Concepts (%d)\n\n", len(concepts))

		for _, concept := range concepts {
			relLink := path.Base(concept.relPath)

			// Format: - [[path]] · Type, tag1, tag2
			//           Summary text
			typeAndTags := concept.docType
			if len(concept.tags) > 0 {
				typeAndTags += ", " + strings.Join(concept.tags, ", ")
			}

			_, _ = fmt.Fprintf(&content, "- [[%s]] · %s\n", strings.TrimSuffix(relLink, ".md"), typeAndTags)

			// Add summary on next line with indent
			if concept.summary != "" {
				_, _ = fmt.Fprintf(&content, "  %s\n", concept.summary)
			}
			content.WriteString("\n")
		}

		indexPath := filepath.Join(opts.OutDir, dir, "index.md")
		if err := os.WriteFile(indexPath, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write index: %w", err)
		}
	}

	return nil
}

// indexTitle returns the title for an index file.
func indexTitle(dir string, opts WriteOptions) string {
	if dir == "." {
		if opts.Title != "" {
			return opts.Title
		}
		if opts.SourceName != "" {
			return opts.SourceName
		}
		return "OKF Bundle"
	}
	// Use the directory name, title cased
	leaf := path.Base(dir)
	words := regexp.MustCompile(`[-_\s]+`).Split(leaf, -1)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// ensureCleanOutDir prepares the output directory.
func ensureCleanOutDir(opts WriteOptions) error {
	if opts.Force {
		if err := assertSafeForceOutDir(opts); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(opts.OutDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(opts.OutDir, 0755)
		}
		return err
	}

	if len(entries) > 0 {
		if !opts.Force {
			return fmt.Errorf("output directory is not empty: %s. Use --force to overwrite", opts.OutDir)
		}
		if err := os.RemoveAll(opts.OutDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	return os.MkdirAll(opts.OutDir, 0755)
}

// assertSafeForceOutDir checks that --force won't delete dangerous paths.
func assertSafeForceOutDir(opts WriteOptions) error {
	if opts.DangerouslyAllowUnsafeOutput {
		return nil
	}

	outDir := strings.TrimSpace(opts.OutDir)
	if outDir == "" {
		return fmt.Errorf("unsafe output directory for --force: empty path")
	}

	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	// Check against forbidden paths
	forbidden := make(map[string]string)

	// Filesystem root
	forbidden[filepath.VolumeName(absOut)+string(filepath.Separator)] = "filesystem root"

	// Home directory
	if home, err := os.UserHomeDir(); err == nil {
		forbidden[home] = "home directory"
	}

	// Current working directory
	if cwd, err := os.Getwd(); err == nil {
		forbidden[cwd] = "current working directory"
	}

	// Input path
	if opts.InputPath != "" {
		if absInput, err := filepath.Abs(opts.InputPath); err == nil {
			forbidden[absInput] = "input path"
			forbidden[filepath.Dir(absInput)] = "parent of input path"
		}
	}

	// Check for git repo root
	if repoRoot := findRepoRoot(absOut); repoRoot != "" {
		forbidden[repoRoot] = "repository root"
	}

	if reason, ok := forbidden[absOut]; ok {
		return fmt.Errorf("unsafe output directory for --force: refusing to delete %s (%s)", reason, absOut)
	}

	return nil
}

// findRepoRoot finds the git repository root.
func findRepoRoot(start string) string {
	current := start
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
