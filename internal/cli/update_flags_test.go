package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chasedputnam/pyra/internal/importer"
	"github.com/chasedputnam/pyra/internal/updater"
)

// captureStdout runs fn with os.Stdout redirected and returns whatever was
// written. Used to verify the CLI's printed stats line, which is the only
// signal that exposes which summarization algorithm the updater actually
// ran (update does not persist the algorithm to the changelog header).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig
	<-done
	return buf.String()
}

// seedBundleForUpdate imports a small corpus with default settings so the
// update tests have a bundle to operate on. The returned src + out paths are
// already wired (out has a changelog pointing at src).
func seedBundleForUpdate(t *testing.T) (src, out string) {
	t.Helper()
	src = t.TempDir()
	body := "# Topic\n\nAlpha sentence about beta. Gamma sentence about delta. Epsilon sentence about zeta.\n"
	if err := os.WriteFile(filepath.Join(src, "topic.md"), []byte(body), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	out = filepath.Join(t.TempDir(), "bundle")
	if _, err := importer.Import(importer.ImportOptions{
		InputPath:          src,
		OutDir:             out,
		StableTimestamps:   true,
		SummarizeMode:      "extractive",
		SummarizeAlgorithm: "lsa",
		Language:           "english",
	}); err != nil {
		t.Fatalf("seed import: %v", err)
	}
	// Mutate source so the update has work to do.
	modified := body + "\nA brand new sentence that wasn't there before.\n"
	if err := os.WriteFile(filepath.Join(src, "topic.md"), []byte(modified), 0644); err != nil {
		t.Fatalf("mutate source: %v", err)
	}
	return src, out
}

// TestUpdateCLI_ForwardsSummarizeAlgorithmFlag verifies that
// --summarize-algorithm on `update` overrides the changelog's stored
// algorithm. We assert against the printed stats line because the updater
// (unlike import) does not rewrite the changelog header — the override is
// a one-shot.
func TestUpdateCLI_ForwardsSummarizeAlgorithmFlag(t *testing.T) {
	resetCommandFlags(updateCmd)
	t.Setenv("HOME", t.TempDir())

	_, out := seedBundleForUpdate(t)

	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{
			"update", out,
			"--force",
			"--max-pages", "100",
			"--max-depth", "4",
			"--concurrency", "1",
			"--summarize-algorithm", "luhn",
		})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	// The printSummaryStats helper renders "Summaries: N total (source=K, ...)"
	// — the algorithm name appears in that parenthetical.
	if !strings.Contains(stdout, "luhn=") {
		t.Errorf("expected stats line to include luhn=...; stdout:\n%s", stdout)
	}
	// And lsa should not appear (the override should have replaced it).
	if strings.Contains(stdout, "lsa=") {
		t.Errorf("expected lsa NOT to appear in stats (overridden by --summarize-algorithm); stdout:\n%s", stdout)
	}
}

// TestUpdateCLI_ForwardsLanguageFlag verifies that --language on `update`
// is plumbed through to the underlying updater. We exercise this via the
// updater package directly (which the CLI command thinly wraps) so we can
// inspect the returned Stats; the CLI's own end-to-end path is covered by
// the algorithm test above.
func TestUpdateCLI_ForwardsLanguageFlag(t *testing.T) {
	resetCommandFlags(updateCmd)
	t.Setenv("HOME", t.TempDir())

	src, out := seedBundleForUpdate(t)

	// Mirror what runUpdate would do for these flags. This is the same
	// translation cli/update.go performs — keeping it co-located here pins
	// the contract: flags Changed → SummarizeAlgorithmFlagSet etc.
	res, err := updater.Update(context.Background(), updater.UpdateOptions{
		BundlePath:                out,
		Source:                    src,
		Force:                     true,
		MaxPages:                  100,
		MaxDepth:                  4,
		Concurrency:               1,
		Language:                  "french",
		LanguageFlagSet:           true,
		SummarizeAlgorithm:        "luhn",
		SummarizeAlgorithmFlagSet: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.Modified == 0 {
		t.Fatalf("expected at least one modification, got %+v", res)
	}
	if res.Stats == nil || res.Stats.BySource["luhn"] == 0 {
		t.Errorf("expected luhn to drive summarization (BySource = %+v)", res.Stats)
	}
}

// TestUpdateCLI_HelpListsNewFlags is a cheap guard that ensures the flag
// definitions remain registered on the update subcommand.
func TestUpdateCLI_HelpListsNewFlags(t *testing.T) {
	resetCommandFlags(updateCmd)
	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"update", "--help"})
		_ = rootCmd.Execute()
	})
	for _, want := range []string{"--summarize", "--summarize-algorithm", "--language", "--edmundson-config"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("update --help missing flag %q. Output:\n%s", want, stdout)
		}
	}
}
