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
