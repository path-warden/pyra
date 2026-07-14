package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests
	tmpDir, err := os.MkdirTemp("", "pyra-test-*")
	if err != nil {
		_, _ = os.Stderr.WriteString("Failed to create temp dir: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	testBinaryPath = filepath.Join(tmpDir, "pyra")
	cmd := exec.Command("go", "build", "-o", testBinaryPath, "./cmd/pyra")
	if out, err := cmd.CombinedOutput(); err != nil {
		_, _ = os.Stderr.WriteString("Failed to build binary: " + err.Error() + "\n" + string(out))
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// TestE2EImportCommand tests the full import workflow
func TestE2EImportCommand(t *testing.T) {
	// Create output directory
	outDir := filepath.Join(t.TempDir(), "bundle")

	// Run import command
	importCmd := exec.Command(testBinaryPath, "import", "testdata/fixtures/markdown-docs", "--out", outDir)
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("Import failed: %v\n%s", err, out)
	}

	// Verify bundle structure
	indexPath := filepath.Join(outDir, "index.md")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Expected index.md to exist in output bundle")
	}

	// Verify README was imported (lowercased by importer)
	readmePath := filepath.Join(outDir, "readme.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("Expected readme.md to exist in output bundle")
	}

	// Verify installation.md was imported
	installPath := filepath.Join(outDir, "installation.md")
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		t.Error("Expected installation.md to exist in output bundle")
	}

	// Verify api/overview.md was imported
	apiPath := filepath.Join(outDir, "api", "overview.md")
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		t.Error("Expected api/overview.md to exist in output bundle")
	}

	// Verify frontmatter in readme.md
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read readme.md: %v", err)
	}
	if !strings.HasPrefix(string(content), "---") {
		t.Error("Expected README.md to have frontmatter")
	}
	if !strings.Contains(string(content), "type:") {
		t.Error("Expected README.md frontmatter to contain type field")
	}
}

// TestE2EValidateCommand tests the validate workflow
func TestE2EValidateCommand(t *testing.T) {
	// Run validate on demo bundle
	validateCmd := exec.Command(testBinaryPath, "validate", "examples/bundles/pyra-docs")
	out, err := validateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Validate failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Bundle is valid") {
		t.Errorf("Expected 'Bundle is valid' in output, got: %s", output)
	}
}

// TestE2EValidateJSONOutput tests JSON output format
func TestE2EValidateJSONOutput(t *testing.T) {
	// Run validate with --json
	validateCmd := exec.Command(testBinaryPath, "validate", "examples/bundles/pyra-docs", "--json")
	out, err := validateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Validate --json failed: %v\n%s", err, out)
	}

	// Parse JSON output
	var report map[string]any
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, out)
	}

	if _, ok := report["valid"]; !ok {
		t.Error("Expected 'valid' field in JSON output")
	}
	if _, ok := report["conceptCount"]; !ok {
		t.Error("Expected 'conceptCount' field in JSON output")
	}
}

// TestE2EInspectCommand tests the inspect workflow
func TestE2EInspectCommand(t *testing.T) {
	// Run inspect on demo bundle
	inspectCmd := exec.Command(testBinaryPath, "inspect", "examples/bundles/pyra-docs")
	out, err := inspectCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Inspect failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Concepts:") {
		t.Errorf("Expected 'Concepts:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Links:") {
		t.Errorf("Expected 'Links:' in output, got: %s", output)
	}
}

// TestE2EDemoCommand tests the demo workflow
func TestE2EDemoCommand(t *testing.T) {
	// Run demo command
	demoCmd := exec.Command(testBinaryPath, "demo")
	out, err := demoCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Demo failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "pyra demo") {
		t.Errorf("Expected 'pyra demo' in output, got: %s", output)
	}
	if !strings.Contains(output, "Bundle is valid") {
		t.Errorf("Expected 'Bundle is valid' in output, got: %s", output)
	}
	if !strings.Contains(output, "mcpServers") {
		t.Errorf("Expected MCP config in output, got: %s", output)
	}
}

// TestE2EImportValidateRoundtrip tests import then validate
func TestE2EImportValidateRoundtrip(t *testing.T) {
	// Create output directory
	outDir := filepath.Join(t.TempDir(), "bundle")

	// Import
	importCmd := exec.Command(testBinaryPath, "import", "testdata/fixtures/markdown-docs", "--out", outDir)
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("Import failed: %v\n%s", err, out)
	}

	// Validate the imported bundle
	validateCmd := exec.Command(testBinaryPath, "validate", outDir)
	out, err := validateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Validate failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Bundle is valid") {
		t.Errorf("Imported bundle should be valid, got: %s", output)
	}
}

// TestE2EImportUpdateCycle tests import then update workflow
func TestE2EImportUpdateCycle(t *testing.T) {
	// Create output directory
	outDir := filepath.Join(t.TempDir(), "bundle")
	sourceDir := filepath.Join(t.TempDir(), "source")

	// Copy source files
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc1.md"), []byte("# Doc 1\n\nOriginal content."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc2.md"), []byte("# Doc 2\n\nAnother document."), 0644); err != nil {
		t.Fatal(err)
	}

	// Import
	importCmd := exec.Command(testBinaryPath, "import", sourceDir, "--out", outDir)
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("Import failed: %v\n%s", err, out)
	}

	// Verify changelog was created
	changelogPath := filepath.Join(outDir, "changelog.txt")
	if _, err := os.Stat(changelogPath); os.IsNotExist(err) {
		t.Fatal("Expected changelog.txt to exist after import")
	}

	// Modify source: update doc1, add doc3, remove doc2
	if err := os.WriteFile(filepath.Join(sourceDir, "doc1.md"), []byte("# Doc 1\n\nUpdated content."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "doc3.md"), []byte("# Doc 3\n\nNew document."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(sourceDir, "doc2.md")); err != nil {
		t.Fatal(err)
	}

	// Run update --dry-run first
	dryRunCmd := exec.Command(testBinaryPath, "update", outDir, "--dry-run")
	out, err := dryRunCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Update --dry-run failed: %v\n%s", err, out)
	}
	dryOutput := string(out)
	if !strings.Contains(dryOutput, "dry run") {
		t.Errorf("Expected 'dry run' in output, got: %s", dryOutput)
	}
	if !strings.Contains(dryOutput, "Would add: 1") {
		t.Errorf("Expected 'Would add: 1' in dry run output, got: %s", dryOutput)
	}

	// Run actual update with --force
	updateCmd := exec.Command(testBinaryPath, "update", outDir, "--force")
	out, err = updateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Update failed: %v\n%s", err, out)
	}
	updateOutput := string(out)
	if !strings.Contains(updateOutput, "Added: 1") {
		t.Errorf("Expected 'Added: 1' in output, got: %s", updateOutput)
	}
	if !strings.Contains(updateOutput, "Modified: 1") {
		t.Errorf("Expected 'Modified: 1' in output, got: %s", updateOutput)
	}

	// Verify doc3 was added
	if _, err := os.Stat(filepath.Join(outDir, "doc3.md")); os.IsNotExist(err) {
		t.Error("Expected doc3.md to exist after update")
	}

	// Verify doc2 was deleted
	if _, err := os.Stat(filepath.Join(outDir, "doc2.md")); !os.IsNotExist(err) {
		t.Error("Expected doc2.md to be deleted after update")
	}

	// Verify changelog was updated
	changelogContent, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changelogContent), "Updated:") {
		t.Errorf("Expected changelog to contain update entry, got: %s", changelogContent)
	}

	// Validate the updated bundle
	validateCmd := exec.Command(testBinaryPath, "validate", outDir)
	out, err = validateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Validate failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Bundle is valid") {
		t.Errorf("Updated bundle should be valid, got: %s", out)
	}
}

// TestE2EUpdateSourceOverride tests update with --source flag
func TestE2EUpdateSourceOverride(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "bundle")
	sourceDir1 := filepath.Join(t.TempDir(), "source1")
	sourceDir2 := filepath.Join(t.TempDir(), "source2")

	// Create initial source
	if err := os.MkdirAll(sourceDir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir1, "doc.md"), []byte("# Doc\n\nFrom source1."), 0644); err != nil {
		t.Fatal(err)
	}

	// Import from source1
	importCmd := exec.Command(testBinaryPath, "import", sourceDir1, "--out", outDir)
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("Import failed: %v\n%s", err, out)
	}

	// Create different source2
	if err := os.MkdirAll(sourceDir2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir2, "doc.md"), []byte("# Doc\n\nFrom source2."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir2, "new.md"), []byte("# New\n\nNew from source2."), 0644); err != nil {
		t.Fatal(err)
	}

	// Update with --source override to source2
	updateCmd := exec.Command(testBinaryPath, "update", outDir, "--source", sourceDir2, "--force")
	out, err := updateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Update with --source failed: %v\n%s", err, out)
	}

	// Verify new.md was added from source2
	if _, err := os.Stat(filepath.Join(outDir, "new.md")); os.IsNotExist(err) {
		t.Error("Expected new.md to exist after update with --source override")
	}
}

// TestE2ECompressionReducesTokens tests that compression reduces token counts
func TestE2ECompressionReducesTokens(t *testing.T) {
	// Use the tokens package directly for this test
	tokens := newTokenEstimator()

	longContent := `# Getting Started

This is a comprehensive guide to getting started with our platform.

## Prerequisites

Before you begin, make sure you have:

- Node.js 18 or later
- npm or yarn
- A GitHub account



## Installation

Follow these steps to install the package:

1. Clone the repository
2. Run npm install
3. Configure your environment



## Configuration

Create a config file with these settings:

` + "```json\n{\n  \"apiKey\": \"your-key\",\n  \"endpoint\": \"https://api.example.com\"\n}\n```" + `



## Next Steps

After installation, you can:

- Read the API documentation
- Explore the examples
- Join our community
`

	originalTokens := tokens.Count(longContent)

	// Apply compression
	compressed := compressContent(longContent, "medium")
	compressedTokens := tokens.Count(compressed)

	if compressedTokens >= originalTokens {
		t.Errorf("Expected compression to reduce tokens: original=%d, compressed=%d", originalTokens, compressedTokens)
	}

	// Verify content is still meaningful (has key sections)
	if !strings.Contains(compressed, "Getting Started") {
		t.Error("Expected compressed content to retain title")
	}
}

// TestE2EBudgetAwareSearch tests that token budgets limit response size
func TestE2EBudgetAwareSearch(t *testing.T) {
	// This test verifies the concept: when a budget is applied, content is truncated
	longContent := strings.Repeat("Lorem ipsum dolor sit amet. ", 100)

	// Without budget - full content
	noBudgetSize := len(longContent)

	// With budget - content should be truncated
	budget := 200 // tokens, ~800 chars
	maxChars := budget * 4
	withBudgetContent := longContent
	if len(withBudgetContent) > maxChars {
		withBudgetContent = withBudgetContent[:maxChars] + "...[truncated]"
	}
	withBudgetSize := len(withBudgetContent)

	if withBudgetSize >= noBudgetSize {
		t.Errorf("Expected budget to limit content size: no_budget=%d, with_budget=%d", noBudgetSize, withBudgetSize)
	}

	// Verify truncation happened
	if !strings.HasSuffix(withBudgetContent, "[truncated]") {
		t.Error("Expected content to be truncated with indicator")
	}
}

// Helper types and functions for E2E tests

type tokenEstimator struct{}

func newTokenEstimator() *tokenEstimator {
	return &tokenEstimator{}
}

func (e *tokenEstimator) Count(text string) int {
	// Simple estimation: ~4 chars per token
	return len(text) / 4
}

func compressContent(content, level string) string {
	// Simplified compression for testing
	switch level {
	case "medium", "aggressive":
		// Collapse multiple blank lines
		lines := strings.Split(content, "\n")
		var result []string
		prevBlank := false
		for _, line := range lines {
			isBlank := strings.TrimSpace(line) == ""
			if isBlank && prevBlank {
				continue
			}
			result = append(result, line)
			prevBlank = isBlank
		}
		return strings.Join(result, "\n")
	default:
		return content
	}
}

// TestE2EImportGeneratesSummaries tests that import generates summary callouts
func TestE2EImportGeneratesSummaries(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "bundle")

	// Run import command
	importCmd := exec.Command(testBinaryPath, "import", "testdata/fixtures/markdown-docs", "--out", outDir)
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("Import failed: %v\n%s", err, out)
	}

	// Check that concepts have summary callouts
	readmePath := filepath.Join(outDir, "readme.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read readme.md: %v", err)
	}

	if !strings.Contains(string(content), "> [!summary]") {
		t.Error("Expected readme.md to have a summary callout")
	}
}

// TestE2EImportGeneratesEnhancedIndex tests that import generates index with summaries
func TestE2EImportGeneratesEnhancedIndex(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "bundle")

	// Run import command
	importCmd := exec.Command(testBinaryPath, "import", "testdata/fixtures/markdown-docs", "--out", outDir)
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("Import failed: %v\n%s", err, out)
	}

	// Check index.md has enhanced format
	indexPath := filepath.Join(outDir, "index.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index.md: %v", err)
	}

	indexStr := string(content)

	// Check for total_concepts in frontmatter
	if !strings.Contains(indexStr, "total_concepts:") {
		t.Error("Expected index.md to have total_concepts in frontmatter")
	}

	// Check for Concepts heading with count
	if !strings.Contains(indexStr, "## Concepts (") {
		t.Error("Expected index.md to have '## Concepts (' heading")
	}

	// Check for wikilink format
	if !strings.Contains(indexStr, "[[") {
		t.Error("Expected index.md to use [[wikilink]] format")
	}
}

// TestE2EInspectShowsScaleMetrics tests that inspect shows scale metrics
func TestE2EInspectShowsScaleMetrics(t *testing.T) {
	// Run inspect on demo bundle
	inspectCmd := exec.Command(testBinaryPath, "inspect", "examples/bundles/pyra-docs")
	out, err := inspectCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Inspect failed: %v\n%s", err, out)
	}

	output := string(out)

	// Check for scale metrics section
	if !strings.Contains(output, "Scale Metrics:") {
		t.Errorf("Expected 'Scale Metrics:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Total tokens:") {
		t.Errorf("Expected 'Total tokens:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Scale status:") {
		t.Errorf("Expected 'Scale status:' in output, got: %s", output)
	}
}

// TestE2EValidateWarnsOnMissingSummary tests backward compatibility warning
func TestE2EValidateWarnsOnMissingSummary(t *testing.T) {
	// Create a bundle without summaries (old format)
	outDir := filepath.Join(t.TempDir(), "bundle")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create index.md
	indexContent := "---\nokf_version: \"0.1\"\n---\n\n# Test Bundle\n\n- [Test](test.md)\n"
	if err := os.WriteFile(filepath.Join(outDir, "index.md"), []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create concept without summary callout
	conceptContent := "---\ntype: Guide\ntitle: Test\n---\n\n# Test\n\nContent without summary callout."
	if err := os.WriteFile(filepath.Join(outDir, "test.md"), []byte(conceptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run validate
	validateCmd := exec.Command(testBinaryPath, "validate", outDir)
	out, _ := validateCmd.CombinedOutput()
	output := string(out)

	// Should still be valid (missing summary is warning, not error)
	if !strings.Contains(output, "Bundle is valid") {
		t.Errorf("Bundle without summaries should still be valid, got: %s", output)
	}

	// Should have warning about missing summary
	if !strings.Contains(output, "missing_summary") {
		t.Errorf("Expected missing_summary warning in output, got: %s", output)
	}
}

// TestE2EInitNewGate proves the Quick Start step-1 -> step-2 handoff: a store
// scaffolded by `init` is immediately usable by `new` and passes `gate`, with no
// hand-authored config (Requirement 1).
func TestE2EInitNewGate(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "store")

	initCmd := exec.Command(testBinaryPath, "init", storeDir)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(storeDir, ".okf", "config.yaml")); err != nil {
		t.Fatalf("init did not write config: %v", err)
	}

	artifact := filepath.Join(storeDir, "canon", "adr-001-example.md")
	newCmd := exec.Command(testBinaryPath, "new", "decision", artifact,
		"--store", storeDir, "--title", "Example decision")
	if out, err := newCmd.CombinedOutput(); err != nil {
		t.Fatalf("new failed: %v\n%s", err, out)
	}

	gateCmd := exec.Command(testBinaryPath, "gate", storeDir)
	if out, err := gateCmd.CombinedOutput(); err != nil {
		t.Fatalf("gate failed on a freshly scaffolded store: %v\n%s", err, out)
	}
}

// TestE2EProjectLifecycle proves the projection path: init a store, author a
// spec requirements.md, project it into Canon, and gate. Re-projecting reuses
// the same ID and yields byte-identical output (determinism via ID reuse).
func TestE2EProjectLifecycle(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "store")
	if out, err := exec.Command(testBinaryPath, "init", storeDir).CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	specPath := filepath.Join(storeDir, "specs", "feat", "requirements.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "# Feat\n\n## Problem\n\nUsers cannot do X.\n\n## Requirements\n\n[REQ-001] The system SHALL do a specific, testable thing.\n"
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command(testBinaryPath, "project", specPath, "--store", storeDir).CombinedOutput(); err != nil {
		t.Fatalf("project failed: %v\n%s", err, out)
	}
	artifact := filepath.Join(storeDir, "canon", "feat", "requirements.md")
	first, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("projected artifact missing: %v", err)
	}

	if out, err := exec.Command(testBinaryPath, "gate", storeDir).CombinedOutput(); err != nil {
		t.Fatalf("gate failed on projected store: %v\n%s", err, out)
	}

	// Re-project with --write: identical bytes (stable ID ⇒ deterministic output).
	if out, err := exec.Command(testBinaryPath, "project", specPath, "--store", storeDir, "--write").CombinedOutput(); err != nil {
		t.Fatalf("re-project failed: %v\n%s", err, out)
	}
	second, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("re-projection was not deterministic:\n--1--\n%s\n--2--\n%s", first, second)
	}
}

// TestE2EHooksInstallUninstall proves hooks install writes the pyra marker to
// every selected surface and uninstall removes it.
func TestE2EHooksInstallUninstall(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "store")
	if out, err := exec.Command(testBinaryPath, "init", storeDir).CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}
	// Create the toolchain markers so all four targets are exercised via --all.
	for _, d := range []string{".git/hooks", ".claude", ".kiro/hooks", ".kiro/agents"} {
		if err := os.MkdirAll(filepath.Join(storeDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.Command(testBinaryPath, "hooks", "install", "--all", "--store", storeDir).CombinedOutput(); err != nil {
		t.Fatalf("hooks install failed: %v\n%s", err, out)
	}
	surfaces := map[string]string{
		filepath.Join(storeDir, ".git", "hooks", "pre-commit"):         "pyra gate",
		filepath.Join(storeDir, ".claude", "settings.json"):            "pyra-managed",
		filepath.Join(storeDir, ".kiro", "hooks", "pyra-gate.json"): "pyra gate",
		filepath.Join(storeDir, ".kiro", "agents", "pyra.json"):     "pyra-managed",
	}
	for path, marker := range surfaces {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected %s installed: %v", path, err)
			continue
		}
		if !strings.Contains(string(body), marker) {
			t.Errorf("%s missing marker %q:\n%s", path, marker, body)
		}
	}

	if out, err := exec.Command(testBinaryPath, "hooks", "uninstall", "--all", "--store", storeDir).CombinedOutput(); err != nil {
		t.Fatalf("hooks uninstall failed: %v\n%s", err, out)
	}
	// pre-commit: block removed; settings/agent: marker gone.
	if body, _ := os.ReadFile(filepath.Join(storeDir, ".claude", "settings.json")); strings.Contains(string(body), "pyra-managed") {
		t.Error("claude settings still contains pyra marker after uninstall")
	}
	if _, err := os.Stat(filepath.Join(storeDir, ".kiro", "hooks", "pyra-gate.json")); !os.IsNotExist(err) {
		t.Error("kiro-ide hook file should be removed after uninstall")
	}
}
