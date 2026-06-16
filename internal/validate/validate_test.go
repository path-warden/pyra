package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBundle(t *testing.T) {
	// Create a temporary valid bundle
	tmpDir := t.TempDir()

	// Create a valid concept file
	conceptContent := `---
type: "Guide"
title: "Test Concept"
description: "A test concept"
resource: "test.md"
tags:
  - test
timestamp: "2024-01-01T00:00:00.000Z"
---

# Test Concept

This is test content.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(conceptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create root index.md
	indexContent := `---
okf_version: "0.1"
---

# Test Bundle

- [Test Concept](test.md)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateBundle(tmpDir)
	if err != nil {
		t.Fatalf("ValidateBundle failed: %v", err)
	}

	if !report.Valid {
		t.Errorf("Expected valid bundle, got invalid. Issues: %v", report.Issues)
	}
	if report.ConceptCount != 1 {
		t.Errorf("ConceptCount = %d, want 1", report.ConceptCount)
	}
}

func TestValidateBundleMissingFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a concept without frontmatter
	conceptContent := `# Test Concept

This is test content without frontmatter.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(conceptContent), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateBundle(tmpDir)
	if err != nil {
		t.Fatalf("ValidateBundle failed: %v", err)
	}

	if report.Valid {
		t.Error("Expected invalid bundle for missing frontmatter")
	}

	found := false
	for _, issue := range report.Issues {
		if issue.Code == "missing_frontmatter" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected missing_frontmatter issue")
	}
}

func TestValidateBundleMissingType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a concept without type
	conceptContent := `---
title: "Test"
---

# Test
`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(conceptContent), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateBundle(tmpDir)
	if err != nil {
		t.Fatalf("ValidateBundle failed: %v", err)
	}

	if report.Valid {
		t.Error("Expected invalid bundle for missing type")
	}

	found := false
	for _, issue := range report.Issues {
		if issue.Code == "missing_type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected missing_type issue")
	}
}

func TestInspectBundle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test concepts
	concept1 := `---
type: "Guide"
title: "Intro"
tags:
  - intro
---

# Intro

See [other](./other.md).
`
	concept2 := `---
type: "Guide"
title: "Other"
tags:
  - other
---

# Other

Content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intro.md"), []byte(concept1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "other.md"), []byte(concept2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create index
	indexContent := `---
okf_version: "0.1"
---

# Test Bundle
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := InspectBundle(tmpDir)
	if err != nil {
		t.Fatalf("InspectBundle failed: %v", err)
	}

	if stats.ConceptCount != 2 {
		t.Errorf("ConceptCount = %d, want 2", stats.ConceptCount)
	}
	if stats.LinkCount != 1 {
		t.Errorf("LinkCount = %d, want 1", stats.LinkCount)
	}
	if stats.TypeDistribution["Guide"] != 2 {
		t.Errorf("TypeDistribution[Guide] = %d, want 2", stats.TypeDistribution["Guide"])
	}
}
