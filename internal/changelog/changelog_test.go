package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadChangelog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid changelog
	content := `https://docs.example.com
2024-01-15T10:30:00Z - Initial bundle created with 42 concepts
2024-01-20T14:00:00Z - Updated: 3 added, 5 modified, 1 deleted
`
	err := os.WriteFile(filepath.Join(tmpDir, ChangelogFile), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cl, err := ReadChangelog(tmpDir)
	if err != nil {
		t.Fatalf("ReadChangelog failed: %v", err)
	}

	if cl.Source != "https://docs.example.com" {
		t.Errorf("Expected source 'https://docs.example.com', got '%s'", cl.Source)
	}

	if len(cl.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(cl.Entries))
	}

	if cl.Entries[0].Message != "Initial bundle created with 42 concepts" {
		t.Errorf("Unexpected first entry message: %s", cl.Entries[0].Message)
	}
}

func TestReadChangelogMissing(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := ReadChangelog(tmpDir)
	if err == nil {
		t.Error("Expected error for missing changelog")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestReadChangelogEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, ChangelogFile), []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ReadChangelog(tmpDir)
	if err == nil {
		t.Error("Expected error for empty changelog")
	}
}

func TestReadChangelogEmptySource(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
2024-01-15T10:30:00Z - Some entry
`
	err := os.WriteFile(filepath.Join(tmpDir, ChangelogFile), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ReadChangelog(tmpDir)
	if err == nil {
		t.Error("Expected error for empty source line")
	}
}

func TestReadChangelogMalformedEntries(t *testing.T) {
	tmpDir := t.TempDir()

	// Malformed entries should be skipped
	content := `https://docs.example.com
invalid entry without timestamp
2024-01-15T10:30:00Z - Valid entry
another invalid
`
	err := os.WriteFile(filepath.Join(tmpDir, ChangelogFile), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cl, err := ReadChangelog(tmpDir)
	if err != nil {
		t.Fatalf("ReadChangelog failed: %v", err)
	}

	if len(cl.Entries) != 1 {
		t.Errorf("Expected 1 valid entry, got %d", len(cl.Entries))
	}
}

func TestWriteChangelog(t *testing.T) {
	tmpDir := t.TempDir()

	cl := &Changelog{
		Source: "/path/to/docs",
		Entries: []ChangelogEntry{
			{
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Message:   "Initial bundle created with 10 concepts",
			},
		},
	}

	err := WriteChangelog(tmpDir, cl)
	if err != nil {
		t.Fatalf("WriteChangelog failed: %v", err)
	}

	// Read it back
	content, err := os.ReadFile(filepath.Join(tmpDir, ChangelogFile))
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if !strings.Contains(string(content), "/path/to/docs") {
		t.Error("Written file should contain source")
	}
	if !strings.Contains(string(content), "2024-01-15T10:30:00Z") {
		t.Error("Written file should contain timestamp")
	}
	if !strings.Contains(string(content), "Initial bundle created with 10 concepts") {
		t.Error("Written file should contain message")
	}
}

func TestCreateChangelog(t *testing.T) {
	tmpDir := t.TempDir()

	err := CreateChangelog(tmpDir, "https://example.com/docs", 25)
	if err != nil {
		t.Fatalf("CreateChangelog failed: %v", err)
	}

	cl, err := ReadChangelog(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read created changelog: %v", err)
	}

	if cl.Source != "https://example.com/docs" {
		t.Errorf("Expected source 'https://example.com/docs', got '%s'", cl.Source)
	}

	if len(cl.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(cl.Entries))
	}

	if !strings.Contains(cl.Entries[0].Message, "25 concepts") {
		t.Errorf("Expected message to contain '25 concepts', got: %s", cl.Entries[0].Message)
	}
}

func TestAppendEntry(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial changelog
	err := CreateChangelog(tmpDir, "https://example.com", 10)
	if err != nil {
		t.Fatalf("CreateChangelog failed: %v", err)
	}

	// Append entry
	err = AppendEntry(tmpDir, "Updated: 2 added, 1 modified")
	if err != nil {
		t.Fatalf("AppendEntry failed: %v", err)
	}

	// Verify
	cl, err := ReadChangelog(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read changelog: %v", err)
	}

	if len(cl.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(cl.Entries))
	}

	if cl.Entries[1].Message != "Updated: 2 added, 1 modified" {
		t.Errorf("Unexpected second entry message: %s", cl.Entries[1].Message)
	}
}

func TestGetSource(t *testing.T) {
	tmpDir := t.TempDir()

	err := CreateChangelog(tmpDir, "https://test.com/docs", 5)
	if err != nil {
		t.Fatalf("CreateChangelog failed: %v", err)
	}

	source, err := GetSource(tmpDir)
	if err != nil {
		t.Fatalf("GetSource failed: %v", err)
	}

	if source != "https://test.com/docs" {
		t.Errorf("Expected 'https://test.com/docs', got '%s'", source)
	}
}
