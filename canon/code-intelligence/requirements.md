---
schema_version: 1
id: OKF-A9A05Q5BP012
type: requirement
---

# Requirements Document

## Problem

Pyra can tell an agent what the team decided, but it cannot see the code that the decision governs, so the agent must leave Pyra and grep to connect authority to implementation. That round-trip is token-expensive, error-prone, and breaks the link back to Canon. There is no single tool — and no single MCP server — that answers both "what did we decide?" and "what does the code actually do?", and there is no way to resolve an authoritative artifact to the real symbols it governs.

## Requirements

### Requirement 1 — Structural code operations in Pyra

- [REQ-101] WHEN an agent requests a file's definition skeleton THEN Pyra SHALL return an outline of that file's symbols including kind, name, parent, signature, and stable symbol-id, without returning full file bodies.
- [REQ-102] WHEN an agent searches for a symbol by exact name or by substring across a directory THEN Pyra SHALL return the matching symbols with their symbol-ids and source locations.
- [REQ-103] WHEN Pyra walks a directory for a code operation THEN Pyra SHALL honor `.gitignore` and SHALL exclude ignored paths.
- [REQ-104] WHEN an agent requests source by symbol-id THEN Pyra SHALL return only that symbol's source text.
- [REQ-105] WHEN an agent requests callers or references of a name THEN Pyra SHALL return the call and reference sites, each annotated with its enclosing definition and with structural-versus-textual provenance.
- [REQ-106] WHEN an agent requests a directory dependency map THEN Pyra SHALL return the definitions and their outgoing references without symbol bodies.
- [REQ-107] WHEN an agent requests the definition of a name THEN Pyra SHALL return the resolved definition site or sites.
- [REQ-108] WHEN an agent requests the definition of the usage at a file:line:column position THEN Pyra SHALL return the scope-aware resolved definition site or sites.
- [REQ-109] WHEN an agent requests a syntax check of a file THEN Pyra SHALL report each ERROR or MISSING parse node and SHALL signal failure when any exist.
- [REQ-110] WHERE any operation returns a symbol-id THEN a subsequent operation SHALL accept that same symbol-id to retrieve or navigate the referenced symbol.

### Requirement 2 — One MCP server, both faces

- [REQ-201] WHEN `pyra serve` starts THEN the MCP server SHALL register both the existing authority and reference tools and the new code-intelligence tools under one server instance.
- [REQ-202] WHEN a client calls `tools/list` THEN Pyra SHALL include the code-intelligence tools with their LLM-facing descriptions and input schemas.
- [REQ-203] WHEN a code-intelligence tool is invoked over MCP THEN Pyra SHALL return results equivalent to the corresponding CLI operation for the same inputs.
- [REQ-204] IF the store has no Canon because `.okf/config.yaml` is absent THEN the code-intelligence tools SHALL still function.
- [REQ-205] IF a target code directory is unavailable THEN the authority and reference tools SHALL still function.
- [REQ-206] WHEN a code-intelligence tool fails, such as on an unsupported language or a missing file, THEN Pyra SHALL return a structured tool error and SHALL keep the server running.

### Requirement 3 — One self-contained Go binary (native re-implementation)

- [REQ-301] WHEN a user installs Pyra THEN the code-intelligence operations SHALL be invocable from the `pyra` binary with no separate `grove` installation and no Rust runtime present.
- [REQ-302] WHEN a code-intelligence operation executes THEN Pyra SHALL run it through native Go code using tree-sitter Go bindings.
- [REQ-303] WHEN a code-intelligence operation executes THEN Pyra SHALL NOT shell out to, embed, or proxy a prebuilt grove binary.
- [REQ-304] IF a required language grammar is not present THEN Pyra SHALL provision or locate it through a documented cache-based mechanism and SHALL NOT fail silently.
- [REQ-305] WHERE grammars cannot be statically linked into the Go binary THEN Pyra SHALL bundle or fetch-and-cache them while preserving the no-separate-user-install guarantee.

### Requirement 4 — CLI parity for code operations

- [REQ-401] WHEN a user runs a code subcommand (outline, symbols, source, check, callers, map, or definition) THEN Pyra SHALL execute the corresponding operation and SHALL render human-readable output by default.
- [REQ-402] WHEN a user passes the JSON output flag to a code subcommand THEN Pyra SHALL emit stable machine-parseable JSON that includes symbol-ids.
- [REQ-403] WHEN a syntax-check subcommand finds parse errors THEN Pyra SHALL exit with a non-zero status.
- [REQ-404] WHERE the code subcommands are registered THEN Pyra SHALL follow its existing cobra conventions of self-registering command files and shared output helpers.

### Requirement 5 — Preserve the deterministic, offline authority path

- [REQ-501] WHERE code-intelligence code is added THEN no package under `internal/canon/...` SHALL import network packages such as `net/http` or any LLM dependency.
- [REQ-502] WHEN the test suite runs THEN the existing `internal/canon` architecture test SHALL continue to pass.
- [REQ-503] WHEN `pyra gate` runs THEN its behavior and output SHALL be unchanged by the presence of code-intelligence features.
- [REQ-504] WHERE grammar or artifact provisioning requires network access THEN Pyra SHALL perform that access only outside the authority and gate path.
- [REQ-505] WHERE grammars are provisioned over the network THEN Pyra SHALL cache them so that repeated offline operation succeeds once provisioned.
- [REQ-506] WHEN a code-intelligence operation runs on identical repository state THEN Pyra SHALL produce identical results.

### Requirement 6 — Language coverage and provisioning

- [REQ-601] WHEN a file's language is supported and provisioned THEN Pyra SHALL parse it and SHALL return structural results.
- [REQ-602] IF a file's language is not provisioned THEN Pyra SHALL report the unsupported language clearly and SHALL continue operating on the supported files.
- [REQ-603] WHERE grove supports a language through its runtime grammar registry THEN Pyra SHALL define how that language is made available and SHALL document the supported set.
- [REQ-604] IF grammar provisioning uses a remote registry THEN provisioned grammars SHALL be cached locally and reusable offline.
- [REQ-605] WHERE grammars are provisioned THEN Pyra SHOULD make them integrity-checkable through a content hash or lockfile.

### Requirement 7 — Grounding Canon in real code

- [REQ-701] WHEN a Canon artifact references a code symbol by symbol-id or by a resolvable name THEN Pyra SHALL resolve that reference to the corresponding symbol's outline or source through the code-intelligence operations.
- [REQ-702] WHEN an agent supplies a symbol-id THEN Pyra SHALL provide a path to discover the Canon artifacts that reference that symbol or its file.
- [REQ-703] WHERE grounding is exposed as an MCP tool THEN it SHALL remain read-only and SHALL NOT write Canon.
- [REQ-704] IF a referenced symbol cannot be resolved because it was renamed, moved, or deleted THEN Pyra SHALL report the reference as unresolved rather than returning an incorrect match.

### Requirement 8 — Code intelligence is read-only and safe by default

- [REQ-801] WHEN any code-intelligence operation runs from the CLI or MCP THEN Pyra SHALL only read source files and SHALL NOT modify, create, or delete repository files.
- [REQ-802] WHEN Pyra walks a repository for a code operation THEN Pyra SHALL NOT traverse into gitignored or excluded paths by default.
- [REQ-803] IF an operation is given a path outside the intended working root THEN Pyra SHALL constrain traversal to the provided root and SHALL report clearly rather than escaping it.

## Success Metrics

- A single `pyra` binary answers both "what did we decide?" and "what does the code do?" with no second executable required for core use.
- `pyra serve` exposes one MCP server carrying both tool families; an agent can pass a symbol-id from a code tool into a follow-up code tool, and resolve a Canon reference to real source.
- `pyra gate` remains byte-for-byte deterministic and offline; the `internal/canon` architecture test still passes.
- The seven code operations return results equivalent to grove for the same inputs on supported languages.

## Risks

- **tree-sitter Go bindings and grammar delivery.** Native parsing depends on cgo tree-sitter bindings and per-language grammars; static linking, binary size, and cross-compilation (the Makefile cross-builds five targets) are non-trivial and may pressure the "single self-contained binary" goal. Flagged for design.
- **grove parity.** Reproducing grove's `symbol-id` scheme, `.scm` tag/locals/imports query semantics, and JSON shapes exactly enough for equivalence is a meaningful surface area to verify.
- **Authority-path purity.** Grammar fetching introduces network and cache code that must be kept strictly out of `internal/canon/...` or the architecture test (and the offline-determinism guarantee) breaks.

## Assumptions

TODO

