// Package reader handles reading OKF bundles from disk.
package reader

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/okfy/okf-mcp/internal/types"
	"github.com/okfy/okf-mcp/internal/util"
	"gopkg.in/yaml.v3"
)

// reservedFilenames are OKF reserved filenames that are not concepts.
var reservedFilenames = map[string]bool{
	"index.md": true,
	"log.md":   true,
}

// IsReservedOKFPath checks if a path is a reserved OKF filename.
func IsReservedOKFPath(p string) bool {
	base := strings.ToLower(filepath.Base(util.ToPosixPath(p)))
	return reservedFilenames[base]
}

// IsConceptMarkdownPath checks if a path is a concept markdown file.
func IsConceptMarkdownPath(p string) bool {
	lower := strings.ToLower(p)
	return strings.HasSuffix(lower, ".md") && !IsReservedOKFPath(p)
}

// listMarkdownFiles recursively lists all markdown files in a directory.
func listMarkdownFiles(dir string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			result = append(result, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// stringArray extracts a string array from an interface.
func stringArray(value any) []string {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// parseFrontmatter parses YAML frontmatter from markdown content.
func parseFrontmatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(content, "---") {
		return nil, content, nil
	}

	// Find the closing ---
	rest := content[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return nil, content, nil
	}

	yamlContent := rest[:endIdx]
	body := strings.TrimPrefix(rest[endIdx+4:], "\n")

	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return nil, "", err
	}

	return frontmatter, body, nil
}

// ReadConceptFile reads a single concept file from a bundle.
func ReadConceptFile(bundleDir, absolutePath string) (*types.Concept, error) {
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(bundleDir, absolutePath)
	if err != nil {
		return nil, err
	}
	relPath = util.ToPosixPath(relPath)

	if IsReservedOKFPath(relPath) {
		return nil, nil // Not a concept
	}

	frontmatter, body, err := parseFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	id := util.StripMdExtension(relPath)

	concept := &types.Concept{
		ID:          id,
		Path:        relPath,
		Frontmatter: frontmatter,
		Body:        strings.TrimSpace(body),
	}

	// Extract typed fields from frontmatter
	if frontmatter != nil {
		if t, ok := frontmatter["type"].(string); ok {
			concept.Type = t
		}
		if t, ok := frontmatter["title"].(string); ok {
			concept.Title = t
		}
		if d, ok := frontmatter["description"].(string); ok {
			concept.Description = d
		}
		if r, ok := frontmatter["resource"].(string); ok {
			concept.Resource = r
		}
		concept.Tags = stringArray(frontmatter["tags"])
	}

	return concept, nil
}

// ReadBundle reads all concepts from a bundle directory.
// Returns a map keyed by both ID and path for flexible lookup.
func ReadBundle(bundleDir string) (map[string]*types.Concept, error) {
	files, err := listMarkdownFiles(bundleDir)
	if err != nil {
		return nil, err
	}

	concepts := make(map[string]*types.Concept)
	for _, file := range files {
		relPath, err := filepath.Rel(bundleDir, file)
		if err != nil {
			continue
		}
		relPath = util.ToPosixPath(relPath)

		if !IsConceptMarkdownPath(relPath) {
			continue
		}

		concept, err := ReadConceptFile(bundleDir, file)
		if err != nil {
			continue
		}
		if concept == nil {
			continue
		}

		concepts[concept.ID] = concept
		concepts[concept.Path] = concept
	}

	return concepts, nil
}
