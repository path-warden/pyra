// Package canon loads a Canon corpus from disk: it walks the configured Canon
// roots, parses each Markdown artifact into a Product, hardens its frontmatter,
// classifies its type, and resolves its identity. The resulting Artifact slice
// is the shared input to the gate (canon/gate) and the unified store
// (internal/store).
package canon

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/frontmatter"
	"github.com/chasedputnam/pyra/internal/canon/identity"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/parse"
	"github.com/chasedputnam/pyra/internal/config"
)

// Artifact is one loaded Canon artifact with its derived classification.
type Artifact struct {
	ID             string                  `json:"id"`
	Path           string                  `json:"path"`
	Type           string                  `json:"type"`
	Status         string                  `json:"status,omitempty"`
	Retired        bool                    `json:"retired"`
	Confidence     float64                 `json:"confidence"`
	Aliases        []string                `json:"-"` // alternate ids for cross-reference resolution
	Product        *model.Product          `json:"-"`
	Classification classify.Classification `json:"-"`
	Frontmatter    model.Frontmatter       `json:"-"`
	LoadIssues     []model.Issue           `json:"-"` // parse/frontmatter/identity issues
}

var reserved = map[string]bool{"index.md": true, "log.md": true}

// LoadCorpus loads all Canon artifacts under the configured roots of storeRoot.
// Returns the artifacts and any global load issues (currently none beyond
// per-artifact LoadIssues, reserved for future use).
func LoadCorpus(storeRoot string, cfg config.Config) ([]Artifact, error) {
	reg := artifacts.Default()
	var out []Artifact

	for _, root := range cfg.CanonRoots {
		dir := filepath.Join(storeRoot, root)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue // a configured-but-absent canon root is not an error
		}
		err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := strings.ToLower(d.Name())
			if !strings.HasSuffix(name, ".md") || reserved[name] {
				return nil
			}
			art, lerr := loadArtifact(storeRoot, path, reg)
			if lerr != nil {
				return lerr
			}
			out = append(out, art)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func loadArtifact(storeRoot, path string, reg artifacts.Registry) (Artifact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, err
	}
	rel, rerr := filepath.Rel(storeRoot, path)
	if rerr != nil {
		rel = path
	}

	p := parse.Parse(raw)
	c := classify.Classify(p, reg)
	status := statusOf(p)

	var issues []model.Issue
	if !p.Metadata.Present {
		issues = append(issues, model.Issue{
			Severity: model.SeverityError, Code: "missing_frontmatter",
			Message: "canon artifact has no frontmatter envelope", Path: rel,
		})
	} else {
		// Strict re-parse of frontmatter for hardening issues.
		if fmBytes, ok := frontmatterBytes(raw); ok {
			_, fmIssues := frontmatter.Parse(fmBytes)
			for i := range fmIssues {
				fmIssues[i].Path = rel
			}
			issues = append(issues, fmIssues...)
		}
	}

	id, structured := identity.Resolve(p, path)
	if p.Metadata.Present && strings.TrimSpace(p.Metadata.ID) != "" && !identity.ValidID(id) {
		issues = append(issues, model.Issue{
			Severity: model.SeverityWarning, Code: "invalid_id",
			Message: "frontmatter id is not a well-formed artifact id: " + id, Path: rel,
		})
	} else if !structured {
		issues = append(issues, model.Issue{
			Severity: model.SeverityWarning, Code: "unresolved_id",
			Message: "artifact has no explicit id or id-bearing filename; using stem", Path: rel,
		})
	}

	return Artifact{
		ID:             id,
		Path:           rel,
		Type:           c.Type,
		Status:         status,
		Retired:        reg.IsRetired(c.Type, status),
		Confidence:     c.Confidence,
		Aliases:        identity.Aliases(path),
		Product:        p,
		Classification: c,
		Frontmatter:    p.Metadata,
		LoadIssues:     issues,
	}, nil
}

// statusOf returns the normalized first token of the Status section, if any.
func statusOf(p *model.Product) string {
	body, ok := p.Section("status")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		return parse.Normalize(strings.Trim(fields[0], ".,;:"))
	}
	return ""
}

// frontmatterBytes extracts the raw YAML frontmatter block from a document.
func frontmatterBytes(raw []byte) ([]byte, bool) {
	lines := strings.SplitAfter(string(raw), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return nil, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			return []byte(strings.Join(lines[1:i], "")), true
		}
	}
	return nil, false
}
