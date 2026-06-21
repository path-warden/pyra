// Package updater orchestrates incremental updates to OKF bundles.
package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okfy/okf-mcp/internal/changelog"
	"github.com/okfy/okf-mcp/internal/crawler"
	"github.com/okfy/okf-mcp/internal/differ"
	"github.com/okfy/okf-mcp/internal/importer"

	"github.com/okfy/okf-mcp/internal/types"
	"github.com/okfy/okf-mcp/internal/util"
	"github.com/okfy/okf-mcp/internal/writer"
)

// UpdateOptions configures the update operation.
type UpdateOptions struct {
	BundlePath  string
	Source      string   // optional: override source from changelog
	Force       bool     // skip prompts, apply all changes
	DryRun      bool     // show changes without applying
	Include     []string // patterns for crawl/import
	Exclude     []string
	MaxPages    int // for crawl
	MaxDepth    int // for crawl
	Concurrency int // for crawl

	// Callback for prompting user about changes
	// Returns: apply (apply this change), applyAll (apply all remaining), cancel (cancel update)
	OnPrompt func(changeType differ.ChangeType, files []differ.FileChange) (apply bool, applyAll bool, cancel bool)

	// Callback for progress updates
	OnProgress func(phase string, message string)
}

// UpdateResult contains the result of an update operation.
type UpdateResult struct {
	Added         int
	Modified      int
	Deleted       int
	Skipped       int
	DryRun        bool
	AddedFiles    []string
	ModifiedFiles []string
	DeletedFiles  []string
}

// isURL checks if a string looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Update performs an incremental update of an existing bundle.
func Update(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	// Validate bundle exists
	if _, err := os.Stat(opts.BundlePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("bundle not found: %s", opts.BundlePath)
	}

	// Determine source
	source := opts.Source
	if source == "" {
		var err error
		source, err = changelog.GetSource(opts.BundlePath)
		if err != nil {
			return nil, fmt.Errorf("no source specified and %v. Use --source to specify the source location", err)
		}
	}

	if opts.OnProgress != nil {
		opts.OnProgress("fetching", fmt.Sprintf("Fetching content from %s", source))
	}

	// Fetch new content
	var newDocs []types.NormalizedDocument
	var err error

	if isURL(source) {
		newDocs, err = fetchFromURL(ctx, source, opts)
	} else {
		newDocs, err = fetchFromPath(source, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	if opts.OnProgress != nil {
		opts.OnProgress("diffing", fmt.Sprintf("Comparing %d documents against existing bundle", len(newDocs)))
	}

	// Assign output paths to new docs (needed for diffing)
	assignOutputPaths(newDocs)

	// Diff against existing bundle
	diffResult, err := differ.DiffBundles(opts.BundlePath, newDocs)
	if err != nil {
		return nil, fmt.Errorf("failed to diff bundles: %w", err)
	}

	// Dry run mode - just report and exit
	if opts.DryRun {
		var addedFiles, modifiedFiles, deletedFiles []string
		for _, c := range diffResult.Added {
			addedFiles = append(addedFiles, c.Path)
		}
		for _, c := range diffResult.Modified {
			modifiedFiles = append(modifiedFiles, c.Path)
		}
		for _, c := range diffResult.Deleted {
			deletedFiles = append(deletedFiles, c.Path)
		}
		return &UpdateResult{
			Added:         len(diffResult.Added),
			Modified:      len(diffResult.Modified),
			Deleted:       len(diffResult.Deleted),
			DryRun:        true,
			AddedFiles:    addedFiles,
			ModifiedFiles: modifiedFiles,
			DeletedFiles:  deletedFiles,
		}, nil
	}

	if !diffResult.HasChanges() {
		return &UpdateResult{}, nil
	}

	result := &UpdateResult{}

	// Process additions (always apply without prompting)
	if len(diffResult.Added) > 0 {
		if opts.OnProgress != nil {
			opts.OnProgress("applying", fmt.Sprintf("Adding %d new files", len(diffResult.Added)))
		}
		for _, change := range diffResult.Added {
			if err := applyAdd(opts.BundlePath, change, newDocs); err != nil {
				return nil, err
			}
			result.Added++
			result.AddedFiles = append(result.AddedFiles, change.Path)
		}
	}

	// Process modifications
	if len(diffResult.Modified) > 0 {
		applyAll := opts.Force
		for _, change := range diffResult.Modified {
			if !applyAll && opts.OnPrompt != nil {
				apply, all, cancel := opts.OnPrompt(differ.ChangeModified, []differ.FileChange{change})
				if cancel {
					return result, nil
				}
				if all {
					applyAll = true
				}
				if !apply && !all {
					result.Skipped++
					continue
				}
			}
			if err := applyModify(opts.BundlePath, change, newDocs); err != nil {
				return nil, err
			}
			result.Modified++
			result.ModifiedFiles = append(result.ModifiedFiles, change.Path)
		}
	}

	// Process deletions
	if len(diffResult.Deleted) > 0 {
		applyAll := opts.Force
		for _, change := range diffResult.Deleted {
			if !applyAll && opts.OnPrompt != nil {
				apply, all, cancel := opts.OnPrompt(differ.ChangeDeleted, []differ.FileChange{change})
				if cancel {
					return result, nil
				}
				if all {
					applyAll = true
				}
				if !apply && !all {
					result.Skipped++
					continue
				}
			}
			if err := applyDelete(opts.BundlePath, change); err != nil {
				return nil, err
			}
			result.Deleted++
			result.DeletedFiles = append(result.DeletedFiles, change.Path)
		}
	}

	// Update changelog
	if result.Added > 0 || result.Modified > 0 || result.Deleted > 0 {
		msg := fmt.Sprintf("Updated: %d added, %d modified, %d deleted", result.Added, result.Modified, result.Deleted)
		if err := changelog.AppendEntry(opts.BundlePath, msg); err != nil {
			// Non-fatal, just log
			if opts.OnProgress != nil {
				opts.OnProgress("warning", fmt.Sprintf("Failed to update changelog: %v", err))
			}
		}
	}

	return result, nil
}

// fetchFromURL fetches content from a URL using the crawler.
func fetchFromURL(ctx context.Context, url string, opts UpdateOptions) ([]types.NormalizedDocument, error) {
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 100
	}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 4
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	// Create a temporary directory for crawl output
	tmpDir, err := os.MkdirTemp("", "okf-update-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	result, err := crawler.Crawl(ctx, crawler.CrawlOptions{
		SeedURL:       url,
		OutDir:        tmpDir,
		MaxPages:      maxPages,
		MaxDepth:      maxDepth,
		Include:       opts.Include,
		Exclude:       opts.Exclude,
		SameOrigin:    true,
		RespectRobots: true,
		Concurrency:   concurrency,
	})
	if err != nil {
		return nil, err
	}

	return result.Documents, nil
}

// fetchFromPath fetches content from a local path using the importer.
func fetchFromPath(path string, opts UpdateOptions) ([]types.NormalizedDocument, error) {
	// Check path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("source path not found: %s", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", path)
	}

	// Create temporary output for import
	tmpDir, err := os.MkdirTemp("", "okf-update-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	result, err := importer.Import(importer.ImportOptions{
		InputPath: path,
		OutDir:    tmpDir,
		Include:   opts.Include,
		Exclude:   opts.Exclude,
	})
	if err != nil {
		return nil, err
	}

	return result.Documents, nil
}

// assignOutputPaths assigns output paths to documents.
func assignOutputPaths(docs []types.NormalizedDocument) {
	used := make(map[string]bool)
	for i := range docs {
		doc := &docs[i]
		var base string
		if doc.Resource != "" {
			base = util.URLToOutputPath(doc.Resource)
		} else {
			base = util.EnsureMarkdownPath(doc.SourcePath)
		}
		
		candidate := base
		index := 2
		for used[candidate] {
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			candidate = fmt.Sprintf("%s-%d%s", name, index, ext)
			index++
		}
		used[candidate] = true
		doc.OutputPath = candidate
	}
}

// applyAdd adds a new file to the bundle.
func applyAdd(bundlePath string, change differ.FileChange, newDocs []types.NormalizedDocument) error {
	doc := findDoc(newDocs, change.Path)
	if doc == nil {
		return fmt.Errorf("document not found for path: %s", change.Path)
	}

	outPath := filepath.Join(bundlePath, change.Path)
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	content := generateContent(*doc)
	return os.WriteFile(outPath, []byte(content), 0644)
}

// applyModify modifies an existing file in the bundle.
func applyModify(bundlePath string, change differ.FileChange, newDocs []types.NormalizedDocument) error {
	doc := findDoc(newDocs, change.Path)
	if doc == nil {
		return fmt.Errorf("document not found for path: %s", change.Path)
	}

	outPath := filepath.Join(bundlePath, change.Path)
	content := generateContent(*doc)
	return os.WriteFile(outPath, []byte(content), 0644)
}

// applyDelete deletes a file from the bundle and cleans up empty parent directories.
func applyDelete(bundlePath string, change differ.FileChange) error {
	outPath := filepath.Join(bundlePath, change.Path)
	if err := os.Remove(outPath); err != nil {
		return err
	}

	// Clean up empty parent directories
	dir := filepath.Dir(outPath)
	for dir != bundlePath && dir != "." {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			break
		}
		dir = filepath.Dir(dir)
	}

	return nil
}

// findDoc finds a document by output path.
func findDoc(docs []types.NormalizedDocument, path string) *types.NormalizedDocument {
	for i := range docs {
		if docs[i].OutputPath == path {
			return &docs[i]
		}
	}
	return nil
}

// generateContent generates the full markdown content with frontmatter.
func generateContent(doc types.NormalizedDocument) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	fm := writer.GenerateFrontmatter(doc, timestamp)
	markdown := writer.WithTitle(doc.Title, doc.Markdown)
	return fm + markdown
}
