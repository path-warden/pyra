// Package embed provides embedded assets for OKFy.
package embed

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed demo-bundle
var demoBundleFS embed.FS

// ExtractDemoBundle extracts the embedded demo bundle to a temporary directory.
// Returns the path to the extracted bundle.
func ExtractDemoBundle() (tmpDir string, err error) {
	tmpDir, err = os.MkdirTemp("", "okf-cli-demo-*")
	if err != nil {
		return "", err
	}

	// Clean up on error
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir)
			tmpDir = ""
		}
	}()

	err = fs.WalkDir(demoBundleFS, "demo-bundle", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Get relative path from demo-bundle root
		relPath, relErr := filepath.Rel("demo-bundle", path)
		if relErr != nil {
			return relErr
		}

		targetPath := filepath.Join(tmpDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		content, readErr := demoBundleFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		return os.WriteFile(targetPath, content, 0644)
	})

	return tmpDir, err
}

// HasDemoBundle returns true if the embedded demo bundle exists.
func HasDemoBundle() bool {
	entries, err := demoBundleFS.ReadDir("demo-bundle")
	return err == nil && len(entries) > 0
}
