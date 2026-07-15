// Package agents plans repository-local coding-agent setup for Pyra.
package agents

import (
	"fmt"
	"strings"
)

// ID is a stable command-line identifier for a supported coding agent.
type ID string

const (
	Claude   ID = "claude"
	Codex    ID = "codex"
	OpenCode ID = "opencode"
	Pi       ID = "pi"
	Kiro     ID = "kiro"
)

// Definition describes one supported coding agent.
type Definition struct {
	ID   ID
	Name string
}

var definitions = []Definition{
	{ID: Claude, Name: "Claude Code"},
	{ID: Codex, Name: "Codex"},
	{ID: OpenCode, Name: "OpenCode"},
	{ID: Pi, Name: "Pi"},
	{ID: Kiro, Name: "Kiro"},
}

// Definitions returns the supported agents in stable display order.
func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

// ParseIDs validates, deduplicates, and returns agent IDs in stable order.
func ParseIDs(values []string) ([]ID, error) {
	known := make(map[ID]bool, len(definitions))
	for _, d := range definitions {
		known[d.ID] = true
	}
	want := map[ID]bool{}
	for _, raw := range values {
		id := ID(strings.TrimSpace(raw))
		if !known[id] {
			return nil, fmt.Errorf("unsupported agent %q; supported: %s", raw, supportedIDs())
		}
		want[id] = true
	}
	out := make([]ID, 0, len(want))
	for _, d := range definitions {
		if want[d.ID] {
			out = append(out, d.ID)
		}
	}
	return out, nil
}

func supportedIDs() string {
	ids := make([]string, 0, len(definitions))
	for _, d := range definitions {
		ids = append(ids, string(d.ID))
	}
	return strings.Join(ids, ", ")
}

const (
	agentsBegin = "<!-- >>> pyra managed instructions >>> -->"
	agentsEnd   = "<!-- <<< pyra managed instructions <<< -->"
)

const managedInstructions = `## Pyra authority workflow (managed)

Apply these rules to equivalent specification, planning, implementation, and review work, regardless of skill, command, prompt, or workflow name.

### Plan

- Obtain explicit human approval before requirements → design and design → implementation planning.
- Before design: ` + "`get_artifact`" + ` approved requirements; ` + "`find_decisions`" + `/` + "`get_context`" + ` applicable Canon; preserve literal IDs and relationships.
- After requirements/design approval: ` + "`pyra project <file>`" + ` (` + "`--write`" + ` if existing), then ` + "`pyra gate .`" + `. Tasks must trace to approved requirements/design. Do not implement unapproved or failing authority.

### Implement

- Before code exploration/change, query Pyra MCP: ` + "`find_decisions`" + ` (area), ` + "`get_artifact`" + ` (governing Canon), ` + "`get_context`" + ` (Canon-first context).
- Accepted Canon is binding. On conflict, stop and report; never work around it.
- After authority status/relationship changes, run ` + "`pyra rebuild .`" + ` before further grounding.

### Review and complete

- For any code/change/pre-commit/equivalent review, check correctness, security, tests, clarity, and conventions; also run ` + "`pyra gate . --sarif`" + ` and ` + "`pyra relationships . --summary --validate`" + `.
- Blocking gate findings block approval: cite known Canon, fix, and rerun.
- Claim completion only with test evidence, a passing gate, and no blocking relationship-integrity failures.
- Keep ` + "`pyra`" + ` on PATH; repository-local MCP configuration starts ` + "`pyra serve`" + ` automatically.`

func renderAgents(existing string) (string, error) {
	starts := strings.Count(existing, agentsBegin)
	ends := strings.Count(existing, agentsEnd)
	if starts > 1 || ends > 1 || starts != ends {
		return "", fmt.Errorf("AGENTS.md contains malformed or duplicate Pyra managed markers")
	}
	block := agentsBegin + "\n" + managedInstructions + agentsEnd + "\n"
	if starts == 1 {
		start := strings.Index(existing, agentsBegin)
		endRel := strings.Index(existing[start:], agentsEnd)
		if endRel < 0 {
			return "", fmt.Errorf("AGENTS.md contains an unterminated Pyra managed block")
		}
		end := start + endRel + len(agentsEnd)
		rest := strings.TrimPrefix(existing[end:], "\n")
		return existing[:start] + block + rest, nil
	}
	if existing == "" {
		return block, nil
	}
	separator := "\n\n"
	if strings.HasSuffix(existing, "\n\n") {
		separator = ""
	} else if strings.HasSuffix(existing, "\n") {
		separator = "\n"
	}
	return existing + separator + block, nil
}
