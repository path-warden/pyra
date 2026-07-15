package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !strings.Contains(output, "pyra") {
		t.Errorf("Expected help to contain 'pyra', got %s", output)
	}

	if !strings.Contains(output, "crawl") {
		t.Errorf("Expected help to list crawl command")
	}

	if !strings.Contains(output, "import") {
		t.Errorf("Expected help to list import command")
	}

	if !strings.Contains(output, "validate") {
		t.Errorf("Expected help to list validate command")
	}

	if !strings.Contains(output, "inspect") {
		t.Errorf("Expected help to list inspect command")
	}

	if !strings.Contains(output, "serve") {
		t.Errorf("Expected help to list serve command")
	}

	if !strings.Contains(output, "demo") {
		t.Errorf("Expected help to list demo command")
	}
}

func TestValidateCommand(t *testing.T) {
	// Create a minimal valid bundle
	tmpDir := t.TempDir()
	bundleDir := filepath.Join(tmpDir, "bundle")
	_ = os.MkdirAll(bundleDir, 0755)

	// Create root index.md
	indexContent := `---
okf_version: "1.0"
---
# Test Bundle
`
	_ = os.WriteFile(filepath.Join(bundleDir, "index.md"), []byte(indexContent), 0644)

	// Create a concept file
	conceptContent := `---
type: Concept
title: Test Concept
---
# Test Concept

This is a test concept.
`
	_ = os.WriteFile(filepath.Join(bundleDir, "test.md"), []byte(conceptContent), 0644)

	// Reset command for testing
	rootCmd.SetArgs([]string{"validate", bundleDir})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("Expected no error for valid bundle, got %v", err)
	}
}

func TestImportCommandHelp(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"import", "--help"})
	_ = rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "--out") {
		t.Errorf("Expected import help to show --out flag")
	}

	if !strings.Contains(output, "--source-name") {
		t.Errorf("Expected import help to show --source-name flag")
	}

	if !strings.Contains(output, "--include") {
		t.Errorf("Expected import help to show --include flag")
	}
}

func TestCrawlCommandHelp(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"crawl", "--help"})
	_ = rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "--max-pages") {
		t.Errorf("Expected crawl help to show --max-pages flag")
	}

	if !strings.Contains(output, "--max-depth") {
		t.Errorf("Expected crawl help to show --max-depth flag")
	}

	if !strings.Contains(output, "--same-origin") {
		t.Errorf("Expected crawl help to show --same-origin flag")
	}

	if !strings.Contains(output, "--respect-robots") {
		t.Errorf("Expected crawl help to show --respect-robots flag")
	}
}

func TestServeCommandHelp(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"serve", "--help"})
	_ = rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "--mcp") {
		t.Errorf("Expected serve help to show --mcp flag")
	}

	if !strings.Contains(output, "--name") {
		t.Errorf("Expected serve help to show --name flag")
	}
}

func TestInitCommandHelpListsLocalAgentSetup(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"init", "--help"})
	err := rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("init help failed: %v", err)
	}
	for _, want := range []string{"--agent", "--kiro-agent", "--list-agents", "AGENTS.md", "MCP configuration"} {
		if !strings.Contains(output, want) {
			t.Errorf("init help missing %q:\n%s", want, output)
		}
	}
}

func TestInspectCommandHelp(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"inspect", "--help"})
	_ = rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Inspect an OKF bundle") {
		t.Errorf("Expected inspect help to describe command")
	}
}
