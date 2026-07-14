// Package changegate implements the change-aware gate: given a set of changed
// files, it reports which Accepted Canon artifacts govern each file so a
// developer or agent argues from authority instead of unknowingly contradicting
// it.
//
// It deliberately lives OUTSIDE internal/canon/... . Governance resolution needs
// both internal/store and internal/codeintel, and the authority-path invariant
// forbids any package under internal/canon from importing code intelligence, the
// tree-sitter runtime, net/http, or an LLM. Quarantining this logic here — the
// same way internal/codeintel is quarantined — keeps `pyra gate`'s corpus
// path deterministic, offline, and AI-free. A boundary test enforces that
// internal/canon never depends on this package.
//
// The evaluation is deterministic (it iterates the full Canon set in load order,
// never the fuzzy search index) and matches governance from literal references
// only (a file path or a symbol-id whose path is that file), never fuzzy or
// LLM-based matching.
package changegate
