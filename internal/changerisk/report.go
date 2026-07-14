package changerisk

import (
	"fmt"
	"math"
	"sort"

	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/codeintel"
	"github.com/chasedputnam/pyra/internal/gitint"
	"github.com/chasedputnam/pyra/internal/store"
)

// Defaults for assessment.
const (
	DefaultBaseline = 200 // recent commits sampled for the repo-relative rank
	DefaultWindow   = 500 // git-history window for churn / co-change
)

// Options tunes an assessment. Zero values fall back to the defaults.
type Options struct {
	Baseline int      // recent commits for ranking
	Window   int      // history window for the gitint substrate
	Exts     []string // restrict counted files to these suffixes
}

func (o Options) withDefaults() Options {
	if o.Baseline <= 0 {
		o.Baseline = DefaultBaseline
	}
	if o.Window <= 0 {
		o.Window = DefaultWindow
	}
	return o
}

// Report is the assembled change-risk result.
type Report struct {
	Score       float64        `json:"score"`       // raw 0–10 (secondary, corpus-anchored)
	Probability float64        `json:"probability"` //
	Priority    string         `json:"priority"`    // repo-relative tercile; "" when unavailable
	Percentile  float64        `json:"percentile"`  // repo-relative percentile
	RankKnown   bool           `json:"rank_known"`  // false when too few commits to rank
	Drivers     []RiskDriver   `json:"drivers"`
	Directives  []Finding      `json:"directives"`
	Features    ChangeFeatures `json:"features"`
	Ref         string         `json:"ref"`
}

// Assess scores a change and assembles its report. It is deterministic and
// offline. When repo is not a git repository it degrades to diff-only metrics
// (no ranking, no co-change), rather than failing.
func Assess(repo, storeRoot string, ch Change, st *store.Store, ops *codeintel.Ops, cfg Options) (Report, error) {
	opts := cfg.withDefaults()

	features, changed := ch.Extract(repo, opts.Exts)
	raw := ScoreChange(features)

	rep := Report{
		Score:       raw.Score,
		Probability: raw.Probability,
		Drivers:     raw.Drivers,
		Features:    features,
		Ref:         features.Ref,
	}

	// Repo-relative ranking, using exp-unknown scores on both sides (like-with-
	// like). Skipped entirely when there is no git history to sample.
	anchor, exclude := ch.Anchor()
	baseline := BaselineScores(repo, anchor, opts.Baseline, opts.Exts, exclude)
	if len(baseline) > 0 {
		norm := NewNormalizer(baseline)
		rankless := features
		rankless.Exp = nil // rank without author tenure, matching the baseline
		rankScore := ScoreChange(rankless).Score
		rep.Percentile = norm.Percentile(rankScore)
		rep.RankKnown = norm.Available()
		if rep.RankKnown {
			rep.Priority = norm.Priority(rankScore)
		}
	}

	// Directives over the change set.
	changeSet := make(map[string]bool, len(changed))
	for _, f := range changed {
		changeSet[f] = true
	}
	var directives []Finding
	directives = append(directives, missingTestsDirectives(changed, changeSet)...)

	graph := buildCodeGraph(ops, storeRoot)
	directives = append(directives, willBreakDirectives(graph, changed, changeSet)...)

	if h, ok := gitint.New(repo, opts.Window); ok {
		directives = append(directives, missingCoChangeDirectives(h, graph, changed, changeSet)...)
	}
	directives = append(directives, governanceDirectives(st, ops, changed)...)

	sortFindings(directives)
	rep.Directives = directives
	return rep, nil
}

// HeadlineText renders the repo-relative headline (priority + percentile) with
// the raw score kept as a clearly-secondary number.
func (r Report) HeadlineText() string {
	if r.RankKnown {
		return fmt.Sprintf("%s (%.0fth pct of this repo) — raw %.1f/10 (uncalibrated)",
			r.Priority, r.Percentile, r.Score)
	}
	return fmt.Sprintf("ranking unavailable (too few commits) — raw %.1f/10 (uncalibrated)", r.Score)
}

// TopDriversText renders the drivers sorted by absolute contribution (strongest
// first) as human-readable lines, with the signed push and the standing label.
func (r Report) TopDriversText() []string {
	ds := append([]RiskDriver(nil), r.Drivers...)
	sort.SliceStable(ds, func(i, j int) bool {
		return math.Abs(ds[i].Contribution) > math.Abs(ds[j].Contribution)
	})
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		if !d.Known {
			out = append(out, fmt.Sprintf("%s: unknown (no push)", d.Feature))
			continue
		}
		out = append(out, fmt.Sprintf("%s (%+.2f) — %s", d.Feature, d.Contribution, d.Label))
	}
	return out
}

// Issues maps the report to gate findings: one repo-level `change-risk` headline
// plus one finding per directive (Path = changed file), all severity warning.
// Deterministic order.
func (r Report) Issues() []model.Issue {
	out := make([]model.Issue, 0, len(r.Directives)+1)
	out = append(out, model.Issue{
		Severity: model.SeverityWarning,
		Code:     CodeRisk,
		Message:  r.HeadlineText(),
	})
	for _, d := range r.Directives {
		out = append(out, model.Issue{
			Severity: model.SeverityWarning,
			Code:     d.Code,
			Message:  d.Message,
			Path:     d.File,
		})
	}
	return out
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		if fs[i].Code != fs[j].Code {
			return fs[i].Code < fs[j].Code
		}
		return fs[i].Message < fs[j].Message
	})
}
