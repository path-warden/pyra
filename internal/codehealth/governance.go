package codehealth

import "github.com/chasedputnam/pyra/internal/changegate"

const staleGovernanceChurn = 5 // recent commits on a governed file that suggest a stale decision

// governanceDetectors returns the per-file authority tie-in detectors. Both
// return nil when the store has no Canon. (contradictory_decision is repo-level —
// it concerns Canon artifacts, not code files — and is surfaced on the Report.)
func governanceDetectors() []Detector {
	return []Detector{ungovernedHotspot, staleGovernance}
}

// governedFiles returns the set of files governed by Accepted Canon, reusing the
// change-aware gate's governance resolution.
func governedFiles(in *Inputs, files []string) map[string]bool {
	out := map[string]bool{}
	if in.Store == nil || !in.Store.HasCanon() || in.Ops == nil {
		return out
	}
	for _, iss := range changegate.Evaluate(in.Store, in.Ops, files) {
		if iss.Code == changegate.CodeGovernedChange {
			out[iss.Path] = true
		}
	}
	return out
}

// ungovernedHotspot flags a git hotspot that no Accepted Canon governs.
func ungovernedHotspot(fc *FileContext, in *Inputs) []Finding {
	if in.Store == nil || !in.Store.HasCanon() {
		return nil
	}
	if fc.IsHotspot && !fc.Governed {
		return one("ungoverned_hotspot", fc.Path, "medium")
	}
	return nil
}

// staleGovernance flags governed code that churns heavily — a signal the governing
// decision may be out of date (documented churn-based approximation).
func staleGovernance(fc *FileContext, in *Inputs) []Finding {
	if in.Store == nil || !in.Store.HasCanon() || !fc.Governed || fc.Git == nil {
		return nil
	}
	if fc.Git.Commits90d >= staleGovernanceChurn {
		return one("stale_governance", fc.Path, "low")
	}
	return nil
}

// detectContradictions returns the Canon decision paths that supersede a target
// still marked live (not superseded) — a reversal without a proper supersede,
// i.e. two active decisions where one claims to replace the other. Repo-level,
// surfaced on the Report as contradictory_decision.
func detectContradictions(in *Inputs) []string {
	seen := map[string]bool{}
	if in.Store == nil || in.Store.Graph == nil {
		return nil
	}
	for _, edges := range in.Store.Graph.Edges {
		for _, e := range edges {
			if e.Kind != "supersedes" {
				continue
			}
			target := in.Store.ByID(e.To)
			source := in.Store.ByID(e.From)
			if target != nil && source != nil && target.Status != "superseded" && target.Status != "" {
				seen[source.Path] = true
			}
		}
	}
	return sortedStrings(seen)
}
