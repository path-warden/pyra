package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/changelog"
	"github.com/chasedputnam/pyra/internal/importer"
)

// TestUpdate_UsesChangelogPreferences verifies that when no flags are
// provided to update, the summarization preferences stored in the changelog
// at import time are used to regenerate summaries.
func TestUpdate_UsesChangelogPreferences(t *testing.T) {
	src := t.TempDir()
	body := "# Photosynthesis\n\nPhotosynthesis converts sunlight into chemical energy. Plants use this process to grow. Most life depends on it.\n"
	if err := os.WriteFile(filepath.Join(src, "photo.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out := filepath.Join(t.TempDir(), "bundle")
	if _, err := importer.Import(importer.ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "lexrank",
		Language:           "english",
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Confirm preferences were recorded.
	cl, err := changelog.ReadChangelog(out)
	if err != nil {
		t.Fatalf("ReadChangelog: %v", err)
	}
	if cl.SummarizeAlgorithm != "lexrank" {
		t.Fatalf("expected lexrank, got %q", cl.SummarizeAlgorithm)
	}

	// Modify the source file so update has actual work to do, then call
	// Update without flags — it should pick up lexrank from the changelog.
	if err := os.WriteFile(filepath.Join(src, "photo.md"), []byte(body+"\nA brand new sentence that did not exist before.\n"), 0644); err != nil {
		t.Fatalf("modify: %v", err)
	}

	_, err = Update(context.Background(), UpdateOptions{
		BundlePath:  out,
		Source:      src,
		Force:       true,
		MaxPages:    100,
		MaxDepth:    4,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(out, "photo.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(updated), "> [!summary]") {
		t.Errorf("expected summary callout in updated bundle")
	}
}

// TestUpdate_PreservesUnchangedSummaries verifies the spec requirement:
// "WHEN a file is unchanged during update THEN the system SHALL preserve the
// existing summary." With the differ stripping the injected `> [!summary]`
// callout before comparing, an update against an identical source must
// detect zero modifications, so the original summary text survives byte-for-
// byte. (Before the differ fix, every file looked modified on every update
// and the summary was regenerated each time, defeating this requirement.)
func TestUpdate_PreservesUnchangedSummaries(t *testing.T) {
	src := t.TempDir()
	body := "# Photosynthesis\n\nPhotosynthesis converts sunlight into chemical energy. Plants use this process to grow. Most life on Earth depends on it.\n"
	if err := os.WriteFile(filepath.Join(src, "photo.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out := filepath.Join(t.TempDir(), "bundle")
	if _, err := importer.Import(importer.ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "lsa",
		Language:           "english",
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	original, err := os.ReadFile(filepath.Join(out, "photo.md"))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if !strings.Contains(string(original), "> [!summary]") {
		t.Fatalf("import did not inject summary callout")
	}

	// Source is byte-identical to what was imported; update must be a no-op
	// against the body and must NOT rewrite the file (which would change
	// the timestamp in the frontmatter).
	res, err := Update(context.Background(), UpdateOptions{
		BundlePath:  out,
		Source:      src,
		Force:       true,
		MaxPages:    100,
		MaxDepth:    4,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Added != 0 || res.Modified != 0 || res.Deleted != 0 {
		t.Errorf("expected zero changes, got added=%d modified=%d deleted=%d", res.Added, res.Modified, res.Deleted)
	}

	after, err := os.ReadFile(filepath.Join(out, "photo.md"))
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != string(original) {
		t.Errorf("expected file to be untouched.\n--- original ---\n%s\n--- after ---\n%s", original, after)
	}
}

// TestUpdate_StatsAndProgressSurfaced verifies that an update which rewrites
// files reports per-source summarization counts in result.Stats AND fires
// the per-file OnSummarizeProgress callback exactly once per (re)written
// file. Before this fix, update silently regenerated summaries without
// aggregating or surfacing any stats.
func TestUpdate_StatsAndProgressSurfaced(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"a.md": "# A\n\nFirst doc body. Second sentence about A. Third sentence.\n",
		"b.md": "# B\n\nFirst doc body. Second sentence about B. Third sentence.\n",
		"c.md": "# C\n\nFirst doc body. Second sentence about C. Third sentence.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	out := filepath.Join(t.TempDir(), "bundle")
	if _, err := importer.Import(importer.ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "lsa",
		Language:           "english",
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Modify all 3 source files so update has work to do for each.
	for name, body := range files {
		modified := body + "\nAdded line that changes the body.\n"
		if err := os.WriteFile(filepath.Join(src, name), []byte(modified), 0644); err != nil {
			t.Fatalf("modify %s: %v", name, err)
		}
	}

	var progressCalls []string
	var lastIndex, lastTotal int
	res, err := Update(context.Background(), UpdateOptions{
		BundlePath:  out,
		Source:      src,
		Force:       true,
		MaxPages:    100,
		MaxDepth:    4,
		Concurrency: 1,
		OnSummarizeProgress: func(index, total int, path string) {
			progressCalls = append(progressCalls, path)
			lastIndex = index
			lastTotal = total
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if res.Modified != 3 {
		t.Fatalf("expected 3 modified, got added=%d modified=%d", res.Added, res.Modified)
	}

	// Per-file progress callback fired once per modified file.
	if len(progressCalls) != 3 {
		t.Errorf("expected 3 OnSummarizeProgress calls, got %d (%v)", len(progressCalls), progressCalls)
	}
	if lastIndex != 3 || lastTotal != 3 {
		t.Errorf("expected final (index, total) = (3, 3), got (%d, %d)", lastIndex, lastTotal)
	}

	// Stats aggregated and surfaced.
	if res.Stats == nil {
		t.Fatal("expected non-nil Stats on result")
	}
	if res.Stats.Total != 3 {
		t.Errorf("expected Stats.Total=3, got %d", res.Stats.Total)
	}
	// The default isn't guaranteed to be lsa specifically across all files,
	// but BySource should aggregate to the same total.
	sum := 0
	for _, n := range res.Stats.BySource {
		sum += n
	}
	if sum != 3 {
		t.Errorf("expected BySource counts to sum to 3, got %d (%+v)", sum, res.Stats.BySource)
	}
}

// TestUpdate_NoChangesProducesNilStats verifies that an update where nothing
// is rewritten leaves Stats nil (so the CLI doesn't print a misleading
// "Summaries: 0 total" line).
func TestUpdate_NoChangesProducesNilStats(t *testing.T) {
	src := t.TempDir()
	body := "# A\n\nUnchanged body.\n"
	if err := os.WriteFile(filepath.Join(src, "a.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out := filepath.Join(t.TempDir(), "bundle")
	if _, err := importer.Import(importer.ImportOptions{
		InputPath:        src,
		OutDir:           out,
		StableTimestamps: true,
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// No source changes — update should report 0 changes and nil stats.
	res, err := Update(context.Background(), UpdateOptions{
		BundlePath:  out,
		Source:      src,
		Force:       true,
		MaxPages:    100,
		MaxDepth:    4,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Added != 0 || res.Modified != 0 || res.Deleted != 0 {
		t.Errorf("expected zero changes, got added=%d modified=%d deleted=%d", res.Added, res.Modified, res.Deleted)
	}
	if res.Stats != nil {
		t.Errorf("expected nil Stats on no-op update, got %+v", res.Stats)
	}
}

// TestUpdate_RegeneratesSummaryWithConfiguredAlgorithm verifies that update
// with an explicit algorithm flag rewrites the summary using that algorithm.
func TestUpdate_RegeneratesSummaryWithConfiguredAlgorithm(t *testing.T) {
	src := t.TempDir()
	body := "# Renewable Energy\n\nRenewable energy is energy derived from natural sources. Wind and sun replenish themselves continuously. Coal does not.\n"
	if err := os.WriteFile(filepath.Join(src, "energy.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out := filepath.Join(t.TempDir(), "bundle")
	if _, err := importer.Import(importer.ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "lsa",
		Language:           "english",
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Modify source so update has work to do.
	if err := os.WriteFile(filepath.Join(src, "energy.md"), []byte(body+"\nAdditional context line.\n"), 0644); err != nil {
		t.Fatalf("modify: %v", err)
	}

	// Update with explicit algorithm flag — should use luhn this time.
	_, err := Update(context.Background(), UpdateOptions{
		BundlePath:                out,
		Source:                    src,
		Force:                     true,
		MaxPages:                  100,
		MaxDepth:                  4,
		Concurrency:               1,
		SummarizeAlgorithm:        "luhn",
		SummarizeAlgorithmFlagSet: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(out, "energy.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(updated), "> [!summary]") {
		t.Errorf("expected summary callout")
	}
}
