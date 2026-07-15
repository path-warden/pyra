<!-- >>> pyra managed instructions >>> -->
## Pyra authority workflow (managed)

This repository uses Pyra as its authority layer. These rules apply to equivalent activities under any skill, command, prompt, or agent workflow—not only workflows named spec, dev, or code-review.

### Requirements, design, and specification work

- Obtain explicit human approval before advancing from requirements to design or from design to implementation planning.
- Before drafting or updating a design, use `get_artifact` for the approved requirements and `find_decisions`/`get_context` for applicable Canon; preserve literal requirement and Canon IDs in the design's relationships.
- After approving requirements or design, run `pyra project <approved-file>` (add `--write` when updating an existing projection), then run `pyra gate .` before treating that phase as complete.
- Implementation tasks must trace to approved requirements and design authority. Do not implement while governing artifacts are unapproved or the gate fails.

### Implementation work

- Before exploring or changing code, use the configured Pyra MCP server: call `find_decisions` for the area, `get_artifact` for governing Canon, and `get_context` for a Canon-first context pack.
- Treat Accepted Canon as binding. If requested work conflicts with it, stop and surface the conflict instead of working around it.
- If authority status or relationships change, run `pyra rebuild .` before relying on later MCP grounding.

### Review and completion

- For any code review, change review, pre-commit review, or equivalent activity, run `pyra gate . --sarif` and `pyra relationships . --summary --validate` in addition to normal correctness, security, test, clarity, and convention checks.
- Treat every gate-blocking authority finding as a blocking review finding, cite the relevant Canon artifact when known, fix it, and rerun the checks before approval.
- Do not claim completion without reporting applicable test evidence, a passing Pyra gate, and no unresolved blocking relationship-integrity failures.
- Ensure `pyra` is available on PATH; repository-local MCP configuration starts `pyra serve` automatically.<!-- <<< pyra managed instructions <<< -->
