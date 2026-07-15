---
schema_version: 1
id: OKF-DEBA7FA1Z6A5
type: requirement
---

# Requirements Document

## Problem

Today, enabling Pyra in a repository is split between `pyra init` and the
repository-level `install_skills.sh` script. The script depends on a clone of
the Pyra source repository, installs skills into user-global tool directories,
and only recognizes a subset of the agent tools that can consume Pyra over
MCP. A repository initialized with Pyra should instead contain everything an
agent needs to discover and use that repository's Canon, while preserving any
agent instructions and tool configuration the repository already owns.

The minimum viable change moves agent enablement into `pyra init`: it creates
or augments a repository-local `AGENTS.md`, configures one or more user-selected
agent tools to launch the repository's Pyra MCP server, and retains the local
gate enforcement needed for successful use. This replaces the global skill
copying workflow and removes the need to run `install_skills.sh` from a Pyra
source checkout.

## Requirements

### Requirement 1: Repository-Local Agent Instructions

#### Acceptance Criteria

- [REQ-101] WHEN a user initializes a repository with Pyra THEN Pyra SHALL create a root-level `AGENTS.md` containing the Pyra usage instructions if the file does not already exist.
- [REQ-102] WHEN a root-level `AGENTS.md` already exists THEN Pyra SHALL add or update a clearly delimited Pyra-owned section without modifying content outside that section.
- [REQ-103] WHEN initialization is repeated THEN Pyra SHALL produce exactly one current Pyra-owned instruction section and SHALL NOT duplicate the section.
- [REQ-104] The Pyra-owned instruction section SHALL tell agents how to use the repository's Pyra MCP tools to discover applicable Canon before changing code, how to respect accepted authority, and how to run the Pyra gate before claiming completion.
- [REQ-105] The Pyra-owned instruction section SHALL use repository-relative guidance where possible so that cloning or moving the repository does not make the instructions stale.
- [REQ-106] The Pyra-owned instruction section SHALL map work by activity and intent, including equivalent or renamed skills, commands, prompts, and agent-native workflows, rather than requiring the workflow to be invoked through specific skill names such as `/spec`, `/dev`, or `/code-review`.

### Requirement 2: Multi-Tool MCP Enablement

#### Acceptance Criteria

- [REQ-201] WHEN a user selects one supported agent tool during initialization THEN Pyra SHALL create or update that tool's repository-local MCP configuration with a Pyra server entry for the initialized repository.
- [REQ-202] WHEN a user selects multiple supported agent tools during initialization THEN Pyra SHALL configure every selected tool in the same run.
- [REQ-203] The supported-tool set SHALL include Claude Code, Codex, OpenCode, and Pi, and SHALL use each tool's documented repository-local configuration format and location.
- [REQ-204] WHEN a selected tool's local configuration already exists THEN Pyra SHALL preserve unrelated settings and MCP servers while adding or updating only the Pyra-owned server entry.
- [REQ-205] WHEN initialization is repeated for a selected tool THEN Pyra SHALL update the existing Pyra-owned server entry without creating duplicates.
- [REQ-206] The generated MCP server entry SHALL invoke the installed `pyra` executable in MCP mode against the initialized repository and SHALL work independently of the caller's current working directory.
- [REQ-207] WHEN an existing file cannot be safely parsed or merged THEN Pyra SHALL leave it unchanged, identify the affected tool and file, and return a clear error.

### Requirement 3: Tool Selection Experience

#### Acceptance Criteria

- [REQ-301] WHEN the user supplies one or more valid tool selections non-interactively THEN Pyra SHALL configure exactly the deduplicated selected tools.
- [REQ-302] IF any supplied tool identifier is unsupported THEN Pyra SHALL report the unsupported value and the supported values before writing initialization artifacts.
- [REQ-303] WHEN the user requests the supported-tool list THEN Pyra SHALL print stable tool identifiers and human-readable names without modifying the repository.
- [REQ-304] WHEN no tool is selected and interactive selection is unavailable or declined THEN Pyra SHALL still initialize the Pyra store and `AGENTS.md`, SHALL not create an MCP configuration for an unselected tool, and SHALL explain how to enable tools later.
- [REQ-305] WHEN initialization runs with quiet output enabled THEN Pyra SHALL suppress success output while preserving errors and all requested filesystem effects.
- [REQ-306] WHEN the user runs initialization with agent-only mode and one or more selected tools THEN Pyra SHALL update the managed `AGENTS.md` section and exactly the selected repository-local MCP client configurations without reading or writing `.okf/config.yaml`, creating Canon directories, or installing or updating hooks.
- [REQ-307] IF agent-only mode is requested without a selected tool or with a store- or hook-configuration flag THEN Pyra SHALL reject the invocation before writing any files and identify the incompatible input.

### Requirement 4: Local Enforcement and Replacement of the Installer

#### Acceptance Criteria

- [REQ-401] WHEN `pyra init` runs inside a Git repository THEN Pyra SHALL install or update Pyra's repository-local Git gate hooks using the same safety and idempotency guarantees as the existing hook installer.
- [REQ-402] WHERE a selected tool supports repository-local Pyra gate hooks THEN initialization SHALL install or update those hooks without modifying unrelated tool hooks.
- [REQ-403] Pyra initialization SHALL NOT copy bundled skills into user-global agent directories.
- [REQ-404] WHEN the new initialization workflow is released THEN the repository SHALL no longer ship or document `install_skills.sh` as a supported setup step.
- [REQ-405] The user-facing initialization summary SHALL identify the store config, Canon roots, `AGENTS.md`, each configured MCP tool, and each installed local hook integration.

### Requirement 5: Safety, Compatibility, and Verification

#### Acceptance Criteria

- [REQ-501] WHEN initialization input validation fails THEN Pyra SHALL perform no filesystem writes.
- [REQ-502] WHEN an existing Pyra store is initialized without the existing overwrite authorization THEN Pyra SHALL retain the current refusal behavior and SHALL NOT modify `AGENTS.md`, MCP configuration, or hooks.
- [REQ-503] WHEN overwrite authorization is supplied THEN Pyra SHALL overwrite the Pyra store configuration as requested while still preserving non-Pyra content in `AGENTS.md`, tool configuration, and hook files.
- [REQ-504] IF initialization fails after creating or updating one of the new agent setup artifacts THEN Pyra SHALL report which changes completed and which did not, and SHALL NOT claim successful initialization.
- [REQ-505] The initialization implementation SHALL have automated tests covering new files, merges into existing files, repeated runs, multi-tool selection, unsupported tools, malformed existing configuration, quiet mode, hook setup, agent-only isolation, and preservation of unrelated content.
- [REQ-506] Existing `pyra init` configuration flags and default store configuration SHALL remain compatible unless a new tool-selection option is explicitly used.

### Requirement 6: Spec-Driven Authority Lifecycle Mapping

#### Acceptance Criteria

- [REQ-601] WHEN an agent creates or updates requirements as part of a spec-driven, requirements-driven, or equivalent planning workflow THEN the `AGENTS.md` instructions SHALL require the agent to project approved requirements into Pyra Canon and run the Pyra gate before treating the requirements phase as complete.
- [REQ-602] WHEN an agent creates or updates a design as part of a design, planning, or equivalent workflow THEN the `AGENTS.md` instructions SHALL require the agent to ground the design in the approved requirements and applicable Canon, preserve explicit artifact relationships, project the approved design into Pyra Canon, and run the Pyra gate before treating the design phase as complete.
- [REQ-603] WHEN an agent creates an implementation plan or task breakdown THEN the `AGENTS.md` instructions SHALL require every task to trace to approved requirements and design authority and SHALL prohibit implementation from starting while the governing artifacts are unapproved or fail the Pyra gate.
- [REQ-604] WHEN an agent starts or resumes implementation, regardless of the skill or command name used, THEN the `AGENTS.md` instructions SHALL require it to use the Pyra MCP server to retrieve applicable authority before exploring or changing code, including relevant decisions, requirements, artifacts, and a Canon-first context pack.
- [REQ-605] IF implementation work appears to conflict with Accepted Canon THEN the `AGENTS.md` instructions SHALL require the agent to stop and surface the conflict instead of implementing around the authority.
- [REQ-606] WHEN implementation changes the status or relationships of a projected artifact THEN the `AGENTS.md` instructions SHALL require the agent to rebuild Pyra's indexes before relying on subsequent MCP grounding.
- [REQ-607] WHEN an agent performs a code review, change review, pre-commit review, or equivalent verification activity THEN the `AGENTS.md` instructions SHALL require a blocking Pyra gate check and a Canon relationship-integrity summary in addition to the tool's ordinary correctness, security, test, clarity, and convention checks.
- [REQ-608] IF the Pyra gate reports a blocking authority finding during review THEN the `AGENTS.md` instructions SHALL require the agent to classify it as blocking, cite the relevant Canon artifact when known, and withhold approval until the finding is resolved and the gate is rerun successfully.
- [REQ-609] WHEN an agent claims implementation or review completion THEN the `AGENTS.md` instructions SHALL require evidence that applicable tests passed, the Pyra gate passed, and Canon relationship checks contain no unresolved blocking integrity failures.
- [REQ-610] The generated mappings SHALL preserve the behavioral intent of Pyra's bundled example `spec`, `dev`, and `code-review` skills while remaining usable by tools whose skill systems, names, and invocation models differ.

## Success Metrics

TODO

## Risks

TODO

## Assumptions

TODO

