// Package differ compares existing bundle content against new content to detect changes.
package differ

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chasedputnam/pyra/internal/summarize"
	"github.com/chasedputnam/pyra/internal/types"
	"github.com/chasedputnam/pyra/internal/util"
)

// ChangeType indicates the type of change detected.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// FileChange represents a single file change.
type FileChange struct {
	Path       string
	ChangeType ChangeType
	OldContent string // empty for added
	NewContent string // empty for deleted
}

// DiffResult contains all detected changes.
type DiffResult struct {
	Added    []FileChange
	Modified []FileChange
	Deleted  []FileChange
}

// reservedFiles are files that should not be compared (they're managed separately).
var reservedFiles = map[string]bool{
	"index.md":      true,
	"log.md":        true,
	"changelog.txt": true,
}

// isReservedFile checks if a file is a reserved OKF file.
func isReservedFile(relPath string) bool {
	base := strings.ToLower(filepath.Base(relPath))
	return reservedFiles[base]
}

// readExistingBundle reads all markdown files from an existing bundle.
func readExistingBundle(bundlePath string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.WalkDir(bundlePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Only process markdown files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" {
			return nil
		}

		relPath, err := filepath.Rel(bundlePath, path)
		if err != nil {
			return err
		}
		relPath = util.ToPosixPath(relPath)

		// Skip reserved files
		if isReservedFile(relPath) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		files[relPath] = string(content)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// stripFrontmatter removes YAML frontmatter from markdown content for comparison.
// This allows us to ignore timestamp-only changes.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}

	// Find the closing ---
	rest := content[3:]
	_, after, found := strings.Cut(rest, "\n---")
	if !found {
		return content
	}

	return strings.TrimPrefix(after, "\n")
}

// normalizeContent normalizes content for comparison. It strips frontmatter
// (which we own and rewrite on every update with new timestamps and
// backlinks) and the `> [!summary]` callout (which we inject into bundle
// files but is not present in freshly-fetched source markdown). Without
// stripping the callout, every file would appear modified on every update.
func normalizeContent(content string) string {
	body := stripFrontmatter(content)
	body = summarize.StripCallout(body)
	body = strings.TrimSpace(body)
	body = regexp.MustCompile(`\r\n`).ReplaceAllString(body, "\n")
	return body
}

// contentChanged checks if the content has meaningfully changed.
func contentChanged(oldContent, newContent string) bool {
	return normalizeContent(oldContent) != normalizeContent(newContent)
}

// DiffBundles compares existing bundle against new documents.
func DiffBundles(existingPath string, newDocs []types.NormalizedDocument) (*DiffResult, error) {
	// Read existing bundle
	existingFiles, err := readExistingBundle(existingPath)
	if err != nil {
		return nil, err
	}

	result := &DiffResult{}

	// Track which existing files we've seen in new docs
	seen := make(map[string]bool)

	// Check each new document
	for _, doc := range newDocs {
		outputPath := doc.OutputPath
		if outputPath == "" {
			continue
		}

		// Skip reserved files
		if isReservedFile(outputPath) {
			continue
		}

		seen[outputPath] = true

		existingContent, exists := existingFiles[outputPath]
		if !exists {
			// New file
			result.Added = append(result.Added, FileChange{
				Path:       outputPath,
				ChangeType: ChangeAdded,
				NewContent: doc.Markdown,
			})
		} else if contentChanged(existingContent, doc.Markdown) {
			// Modified file
			result.Modified = append(result.Modified, FileChange{
				Path:       outputPath,
				ChangeType: ChangeModified,
				OldContent: existingContent,
				NewContent: doc.Markdown,
			})
		}
		// If content is the same, no change recorded
	}

	// Check for deleted files
	for existingPath, content := range existingFiles {
		if !seen[existingPath] {
			result.Deleted = append(result.Deleted, FileChange{
				Path:       existingPath,
				ChangeType: ChangeDeleted,
				OldContent: content,
			})
		}
	}

	return result, nil
}

// HasChanges returns true if there are any changes.
func (r *DiffResult) HasChanges() bool {
	return len(r.Added) > 0 || len(r.Modified) > 0 || len(r.Deleted) > 0
}

// Summary returns a human-readable summary of changes.
func (r *DiffResult) Summary() string {
	parts := []string{}
	if len(r.Added) > 0 {
		parts = append(parts, fmt.Sprintf("%d added", len(r.Added)))
	}
	if len(r.Modified) > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", len(r.Modified)))
	}
	if len(r.Deleted) > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", len(r.Deleted)))
	}
	if len(parts) == 0 {
		return "No changes"
	}
	return strings.Join(parts, ", ")
}
