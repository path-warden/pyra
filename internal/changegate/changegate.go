package changegate

import (
	"fmt"

	"github.com/chasedputnam/memphis/internal/canon/model"
)

// Stable finding codes (rule IDs) emitted by the change-aware gate. They are
// stable across runs and identical inputs, so they are safe to reference from an
// enforcement policy and from SARIF rule IDs.
const (
	// CodeGovernedChange marks a changed file that is governed by an Accepted
	// Canon artifact (the artifact cites the file path or a symbol-id in it).
	CodeGovernedChange = "canon-governed-change"
	// CodeSymbolUnresolved marks a governing artifact that cites a symbol on a
	// changed file which no longer resolves (renamed, moved, or deleted).
	CodeSymbolUnresolved = "governed-symbol-unresolved"
)

// Finding is one change-aware result before policy classification. It is mapped
// to a model.Issue (severity "warning") so the existing gate classifier,
// aggregation, and renderers (text / JSON / SARIF) handle it with no new
// plumbing.
type Finding struct {
	Code     string // one of the Code* constants
	File     string // changed file, store-root-relative slash path
	Artifact string // governing Canon ID (successor-resolved)
	Type     string // governing artifact type (e.g. "requirement", "decision")
	Status   string // governing artifact lifecycle status
	Title    string // governing artifact title
	Symbol   string // the unresolved symbol-id (CodeSymbolUnresolved only)
}

// toIssue renders a Finding as a model.Issue whose Path is the changed file, so
// SARIF locations point at the changed file rather than at Canon.
func (f Finding) toIssue() model.Issue {
	return model.Issue{
		Severity: model.SeverityWarning,
		Code:     f.Code,
		Message:  f.message(),
		Path:     f.File,
	}
}

func (f Finding) message() string {
	switch f.Code {
	case CodeSymbolUnresolved:
		return fmt.Sprintf("%s cites %s in %s, which no longer resolves (renamed, moved, or deleted)",
			f.Artifact, f.Symbol, f.File)
	default:
		return fmt.Sprintf("%s is governed by %s (%s, %s): %s",
			f.File, f.Artifact, f.Type, f.Status, f.Title)
	}
}

// issues maps a slice of findings to model.Issues.
func issues(fs []Finding) []model.Issue {
	out := make([]model.Issue, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.toIssue())
	}
	return out
}
