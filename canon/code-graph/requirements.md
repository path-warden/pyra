---
schema_version: 1
id: OKF-SGZ2E6T4HZZM
type: requirement
---

# Requirements Document

## Problem

Memphis reads code structurally on demand (`codeintel`: outline, symbols, callers,
map, definition), but it has no *persistent, whole-repo graph*. An agent can ask
"who calls X?" one name at a time, but cannot ask "which symbols are the hubs of
this codebase?", "what are the logical modules?", "where are the dependency
cycles?", or "what is reachable from the entry points?" Those are graph questions,
and answering them today means many `codeintel` round-trips with no global view.

This feature promotes `codeintel`'s on-demand reads into a **persistent, in-memory
code graph** (new package `internal/codegraph`): two-tier file+symbol nodes with
reference, containment, and import edges, plus the standard whole-graph analyses —
**centrality** (which code is central), **communities** (logical modules),
**cycles** (strongly-connected components), and **reachability** (execution flows
from entry points). It is exposed via `memphis graph` and read-only MCP tools, and
its reachable set is the substrate dead-code detection (capability #5) will consume.

Everything is **deterministic, offline, and AI-free**, built with **standard
algorithms and no external graph library and no learned constants** — an
independent implementation, so nothing here carries a third-party license. It lives
**outside `internal/canon/...`** (like `codeintel`, on which it depends) and the
boundary test is extended to forbid the authority path from importing it. Edges are
resolved by the same **honest name-matching** `codeintel` already uses for
`callers` (a referenced name resolves to the definition(s) of that name), never
fuzzy or LLM matching.

### Scope

**Must have:** an in-memory graph built once from a `codeintel` walk over the
configured code roots (file nodes, symbol nodes keyed by symbol-id;
file→symbol containment edges; symbol→symbol reference edges by name; file→file
import edges aggregated from symbol edges); PageRank centrality; deterministic
label-propagation communities; Tarjan strongly-connected components (cycles);
reachability from entry points; a `memphis graph` command with `--centrality`,
`--communities`, `--cycles`, `--reachability` subviews and `--json`; read-only MCP
tools; determinism (identical repo state → byte-identical output).

**Should have:** a bounded, configurable graph size / node cap with a clear signal
when truncated; a way to scope the graph to a subdirectory.

**Nice to have:** per-node in/out degree in the centrality view; naming a community
by its most-central member.

**Out of scope:** betweenness centrality and Leiden community detection (PageRank
and label-propagation are the deterministic, self-contained equivalents — deferred);
the centrality-weighted `will_break` enhancement to change-risk (#2); dead-code
*reporting* (capability #5 consumes this graph's reachable set); persisting the
graph to disk; framework-aware route→handler edges; any LLM-derived edge or label.

## Requirements

### Requirement 1 — Build the two-tier graph from code intelligence

- [REQ-101] WHEN the graph is built THEN Memphis SHALL create a symbol node for every definition found by a `codeintel` walk of the configured code roots, keyed by its stable symbol-id.
- [REQ-102] WHEN the graph is built THEN Memphis SHALL create a file node for every source file that contains at least one definition, and a containment edge from each file node to the symbol nodes it defines.
- [REQ-103] WHEN a definition references a name that resolves to one or more defined symbols THEN Memphis SHALL create a directed reference edge from the referencing symbol to each resolved definition, resolving names via a repo-wide name→definition index (the same literal name-matching `codeintel` callers uses), never fuzzy matching.
- [REQ-104] WHEN symbol reference edges exist THEN Memphis SHALL derive file→file import (depends-on) edges by aggregating the reference edges to the file level, excluding self-loops.
- [REQ-105] WHERE a referenced name resolves to no known definition THEN Memphis SHALL omit an edge rather than create an edge to an incorrect or synthetic node.
- [REQ-106] IF a code file's language is unsupported or unparsable THEN Memphis SHALL skip it and continue building the graph from the remaining files.

### Requirement 2 — Centrality (hubs)

- [REQ-201] WHEN centrality is requested THEN Memphis SHALL compute a PageRank score for every symbol node using power iteration with a fixed damping factor, a fixed iteration cap, and a fixed convergence tolerance.
- [REQ-202] WHEN centrality results are returned THEN Memphis SHALL rank nodes by PageRank descending with a stable tiebreak (symbol-id) so identical repository state yields identical order.
- [REQ-203] WHEN a limit is provided THEN Memphis SHALL return only the top-N most central symbols and SHALL indicate the total node count.

### Requirement 3 — Communities (logical modules)

- [REQ-301] WHEN communities are requested THEN Memphis SHALL partition the symbol nodes into communities using deterministic label propagation (fixed iteration cap; ties broken by a defined, deterministic rule, not map-iteration order).
- [REQ-302] WHEN the graph is unchanged THEN repeated community detection SHALL produce identical partitions.
- [REQ-303] WHEN communities are returned THEN each community SHALL list its member symbols and its size, ordered deterministically.

### Requirement 4 — Cycles (strongly-connected components)

- [REQ-401] WHEN cycles are requested THEN Memphis SHALL compute the strongly-connected components of the symbol reference graph using Tarjan's algorithm and SHALL report every component containing a cycle (size greater than one, or a self-referential node).
- [REQ-402] WHEN cycles are reported THEN each reported component SHALL list its member symbols, ordered deterministically, and single-node acyclic components SHALL NOT be reported.

### Requirement 5 — Reachability (execution flows)

- [REQ-501] WHEN reachability is requested THEN Memphis SHALL compute the set of symbols reachable via reference edges from a set of entry points, where entry points are `main`/program entry symbols and exported/public symbols.
- [REQ-502] WHEN reachability is computed THEN Memphis SHALL return the reachable set and the unreachable remainder, each ordered deterministically, so that a later capability can consume the unreachable set.
- [REQ-503] WHERE a repository has no identifiable entry points THEN Memphis SHALL report an empty reachable set and the full node set as unreachable, rather than failing.

### Requirement 6 — CLI and MCP surfaces

- [REQ-601] WHEN a user runs `memphis graph` with a subview flag (`--centrality`, `--communities`, `--cycles`, or `--reachability`) THEN Memphis SHALL render that view as human-readable text, and SHALL emit machine-readable JSON when `--json` is given.
- [REQ-602] WHEN the MCP server is running THEN Memphis SHALL expose read-only tools returning the centrality, communities, and cycles views, equivalent to the corresponding CLI subview for the same inputs.
- [REQ-603] WHEN any graph surface runs THEN it SHALL be read-only and SHALL NOT mutate the repository.
- [REQ-604] WHEN a scope path or node cap is provided THEN Memphis SHALL restrict the graph accordingly and SHALL signal when the graph was truncated by the cap.

### Requirement 7 — Determinism, offline operation, and the authority boundary

- [REQ-701] WHEN the graph and its analyses run twice on identical repository state THEN Memphis SHALL produce byte-identical output, independent of map iteration order, time, or randomness.
- [REQ-702] WHEN the graph is built or analyzed THEN Memphis SHALL perform no network access and SHALL invoke no LLM, and SHALL use no external graph library and no learned constants.
- [REQ-703] WHERE the code graph is implemented THEN it SHALL remain outside `internal/canon/...`, no package under `internal/canon/...` SHALL depend on it, and the existing authority boundary and architecture tests SHALL continue to pass.
- [REQ-704] WHEN the code graph is built THEN it SHALL derive the graph solely from a bounded `codeintel` walk of the code roots, as a pure function of repository state.

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO

