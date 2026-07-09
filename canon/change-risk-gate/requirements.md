---
schema_version: 1
id: OKF-3YZWFZ1FF1C3
type: requirement
---

# Requirements Document

## Problem

Memphis's change-aware gate answers *"which Accepted Canon governs this diff?"*
but says nothing about *"how risky is this diff?"* A change can touch no governed
code yet still be dangerous — a large, diffuse, test-less edit to a
frequently-changed file is where defects concentrate. Reviewers and agents need a
deterministic, pre-merge signal for that risk, delivered in the same gate they
already run.

This feature adds a **change-risk** signal to Memphis. Given a change (the staged
diff by default, or a `base..HEAD` range), it scores the *shape of the diff* using
Kamei-style just-in-time metrics, leads with a **repo-relative ranking** (how this
change compares to the repo's own recent commits), and emits actionable **PR
directives** — missing tests, absent co-change partners, structural dependents
that may break, and whether governed Canon is touched. It is surfaced both as a
`--risk` flag on `memphis gate` (findings merged into the one gate result and exit
code) and as a standalone `memphis risk <range>` command.

Like the rest of the gate path it must be **deterministic, offline, and AI-free**,
and must not weaken the authority-path boundary (`internal/canon/...` may not
depend on it). To make Memphis **parity-testable against repowise from day one**,
the initial model (feature standardization, the logistic formula, and its learned
constants) is a **faithful port of repowise's change-risk model**, confined to a
single, clearly-attributed, swappable file so the constants can be replaced later
without touching the rest of the feature. (Note: repowise is AGPL-3.0; the ported
constants carry that license, which is relevant if Memphis is distributed under
other terms.) Regardless of the raw model, Memphis still **leads with the
repo-relative ranking**, because repowise itself documents that the absolute band
is not portable across repos while the ranking is the sound signal.

### Scope

**Must have:** a deterministic change-risk score over a diff from Kamei JIT
metrics; a repo-relative headline (tercile + percentile over the repo's own recent
commits); the four PR directives (`missing_tests`, `missing_cochanges`,
`will_break`, `governance_risk`) as advisory findings escalatable by enforcement
policy; a minimal git-intelligence substrate (per-file churn, co-change pairs,
author commit counts) in a new package outside `internal/canon`; a `--risk` flag
on `memphis gate` that merges into the existing `Result` / exit code / JSON /
SARIF; a standalone `memphis risk [range]` command.

**Should have:** attributable per-metric drivers (which features raised or lowered
the raw score) for transparency; a configurable baseline sample size for the
repo-relative distribution; a **parity test** that pins Memphis's raw score to
repowise's on identical diff inputs (the reason for porting the model verbatim);
the `missing_tests` source↔test mapping via a built-in **language-aware
convention map** covering Memphis's supported languages.

**Nice to have:** subsystem/directory diffusion breakdown in verbose output.

**Out of scope:** SZZ bug-inducing-commit labelling at runtime (labelling belongs
to offline calibration, not the runtime path); training Memphis's *own* calibration
corpus (the ported constants stand in until then); an LLM-generated explanation;
the broader git-intelligence surfaces (ownership %, bus factor, hotspot dashboard)
beyond the minimal substrate this needs — those are a separate parity capability;
cross-repo/workspace risk.

## Requirements

### Requirement 1 — Deterministic change-risk score from diff shape

- [REQ-101] WHEN a change is scored THEN Memphis SHALL compute the Kamei just-in-time metrics of the diff: lines added, lines deleted, files touched, distinct directories touched, distinct top-level subsystems touched, and the Shannon entropy of the per-file churn distribution.
- [REQ-102] WHEN an author identity is available for the change THEN Memphis SHALL include the author's prior-commit count as an experience metric, and WHERE it is unavailable Memphis SHALL score experience neutrally rather than failing.
- [REQ-103] WHEN the metrics are combined into a raw score THEN Memphis SHALL use a fixed, documented logistic formula whose per-metric contributions are individually attributable.
- [REQ-104] WHEN the same change is scored twice on identical repository state THEN Memphis SHALL produce an identical score and identical drivers.
- [REQ-105] WHERE the model's learned constants and formula are ported from repowise for parity THEN Memphis SHALL confine them to a single, clearly-attributed source file that can be replaced without changing the metric computation, ranking, directive, or integration code.
- [REQ-106] WHEN Memphis scores a set of reference diffs THEN its raw score SHALL match repowise's score for the same diffs within a documented tolerance, verified by an automated parity test.

### Requirement 2 — Repo-relative ranking as the headline

- [REQ-201] WHEN a change is scored THEN Memphis SHALL report a repo-relative review priority of `Below typical`, `Typical`, or `Elevated`, derived from terciles of the repository's own recent-commit risk distribution.
- [REQ-202] WHEN a change is scored THEN Memphis SHALL report the percentile of the change within the repository's own recent-commit risk distribution.
- [REQ-203] WHEN the raw 0–10 score is displayed THEN Memphis SHALL present it as a secondary, clearly-labeled number and SHALL NOT present it as a portable or defect-calibrated verdict.
- [REQ-204] WHEN the recent-commit distribution is sampled THEN Memphis SHALL sample a bounded number of recent commits, and the sample size SHALL be configurable with a documented default.
- [REQ-205] IF too few commits exist to form a stable distribution THEN Memphis SHALL report the ranking as unavailable and still report the raw score, rather than emitting a misleading percentile.

### Requirement 3 — Actionable PR directives

- [REQ-301] WHEN a changed source file has no corresponding test file present in the same change THEN Memphis SHALL emit a `missing_tests` directive naming that file.
- [REQ-302] WHEN a changed file has recent co-change partners that are absent from the change THEN Memphis SHALL emit a `missing_cochanges` directive naming those partners.
- [REQ-303] WHEN a changed symbol has structural dependents (callers/importers) resolvable via code intelligence THEN Memphis SHALL emit a `will_break` directive naming the dependents that may be affected.
- [REQ-304] WHEN a change touches code governed by Accepted Canon THEN Memphis SHALL emit a `governance_risk` directive citing the governing artifact, reusing the change-aware gate's governance resolution rather than recomputing it.
- [REQ-305] WHEN a directive is emitted THEN Memphis SHALL classify it as advisory by default and SHALL allow the enforcement policy to reclassify it as blocking or disabled by its stable rule code.

### Requirement 4 — Minimal git-intelligence substrate

- [REQ-401] WHEN change-risk needs history THEN Memphis SHALL derive per-file churn, co-change pairs, and author commit counts from `git log` over a bounded, configurable history window, in a package outside `internal/canon/...`.
- [REQ-402] WHEN co-change pairs are computed THEN Memphis SHALL count files that changed together in the same commit, and SHALL exclude pairs that are already linked by a code-intelligence import edge so that only hidden coupling is reported.
- [REQ-403] IF the store is not inside a git repository THEN the substrate SHALL report history as unavailable and change-risk SHALL degrade to diff-only metrics rather than failing.
- [REQ-404] WHEN the substrate reads history on identical repository state THEN it SHALL return identical results.

### Requirement 5 — Integration with the gate and standalone command

- [REQ-501] WHEN `memphis gate` is run with the risk flag THEN Memphis SHALL compute change-risk over the same change set the change-aware gate uses and SHALL merge the risk directives into the single gate `Result`, exit code, JSON, and SARIF outputs.
- [REQ-502] WHEN `memphis risk [range]` is run THEN Memphis SHALL score the given change (defaulting to the staged diff) and render the ranking, raw score, drivers, and directives, with a machine-readable output option.
- [REQ-503] WHEN change-risk emits a directive finding in any format THEN its rule identifier SHALL be stable across runs on identical inputs.
- [REQ-504] WHERE the risk flag is not passed to `memphis gate` THEN the gate SHALL produce output byte-identical to today for identical repository state.
- [REQ-505] WHEN a directive finding is rendered in SARIF THEN its location SHALL point at the relevant changed file.

### Requirement 6 — Preserve determinism, offline operation, and the authority boundary

- [REQ-601] WHERE change-risk is implemented THEN no package under `internal/canon/...` SHALL depend on the change-risk package, the git-intelligence substrate, `internal/codeintel`, `net/http`, or any LLM dependency, and the existing authority boundary and architecture tests SHALL continue to pass.
- [REQ-602] WHEN change-risk runs THEN Memphis SHALL perform no network access and SHALL invoke no LLM.
- [REQ-603] WHEN change-risk runs on identical repository state and identical change input THEN Memphis SHALL produce identical output, independent of map iteration order, time, or randomness.
- [REQ-604] IF code intelligence is unavailable for a changed file (unsupported language, missing file) THEN Memphis SHALL skip the `will_break` computation for that file and continue, rather than failing the run.

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO

