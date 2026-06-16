// Package validate handles OKF bundle validation and inspection.
package validate

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/okfy/okf-mcp/internal/graph"
	"github.com/okfy/okf-mcp/internal/reader"
	"github.com/okfy/okf-mcp/internal/types"
	"github.com/okfy/okf-mcp/internal/util"
	"gopkg.in/yaml.v3"
)

// issue creates a validation issue.
func issue(severity, code, message string, path ...string) types.ValidationIssue {
	p := ""
	if len(path) > 0 {
		p = path[0]
	}
	return types.ValidationIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Path:     p,
	}
}

// firstContentLine returns the first non-empty line of content.
func firstContentLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// validateIndexFile validates an index.md file.
func validateIndexFile(raw, relPath string, issues *[]types.ValidationIssue) {
	body := raw

	if strings.HasPrefix(raw, "---") {
		// Only root index.md may have frontmatter
		if relPath != "index.md" {
			*issues = append(*issues, issue("error", "reserved_index_frontmatter",
				"Only bundle-root index.md may contain okf_version frontmatter.", relPath))
			return
		}

		// Parse frontmatter
		endIdx := strings.Index(raw[3:], "\n---")
		if endIdx == -1 {
			*issues = append(*issues, issue("error", "malformed_frontmatter",
				"Malformed YAML frontmatter.", relPath))
			return
		}

		yamlContent := raw[3 : 3+endIdx]
		body = strings.TrimPrefix(raw[3+endIdx+4:], "\n")

		var frontmatter map[string]any
		if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
			*issues = append(*issues, issue("error", "malformed_frontmatter",
				err.Error(), relPath))
			return
		}

		// Root index.md may only have okf_version
		if len(frontmatter) != 1 {
			*issues = append(*issues, issue("error", "reserved_index_frontmatter",
				"Root index.md frontmatter may contain only string okf_version.", relPath))
		} else if _, ok := frontmatter["okf_version"].(string); !ok {
			*issues = append(*issues, issue("error", "reserved_index_frontmatter",
				"Root index.md frontmatter may contain only string okf_version.", relPath))
		}
	}

	firstLine := firstContentLine(body)
	if !strings.HasPrefix(firstLine, "# ") {
		*issues = append(*issues, issue("error", "invalid_index_structure",
			"index.md must be a markdown directory listing headed by a section title.", relPath))
	}
}

// validateLogFile validates a log.md file.
func validateLogFile(raw, relPath string, issues *[]types.ValidationIssue) {
	if strings.HasPrefix(raw, "---") {
		*issues = append(*issues, issue("error", "reserved_log_frontmatter",
			"log.md must not contain YAML frontmatter.", relPath))
		return
	}

	firstLine := firstContentLine(raw)
	if !strings.HasPrefix(firstLine, "# ") {
		*issues = append(*issues, issue("error", "invalid_log_structure",
			"log.md must be a markdown update log headed by a title.", relPath))
	}

	// Check date headings
	dateRe := regexp.MustCompile(`^##\s+(.+)$`)
	dateFormatRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\b`)
	for _, line := range strings.Split(raw, "\n") {
		match := dateRe.FindStringSubmatch(line)
		if len(match) >= 2 && !dateFormatRe.MatchString(match[1]) {
			*issues = append(*issues, issue("error", "invalid_log_date",
				"log.md date headings must use YYYY-MM-DD.", relPath))
		}
	}
}

// validateReservedFile validates a reserved OKF file.
func validateReservedFile(raw, relPath string, issues *[]types.ValidationIssue) {
	name := strings.ToLower(filepath.Base(relPath))
	if name == "index.md" {
		validateIndexFile(raw, relPath, issues)
	}
	if name == "log.md" {
		validateLogFile(raw, relPath, issues)
	}
}

// ValidateBundle validates an OKF bundle directory.
func ValidateBundle(bundleDir string) (*types.ValidationReport, error) {
	var issues []types.ValidationIssue
	var conceptCount, reservedFileCount int

	// List all markdown files
	var files []string
	err := filepath.WalkDir(bundleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return &types.ValidationReport{
			Valid:  false,
			Issues: []types.ValidationIssue{issue("error", "bundle_unreadable", err.Error())},
		}, nil
	}

	// Separate concept and reserved files
	var conceptFiles, reservedFiles []string
	for _, file := range files {
		relPath, _ := filepath.Rel(bundleDir, file)
		relPath = util.ToPosixPath(relPath)
		if reader.IsConceptMarkdownPath(relPath) {
			conceptFiles = append(conceptFiles, file)
		}
		if reader.IsReservedOKFPath(relPath) {
			reservedFiles = append(reservedFiles, file)
		}
	}

	// Validate reserved files
	for _, file := range reservedFiles {
		relPath, _ := filepath.Rel(bundleDir, file)
		relPath = util.ToPosixPath(relPath)
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		validateReservedFile(string(raw), relPath, &issues)
		reservedFileCount++
	}

	// Validate concept files
	for _, file := range conceptFiles {
		relPath, _ := filepath.Rel(bundleDir, file)
		relPath = util.ToPosixPath(relPath)

		// Check for unsafe paths
		if strings.Contains(relPath, "..") || filepath.IsAbs(relPath) {
			issues = append(issues, issue("error", "unsafe_path", "Concept path is unsafe.", relPath))
		}

		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content := string(raw)

		// Must start with frontmatter
		if !strings.HasPrefix(content, "---") {
			issues = append(issues, issue("error", "missing_frontmatter",
				"Concept file must start with YAML frontmatter.", relPath))
			continue
		}

		// Parse frontmatter
		endIdx := strings.Index(content[3:], "\n---")
		if endIdx == -1 {
			issues = append(issues, issue("error", "malformed_frontmatter",
				"Malformed YAML frontmatter.", relPath))
			continue
		}

		yamlContent := content[3 : 3+endIdx]
		var frontmatter map[string]any
		if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
			issues = append(issues, issue("error", "malformed_frontmatter",
				err.Error(), relPath))
			continue
		}

		// Check required type field
		typeVal, ok := frontmatter["type"]
		if !ok {
			issues = append(issues, issue("error", "missing_type",
				"Frontmatter type must be a non-empty string.", relPath))
		} else if typeStr, ok := typeVal.(string); !ok || strings.TrimSpace(typeStr) == "" {
			issues = append(issues, issue("error", "missing_type",
				"Frontmatter type must be a non-empty string.", relPath))
		}

		// Check optional field types
		for _, key := range []string{"title", "description", "resource", "timestamp"} {
			if val, exists := frontmatter[key]; exists {
				if _, ok := val.(string); !ok {
					issues = append(issues, issue("warning", "bad_field_shape",
						fmt.Sprintf("%s should be a string when present.", key), relPath))
				}
			}
		}

		// Check tags field
		if tags, exists := frontmatter["tags"]; exists {
			if arr, ok := tags.([]any); !ok {
				issues = append(issues, issue("warning", "bad_field_shape",
					"tags should be an array of strings when present.", relPath))
			} else {
				for _, tag := range arr {
					if _, ok := tag.(string); !ok {
						issues = append(issues, issue("warning", "bad_field_shape",
							"tags should be an array of strings when present.", relPath))
						break
					}
				}
			}
		}

		conceptCount++
	}

	// Check for broken internal links
	concepts, _ := reader.ReadBundle(bundleDir)
	canonicalIDs := make(map[string]bool)
	for _, concept := range concepts {
		canonicalIDs[concept.ID] = true
	}

	// Deduplicate concepts
	uniqueConcepts := make(map[string]*types.Concept)
	for _, concept := range concepts {
		uniqueConcepts[concept.ID] = concept
	}

	for _, concept := range uniqueConcepts {
		for _, target := range graph.ExtractInternalLinks(concept) {
			if !canonicalIDs[target] {
				issues = append(issues, issue("warning", "broken_internal_link",
					fmt.Sprintf("Broken internal link to %s.", target), concept.Path))
			}
		}
	}

	// Check for missing folder indexes
	dirs := make(map[string]bool)
	for _, file := range conceptFiles {
		dirs[filepath.Dir(file)] = true
	}
	for dir := range dirs {
		indexPath := filepath.Join(dir, "index.md")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			relDir, _ := filepath.Rel(bundleDir, dir)
			if relDir == "" {
				relDir = "."
			}
			issues = append(issues, issue("warning", "missing_folder_index",
				"Folder has concepts but no index.md.", util.ToPosixPath(relDir)))
		}
	}

	warningCount := 0
	hasErrors := false
	for _, iss := range issues {
		if iss.Severity == "warning" {
			warningCount++
		}
		if iss.Severity == "error" {
			hasErrors = true
		}
	}

	return &types.ValidationReport{
		Valid:             !hasErrors,
		Issues:            issues,
		ConceptCount:      conceptCount,
		ReservedFileCount: reservedFileCount,
		WarningCount:      warningCount,
	}, nil
}

// InspectBundle returns statistics about a bundle.
func InspectBundle(bundleDir string) (*types.BundleStats, error) {
	concepts, err := reader.ReadBundle(bundleDir)
	if err != nil {
		return nil, err
	}

	kg := graph.BuildGraph(concepts)

	// Calculate stats
	typeDistribution := make(map[string]int)
	tagDistribution := make(map[string]int)
	sourceDomains := make(map[string]int)

	for _, concept := range kg.Concepts {
		typeDistribution[concept.Type]++
		for _, tag := range concept.Tags {
			tagDistribution[tag]++
		}
		if concept.Resource != "" && strings.HasPrefix(concept.Resource, "http") {
			if u, err := url.Parse(concept.Resource); err == nil {
				sourceDomains[u.Hostname()]++
			}
		}
	}

	// Count links and broken links
	linkCount := 0
	brokenLinks := 0
	for _, targets := range kg.Outbound {
		linkCount += len(targets)
	}

	// Find orphans (concepts with no backlinks and not the root)
	var orphanConcepts []string
	for id, backlinks := range kg.Backlinks {
		if len(backlinks) == 0 && id != "index" {
			orphanConcepts = append(orphanConcepts, id)
		}
	}
	sort.Strings(orphanConcepts)

	// Top linked concepts
	type linkedCount struct {
		id    string
		title string
		count int
	}
	var linked []linkedCount
	for id, backlinks := range kg.Backlinks {
		concept := kg.Concepts[id]
		title := ""
		if concept != nil {
			title = concept.Title
		}
		linked = append(linked, linkedCount{id, title, len(backlinks)})
	}
	sort.Slice(linked, func(i, j int) bool {
		if linked[i].count != linked[j].count {
			return linked[i].count > linked[j].count
		}
		return linked[i].id < linked[j].id
	})

	topLinked := make([]types.LinkedConcept, 0, 10)
	for i := 0; i < len(linked) && i < 10; i++ {
		topLinked = append(topLinked, types.LinkedConcept{
			ID:    linked[i].id,
			Title: linked[i].title,
			Count: linked[i].count,
		})
	}

	// Get title from root index or use default
	title := "OKF Bundle"
	if rootIndex, err := os.ReadFile(filepath.Join(bundleDir, "index.md")); err == nil {
		if match := regexp.MustCompile(`(?m)^#\s+(.+)$`).FindStringSubmatch(string(rootIndex)); len(match) >= 2 {
			title = strings.TrimSpace(match[1])
		}
	}

	return &types.BundleStats{
		Title:             title,
		ConceptCount:      len(kg.Concepts),
		LinkCount:         linkCount,
		BrokenLinks:       brokenLinks,
		OrphanConcepts:    orphanConcepts,
		TypeDistribution:  typeDistribution,
		TagDistribution:   tagDistribution,
		TopLinkedConcepts: topLinked,
		SourceDomains:     sourceDomains,
	}, nil
}
