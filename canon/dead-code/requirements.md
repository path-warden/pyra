---
schema_version: 1
id: OKF-G7NDSBZ4W37H
type: requirement
---

# Requirements Document

## Problem

Pyra's code graph (#3) already computes which symbols are **reachable** from the
repository's entry points and which are not — but nothing surfaces the unreachable
set as an actionable **dead-code** report. Engineers want to know which definitions
can likely be deleted, how confident that judgment is, and how much cleanup each
buys — and, uniquely for pyra, whether any Accepted Canon still *cites* code
that has gone unreachable (a drift signal).

This feature adds a thin **dead-code layer** (new package `internal/deadcode`)
that **consumes** `codegraph`'s reachability result — it does no new graph work.
For each unreachable symbol it assigns a **confidence tier**, estimates the
**cleanup impact** (line count), and reports the file, kind, and symbol-id. It adds
the pyra-unique **governed-dead-code** signal (an unreachable symbol still cited
by Canon), and is exposed via `pyra dead-code` and a read-only MCP tool.

It is **deterministic, offline, AI-free**, uses **no external library**, and lives
**outside `internal/canon/...`** (the boundary test is extended to forbid the
authority path from importing it).

### Scope

**Must have:** consume `codegraph.Reachability().Unreachable`; a confidence tier
per candidate (high / medium / low, by the rules below); a cleanup-impact estimate
(the symbol's line count, resolved via code intelligence); the candidate's file,
kind, and symbol-id; exclusion of test entry points (`Test*` / `main`) from the
report; `pyra dead-code [store]` (ranked by cleanup impact, `--tier` filter,
`--json`); a read-only `get_dead_code` MCP tool; deterministic sorted output.

**Should have:** the **governed-dead-code** authority tie-in (an unreachable symbol
whose symbol-id is still cited by Accepted Canon); a total-cleanup-impact summary.

**Nice to have:** grouping candidates by file in the text output.

**Out of scope (documented limitations):** framework route→handler entry edges
(pyra has no framework-aware edges, so a handler reachable only via a route may
read as unreachable — the conservative high-tier rule mitigates this, and it is
documented); automatic removal / code generation; and cross-repo consumer
detection.

## Requirements

### Requirement 1 — Report unreachable symbols as candidates

- [REQ-101] WHEN dead-code analysis runs over a built code graph THEN Pyra SHALL treat every symbol in the graph's `Unreachable` set as a dead-code candidate, deriving nothing new about reachability itself.
- [REQ-102] WHEN a candidate is reported THEN Pyra SHALL include its symbol-id, name, kind, and file.
- [REQ-103] WHERE a candidate is a recognized test entry point (a `Test`-prefixed function or a `main`) THEN Pyra SHALL exclude it from the report rather than flag it as dead.
- [REQ-104] IF the code graph is empty or has no unreachable symbols THEN Pyra SHALL return an empty report rather than an error.

### Requirement 2 — Confidence tiers

- [REQ-201] WHEN a candidate is unreachable AND has no textual references anywhere in the code roots AND is not defined in a test file THEN Pyra SHALL assign it the **high** confidence tier.
- [REQ-202] WHEN a candidate is unreachable AND has one or more textual references (a possible dynamic or reflective use) THEN Pyra SHALL assign it the **medium** confidence tier.
- [REQ-203] WHEN a candidate is unreachable AND is defined in a test file THEN Pyra SHALL assign it the **low** confidence tier.
- [REQ-204] WHEN textual references are counted THEN Pyra SHALL use the existing code-intelligence reference search, and SHALL NOT count the candidate's own definition site as a reference.

### Requirement 3 — Cleanup-impact estimate

- [REQ-301] WHEN a candidate is reported THEN Pyra SHALL estimate its cleanup impact as the number of source lines the symbol spans, resolved via code intelligence.
- [REQ-302] IF a candidate's source cannot be resolved (renamed, moved, or unsupported) THEN Pyra SHALL report a cleanup impact of zero rather than failing.
- [REQ-303] WHEN candidates are ranked THEN Pyra SHALL order them by cleanup impact descending, with a stable tiebreak (symbol-id) so identical repository state yields identical order.

### Requirement 4 — Governed dead code (authority tie-in)

- [REQ-401] WHEN a candidate's symbol-id is still cited by an Accepted Canon artifact THEN Pyra SHALL mark it as governed dead code, reusing the existing Canon↔code grounding resolution rather than recomputing it.
- [REQ-402] WHERE the store has no Canon THEN Pyra SHALL report no governed-dead-code marks, rather than treating every candidate as ungoverned.

### Requirement 5 — CLI and MCP surfaces

- [REQ-501] WHEN a user runs `pyra dead-code` THEN Pyra SHALL print the candidates ranked by cleanup impact with their tier, file, and impact, plus a total-cleanup-impact summary, and SHALL emit machine-readable JSON when `--json` is given.
- [REQ-502] WHEN a user passes `--tier <high|medium|low>` THEN Pyra SHALL report only candidates at that tier.
- [REQ-503] WHEN the MCP server is running THEN Pyra SHALL expose a read-only `get_dead_code` tool returning the ranked candidates, equivalent to the CLI for the same inputs.
- [REQ-504] WHEN any dead-code surface runs THEN it SHALL be read-only and SHALL NOT mutate the repository.

### Requirement 6 — Determinism, offline operation, and the authority boundary

- [REQ-601] WHEN the dead-code layer runs twice on identical repository state THEN Pyra SHALL produce byte-identical output, independent of map iteration order, time, or randomness.
- [REQ-602] WHEN the dead-code layer runs THEN Pyra SHALL perform no network access and SHALL invoke no LLM.
- [REQ-603] WHERE the dead-code layer is implemented THEN it SHALL remain outside `internal/canon/...`, no package under `internal/canon/...` SHALL depend on it, and the existing authority boundary and architecture tests SHALL continue to pass.
- [REQ-604] WHEN the dead-code layer builds its report THEN it SHALL derive it from the code-graph reachability, code-intelligence, and Canon layers as a pure function of repository state.

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO

