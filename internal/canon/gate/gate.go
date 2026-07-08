// Package gate runs the unified Canon authority check: it loads the corpus,
// validates every artifact, builds and validates the relationship graph, then
// classifies each finding as blocking, advisory, or disabled per the store's
// enforcement policy. It returns a single aggregated result whose Blocking count
// determines the gate's exit code.
package gate

import (
	"github.com/chasedputnam/memphis/internal/canon"
	"github.com/chasedputnam/memphis/internal/canon/model"
	"github.com/chasedputnam/memphis/internal/canon/relate"
	"github.com/chasedputnam/memphis/internal/canon/validate"
	"github.com/chasedputnam/memphis/internal/config"
)

// Result aggregates all gate findings.
type Result struct {
	Issues        []model.Issue `json:"issues"`
	Blocking      int           `json:"blocking"`
	Advisory      int           `json:"advisory"`
	ArtifactCount int           `json:"artifact_count"`
}

// Passed reports whether the gate has no blocking findings.
func (r Result) Passed() bool { return r.Blocking == 0 }

// Merge combines two results into one aggregate: it sums the blocking, advisory,
// and artifact counts and concatenates the issue lists. It lets the CLI fold an
// independent evaluation (e.g. the change-aware gate) into the corpus result so a
// single exit code reflects both.
func (r Result) Merge(o Result) Result {
	return Result{
		Issues:        append(append([]model.Issue{}, r.Issues...), o.Issues...),
		Blocking:      r.Blocking + o.Blocking,
		Advisory:      r.Advisory + o.Advisory,
		ArtifactCount: r.ArtifactCount + o.ArtifactCount,
	}
}

// ApplyPolicy classifies raw issues against the store's enforcement policy and
// returns an aggregable Result. A disabled code is dropped; otherwise the issue
// is counted as blocking or advisory and its severity normalized to match. This
// is the single classification path shared by the corpus gate (Run) and the
// change-aware gate. It sets no ArtifactCount; callers add that.
func ApplyPolicy(cfg config.Config, raw []model.Issue) Result {
	pol := policyFrom(cfg.Enforcement)
	var res Result
	for _, iss := range raw {
		blocking, drop := pol.classify(iss)
		if drop {
			continue
		}
		if blocking {
			iss.Severity = model.SeverityError
			res.Blocking++
		} else {
			iss.Severity = model.SeverityWarning
			res.Advisory++
		}
		res.Issues = append(res.Issues, iss)
	}
	return res
}

// Policy classifies issue codes. A code in Disabled is dropped; a code in
// Blocking is forced blocking; a code in Advisory is forced advisory; otherwise
// the issue's own severity decides (error = blocking).
type Policy struct {
	blocking map[string]bool
	advisory map[string]bool
	disabled map[string]bool
}

func policyFrom(e config.Enforcement) Policy {
	toSet := func(items []string) map[string]bool {
		m := make(map[string]bool, len(items))
		for _, i := range items {
			m[i] = true
		}
		return m
	}
	return Policy{
		blocking: toSet(e.Blocking),
		advisory: toSet(e.Advisory),
		disabled: toSet(e.Disabled),
	}
}

// classify returns whether an issue is blocking and whether it should be dropped.
func (p Policy) classify(iss model.Issue) (blocking, drop bool) {
	switch {
	case p.disabled[iss.Code]:
		return false, true
	case p.blocking[iss.Code]:
		return true, false
	case p.advisory[iss.Code]:
		return false, false
	default:
		return iss.Severity == model.SeverityError, false
	}
}

// Run executes the gate over the Canon corpus at storeRoot.
func Run(storeRoot string, cfg config.Config) (Result, error) {
	arts, err := canon.LoadCorpus(storeRoot, cfg)
	if err != nil {
		return Result{}, err
	}

	var raw []model.Issue

	// Per-artifact load + validation issues.
	valOpts := validate.Options{TicketProvider: cfg.Ticketing.Provider}
	entries := make([]relate.Entry, 0, len(arts))
	for _, a := range arts {
		raw = append(raw, a.LoadIssues...)
		for _, iss := range validate.Validate(a.Product, a.Classification, valOpts) {
			iss.Path = a.Path
			raw = append(raw, iss)
		}
		entries = append(entries, relate.Entry{
			ID: a.ID, Type: a.Type, Status: a.Status, Retired: a.Retired, Path: a.Path,
			Aliases: a.Aliases, Product: a.Product,
		})
	}

	// Relationship integrity over the whole corpus.
	_, relIssues := relate.Build(entries, relate.DefaultSpecs())
	raw = append(raw, relIssues...)

	// Apply the shared enforcement policy, then annotate with the corpus size.
	res := ApplyPolicy(cfg, raw)
	res.ArtifactCount = len(arts)
	return res, nil
}
