package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/okfy/okf-mcp/internal/changelog"
	"github.com/okfy/okf-mcp/internal/updater"
	"github.com/okfy/okf-mcp/internal/validate"
)

// CheckUpdatesResult is returned by check_updates tool.
type CheckUpdatesResult struct {
	HasChanges  bool       `json:"has_changes"`
	Added       int        `json:"added"`
	Modified    int        `json:"modified"`
	Deleted     int        `json:"deleted"`
	SourceURL   string     `json:"source_url"`
	LastUpdated *time.Time `json:"last_updated,omitempty"`
	StaleSince  *time.Time `json:"stale_since,omitempty"`
}

func (s *Server) handleCheckUpdates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	timeoutF := getArgFloat(args, "timeout_seconds")

	timeout := time.Duration(timeoutF) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Get source from changelog
	source, err := changelog.GetSource(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "no_source",
				"message": "No source found in changelog.txt. Bundle may not have been created with crawl/import.",
			},
		})
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use updater in dry-run mode to check for changes
	result, err := updater.Update(ctx, updater.UpdateOptions{
		BundlePath: s.bundleDir,
		Source:     source,
		DryRun:     true,
		Force:      true, // Don't prompt
	})
	if err != nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "update_check_failed",
				"message": err.Error(),
			},
		})
	}

	hasChanges := result.Added > 0 || result.Modified > 0 || result.Deleted > 0

	response := CheckUpdatesResult{
		HasChanges: hasChanges,
		Added:      result.Added,
		Modified:   result.Modified,
		Deleted:    result.Deleted,
		SourceURL:  source,
	}

	// Get last updated time from changelog
	changelogPath := filepath.Join(s.bundleDir, "changelog.txt")
	if info, err := os.Stat(changelogPath); err == nil {
		modTime := info.ModTime()
		response.LastUpdated = &modTime
		if hasChanges {
			response.StaleSince = &modTime
		}
	}

	return s.jsonResult(response)
}

// ApplyUpdatesResult is returned by apply_updates tool.
type ApplyUpdatesResult struct {
	Success  bool     `json:"success"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
	Errors   []string `json:"errors,omitempty"`
}

func (s *Server) handleApplyUpdates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	dryRun := getArgBool(args, "dry_run", false)
	confirm := getArgBool(args, "confirm", false)

	// Safety check: require explicit confirmation
	if !confirm && !dryRun {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "confirmation_required",
				"message": "Set confirm=true to apply updates, or use dry_run=true to preview changes.",
			},
		})
	}

	// Get source from changelog
	source, err := changelog.GetSource(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "no_source",
				"message": "No source found in changelog.txt.",
			},
		})
	}

	result, err := updater.Update(ctx, updater.UpdateOptions{
		BundlePath: s.bundleDir,
		Source:     source,
		DryRun:     dryRun,
		Force:      true,
		OnProgress: func(phase, detail string) {
			// Could track progress here if needed
		},
	})
	if err != nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "update_failed",
				"message": err.Error(),
			},
		})
	}

	// Reload search index if changes were applied
	if !dryRun && (result.Added > 0 || result.Modified > 0 || result.Deleted > 0) {
		if err := s.reloadSearchIndex(); err != nil {
			return s.jsonResult(ApplyUpdatesResult{
				Success:  true,
				Added:    result.AddedFiles,
				Modified: result.ModifiedFiles,
				Deleted:  result.DeletedFiles,
				Errors:   []string{fmt.Sprintf("Index reload failed: %v", err)},
			})
		}
	}

	return s.jsonResult(ApplyUpdatesResult{
		Success:  true,
		Added:    result.AddedFiles,
		Modified: result.ModifiedFiles,
		Deleted:  result.DeletedFiles,
	})
}

// BundleHealthResult is returned by bundle_health tool.
type BundleHealthResult struct {
	Valid           bool      `json:"valid"`
	ConceptCount    int       `json:"concept_count"`
	LastUpdated     *time.Time `json:"last_updated,omitempty"`
	SourceURL       string    `json:"source_url,omitempty"`
	SourceReachable *bool     `json:"source_reachable,omitempty"`
	BrokenLinks     int       `json:"broken_links"`
	Warnings        []string  `json:"warnings,omitempty"`
}

func (s *Server) handleBundleHealth(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	checkSource := getArgBool(args, "check_source", true)

	// Validate bundle
	report, err := validate.ValidateBundle(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "validation_failed",
				"message": err.Error(),
			},
		})
	}

	// Get bundle stats
	stats, err := validate.InspectBundle(s.bundleDir)
	if err != nil {
		return s.jsonResult(map[string]any{
			"error": map[string]any{
				"code":    "inspection_failed",
				"message": err.Error(),
			},
		})
	}

	result := BundleHealthResult{
		Valid:        report.Valid,
		ConceptCount: stats.ConceptCount,
		BrokenLinks:  stats.BrokenLinks,
	}

	// Collect warnings
	for _, issue := range report.Issues {
		if issue.Severity == "warning" {
			result.Warnings = append(result.Warnings, issue.Message)
		}
	}

	// Get source from changelog
	source, err := changelog.GetSource(s.bundleDir)
	if err == nil {
		result.SourceURL = source
	}

	// Get last updated time
	changelogPath := filepath.Join(s.bundleDir, "changelog.txt")
	if info, err := os.Stat(changelogPath); err == nil {
		modTime := info.ModTime()
		result.LastUpdated = &modTime
	}

	// Check source reachability if requested
	if checkSource && result.SourceURL != "" {
		reachable := checkSourceReachable(ctx, result.SourceURL)
		result.SourceReachable = &reachable
	}

	return s.jsonResult(result)
}

// checkSourceReachable checks if a source URL or path is reachable.
func checkSourceReachable(ctx context.Context, source string) bool {
	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		parsed, err := url.Parse(source)
		if err != nil {
			return false
		}

		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequestWithContext(ctx, "HEAD", parsed.String(), nil)
		if err != nil {
			return false
		}

		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		return resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	// Treat as local path
	_, err := os.Stat(source)
	return err == nil
}
