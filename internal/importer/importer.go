// Package importer handles importing local files into OKF bundles.
package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okfy/okf-mcp/internal/normalize"
	"github.com/okfy/okf-mcp/internal/types"
	"github.com/okfy/okf-mcp/internal/util"
	"github.com/okfy/okf-mcp/internal/writer"
)

// ImportOptions configures the import operation.
type ImportOptions struct {
	InputPath                    string
	OutDir                       string
	SourceName                   string
	Include                      []string
	Exclude                      []string
	Force                        bool
	DangerouslyAllowUnsafeOutput bool
	StableTimestamps             bool
}

// skipDirs are directories to skip during import.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
}

// contentTypeFor returns the content type for a file extension.
func contentTypeFor(file string) types.ContentType {
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".md":
		return types.ContentTypeMarkdown
	case ".mdx":
		return types.ContentTypeMDX
	case ".html", ".htm":
		return types.ContentTypeHTML
	case ".txt":
		return types.ContentTypeText
	default:
		return ""
	}
}

// listFiles recursively lists all supported files in a directory.
func listFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{root}, nil
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// Import imports local files into an OKF bundle.
func Import(opts ImportOptions) (*types.ImportResult, error) {
	root, err := filepath.Abs(opts.InputPath)
	if err != nil {
		return nil, err
	}

	files, err := listFiles(root)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	if opts.StableTimestamps {
		timestamp = "2026-06-14T00:00:00.000Z"
	}

	var docs []types.NormalizedDocument
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			continue
		}
		rel = util.ToPosixPath(rel)

		// Check include patterns
		if len(opts.Include) > 0 && !util.MatchesAnyPattern(rel, opts.Include) {
			continue
		}

		// Check exclude patterns
		if util.MatchesAnyPattern(rel, opts.Exclude) {
			continue
		}

		// Check content type
		contentType := contentTypeFor(file)
		if contentType == "" {
			continue
		}

		// Read file
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		raw := types.RawDocument{
			SourceID:     rel,
			FilePath:     rel,
			ContentType:  contentType,
			Raw:          string(content),
			DiscoveredAt: time.Now(),
		}

		docs = append(docs, normalize.NormalizeDocument(raw))
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no supported Markdown, MDX, HTML, or text files found")
	}

	written, err := writer.WriteOKFBundle(docs, writer.WriteOptions{
		OutDir:                       opts.OutDir,
		Title:                        opts.SourceName,
		SourceName:                   opts.SourceName,
		Force:                        opts.Force,
		InputPath:                    root,
		DangerouslyAllowUnsafeOutput: opts.DangerouslyAllowUnsafeOutput,
		Timestamp:                    timestamp,
	})
	if err != nil {
		return nil, err
	}

	return &types.ImportResult{
		Written:   written,
		Documents: docs,
	}, nil
}
