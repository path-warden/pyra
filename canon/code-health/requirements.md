---
schema_version: 1
id: OKF-9YDTJPJ59W6X
type: requirement
---

# Requirements Document

## Problem

Memphis can now see structure (`codeintel`), behavior (`gitint`, #1), and topology
(`codegraph`, #3), but it cannot answer the question a team actually asks: *"which
files are unhealthy, and why?"* There is no per-file score, no ranking of the worst
code, no composite of the size, complexity, churn, ownership, coupling, coverage,
and duplication signals — and, critically, no tie from a risky file back to whether
the team's Canon even governs it.

This feature adds the **code-health layer** (new package `internal/codehealth`): a
deterministic, offline, per-file score built from the **full defect/maintainability
biomarker roster** — the "25 markers" — aggregated by a scoring kernel into
**three independently-capped signals** (defect risk, maintainability, performance).
It ranks the lowest-scoring files, rolls up repo KPIs, names a concrete refactoring
for the top marker, and surfaces memphis-unique **authority** signals: an
*ungoverned hotspot* (a churn hotspot with no governing Canon), *stale governance*,
and *contradictory decisions*.

The scoring **kernel** (severity deductions, per-biomarker weight multipliers,
per-category caps, the three-dimension composition, and the [1.0, 10.0] clamp) and
its **calibrated constants** are a **faithful port of repowise's**, so memphis is
parity-testable against it; the ported constants are confined to one file so they
can be swapped later. (repowise is AGPL-3.0; those constants carry that license —
the same posture as change-risk.) The biomarkers themselves are memphis's own
deterministic detectors over the layers it owns.

Delivering the full roster requires one **enabler**: extending `internal/codeintel`
with an **AST-metrics pass** (cyclomatic complexity, nesting depth, per-method
field access, error-handling patterns) — the substrate the structural biomarkers
need — plus a small **coverage-ingestion** reader and a **clone detector**.

Everything is **deterministic, offline, and AI-free**, uses **no external library**
beyond the ported constants, lives **outside `internal/canon/...`**, and the
boundary test is extended to forbid the authority path from importing it.

### Scope

**Must have:** the ported scoring kernel + calibrated constants for the full
roster; the `codeintel` AST-metrics enabler; the structural-complexity biomarkers
(complex_method, nested_complexity, brain_method, bumpy_road, complex_conditional,
large_method, god_class, god_file, low_cohesion, primitive_obsession); duplication
(dry_violation via a clone detector); the organizational biomarkers (churn_risk,
change_entropy, co_change_scatter, ownership_risk, code_age_volatility,
function_hotspot, developer_congestion, knowledge_loss, hidden_coupling,
prior_defect); coverage ingestion + coverage biomarkers (untested_hotspot,
coverage_gap, coverage_gradient); test-quality (large_assertion_block,
duplicated_assertion_block) and error_handling; the governance biomarkers
(ungoverned_hotspot, stale_governance, contradictory_decision); repo KPIs; a named
refactoring suggestion per top marker; `memphis health` (+ `--file`, `--json`) and
a read-only `get_health` MCP tool; a parity test pinning the kernel to repowise;
determinism.

**Should have:** a `cyclic_dependency` structural signal from the graph (#3) SCCs
feeding a Break-Cycle suggestion; a configurable window passthrough; suppression of
a dimension to 10.0 when a file has no findings for it.

**Nice to have:** per-finding line ranges surfaced in the `--file` view.

**Out of scope (the single deferral):** the **performance dataflow / loop biomarker
subsystem** (io_in_loop and the ~20 loop/async/SQL performance detectors, and the
control-/data-flow engine they require). Those constitute the *performance*
dimension's detectors; that dimension **ships present-but-empty** (a clean file
scores 10.0 on it) and its detectors land as a follow-up capability. Also out of
scope: health trends/snapshots and refactoring **code generation**.

## Requirements

### Requirement 1 — The scoring kernel and calibrated constants (ported for parity)

- [REQ-101] WHEN a file's biomarker findings are scored THEN Memphis SHALL start the file at 10.0, deduct each finding's severity value times its per-biomarker weight, accumulate per category, cap each category at its configured maximum, and clamp to the range 1.0 to 10.0.
- [REQ-102] WHEN a category's summed deductions exceed its cap THEN Memphis SHALL scale that category's per-finding contributions proportionally so the category total equals the cap.
- [REQ-103] WHERE the kernel's severity deductions, per-biomarker weight multipliers, category assignments, and category caps are ported from repowise THEN Memphis SHALL confine them to a single, clearly-attributed source file that can be replaced without changing biomarker, composition, or surface code.
- [REQ-104] WHEN Memphis scores a fixed set of findings THEN the per-dimension scores SHALL match repowise's `score_file` output for the same findings within a documented tolerance, verified by an automated parity test covering the full roster.

### Requirement 2 — Three independent dimensions

- [REQ-201] WHEN findings are composed THEN Memphis SHALL produce three scores — defect, maintainability, and performance — each computed by the same kernel against its own weight, category, and cap tables, and none SHALL feed into another.
- [REQ-202] WHERE a biomarker contributes to more than one dimension THEN its deduction SHALL be applied independently in each dimension it belongs to.
- [REQ-203] WHEN a file has no findings in a dimension THEN that dimension SHALL score 10.0.
- [REQ-204] WHERE the performance dimension has no detectors in this release THEN every file SHALL score 10.0 on performance without error, and the dimension SHALL remain wired so its detectors can be added later.

### Requirement 3 — Code-intelligence AST-metrics enabler

- [REQ-301] WHEN `codeintel` extracts a function or method THEN Memphis SHALL additionally compute its cyclomatic complexity (decision-point count) and its maximum nesting depth from the tree-sitter AST.
- [REQ-302] WHEN `codeintel` extracts a class/struct and its methods THEN Memphis SHALL compute, per method, the set of instance fields it accesses, sufficient to compute a cohesion (LCOM-style) metric.
- [REQ-303] WHEN `codeintel` extracts a function THEN Memphis SHALL detect error-handling anti-patterns in it (for example an empty/`swallowed` catch, a bare `except`, an ignored error) as structured findings.
- [REQ-304] WHERE the AST-metrics pass is added THEN it SHALL be deterministic, offline, and reuse the existing tree-sitter runtime, and SHALL degrade to zero metrics on unsupported languages rather than failing.
- [REQ-305] WHEN the AST-metrics pass runs on identical source THEN it SHALL return identical metrics.

### Requirement 4 — Structural-complexity biomarkers

- [REQ-401] WHEN a function's cyclomatic complexity, nesting depth, or NLOC exceeds a documented threshold THEN Memphis SHALL emit `complex_method`, `nested_complexity`, or `large_method` respectively, with severity scaling with the excess.
- [REQ-402] WHEN a function exhibits both high complexity and high nesting (a documented combined gate) THEN Memphis SHALL emit `brain_method`, and WHEN a function alternates nesting depth across many blocks THEN Memphis SHALL emit `bumpy_road`.
- [REQ-403] WHEN a boolean condition combines more than a documented number of operators THEN Memphis SHALL emit `complex_conditional`.
- [REQ-404] WHEN a class defines more than a documented number of methods THEN Memphis SHALL emit `god_class`; WHEN a file defines more than a documented number of top-level symbols THEN Memphis SHALL emit `god_file`.
- [REQ-405] WHEN a class's method/field cohesion (LCOM-style) falls below a documented threshold THEN Memphis SHALL emit `low_cohesion`.
- [REQ-406] WHEN a function takes more than a documented number of primitive-typed parameters THEN Memphis SHALL emit `primitive_obsession`.
- [REQ-407] IF a file's language is unsupported or unparsable THEN Memphis SHALL skip its structural biomarkers and continue scoring the remaining files.

### Requirement 5 — Duplication and test-quality biomarkers

- [REQ-501] WHEN two or more code spans are near-identical beyond a documented size THEN Memphis SHALL detect the clone with a deterministic Rabin-Karp fingerprint and emit `dry_violation` on the involved files.
- [REQ-502] WHEN a test function contains a large or duplicated block of assertions beyond a documented threshold THEN Memphis SHALL emit `large_assertion_block` or `duplicated_assertion_block`.
- [REQ-503] WHEN a function contains an error-handling anti-pattern (from REQ-303) THEN Memphis SHALL emit an `error_handling` finding.

### Requirement 6 — Organizational biomarkers (from git intelligence)

- [REQ-601] WHEN a file's churn, change entropy, co-change scatter, single-owner share/bus factor, or age-with-recent-churn crosses a documented threshold THEN Memphis SHALL emit `churn_risk`, `change_entropy`, `co_change_scatter`, `ownership_risk`, or `code_age_volatility` respectively.
- [REQ-602] WHEN a file is both high-churn and high-complexity THEN Memphis SHALL emit `function_hotspot`; WHEN many authors touch a file in a short window THEN Memphis SHALL emit `developer_congestion`; WHEN a file's primary author no longer contributes THEN Memphis SHALL emit `knowledge_loss`.
- [REQ-603] WHEN a file co-changes with files it has no structural link to THEN Memphis SHALL emit `hidden_coupling`; WHEN a file received bug-fix commits in the window (detected by commit-message convention) THEN Memphis SHALL emit `prior_defect`.
- [REQ-604] IF git history is unavailable THEN Memphis SHALL omit organizational biomarkers and still score files from their structural biomarkers.

### Requirement 7 — Coverage ingestion and coverage biomarkers

- [REQ-701] WHEN a user supplies a coverage report in a supported format (LCOV, and at least one of Cobertura/Clover) THEN Memphis SHALL parse per-file line coverage from it, deterministically and offline.
- [REQ-702] WHEN a file's coverage is below a documented threshold THEN Memphis SHALL emit `coverage_gap`, and Memphis SHALL emit a continuous `coverage_gradient` deduction that scales with the uncovered fraction.
- [REQ-703] WHEN a git hotspot file is also poorly covered THEN Memphis SHALL emit `untested_hotspot`.
- [REQ-704] WHERE no coverage report is supplied THEN Memphis SHALL omit all coverage biomarkers rather than treating uncovered files as covered or as gaps.

### Requirement 8 — Governance biomarkers (authority tie-in)

- [REQ-801] WHEN a file is a git hotspot AND no Accepted Canon governs it THEN Memphis SHALL emit `ungoverned_hotspot`, reusing the existing hotspot and Canon-governance resolution.
- [REQ-802] WHEN a governing Canon artifact is materially older than the code it governs (a documented recency gate) THEN Memphis SHALL emit `stale_governance`.
- [REQ-803] WHEN two Accepted Canon decisions conflict (a `conflicts`-type relationship, or a newer decision that reverses an older one without a supersede edge) THEN Memphis SHALL emit `contradictory_decision`.
- [REQ-804] WHERE the store has no Canon THEN Memphis SHALL NOT emit governance biomarkers, rather than flagging every file.

### Requirement 9 — Repo KPIs and refactoring suggestions

- [REQ-901] WHEN files are scored THEN Memphis SHALL compute an NLOC-weighted average health, an NLOC-weighted hotspot health, and the worst-performing file with its score.
- [REQ-902] WHEN a file has findings THEN Memphis SHALL name a concrete refactoring for its highest-impact marker (god_class → Extract Class, large_method/brain_method → Extract Helper, god_file → Split File, a graph cycle → Break Cycle, low_cohesion → Extract Class) as a suggestion label only, with no generated code.
- [REQ-903] WHEN files are ranked THEN Memphis SHALL order them by defect score ascending with a stable tiebreak so identical repository state yields identical order.

### Requirement 10 — CLI and MCP surfaces

- [REQ-1001] WHEN a user runs `memphis health` THEN Memphis SHALL print the KPIs and lowest-scoring files with their three dimension scores and top marker, and SHALL emit JSON when `--json` is given; a `--coverage <file>` flag SHALL supply a coverage report.
- [REQ-1002] WHEN a user runs `memphis health --file <path>` THEN Memphis SHALL print that file's findings, per-finding impact, dimension scores, and refactoring suggestion.
- [REQ-1003] WHEN the MCP server is running THEN Memphis SHALL expose a read-only `get_health` tool returning the KPIs and lowest-scoring files, equivalent to the CLI for the same inputs.
- [REQ-1004] WHEN any health surface runs THEN it SHALL be read-only and SHALL NOT mutate the repository.

### Requirement 11 — Determinism, offline operation, and the authority boundary

- [REQ-1101] WHEN the health layer runs twice on identical repository state and coverage input THEN Memphis SHALL produce byte-identical output, independent of map iteration order, time, or randomness.
- [REQ-1102] WHEN the health layer runs THEN Memphis SHALL perform no network access and SHALL invoke no LLM.
- [REQ-1103] WHERE the health layer is implemented THEN it SHALL remain outside `internal/canon/...`, no package under `internal/canon/...` SHALL depend on it, and the existing authority boundary and architecture tests SHALL continue to pass.
- [REQ-1104] WHEN the health layer builds its inputs THEN it SHALL derive them from the code-intelligence, git-intelligence, graph, and Canon layers as a pure function of repository state (plus any supplied coverage report).

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO

