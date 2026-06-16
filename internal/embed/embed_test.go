package embed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasDemoBundle(t *testing.T) {
	if !HasDemoBundle() {
		t.Error("Expected HasDemoBundle to return true")
	}
}

func TestExtractDemoBundle(t *testing.T) {
	bundleDir, err := ExtractDemoBundle()
	if err != nil {
		t.Fatalf("ExtractDemoBundle failed: %v", err)
	}
	defer os.RemoveAll(bundleDir)

	// Check index.md exists
	indexPath := filepath.Join(bundleDir, "index.md")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Expected index.md to exist in extracted bundle")
	}

	// Check getting-started.md exists
	gsPath := filepath.Join(bundleDir, "getting-started.md")
	if _, err := os.Stat(gsPath); os.IsNotExist(err) {
		t.Error("Expected getting-started.md to exist in extracted bundle")
	}

	// Check concepts directory exists
	conceptsDir := filepath.Join(bundleDir, "concepts")
	if info, err := os.Stat(conceptsDir); os.IsNotExist(err) || !info.IsDir() {
		t.Error("Expected concepts directory to exist in extracted bundle")
	}

	// Check cli directory exists
	cliDir := filepath.Join(bundleDir, "cli")
	if info, err := os.Stat(cliDir); os.IsNotExist(err) || !info.IsDir() {
		t.Error("Expected cli directory to exist in extracted bundle")
	}
}
