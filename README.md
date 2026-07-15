<p align="center">
<img width="1280" height="640" alt="pyra-banner" src="https://github.com/user-attachments/assets/395821b6-f392-4a89-b6b9-256b356af405" />
</p>

> **Ensure your codebases stand the test of time, just like the Great Pyramid of Giza.**

# Pyra

**Pyra is an enforceable authority layer for AI coding agents.** It is a single Go binary that turns the decisions, requirements, and designs your team agrees to into **'Canon'**, which is: typed Markdown artifacts, validated against real standards, wired into a blocking **gate** system, and served to agents over **MCP**. No decision goes unrecorded, and no agent silently violates one.

The problem Pyra solves is simple to state and expensive to ignore: an AI agent's only real constraint is its context window, and most "memory" tools conflate two very different properties. **Authority** asks whether something is the canonical truth the team agreed to. **Discoverability** asks whether the right piece can be found at the right moment. Vector stores optimize discoverability and have no concept of authority. Pyra makes authority a first-class, *enforced* property: Canon artifacts are typed, their relationships are integrity-checked, and a deterministic gate rejects malformed or conflicting authority before it lands, with no LLM and no network in that path.

Pyra is built directly for **spec-driven development**. The specs your workflow already produces (`requirements.md`, `design.md`) become typed Canon with one command, and the agents that read them are held to that Canon automatically, whether you drive the `/spec → /dev → /code-review` skills in Claude Code or Kiro, or any MCP client.

**Memory is Canon. Context is the *budgeted projection* of Canon (and optional Reference). AI lives only in the projection. The substrate is Git.**

The Canon authority model conforms to the concept of **Requirements-as-Code (RaC)**.

Pyra also speaks **code**. The same binary and the same MCP server expose **structural code intelligence** — byte-precise, token-cheap search and navigation over your source via tree-sitter (`outline`, `symbols`, go-to-`definition`, `callers`, and more) so one agent using one server can answer both *what the team decided* (Canon) and *what the code actually does*. The two connect: a Canon artifact can **ground** in real code, resolving an authoritative decision to the exact symbols it governs, and a symbol back to the decisions that constrain it. Authoritative decisions and real code search are centralized into a single binary and MCP server.

---

## Why Pyra? - A Common Use Case

You make an architectural decision — "use Bleve for search" — while working a feature through the `/spec` skill. You don't file it anywhere by hand: when you approve the design, the skill records it as Canon and gates it for you.

```text
/spec   → approve design.md (it chooses Bleve)
        → skill projects it into typed Canon and runs the gate   → the decision lands as authority
```

Two weeks later, in a brand-new session with no memory of that conversation, an agent proposes ripping out Bleve for a vector database. Because Pyra is wired into the workflow through the same skills:

- `/dev` **grounds on Canon over MCP** first. `find_decisions` surfaces the "Use Bleve for search" artifact with status **Accepted** and its consequences, so the agent argues *from* the decision instead of around it.
- If a change still lands that contradicts Accepted Canon, the **gate blocks it** at commit time (git hook), inside `/code-review`, and in CI (`gate --sarif`), citing the exact artifact and rule.
- When the decision genuinely *should* change, you mint a successor that `## Supersedes` the old one. Pyra follows the supersede chain so agents always see the current truth, never a stale one.

That is the whole point: **the decision is recorded once and respected thereafter**, by people and agents alike, without anyone having to remember it.

---

## The core model

Pyra gives an agent two kinds of knowledge over one substrate, plain Markdown plus YAML frontmatter, versioned in Git:

| | **Canon** (authority) | **Reference** (recall, optional) |
|---|---|---|
| Answers | **What is true**: what the team decided and what must hold | **How things work**: supporting documentation |
| Content | Requirements, decisions, designs, roadmaps, prompts | Ingested docs (crawled sites, imported repos) |
| Created by | `pyra new` / `pyra project` / `pyra promote` | `pyra crawl` / `pyra import` |
| Validation | Typed, standards-checked, relationship-integrity-checked, **gated in CI** | Permissive, abundant and searchable |
| Determinism | Pure function of repo state, **no LLM, no network** | AI may summarize and rank in the discovery layer |

A **store** is one directory in Git holding both tiers. The only thing separating them is `canon_roots` in `.okf/config.yaml`: files under those roots are Canon, and everything else is Reference. Canon is the hero of Pyra. Reference is an optional convenience for teams that also want a large docs corpus searchable as agent memory (see the [Appendix](#appendix-reference-tier-and-okf-format)).

### The five Canon artifact types

Each is typed Markdown with required sections, a minted opaque ID (`<repository-key>-<12-char Crockford base32>`, for example `OKF-KTQ63DPSMF19`), a lifecycle status, and typed relationships (`## Related <Type>`, `## Supersedes`).

| Type | Captures | Key required sections |
|---|---|---|
| `requirement` | What must hold | `## Problem`, `## Requirements` (`[REQ-NNN] … SHALL …`) |
| `decision` | A choice and its rationale (ADR) | `## Context`, `## Decision`, `## Consequences` |
| `design` | How something is built | `## Context`, `## User Need`, `## Design`, `## Constraints` |
| `roadmap` | Intended outcomes over time | `## Outcomes`, `## Initiatives` |
| `prompt` | A reusable, versioned prompt | `## Objective`, `## Input`, `## Instructions`, `## Output` |

### The gate

`pyra gate` is the enforcement mechanism and the heart of the authority model. It loads the corpus, validates every artifact, checks relationship integrity, applies your enforcement policy, and exits non-zero on any blocking finding, emitting SARIF for required-checks. It is **deterministic and offline** (a build-failing test forbids `net/http` or any LLM dependency in the authority path), so it is safe in pre-commit hooks and CI. Validation includes:

> **Change-aware mode (`--diff`).** By default the gate checks whether *Canon* is well-formed. Add `--diff` (staged diff), `--since <ref>`, or `--changed <files>` and the gate additionally reports which **Accepted Canon artifacts govern each changed file** — an artifact governs a file when its prose cites that file path or a symbol-id in it — so a change that touches governed code is surfaced with a citation, and drift (a cited symbol that no longer resolves) is flagged. These findings are **advisory by default**; escalate them to blocking via the enforcement rule codes `canon-governed-change` and `governed-symbol-unresolved`. The mode reuses the same policy, exit code, and SARIF output, and stays deterministic and offline (the mapping is purely structural — it lives in `internal/changegate`, outside the authority path).

> **Change-risk (`--risk`).** Add `--risk` (or run `pyra risk`) and the gate also scores the change itself for **defect risk** from the shape of the diff (Kamei just-in-time metrics: lines ±, files, directories, subsystems, diffusion entropy, author experience). The headline is **repo-relative** — `Below typical` / `Typical` / `Elevated` plus a percentile against the repo's *own* recent commits (the portable signal; the raw 0–10 is shown only as a secondary, uncalibrated number). It emits PR **directives** as advisory findings: `risk-missing-tests` (changed source with no test in the diff), `risk-missing-cochanges` (files that usually change together but were omitted — hidden coupling, minus structural import links), `risk-will-break` (structural dependents of changed symbols), and `risk-governance` (the change touches Accepted Canon). All merge into the same result, exit code, JSON, and SARIF. Deterministic and offline — no LLM, no network — in `internal/changerisk` (with a minimal git-history substrate in `internal/gitint`), both outside the authority path. The scoring model is a **faithful port of [repowise](https://github.com/repowise-dev/repowise)'s change-risk model** (AGPL-3.0), isolated to one swappable file and pinned by a parity test; see [`docs/REPOWISE_PARITY.md`](docs/REPOWISE_PARITY.md).

- **BCP-14 / RFC 8174**: only ALL-CAPS `MUST`/`SHALL`/`SHOULD` carry normative weight.
- **ISO/IEC/IEEE 29148**: requirements should be singular and testable.
- **EARS**: Easy Approach to Requirements Syntax conformance.
- **Relationship integrity**: no dangling, ambiguous, miscast, or cyclic references, and live artifacts don't depend on retired ones (except via `## Supersedes`).

---

## Code intelligence

Beyond memory, Pyra reads code. The same `pyra` binary and MCP server expose seven **read-only, structural** operations built on a pure-Go tree-sitter runtime — no cgo, grammars embedded, fully offline:

| Operation | Answers |
|---|---|
| `outline <file>` | The file's definitions as a compact skeleton (kind, name, parent, signature, id) — read this instead of the whole file. |
| `symbols <dir>` | Where a name is defined across a tree (gitignore-aware); exact or substring. |
| `source <symbol-id>` | The exact source of one symbol — one symbol at a time, by bytes. |
| `check <file>` | Syntax errors (ERROR/MISSING nodes); exits non-zero when any exist. |
| `callers <name>` | Every reference to a name, tagged **structural** (parsed) or **textual** (whole-word). |
| `map <dir>` | A directory's dependency graph: each definition and its outgoing references, no bodies. |
| `definition <name> \| --at file:line:col` | Go-to-definition; the `--at` form is scope-aware and follows imports across files. |

Every result carries a stable **symbol-id** — `<lang>:<relpath>#<name>@<line>` (1-based) — that an agent passes from one call to the next. This is deliberately *not* an LSP: it is purely syntactic, token-cheap, and offline, and it replaces an agent's habit of grepping and reading whole files with one symbol at a time, by exact bytes. It never mutates source, honors `.gitignore`, and confines traversal to the working root.

Supported languages by default: **Go, Python, JavaScript, TypeScript/TSX, Java, Rust** to keep the binary size small.

Additional languages require building from source (see Instalation below).

### How Canon maps to code (grounding)

Authority and code meet through the symbol-id. **A Canon artifact names the code it governs simply by mentioning a symbol-id in its prose** — the same way relationships between artifacts are inferred from literal `OKF-…` IDs, never fuzzy matching. Two read-only tools (and `pyra ground`) walk that bridge in both directions:

- **Artifact → code** (`code_for_artifact`): given a Canon artifact ID, resolve every symbol-id in its body to the current source, and report any that no longer resolve (renamed, moved, or deleted) rather than returning an incorrect match.
- **Code → artifacts** (`artifacts_for_symbol`): given a symbol-id or file path, find the Canon decisions, requirements, and designs that reference it.

```text
decision  OKF-…  "cache documents in memory"   ──cites──►   go:internal/cache/store.go#Put@42   (real source)
symbol    go:internal/cache/store.go#Put@42     ──whose?──►   OKF-…  the decisions & requirements that govern it
```

So a decision can point at the exact function that implements it; an agent can ask *"which decisions govern this symbol?"* before changing it; and a reviewer can see when an artifact's cited code has drifted out from under it. Authority and implementation stay tied together — **Canon is the memory, the symbol-id is the anchor into live code, and both live in one binary.**

---

## Installation

### Download a binary

Download the latest binary for your platform from the [releases page](https://github.com/chasedputnam/pyra/releases).

### Build from source

```bash
go install github.com/chasedputnam/pyra/cmd/pyra@latest
```

Or clone and build (Go 1.25+):

```bash
git clone https://github.com/chasedputnam/pyra.git
cd pyra
make build
```

### Additional Language Grammar Inclusions

The binary is **pure Go, no cgo**, and cross-compiles to every target with plain `go build`; code intelligence uses a pure-Go tree-sitter runtime to keep it that way. `make build` embeds only the grammars for the supported languages (via `grammar_subset` build tags) for a lean binary; a plain `go install`/`go build` without those tags embeds the runtime's full grammar set and produces a larger binary.

> **Apple Intelligence (optional, Reference summaries only):** on macOS 26 Tahoe with Apple Silicon, Pyra can summarize Reference docs through Apple's on-device Foundation Models via the opt-in `applefm` build tag. See [docs/APPLE_INTELLIGENCE.md](docs/APPLE_INTELLIGENCE.md). This never touches the Canon authority path.

---

## Quick start

Pyra is meant to sit underneath the spec, development, and review workflows your agents already use. `pyra init` writes repository-local instructions that map those activities to the right Pyra CLI and MCP operations, plus local MCP configuration for each selected tool.

### One-time setup

```bash
# Initialize a repository and enable one or more local agent integrations.
# Supported IDs: claude, codex, opencode, pi, kiro.
mkdir my-store && cd my-store && git init
pyra init . --agent claude --agent codex
```

The repeatable `--agent` flag configures any combination of Claude Code, Codex, OpenCode, Pi, and Kiro. Initialization always creates or updates a marker-delimited Pyra section in the repository's `AGENTS.md`; existing instructions remain intact. It also installs Git gate hooks when `.git` exists and the applicable repository-local Claude, Codex, or Kiro gate hooks. List the accepted identifiers without writing anything with:

```bash
pyra init --list-agents
```

To add or refresh agent integrations on an existing repository without changing
`.okf/config.yaml`, Canon directories, or hooks, select one or more tools with:

```bash
pyra init . --agents-only --agent codex --agent opencode
```

Agent-only mode updates the managed `AGENTS.md` section and only the selected
repository-local MCP client files. It does not require `--force`.

The generated MCP commands use the repository's absolute path, so clients do not depend on their launch directory. If the repository moves, rerun `pyra init . --force` with the same `--agent` flags. Codex and Pi may ask you to trust the project; Pi may also ask permission to install its project-scoped MCP adapter. Restart a client after changing its configuration.

> **Installed the binary only (`go install`)?** That is sufficient. Repository instructions, MCP setup, and hooks are generated by the `pyra` binary; no global skill copying or source checkout is required.

### The everyday loop

```text
spec or design work       → requirements.md / design.md  (pyra project + gate)
implementation work       → ground in Canon over MCP     (find_decisions / get_context, then rebuild)
code review or equivalent → check against authority      (pyra gate --sarif + relationships --validate)
```

The generated `AGENTS.md` expresses this mapping by activity rather than requiring particular skill names, so equivalent requirements, design, implementation, and review workflows use Pyra consistently in every supported agent.

### Authoring Canon directly (optional)

When you want to record a decision outside a spec, author it by hand and gate it:

```bash
pyra new decision canon/adr-001-use-bleve.md --title "Use Bleve for search"
$EDITOR canon/adr-001-use-bleve.md      # fill ## Status (Accepted), ## Decision, ## Consequences
pyra gate .                          # blocks on any structural, standards, or integrity failure
```

Then point your MCP client at the **store root**:

```json
{
  "mcpServers": {
    "my-store": {
      "command": "pyra",
      "args": ["serve", "/abs/path/to/my-store", "--mcp"]
    }
  }
}
```

The generated `.okf/config.yaml` is self-documenting:

```yaml
# Repository key: prefix for minted Canon artifact IDs (e.g. OKF-3F8A...).
repository_key: OKF

# Canon roots: directories that hold the authoritative tier. Everything else
# under the store is treated as Reference. Files here are validated by `pyra gate`.
canon_roots:
  - canon

# Spec roots: directories scanned for spec documents (requirements.md,
# design.md) that `pyra project` turns into typed Canon. Covers the local
# specs/ layout and Kiro's .kiro/specs/ layout by default.
spec_roots:
  - specs
  - .kiro/specs

# Code roots: directories that structural code-intelligence operations
# (outline, symbols, map, ...) search by default when no path is given.
code_roots:
  - .

# Ticketing provider: format-lints external "## Related Tickets" links.
# One of: github, jira, linear, azure-devops, servicenow, none.
ticketing:
  provider: github

# Enforcement: reclassify gate findings by rule code. Empty = each rule keeps
# its default severity. Uncomment and list rule codes to override.
enforcement: {}
```

---

## Spec-driven development with Pyra

Pyra is the authoritative memory beneath your spec-driven workflow. The specs your agent already writes become Canon, the gate keeps that Canon honest, and MCP feeds it back to the agent on every task. The same flow works whether you drive **Claude Code** or **Kiro** — both run the `/spec → /dev → /code-review` skills and emit the same `requirements.md` / `design.md` contract, so one projector serves both.

The recommended way to run this loop is through the bundled skills (see [Quick start](#quick-start)) — they invoke the commands below for you at each phase. The raw commands are documented here so you can run them directly or adapt them to another toolchain.

### The lifecycle, end to end

These are the commands the `/spec`, `/dev`, and `/code-review` skills run on your behalf:

```bash
# 0. Once per repo
pyra init . --agent <tool>               # local MCP + instructions + applicable hooks

# 1. Requirements: /spec writes specs/<feature>/requirements.md, then projects it into
#    typed Canon (mints a stable ID, fills sections, infers relationships):
pyra project specs/<feature>/requirements.md
#    Or project the whole spec directory at once (skips tasks.md):
pyra project specs/<feature>/

# 2. Design: /spec produces design.md; it projects that to a design artifact:
pyra project specs/<feature>/design.md

# 3. Development: /dev grounds on Canon over MCP before writing code.
#    find_decisions / get_artifact / get_context return the authoritative requirements
#    and decisions the task must honor (Canon-first, with citations and live status).

# 4. Code review: /code-review runs the gate as a required check and cites what changed:
pyra gate . --sarif > pyra.sarif
pyra gate . --diff                  # change-aware: which Accepted Canon governs the staged diff
pyra relationships . --summary
```

`pyra project` is **ratify-or-correct**: it never rewords your prose or silently overwrites. A new artifact is created, an existing one is only changed with `--write` (or interactive confirmation), and `--dry-run` previews the diff. Re-projecting reuses the artifact's ID, so identity is stable across iterations. Relationships are inferred only from **literal** `OKF-…` and alias references in the prose, for high precision and never fuzzy matching.

### Bootstrap from existing docs

If a decision already lives in ingested Reference, graduate it into Canon instead of retyping it:

```bash
pyra promote <concept-id-or-path> --type decision
```

### Integrating into your agent's skills

Pyra is designed to disappear into your workflow. Each phase of spec-driven development emits authoritative memory as a natural byproduct of the work the agent is already doing: requirements become Canon the moment they're approved, decisions are captured as they're made, and the gate enforces all of it continuously. Drop these commands into the skill definitions you already use, and the loop runs itself. Every spec strengthens the memory, every task is grounded in it, and every review is checked against it.

**Ready-to-use skill examples ship in this repo.** [`examples/agents/skills/`](examples/agents/skills/) contains three tool-neutral reference workflows that demonstrate the same activity mapping generated into `AGENTS.md`:

| Skill | Phase | Pyra integration |
|---|---|---|
| [`spec`](examples/agents/skills/spec/SKILL.md) | Requirements → Design → Tasks | Projects each approved `requirements.md` / `design.md` into typed Canon and gates it. |
| [`dev`](examples/agents/skills/dev/SKILL.md) | Implementation | Grounds the work in Canon over MCP (`find_decisions` / `get_artifact` / `get_context`) before writing code; rebuilds indexes after status changes. |
| [`code-review`](examples/agents/skills/code-review/SKILL.md) | Review | Runs `pyra gate --sarif` as a required authority check and cites touched artifacts via `pyra relationships --summary`. |

Every Pyra step in these examples is guarded by an "if this is a Pyra store" check. You do not need to copy them globally: initialize each repository for the tools that will work in it:

```bash
pyra init . --agent claude --agent codex --agent opencode --agent pi --agent kiro
```

The commands each example workflow runs are summarized below.

**Claude Code**:

```bash
# /spec: at each approval gate, project the just-approved doc into Canon and enforce it
pyra project "specs/${FEATURE}/requirements.md"
pyra gate .            # block approval on a failing gate

# /dev: ground the implementation in authority before writing code (MCP, already running):
#   find_decisions("<area>"), get_artifact("OKF-..."), get_context("<task>")
pyra rebuild .         # refresh derived indexes after status changes

# /code-review: make the gate a required check and cite touched authority
pyra gate . --sarif > pyra.sarif
pyra relationships . --summary
```

Install the on-write hook so the gate runs inside the agent loop:

```bash
pyra hooks install --claude     # PostToolUse hook → pyra gate after Write/Edit
```

**Kiro** (specs in `.kiro/specs/`, hooks in `.kiro/hooks/` for the IDE and `.kiro/agents/*.json` for the CLI):

Kiro can follow the same generated repository instructions. The projector and hooks use Kiro's layout:

```bash
pyra project .kiro/specs/${FEATURE}/    # same projector, Kiro layout
pyra hooks install --kiro               # writes the Kiro IDE + CLI gate hooks
```

**Any MCP client** (no skills required): run `pyra serve . --mcp`, point the client at the store root, and the agent gets the authority tools (`find_decisions`, `get_artifact`, `get_context`, and the rest) directly.

The result is a compounding system. The more your team specs, decides, and ships, the richer and more authoritative the agent's memory becomes, while the gate guarantees it never drifts from what the team actually agreed to.

---

## Commands

In rough order of use. Store-scoped commands default to the current directory (`.`).

### Store setup

| Command | Purpose |
|---|---|
| `pyra init [path]` | Scaffold a store, update repository-local `AGENTS.md`, configure selected MCP clients, and install applicable local hooks. `--agents-only` updates only `AGENTS.md` and selected MCP client files, skipping store configuration, Canon directories, and hooks. Flags: `--agent` (repeatable: `claude`, `codex`, `opencode`, `pi`, `kiro`), `--agents-only`, `--kiro-agent` (select among multiple Kiro CLI agents), `--list-agents`, `--repository-key`, `--canon-root` (repeatable), `--ticketing`, `--force`, `--quiet`. |

### Authoring Canon

| Command | Purpose |
|---|---|
| `pyra new <type> <path>` | Scaffold a typed artifact with a minted ID and the type's sections. Flags: `--store`, `--title`. |
| `pyra project <spec-doc-or-dir>` | Project an approved `requirements.md`/`design.md` (local `specs/` or Kiro `.kiro/specs/`) into typed Canon: reuse or mint a stable ID, fill sections from the prose, infer literal relationships, validate. Flags: `--store`, `--type`, `--dry-run`, `--write`/`--force`, `--kiro-agent`, `--json`, `--quiet`. |
| `pyra promote <concept-id-or-path>` | Graduate an ingested Reference concept into a typed Canon draft. Flags: `--store`, `--type`, `--out`. |

### Authority

| Command | Purpose |
|---|---|
| `pyra gate [store]` | Run the unified authority gate (validate + relationships + policy). Exits non-zero on any blocking finding. Flags: `--json`, `--sarif`; change-aware: `--diff` (staged), `--since <ref>`, `--changed <a,b>`; change-risk: `--risk`. |
| `pyra risk [commit \| base..head]` | Score a change for defect risk (repo-relative ranking + PR directives). No arg scores the staged diff. Flags: `--store`, `--baseline`, `--ext`, `--json`. |
| `pyra relationships [store]` | Report and validate the typed relationship graph. Flags: `--validate`, `--summary`, `--json`. |

### Code intelligence

Read-only structural search and navigation over source. Every command takes `--json`; results carry stable symbol-ids.

| Command | Purpose |
|---|---|
| `pyra outline <file>` | List a file's definitions as a skeleton. Flags: `--kind`, `--detail` (0/1/2), `--json`. |
| `pyra symbols <dir>` | Find symbols across a directory. Flags: `--name`, `--name-contains`, `--kind`, `--refs`, `--json`. |
| `pyra source <symbol-id>` | Print one symbol's source (or `--file` + `--name`). Flag: `--json`. |
| `pyra check <file>` | Report syntax errors; exits non-zero if any. Flag: `--json`. |
| `pyra callers <name>` | Find references, tagged `[S]`tructural / `[T]`extual. Flags: `--dir`, `--json`. |
| `pyra map <dir>` | Directory dependency graph. Flags: `--kind`, `--name`, `--name-contains`, `--json`. |
| `pyra definition [name]` | Go-to-definition by name or `--at file:line:col`. Flags: `--at`, `--dir`, `--json`. |
| `pyra ground <artifact-id \| symbol-id>` | Bridge Canon and code: resolve an artifact's cited symbols, or find artifacts that cite a symbol. Flags: `--store`, `--json`. |

### Git intelligence

Behavioral signals mined from git history — deterministic, offline, no LLM, anchored to HEAD's commit time so identical repository state yields identical output.

| Command | Purpose |
|---|---|
| `pyra hotspots [store]` | Rank files by temporally-decayed churn (top-quartile hotspots that clear activity floors), with churn %, commits, owner, and bus factor. Flags: `--limit`, `--window`, `--json`. |
| `pyra ownership [path]` | Ownership %, recent owner, contributor count, and bus factor for a file; module (top-level directory) rollups for a directory or no path. Flags: `--store`, `--window`, `--json`. |

Metrics: per-file commit windows, line churn, age, temporal hotspot score (180-day half-life), primary/recent owner, contributor count, bus factor (min authors reaching 80% of commits), co-change partners; repo-level hotspot ranking; and per-module rollups. Ownership is by commit-author distribution (`git blame` line-ownership and the churn×complexity hotspot refinement arrive with code health). See [`docs/REPOWISE_PARITY.md`](docs/REPOWISE_PARITY.md).

### Code graph

A persistent two-tier (file + symbol) dependency graph built from code intelligence — deterministic, offline, no LLM, no external graph library.

| Command | Purpose |
|---|---|
| `pyra graph [store]` | Query the code dependency graph. Subviews: `--centrality` (PageRank hubs), `--communities` (logical modules via label propagation), `--cycles` (dependency cycles / SCCs), `--reachability` (reachable-from-entry-points vs. the unreachable rest). Flags: `--limit`, `--scope`, `--node-cap`, `--json`. |

Symbol nodes (keyed by symbol-id) + file nodes; reference edges resolved by the same honest name-matching `callers` uses (edge to every matching definition on a name collision, never a guessed target). Analyses are standard and self-contained — PageRank, deterministic label propagation, Tarjan SCC, and entry-point reachability (whose unreachable set feeds dead-code detection). Betweenness centrality and Leiden are intentionally deferred.

| Command | Purpose |
|---|---|
| `pyra dead-code [store]` | Report likely-unreachable definitions (no path from entry points in the graph), ranked by cleanup impact, each with a confidence tier — **high** (no textual references, safe to delete), **medium** (has textual/possibly-dynamic references), **low** (a test-file helper) — and a `[governed]` marker when Accepted Canon still cites the now-unreachable code (drift). Flags: `--tier`, `--limit`, `--json`. |

Dead-code **consumes** the graph's reachability (no new analysis); it excludes `Test*`/`main` entry points and, because pyra has no framework route→handler edges, uses the conservative high tier to avoid flagging route-only handlers (which are usually exported, hence reachable). No auto-removal.

### Code health

Per-file **health scoring** across three signals — **defect risk · maintainability · performance** — from a deterministic biomarker roster over code structure, git history, the dependency graph, and Canon governance. Zero LLM, offline.

| Command | Purpose |
|---|---|
| `pyra health [store]` | Repo KPIs (NLOC-weighted average + hotspot health, worst performer) and the lowest-scoring files with their three dimension scores and top marker. Flags: `--file <path>` (per-file findings + impacts + refactoring suggestion), `--coverage <report>` (ingest LCOV/Cobertura), `--limit`, `--json`. |

The roster (~28 markers) spans **structural** (cyclomatic complexity, nesting, `god_class`/`god_file`, LCOM4 cohesion, …), **organizational** (churn, change entropy, co-change scatter, ownership risk, prior defects, …), **duplication** (Rabin–Karp clone detection), **coverage** (when a report is supplied), and pyra-unique **governance** markers — `ungoverned_hotspot` (a churn hotspot with no governing Canon), `stale_governance`, and `contradictory_decision`. Each file starts at 10 and biomarkers deduct under per-category caps; the scoring kernel is a **faithful port of [repowise](https://github.com/repowise-dev/repowise)'s** (AGPL-3.0), isolated to one swappable file and pinned by a parity test. The **performance dimension ships present-but-empty** (its loop/dataflow detectors are a follow-up); see [`docs/REPOWISE_PARITY.md`](docs/REPOWISE_PARITY.md).

### Automation (event hooks)

| Command | Purpose |
|---|---|
| `pyra hooks install` | Install hooks that run the gate automatically. git is always installed (`pre-commit` runs the blocking gate, `post-merge` runs the integrity guard), and agent targets are auto-detected. Target flags: `--git`, `--claude`, `--codex`, `--kiro-ide`, `--kiro-cli`, `--kiro`, `--all`, plus `--kiro-agent`, `--store`. |
| `pyra hooks uninstall` | Remove only Pyra-managed hook content, leaving other hooks intact. |
| `pyra hooks status` | Show which Pyra hooks are installed per target. |

Surfaces written: git (`.git/hooks/`), Claude Code (`.claude/settings.json` PostToolUse), Codex (`.codex/hooks.json` PostToolUse), Kiro IDE (`.kiro/hooks/pyra-gate.json`), and Kiro CLI (`.kiro/agents/<agent>.json` under `hooks.postToolUse`). Every install is marker-delimited and idempotent.

### Operating the store

| Command | Purpose |
|---|---|
| `pyra rebuild [store]` | Regenerate derived indexes (full-text search + relationship graph) from the Markdown source of truth. |
| `pyra serve <store>` | Serve the store over MCP. Flags: `--mcp` (default), `--name`, `--max-result-chars`. |
| `pyra export [store]` | Export Reference knowledge for scale-out (documents/graph). |
| `pyra demo` | Run an offline demo with a bundled example. |

### Optional: Reference ingestion (secondary)

For teams that also want a large docs corpus searchable as agent memory. These populate the Reference tier and never touch Canon.

| Command | Purpose |
|---|---|
| `pyra crawl <url>` | Crawl a documentation website into an OKF bundle. |
| `pyra import <path>` | Import local Markdown into an OKF bundle. |
| `pyra update <bundle>` | Update an existing bundle from its source. |
| `pyra validate <bundle>` | Validate an OKF bundle. |
| `pyra inspect <bundle>` | Inspect a bundle and show statistics. |

---

## MCP tools

`pyra serve <store> --mcp` exposes the store to any MCP client. Tools are grouped by job.

### Authority (Canon)

| Tool | Returns |
|---|---|
| `find_decisions` | Canon artifacts matching a query, authority-first, with citations and lifecycle status. |
| `get_artifact` | A specific artifact by ID (resolving `## Supersedes` to the current successor). |
| `get_context` | A budgeted, Canon-first context pack for a task, with normative requirement text preserved verbatim. |
| `get_related` | Typed relationships of an artifact (related requirements, decisions, and so on). |
| `get_neighbors` | The relationship neighborhood of an artifact within N hops. |

### Code intelligence

| Tool | Returns |
|---|---|
| `outline` | A file's definitions as a skeleton, with stable symbol-ids. |
| `symbols` | Symbols matching a name/kind across a directory (gitignore-aware). |
| `source` | The exact source of one symbol (by id, or file + name). |
| `check` | Syntax errors (ERROR/MISSING nodes) in a file. |
| `callers` | References to a name, tagged structural vs textual. |
| `map` | A directory's dependency graph (definitions + outgoing references). |
| `definition` | Go-to-definition by name, or scope-aware/cross-file by position. |

### Grounding (Canon ↔ code)

| Tool | Returns |
|---|---|
| `code_for_artifact` | The current source for every symbol-id a Canon artifact cites; lists any that no longer resolve. |
| `artifacts_for_symbol` | The Canon artifacts that reference a given symbol-id or file. |

### Git intelligence

| Tool | Returns |
|---|---|
| `get_hotspots` | The repository's churn hotspots, ranked, with churn %, commit counts, owner, and bus factor. |
| `get_ownership` | Ownership / bus factor for a file, or the top-level module rollups for a directory or empty path. |

### Code graph

| Tool | Returns |
|---|---|
| `get_graph_centrality` | Symbols ranked by PageRank over the dependency graph (the hubs). |
| `get_communities` | The graph partitioned into communities (logical modules). |
| `get_cycles` | Dependency cycles (strongly-connected components). |
| `get_dead_code` | Likely-unreachable definitions ranked by cleanup impact, with confidence tier and governed flag. |

### Code health

| Tool | Returns |
|---|---|
| `get_health` | Repo health KPIs and the lowest-scoring files (defect / maintainability / performance) with top marker and refactoring suggestion. |

### Recall (Reference)

| Tool | Returns |
|---|---|
| `search_concepts` | Full-text search across the Reference tier. |
| `read_concept` | A single concept's full content. |
| `get_summary` | A concept's summary callout. |
| `list_types` / `list_tags` | The vocabulary present in the store. |

### Live updates and utility

`check_updates`, `apply_updates` (use `dry_run: true` to preview), `bundle_health`, `bundle_summary`, `compression_stats`.

---

## Configuration: `.okf/config.yaml`

| Key | Meaning |
|---|---|
| `repository_key` | Prefix for minted Canon IDs (for example `OKF`). |
| `canon_roots` | Directories that hold the authority tier; everything else is Reference. |
| `spec_roots` | Directories `pyra project` scans for spec docs. Default: `["specs", ".kiro/specs"]`. |
| `code_roots` | Directories code-intelligence operations search by default when no path is given. Default: `["."]`. |
| `ticketing.provider` | Format-lints `## Related Tickets` links. One of `github`, `jira`, `linear`, `azure-devops`, `servicenow`, `none`. |
| `enforcement` | Reclassify gate findings by rule code into `blocking` / `advisory` / `disabled`. Empty means each rule keeps its default severity. |

`config.yaml` is the only thing that separates the tiers, and the rendered output round-trips through load, so you can edit it by hand or regenerate it with `pyra init --force`.

---

## Appendix: Reference tier and OKF format

The Reference tier is optional supporting material (abundant, summarized, searchable) rendered as an **Open Knowledge Format** (OKF) bundle: human- and agent-readable Markdown with YAML frontmatter, exchangeable without a central registry ([What is OKF?cccccbhdfjdlr](https://openknowledgeformat.com/what-is-okf)). It is the right tool when you want a large docs corpus usable as agent memory without standing up a vector store.

### Retrofit an existing repository (Reference-only)

```bash
# Import a repo's Markdown into a self-contained bundle and serve it directly
pyra import ~/repo/my-project --out ~/repo/my-project/.okf --source-name "My Project"
pyra serve ~/repo/my-project/.okf --mcp
```

With no `canon_roots` populated, the bundle stays pure Reference and behaves like a standalone searchable knowledge base. `pyra promote` is the bridge when a Reference concept matures into a decision worth enforcing as Canon.

### Concept format

Each Reference concept is a Markdown file with frontmatter:

```yaml
---
type: "Guide"
title: "Getting Started"
description: "How to get started"
resource: "https://example.com/docs/getting-started"
tags: ["setup", "onboarding"]
timestamp: "2024-01-01T00:00:00.000Z"
---
```

`type` and `title` are required; `description`, `resource`, `tags`, and `timestamp` are optional. An `index.md` provides summary-first navigation and backlinks across the bundle.

### Summarization

Reference summaries can be generated by fast **extractive** algorithms (offline, deterministic) or, optionally, an **LLM** mode via an external OpenAI-compatible endpoint or a local Ollama fallback. Summarization applies only to Reference and never participates in the Canon authority path.

### Scale ceiling

Summary-first navigation works well up to roughly **100 concepts / ~400K tokens**. Past that, graduate the fuzzy half to an external RAG system via `pyra export`, while Canon always stays canonical in the repo.

## License

Pyra is released under the [Business Source License 1.1](./LICENSE) (`BUSL-1.1`), converting to the
**Apache License, Version 2.0** four years after each version's release. BSL is source-available, **not**
an OSI-approved "open source" license; please describe it as *source-available* until the Change Date.

> **Distribution note:** Pyra's change-risk constants are currently ported from
> [repowise](https://github.com/repowise-dev/repowise) (AGPL-3.0). See [`NOTICE`](./NOTICE) — that ported
> material must be replaced with an independent implementation before Pyra can be distributed under
> the BSL. The LICENSE states the intended terms; it does not authorize distributing the current tree.
