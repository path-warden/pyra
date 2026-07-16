---
schema_version: 1
id: OKF-BGBM6E7V1CPH
type: requirement
---

# Requirements Document

## Problem

Pyra reads code structurally (`codeintel`) and enforces authority (Canon), but
it is nearly blind to the *behavioral* signal that only git history carries: which
files churn, who owns them, where knowledge is siloed, and which files change
together without any structural link. Change-risk already needed a sliver of this
and bootstrapped a minimal `internal/gitint` (per-file churn, co-change pairs,
author counts). That sliver is not a general layer: there is no ownership, no bus
factor, no hotspot ranking, no module rollup, and nothing an agent or a human can
query directly.

This feature promotes `internal/gitint` into a **complete git-intelligence layer**:
a deterministic, offline, AI-free read of git history — computed in one bounded
`git log` walk — that yields per-file behavioral metrics, repo-level hotspot
ranking, and module rollups, exposed over the CLI and MCP. It is the shared
substrate change-risk consumes today and code-health (parity capability #4) will
build on next.

The signals are computed from **public, well-described methods** (Kamei-style
change metrics, temporal/decayed churn, commit-author ownership).

The hard invariant it inherits: everything stays **deterministic and offline**. In
particular, all recency windows are anchored to **HEAD's commit timestamp**, never
wall-clock time, so re-running on identical repository state yields byte-identical
output. It lives outside `internal/canon/...` (the authority path must never depend
on it) and the existing boundary test already forbids that import.

### Scope

**Must have:** a per-file history model (commit counts total + windowed, line
churn, age, first/last commit, temporal hotspot score, primary owner + ownership %,
recent owner, contributor count, bus factor) computed in one bounded walk anchored
to HEAD; repo-level churn percentile + `is_hotspot` (top-quartile temporal churn
gated by absolute activity floors); module (top-level directory) rollups; CLI
`pyra hotspots` and `pyra ownership [path]` with `--json`; read-only MCP
tools; determinism anchored to HEAD; the existing `Churn` / `CoChangePartners` /
`AuthorCommits` API preserved for change-risk.

**Should have:** a configurable history window and commit-count cap; co-change
partners surfaced in the per-file model (already computed); graceful degradation to
a clear "history unavailable" outside a git repository.

**Nice to have:** commit-category mix per file (feat/fix/refactor/…); a
"significant commits" excerpt per file.

**Out of scope:** `git blame` line-level ownership (commit-author ownership is the
deterministic baseline; blame is a later refinement); the churn×**complexity**
hotspot intersection (needs code-health markers — capability #4); contributor
profile pages, reviewer suggestions, agent provenance, prior-defect labelling, and
any LLM-derived signal.

## Requirements

### Requirement 1 — Per-file git history metrics

- [REQ-101] WHEN the layer indexes a repository THEN Pyra SHALL compute, for each tracked file in the history window, its total commit count and its commit counts within the 30-day and 90-day windows.
- [REQ-102] WHEN a file's history is indexed THEN Pyra SHALL compute its added and deleted line churn within the window, its first and last commit timestamps, and its age in days.
- [REQ-103] WHEN a file's history is indexed THEN Pyra SHALL compute a temporal hotspot score as a sum of per-commit churn weighted by exponential time decay with a 180-day half-life.
- [REQ-104] WHEN a file's authorship is aggregated THEN Pyra SHALL identify the primary owner as the author with the most commits to that file and SHALL report that owner's share of the file's commits.
- [REQ-105] WHEN a file's authorship is aggregated THEN Pyra SHALL report the contributor count and the bus factor, where bus factor is the minimum number of top authors whose cumulative commit share first reaches 80%.
- [REQ-106] WHEN a file has commits within the 90-day window THEN Pyra SHALL report the recent owner as the most-active author in that window; otherwise the recent owner SHALL be reported as absent.

### Requirement 2 — Repo-level hotspot ranking

- [REQ-201] WHEN all files are indexed THEN Pyra SHALL assign each file a churn percentile by ranking files on their temporal hotspot score (with the recent commit count as a tiebreak).
- [REQ-202] WHEN a file is in the top quartile by churn percentile AND meets the absolute activity floors THEN Pyra SHALL mark it as a hotspot; otherwise it SHALL NOT.
- [REQ-203] WHERE the repository's activity is too low to exceed the floors THEN Pyra SHALL mark no file a hotspot, rather than degrading the top quartile into "every recently touched file".
- [REQ-204] WHEN hotspots are requested THEN Pyra SHALL return them ranked by temporal hotspot score descending, with a stable tiebreak so identical states produce identical order.

### Requirement 3 — Module (directory) rollups

- [REQ-301] WHEN the layer aggregates by top-level module (directory) THEN Pyra SHALL report per module the file count, the hotspot count and density, the average churn, the median bus factor, and the module's primary owner.
- [REQ-302] WHEN modules are reported THEN Pyra SHALL order them deterministically (by a defined key) so identical repository state yields identical output.

### Requirement 4 — Co-change coupling (preserved and surfaced)

- [REQ-401] WHEN a file's history is indexed THEN Pyra SHALL retain its co-change partners (files changing in the same commits) with shared-commit counts, as already computed by the substrate.
- [REQ-402] WHERE co-change partners are surfaced THEN the layer SHALL preserve the existing `Churn`, `CoChangePartners`, and `AuthorCommits` API so change-risk continues to function unchanged.

### Requirement 5 — CLI and MCP surfaces

- [REQ-501] WHEN a user runs `pyra hotspots` THEN Pyra SHALL print the ranked hotspots with their churn, owner, and bus factor, and SHALL emit machine-readable JSON when `--json` is given.
- [REQ-502] WHEN a user runs `pyra ownership [path]` THEN Pyra SHALL print ownership, bus factor, and contributor count for the file or module at that path (defaulting to the whole repo), with a `--json` option.
- [REQ-503] WHEN the MCP server is running THEN Pyra SHALL expose read-only tools that return the hotspot ranking and the ownership/bus-factor signals, returning results equivalent to the corresponding CLI command for the same inputs.
- [REQ-504] WHEN any surface reports a metric derived from git history THEN it SHALL be read-only and SHALL NOT mutate the repository.

### Requirement 6 — Determinism, offline operation, and the authority boundary

- [REQ-601] WHEN the layer computes any recency-dependent metric THEN Pyra SHALL anchor the reference time to HEAD's commit timestamp and SHALL NOT use wall-clock time.
- [REQ-602] WHEN the layer runs twice on identical repository state THEN Pyra SHALL produce byte-identical output, independent of map iteration order, wall-clock time, or randomness.
- [REQ-603] WHEN the layer runs THEN Pyra SHALL perform no network access and SHALL invoke no LLM.
- [REQ-604] WHERE the git-intelligence layer is implemented THEN it SHALL remain outside `internal/canon/...`, no package under `internal/canon/...` SHALL depend on it, and the existing authority boundary and architecture tests SHALL continue to pass.
- [REQ-605] IF the store is not inside a git repository THEN Pyra SHALL report history as unavailable and SHALL exit cleanly rather than failing or emitting misleading zeros.

### Requirement 7 — Bounded, configurable history

- [REQ-701] WHEN the layer walks history THEN Pyra SHALL bound the walk to a configurable window of recent commits with a documented default, and SHALL flag when a file's history is capped by that bound.
- [REQ-702] WHEN indexing a repository THEN Pyra SHALL derive all per-file metrics from a single bounded history walk rather than one subprocess per file.

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO
