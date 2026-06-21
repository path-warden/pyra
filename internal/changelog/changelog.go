// Package changelog handles reading and writing changelog.txt files for OKF bundles.
package changelog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ChangelogFile = "changelog.txt"

// ChangelogEntry represents a single changelog entry.
type ChangelogEntry struct {
	Timestamp time.Time
	Message   string
}

// Changelog represents the changelog file contents.
type Changelog struct {
	Source  string           // First line: URL or file path
	Entries []ChangelogEntry // Subsequent timestamped entries
}

// ReadChangelog reads and parses a changelog.txt file from the bundle.
func ReadChangelog(bundlePath string) (*Changelog, error) {
	changelogPath := filepath.Join(bundlePath, ChangelogFile)

	file, err := os.Open(changelogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("changelog.txt not found in bundle")
		}
		return nil, fmt.Errorf("failed to open changelog: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	cl := &Changelog{}

	// First line is the source
	if scanner.Scan() {
		cl.Source = strings.TrimSpace(scanner.Text())
	} else {
		return nil, fmt.Errorf("changelog.txt is empty")
	}

	if cl.Source == "" {
		return nil, fmt.Errorf("changelog.txt has empty source line")
	}

	// Subsequent lines are entries
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry, err := parseEntry(line)
		if err != nil {
			// Skip malformed entries but continue parsing
			continue
		}
		cl.Entries = append(cl.Entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading changelog: %w", err)
	}

	return cl, nil
}

// parseEntry parses a changelog entry line.
// Format: "2024-01-15T10:30:00Z - Message here"
func parseEntry(line string) (ChangelogEntry, error) {
	parts := strings.SplitN(line, " - ", 2)
	if len(parts) != 2 {
		return ChangelogEntry{}, fmt.Errorf("invalid entry format")
	}

	timestamp, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
	if err != nil {
		return ChangelogEntry{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	return ChangelogEntry{
		Timestamp: timestamp,
		Message:   strings.TrimSpace(parts[1]),
	}, nil
}

// WriteChangelog writes a changelog.txt file to the bundle.
func WriteChangelog(bundlePath string, cl *Changelog) error {
	changelogPath := filepath.Join(bundlePath, ChangelogFile)

	var lines []string
	lines = append(lines, cl.Source)

	for _, entry := range cl.Entries {
		lines = append(lines, fmt.Sprintf("%s - %s", entry.Timestamp.Format(time.RFC3339), entry.Message))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(changelogPath, []byte(content), 0644)
}

// CreateChangelog creates a new changelog with an initial entry.
func CreateChangelog(bundlePath, source string, conceptCount int) error {
	cl := &Changelog{
		Source: source,
		Entries: []ChangelogEntry{
			{
				Timestamp: time.Now().UTC(),
				Message:   fmt.Sprintf("Initial bundle created with %d concepts", conceptCount),
			},
		},
	}
	return WriteChangelog(bundlePath, cl)
}

// AppendEntry adds a new entry to an existing changelog.
func AppendEntry(bundlePath string, message string) error {
	cl, err := ReadChangelog(bundlePath)
	if err != nil {
		return err
	}

	cl.Entries = append(cl.Entries, ChangelogEntry{
		Timestamp: time.Now().UTC(),
		Message:   message,
	})

	return WriteChangelog(bundlePath, cl)
}

// GetSource reads just the source from a changelog file.
func GetSource(bundlePath string) (string, error) {
	cl, err := ReadChangelog(bundlePath)
	if err != nil {
		return "", err
	}
	return cl.Source, nil
}
