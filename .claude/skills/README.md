# Pyra agent skills

These three skills implement the spec-driven development lifecycle that Pyra is built for, with the `pyra project` / `gate` / `hooks` / MCP grounding steps wired directly into each phase. They use the `SKILL.md` format shared by **Claude Code** ([docs](https://code.claude.com/docs/en/skills)) and **Kiro** ([docs](https://kiro.dev/docs/cli/skills/)), so the same folders install into both:

| Skill | Phase | Pyra integration |
|---|---|---|
| `spec` | Requirements -> Design -> Tasks | Projects each approved `requirements.md` / `design.md` into typed Canon and gates it. |
| `dev` | Implementation | Grounds the work in Canon over MCP (`find_decisions` / `get_artifact` / `get_context`) before writing code; rebuilds indexes after status changes. |
| `code-review` | Review | Runs `pyra gate --sarif` as a required authority check and cites touched artifacts via `pyra relationships --summary`. |

Every Pyra step is guarded by "if this is a Pyra store (a `.okf/config.yaml` exists)", so the skills also work unchanged in repositories that don't use Pyra.

## Repository setup

Use the Pyra binary to generate repository-local activity guidance, MCP configuration, and applicable gate hooks. Select every tool used in the repository; the flag is repeatable:

```bash
pyra init . --agent claude --agent kiro
```

These folders remain examples of `/spec`, `/dev`, and `/code-review` workflows. The generated `AGENTS.md` maps equivalent requirements, design, implementation, and review activities to the same Pyra operations without requiring a particular skill name or global installation.

## The loop

```
/spec         requirements.md / design.md  ->  pyra project  ->  Canon (gated)
/dev          read Canon over MCP          ->  implement (TDD)   ->  pyra rebuild
/code-review  pyra gate --sarif         ->  findings cite the Canon they touch
```

`pyra init` installs applicable local hooks so the gate also fires automatically on write, commit, and merge across git, Claude Code, Codex, and Kiro. See the project [README](../../README.md) for the full command set.
