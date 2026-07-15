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

This repository uses Pyra as its authority layer. These rules apply to equivalent activities under any skill, command, prompt, or agent workflow—not only workflows named spec, dev, or code-review.

### Requirements, design, and specification work

- Obtain explicit human approval before advancing from requirements to design or from design to implementation planning.
- Before drafting or updating a design, use ` + "`get_artifact`" + ` for the approved requirements and ` + "`find_decisions`" + `/` + "`get_context`" + ` for applicable Canon; preserve literal requirement and Canon IDs in the design's relationships.
- After approving requirements or design, run ` + "`pyra project <approved-file>`" + ` (add ` + "`--write`" + ` when updating an existing projection), then run ` + "`pyra gate .`" + ` before treating that phase as complete.
- Implementation tasks must trace to approved requirements and design authority. Do not implement while governing artifacts are unapproved or the gate fails.

### Implementation work

- Before exploring or changing code, use the configured Pyra MCP server: call ` + "`find_decisions`" + ` for the area, ` + "`get_artifact`" + ` for governing Canon, and ` + "`get_context`" + ` for a Canon-first context pack.
- Treat Accepted Canon as binding. If requested work conflicts with it, stop and surface the conflict instead of working around it.
- If authority status or relationships change, run ` + "`pyra rebuild .`" + ` before relying on later MCP grounding.

### Review and completion

- For any code review, change review, pre-commit review, or equivalent activity, run ` + "`pyra gate . --sarif`" + ` and ` + "`pyra relationships . --summary --validate`" + ` in addition to normal correctness, security, test, clarity, and convention checks.
- Treat every gate-blocking authority finding as a blocking review finding, cite the relevant Canon artifact when known, fix it, and rerun the checks before approval.
- Do not claim completion without reporting applicable test evidence, a passing Pyra gate, and no unresolved blocking relationship-integrity failures.
- Ensure ` + "`pyra`" + ` is available on PATH; repository-local MCP configuration starts ` + "`pyra serve`" + ` automatically.`

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
