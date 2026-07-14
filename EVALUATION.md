# First Principles Evaluation: Pyra vs. its core outcome

> **Scope of this evaluation.** Assessing Pyra against the stated core outcome:
> track established and approved requirements, evaluate updates against them, and
> give AI agents efficient reference and relationship understanding across a
> codebase. Produced 2026-07-08.

## 1. Problem Essence

**Core problem:** A team's approved requirements/decisions must (a) be captured as
durable truth, (b) constrain what changes are allowed to land, and (c) be connected
to the real code so agents reason *from* authority instead of around it — across
every session, without anyone remembering.

**Success criteria:**

- An approved requirement cannot be silently lost, malformed, or contradicted.
- A *change* that violates or drifts from authority is caught before it lands, with a citation.
- An agent, cold, can go requirement → design → decision → the exact code + test that satisfies it, cheaply.

The honest finding: **Pyra already fully solves (a) and most of (c). The stated
"core outcome" is largely the shipped mission.** The real leverage is in (b) —
*evaluating updates against Canon* — and in one specific seam of (c): the Canon↔code
link is present but thin. So this is not "what's wrong," it is "where does the
current design stop short of the outcome, and what earns its place next."

## 2. Assumptions Challenged

| Assumption (as built) | Challenge | Verdict |
|---|---|---|
| "The gate evaluates updates against Canon." | The gate (`internal/canon/gate/gate.go`) validates Canon's *internal* correctness — structure, standards, relationship integrity over the whole corpus. It never sees a **diff**, and never evaluates **code** against a requirement. "Evaluate updates against these" is only true for edits *to Canon itself*. | **Modify** — this is the central gap |
| "Grounding connects authority to implementation." | It does, but only as a **manual, read-only lookup**: a human types a `symbol-id` into prose; `code_for_artifact` regex-scans for it. Nothing *requires* a requirement to be grounded, nothing proposes the link, and drift is only found if someone runs `ground`. | **Modify** — link exists, coverage + enforcement don't |
| "Approved" is a first-class state. | Approval is implicit: an artifact has a lifecycle `status` (Accepted…), but the trust boundary is "whoever can merge the PR." Nothing binds a status transition to a defined approver or records the approval event. | **Keep, but name the limit** — fine for now, but "approved" is convention, not enforced |
| "Superseding is how truth changes." | True for Canon-to-Canon. But when *code* changes under a stable requirement, there is no evaluation that the requirement is still satisfied — only that the cited symbol still *exists* (`unresolved`). | **Modify** — existence ≠ conformance |
| Reference tier is part of the core outcome. | The three outcomes never mention ingested docs. Reference is orthogonal convenience. | **Keep, but deprioritize** — don't invest here for this goal |

## 3. Ground Truths

- Authority is only worth capturing if **a change that violates it is stopped or flagged**. Capture without enforcement-on-change is a filing cabinet.
- A requirement means nothing to an agent until it is **tied to the code that must satisfy it** — and that tie must survive code motion.
- Evaluation must run **against a delta** (a diff / a proposed change), because that is the moment a violation is cheap to catch and expensive to miss.
- The authority path must stay **deterministic, offline, AI-free** (a hard invariant, enforced by `internal/canon/archcheck_test.go` — any addition must respect it).
- The link between an authoritative artifact and a symbol must be **honest**: report "unresolved" rather than guess (already a ground truth in REQ-704).

## 4. Reasoning Chain

Ground truth *"authority must constrain change"* → the gate today only guards
Canon-internal validity → therefore the missing piece is a **change-evaluation pass**
that, given a diff, answers *"which Accepted Canon governs the symbols this diff
touches, and is any of it now unsatisfied or drifted?"* → that requires a **coverage
relationship** (requirement ⇄ symbol) that is *enforced* (a requirement with no code
link is a finding) and *maintained* (drift is a finding), not just *queryable*.

The buildable additions, ranked by leverage:

### 1. Change-aware gate (`pyra gate --diff` / hook-fed)

Today the pre-commit/CI gate re-validates the whole corpus. Add a mode that takes the
staged diff, maps changed files → symbols (using the existing `codeintel` +
`artifacts_for_symbol`), and surfaces every Accepted requirement/decision governing a
touched symbol — as an advisory citation at minimum, blocking on a configurable
policy. This is the literal implementation of outcome #2 and makes the README's
promise ("no agent silently violates one") true for **code**, not just for Canon
edits. Stays deterministic/offline — pure structural mapping, no LLM.

### 2. Grounding coverage as a gate finding

Add a validation code like `requirement-ungrounded`: an Accepted `requirement` whose
body cites *no* resolvable symbol-id is reported (advisory by default, blocking by
policy). This turns grounding from optional lookup into a **traceability guarantee**,
and is a pure function of repo state — fits beside the existing relationship-integrity
checks in `relate`/`validate`. Pair it with a `pyra coverage` /
`traceability --summary` view (mirroring `relationships --summary`): requirement →
design → decision → symbol → *test symbol*, with orphans and drift called out.

### 3. Assisted grounding (author-time, outside the authority path)

The link is only as complete as what humans type. Add a *suggestion* step —
`pyra project`/`promote` proposes candidate symbol-ids for a requirement
(name/heuristic match via `codeintel`), a human ratifies (consistent with `project`'s
existing "ratify-or-correct, never silently overwrite" contract). Keep this strictly
in the AI-allowed/CLI half; the gate still only trusts literal citations.

### 4. First-class approval (smaller, optional)

If "approved" must be provable, record the approval event (approver + ref) in
frontmatter and let the gate assert that an `Accepted` artifact carries it. Weakest-
leverage item — only worth it if audit/traceability of *who* approved is an actual
requirement.

## 5. Conclusion

**Recommended approach:** Don't add new capability surface — **close the loop that
already exists.** The pieces (typed Canon, the deterministic gate, `codeintel`,
bidirectional grounding) are all present, but the gate evaluates the *wrong thing* for
outcome #2: it checks that Canon is well-formed, not that a *change* respects it. Add
(1) a **change-aware/diff mode to the gate** and (2) **grounding-coverage as a gate
finding**, and Pyra moves from "records and validates authority" to "evaluates
every update against authority" — which is exactly outcome #2, and it hardens outcome
#3's Canon↔code seam at the same time.

**Key insight:** Pyra's grounding is currently a *lookup*, not a *contract*. The
whole system's promise ("no agent silently violates a decision") is only enforced
against edits to Canon — not against the code changes that are the actual risk. The
minimum change that fulfills the stated core outcome is to make the gate diff-aware and
to make coverage a finding.

**Trade-offs acknowledged:**

- Diff-mapping is structural (symbol-id level), so it flags *"you touched code governed
  by REQ-X"* — it cannot semantically decide *"you violated REQ-X."* That is correct
  and honest: surface + cite, let the human/agent judge. Semantic conformance would
  require an LLM and must stay out of the authority path (REQ-501).
- Coverage-as-finding adds friction to authoring (every Accepted requirement now
  "wants" a code link). Default it to advisory; let `enforcement` policy escalate —
  matches the existing type-conditional-strictness principle.
- None of this touches the Reference tier, which is right: it is orthogonal to the
  three outcomes.

## Appendix: evidence

- `internal/canon/gate/gate.go` — the gate loads the corpus, validates each artifact,
  builds/validates the relationship graph, applies enforcement policy. It operates over
  whole-corpus state; it takes no diff and reads no source code.
- `internal/retrieval/retrieval.go` — discover → ground → assemble; grounding here
  resolves superseded → successor, not code.
- `internal/mcp/codeintel.go`, `internal/cli/codeintel.go` — grounding (`code_for_artifact`,
  `artifacts_for_symbol`, `pyra ground`) regex-scans an artifact body for literal
  symbol-ids; read-only lookup, no coverage requirement, no author-time assistance.
- `canon/code-intelligence/requirements.md` — REQ-701..704 define grounding as
  resolve/lookup and mandate honest `unresolved` reporting; REQ-501/503 pin the
  deterministic, offline, AI-free authority path any addition must preserve.
