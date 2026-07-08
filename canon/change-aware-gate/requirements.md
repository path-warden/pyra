---
schema_version: 1
id: OKF-E6BA6PHEFK4Q
type: requirement
---

# Requirements Document

## Problem

The Memphis gate today validates Canon's *internal* correctness — structure,
standards, and relationship integrity over the whole corpus. It never inspects a
proposed **code change**, so its promise ("no agent silently violates a decision") is
only enforced against edits to Canon itself, not against the code changes that are the
actual risk.

The **change-aware gate** closes that loop. Given a set of changed files (the staged
diff by default), it resolves each changed file to the Accepted Canon artifacts
(requirements, decisions, designs) that govern it — an artifact governs a file when its
prose cites that file's path or the symbol-id of any symbol defined in it — and surfaces
them as citations. This is advisory by default — it tells a developer or agent *"the
code you are touching is governed by REQ-X; here it is"* — and can be escalated to
blocking by enforcement policy.

Governance is matched at **file granularity**: a changed file surfaces every artifact
that cites that file path or any symbol-id whose file is that path. Symbol/line-precise
matching (mapping diff hunks to enclosing symbols so only *touched* symbols count) is a
deliberate follow-up, not part of this spec.

The evaluation must remain **deterministic, offline, and AI-free**, and must not break
the authority-path boundary invariant (`internal/canon/...` may not depend on
`internal/codeintel`). It reports *coverage and proximity* ("you touched governed
code"), never a semantic verdict ("you violated it").

### Scope

**Must have:** a change-aware mode selected by a `--diff` flag on `memphis gate`,
sourcing changes from the git staged diff, resolving changed files to governing Canon at
file granularity, emitting citations as findings classified by enforcement policy, with
`--json` and SARIF output, while leaving the existing corpus gate behavior byte-for-byte
unchanged when the mode is off.

**Should have:** an explicit file-list input (`--changed a.go,b.go`, bypassing git) for
CI and testing; a `--since <ref>` input to source changes from a git ref range.

**Nice to have:** reporting when a changed file was cited by an artifact whose cited
symbol no longer resolves (drift surfaced during a change); a summary count of ungoverned
changed files.

**Out of scope:** symbol/line-precise hunk-to-symbol matching; semantic conformance
judgment ("does this code satisfy REQ-X"); LLM-assisted matching; author-time grounding
suggestions (a separate recommendation); mutating Canon or source.

## Requirements

### Requirement 1 — A change-aware evaluation mode on the gate

- [REQ-101] WHEN `memphis gate --diff` runs with a set of changed files THEN Memphis SHALL resolve, for each changed file, the Canon artifacts that cite that file path or a symbol-id defined in it.
- [REQ-102] WHEN a changed file maps to one or more governing Accepted Canon artifacts THEN Memphis SHALL emit one finding per governing artifact that cites the artifact ID, path, type, and lifecycle status alongside the changed file.
- [REQ-103] WHEN a changed file maps to no governing Canon artifact THEN Memphis SHALL NOT emit a governance finding for that file.
- [REQ-104] IF a governing artifact has a superseded lifecycle status THEN Memphis SHALL resolve to the current successor where one exists and SHALL cite the successor.
- [REQ-105] WHERE `--diff` is not passed THEN `memphis gate` SHALL produce byte-identical output to today for identical repository state.

### Requirement 2 — Sourcing the set of changed files

- [REQ-201] WHEN `--diff` is passed with no explicit file source THEN Memphis SHALL derive the changed files from the git staged index.
- [REQ-202] WHEN a user passes `--changed` with an explicit file list THEN Memphis SHALL evaluate exactly that list without consulting git.
- [REQ-203] WHEN a user passes `--since <ref>` THEN Memphis SHALL derive the changed files from the diff between that ref and the working state.
- [REQ-204] IF the store is not inside a git repository and no explicit file list is given THEN Memphis SHALL report that the change source is unavailable rather than reporting zero governed changes.
- [REQ-205] IF a changed file is deleted, renamed, or of an unsupported language THEN Memphis SHALL skip symbol extraction for it, still attempt file-path-level resolution, and continue the run.

### Requirement 3 — Classification and exit behavior via enforcement policy

- [REQ-301] WHEN a governance finding is produced THEN Memphis SHALL classify it as advisory by default.
- [REQ-302] WHEN the enforcement policy maps the governance finding code to blocking THEN Memphis SHALL classify matching findings as blocking.
- [REQ-303] WHEN the enforcement policy maps the governance finding code to disabled THEN Memphis SHALL drop matching findings.
- [REQ-304] WHEN any change-aware finding is classified as blocking THEN Memphis SHALL exit non-zero.
- [REQ-305] WHERE the corpus gate and the change-aware evaluation run in one invocation THEN Memphis SHALL aggregate both finding sets into one result and exit non-zero if either has a blocking finding.

### Requirement 4 — Output formats and CI integration

- [REQ-401] WHEN `--diff` runs with the JSON flag THEN Memphis SHALL emit machine-parseable JSON including each governance finding's code, changed file, governing artifact ID, and severity.
- [REQ-402] WHEN `--diff` runs with the SARIF flag THEN Memphis SHALL emit SARIF 2.1.0 in which each governance finding carries a stable rule ID and a location pointing at the changed file.
- [REQ-403] WHEN `--diff` runs with neither format flag THEN Memphis SHALL render human-readable output listing each changed file with its governing artifacts and citations.
- [REQ-404] WHERE a governance finding is emitted THEN its rule identifier SHALL be stable across runs on identical inputs.

### Requirement 5 — Preserve the deterministic, offline, AI-free authority path

- [REQ-501] WHERE the change-aware evaluation is implemented THEN no package under `internal/canon/...` SHALL gain a dependency on `internal/codeintel`, the tree-sitter runtime, `net/http`, or any LLM dependency.
- [REQ-502] WHEN the test suite runs THEN the existing `internal/canon` architecture and boundary tests SHALL continue to pass.
- [REQ-503] WHEN the change-aware evaluation runs on identical repository state and identical changed-file input THEN Memphis SHALL produce identical findings.
- [REQ-504] WHEN the change-aware evaluation resolves governance THEN it SHALL rely only on literal symbol-id and file references in Canon prose, never fuzzy or LLM-based matching.
- [REQ-505] IF the evaluation cannot resolve a symbol a Canon artifact cites THEN Memphis SHALL report it as unresolved rather than matching an incorrect symbol.

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO

