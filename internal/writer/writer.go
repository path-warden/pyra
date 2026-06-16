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

	"github.com/okfy/okf-mcp/internal/normalize"
	"github.com/okfy/okf-mcp/internal/types"
	"github.com/okfy/okf-mcp/internal/util"
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

// withTitle ensures markdown has a title heading.
func withTitle(title, markdown string) string {
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

// generateFrontmatter generates YAML frontmatter for a document.
func generateFrontmatter(doc types.NormalizedDocument, timestamp string) string {
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
	lines = append(lines, "---")
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// WriteOKFBundle writes documents as an OKF bundle.
func WriteOKFBundle(docs []types.NormalizedDocument, opts WriteOptions) ([]string, error) {
	if err := ensureCleanOutDir(opts); err != nil {
		return nil, err
	}

	sourceToOutput := assignOutputPaths(docs)
	timestamp := opts.Timestamp
	if timestamp == "" {
		timestamp = "2024-01-01T00:00:00.000Z"
	}

	written := make([]string, 0, len(docs))
	conceptsByDir := make(map[string][]writtenConcept)

	for i := range docs {
		doc := &docs[i]

		// Rewrite links
		markdown := rewriteLinks(*doc, sourceToOutput)
		markdown = withTitle(doc.Title, markdown)

		// Generate frontmatter
		fm := generateFrontmatter(*doc, timestamp)

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

		// Track for index generation
		dir := path.Dir(doc.OutputPath)
		if dir == "" {
			dir = "."
		}
		conceptsByDir[dir] = append(conceptsByDir[dir], writtenConcept{
			relPath:     doc.OutputPath,
			title:       doc.Title,
			description: normalize.DescriptionFromMarkdown(doc.Markdown),
		})
	}

	// Generate index files
	if err := writeIndexFiles(opts, conceptsByDir); err != nil {
		return nil, err
	}

	return written, nil
}

// writeIndexFiles generates index.md files for each directory.
func writeIndexFiles(opts WriteOptions, conceptsByDir map[string][]writtenConcept) error {
	// Get all directories
	dirs := make([]string, 0, len(conceptsByDir))
	for dir := range conceptsByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		concepts := conceptsByDir[dir]
		sort.Slice(concepts, func(i, j int) bool {
			return concepts[i].title < concepts[j].title
		})

		title := indexTitle(dir, opts)
		var content strings.Builder

		// Root index has okf_version frontmatter
		if dir == "." {
			content.WriteString("---\nokf_version: \"0.1\"\n---\n\n")
		}

		content.WriteString("# " + title + "\n\n")

		for _, concept := range concepts {
			relLink := path.Base(concept.relPath)
			content.WriteString(fmt.Sprintf("- [%s](%s)\n", concept.title, relLink))
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
