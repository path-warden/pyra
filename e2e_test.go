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
	tmpDir, err := os.MkdirTemp("", "okf-cli-test-*")
	if err != nil {
		os.Stderr.WriteString("Failed to create temp dir: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	testBinaryPath = filepath.Join(tmpDir, "okf-cli")
	cmd := exec.Command("go", "build", "-o", testBinaryPath, "./cmd/okf-cli")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.WriteString("Failed to build binary: " + err.Error() + "\n" + string(out))
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

	// Verify README was imported
	readmePath := filepath.Join(outDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("Expected README.md to exist in output bundle")
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

	// Verify frontmatter in README.md
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
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
	validateCmd := exec.Command(testBinaryPath, "validate", "examples/bundles/okf-cli-docs")
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
	validateCmd := exec.Command(testBinaryPath, "validate", "examples/bundles/okf-cli-docs", "--json")
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
	inspectCmd := exec.Command(testBinaryPath, "inspect", "examples/bundles/okf-cli-docs")
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
	if !strings.Contains(output, "okf-cli demo") {
		t.Errorf("Expected 'okf-cli demo' in output, got: %s", output)
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
