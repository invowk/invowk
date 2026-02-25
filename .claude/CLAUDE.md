# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.26+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invowkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules, which are filesystem directories named as `<module-id>.invowkmod` (preferably using the RDNS convention) that contain:
  - an `invowkmod.cue` file
  - an `invowkfile.cue` file

  Modules can require other modules as dependencies, which is how Invowk effectively provides modularity and `cmd` re-use for users. Additionally, modules also serve as a means to bundle scripts and ad-hoc files required for `cmd` execution.

  The only guarantee Invowk provides about cross `cmd`/module visibility is that `cmds` from a given module (e.g: `module foo`) that requires another module (e.g.: `module bar`) will be able to see/call `cmds` from the required module -- or, in other words, even though transitive dependencies are supported, only first-level dependencies are effectively exposed to the caller (e.g.: `cmds` from `module foo` will be able to see/call `cmds` from `module bar`, but not from the dependencies of `module bar`).

## Rules for Agents (Critical)

### Compaction

- Prioritize the keeping and remembering of file paths, function and symbol names, identified issues and goals, current architectural decisions, semantic learnings, and next steps. Do not discard the output of the latest ~5 tool calls; discard the oldest ones.
- Never compact the content of CLAUDE.MD or rule/agent/skill definitions.

### Workflow Orchestration

**CRITICAL:** Whenever possible and appropriate, multiple Tasks, Teammates, and Subagents must be used.

**CRITICAL:** Teammates must always require plan approval before they make any changes.

### Rules

**CRITICAL:** The files in `.agents/rules/` define the authoritative rules for agents. EVERYTIME there is ANY change to files/rules inside `.agents/rules` (new file, file rename, file removed, etc.), the index/sync map in this file MUST be updated accordingly.

**Rules Index / Sync Map (must match `.agents/rules/`):**
- [`.agents/rules/checklist.md`](.agents/rules/checklist.md) - Pre-completion verification steps.
- [`.agents/rules/commands.md`](.agents/rules/commands.md) - Build, test, and release commands.
- [`.agents/rules/cue-patterns.md`](.agents/rules/cue-patterns.md) - CUE schema patterns, string validation, common pitfalls.
- [`.agents/rules/general-rules.md`](.agents/rules/general-rules.md) - Instruction priority, code quality, documentation.
- [`.agents/rules/git.md`](.agents/rules/git.md) - Commit signing, squash merge, message format.
- [`.agents/rules/go-patterns.md`](.agents/rules/go-patterns.md) - Go style, naming, errors, interfaces, comments.
- [`.agents/rules/licensing.md`](.agents/rules/licensing.md) - SPDX headers and MPL-2.0 rules.
- [`.agents/rules/package-design.md`](.agents/rules/package-design.md) - Package boundaries and module design.
- [`.agents/rules/testing.md`](.agents/rules/testing.md) - Test patterns, cross-platform testing, skipOnWindows.
- [`.agents/rules/version-pinning.md`](.agents/rules/version-pinning.md) - Version pinning policy for deps, tools, images.
- [`.agents/rules/windows.md`](.agents/rules/windows.md) - Windows-specific constraints and guidance.

### Agents

**Agents Index (`.agents/agents/`):**

Agents are specialized reviewers and generators that can be spawned as subagents for focused tasks.

- [`.agents/agents/code-reviewer.md`](.agents/agents/code-reviewer.md) - Go code review: decorder, sentinel errors, wrapcheck, SPDX headers, guardrail compliance.
- [`.agents/agents/cue-schema-agent.md`](.agents/agents/cue-schema-agent.md) - CUE schema specialist: 3-step parse flow, sync tests, validation matrix.
- [`.agents/agents/doc-updater.md`](.agents/agents/doc-updater.md) - Documentation sync: code→doc sync map, MDX snippets, i18n mirrors, diagram updates.
- [`.agents/agents/performance-analyzer.md`](.agents/agents/performance-analyzer.md) - Benchmark-aware reviewer: CUE hot path, discovery traversal, PGO profile maintenance.
- [`.agents/agents/security-reviewer.md`](.agents/agents/security-reviewer.md) - Security reviewer: SSH auth, container injection, gosec exclusions, env var handling.
- [`.agents/agents/test-writer.md`](.agents/agents/test-writer.md) - Testscript generator: virtual/native txtar pairs, platform-split CUE, exemption rules.

### Commands

**Commands Index (`.agents/commands/`):**

Commands are user-invokable slash commands (e.g., `/review-docs`) that execute multi-step workflows.

- [`.agents/commands/fix-it.md`](.agents/commands/fix-it.md) - Analyze issues and propose robust fix plan with prevention strategy.
- [`.agents/commands/fix-it-simple.md`](.agents/commands/fix-it-simple.md) - Analyze issues and propose concise fix with prevention.
- [`.agents/commands/improve-type-system.md`](.agents/commands/improve-type-system.md) - Identify next type definitions/structs to convert to DDD Value Types with IsValid().
- [`.agents/commands/review-docs.md`](.agents/commands/review-docs.md) - Review README and website docs for accuracy against current architecture and behaviors.
- [`.agents/commands/review-rules.md`](.agents/commands/review-rules.md) - Review rules files for contradictions, ambiguities, incoherence, or excessive noise.
- [`.agents/commands/review-tests.md`](.agents/commands/review-tests.md) - Review test suite for semantic comprehensiveness, signal-to-noise, and E2E coverage.
- [`.agents/commands/review-type-system.md`](.agents/commands/review-type-system.md) - Review Go type system for type safety improvements and abstraction opportunities.
- [`.agents/commands/rust-alt.md`](.agents/commands/rust-alt.md) - Identify next items and plan Go→Rust conversion with DDD and 1000-line file limit.

### Skills

**Skills Index (`.agents/skills/`):**

Skills provide domain-specific procedural guidance. They are invoked when working on specific components.

- [`.agents/skills/cli/`](.agents/skills/cli/) - CLI command structure, Cobra patterns, execution flow, hidden internal commands.
- [`.agents/skills/container/`](.agents/skills/container/) - Container engine abstraction, Docker/Podman patterns, path handling, Linux-only policy.
- [`.agents/skills/cue/`](.agents/skills/cue/) - CUE schema parsing, 3-step parse flow, validation matrix, schema sync tests.
- [`.agents/skills/d2-diagrams/`](.agents/skills/d2-diagrams/) - Agent-optimized D2 diagram generation with TALA layout, validation pipeline, deterministic output. **DEFAULT for new diagrams.**
- [`.agents/skills/discovery/`](.agents/skills/discovery/) - Module/command discovery, precedence order, collision detection, source tracking.
- [`.agents/skills/docs/`](.agents/skills/docs/) - Documentation workflow and Docusaurus website development.
- [`.agents/skills/invowk-schema/`](.agents/skills/invowk-schema/) - Invowkfile/invowkmod schema guidelines, cross-platform runtime patterns.
- [`.agents/skills/native-mirror/`](.agents/skills/native-mirror/) - User-invokable (`/native-mirror`). Generate native_*.txtar mirrors from virtual tests with platform-split CUE.
- [`.agents/skills/schema-sync-check/`](.agents/skills/schema-sync-check/) - User-invokable (`/schema-sync-check`). Validate CUE schema ↔ Go struct JSON tag alignment.
- [`.agents/skills/server/`](.agents/skills/server/) - Server state machine pattern for SSH and TUI servers.
- [`.agents/skills/shell/`](.agents/skills/shell/) - Shell runtime rules for mvdan/sh virtual shell.
- [`.agents/skills/testing/`](.agents/skills/testing/) - Testing patterns, testscript CLI tests, race conditions, TUI/container testing.
- [`.agents/skills/tmux-testing/`](.agents/skills/tmux-testing/) - tmux-based TUI testing for fast, CI-friendly text and ANSI verification.
- [`.agents/skills/tui-testing/`](.agents/skills/tui-testing/) - VHS-based TUI testing workflow for autonomous visual analysis.
- [`.agents/skills/uroot/`](.agents/skills/uroot/) - u-root utility implementation patterns.
- [`.agents/skills/learn/`](.agents/skills/learn/) - User-invokable (`/learn`). Post-work learning review to keep CLAUDE.md, hooks, rules, and skills up-to-date.
- [`.agents/skills/pr/`](.agents/skills/pr/) - GitHub PR workflow: tests, lints, license check, branch creation, conventional commits, and PR description.
- [`.agents/skills/changelog/`](.agents/skills/changelog/) - User-invokable (`/changelog`). Generate release notes from conventional commits since last tag.
- [`.agents/skills/dep-audit/`](.agents/skills/dep-audit/) - User-invokable (`/dep-audit`). Audit Go dependencies for vulnerabilities and available updates.
- [`.agents/skills/speckit.specify/`](.agents/skills/speckit.specify/) - **User-only** (`/speckit.specify`). Create or update feature specification from natural language description. **Never auto-invoke.**
- [`.agents/skills/speckit.clarify/`](.agents/skills/speckit.clarify/) - **User-only** (`/speckit.clarify`). Identify underspecified areas in feature spec via targeted clarification questions. **Never auto-invoke.**
- [`.agents/skills/speckit.plan/`](.agents/skills/speckit.plan/) - **User-only** (`/speckit.plan`). Generate implementation plan from feature specification. **Never auto-invoke.**
- [`.agents/skills/speckit.tasks/`](.agents/skills/speckit.tasks/) - **User-only** (`/speckit.tasks`). Generate dependency-ordered tasks.md from design artifacts. **Never auto-invoke.**
- [`.agents/skills/speckit.taskstoissues/`](.agents/skills/speckit.taskstoissues/) - **User-only** (`/speckit.taskstoissues`). Convert tasks.md into GitHub issues with dependency ordering. **Never auto-invoke.**
- [`.agents/skills/speckit.implement/`](.agents/skills/speckit.implement/) - **User-only** (`/speckit.implement`). Execute implementation plan by processing tasks from tasks.md. **Never auto-invoke.**
- [`.agents/skills/speckit.analyze/`](.agents/skills/speckit.analyze/) - **User-only** (`/speckit.analyze`). Cross-artifact consistency and quality analysis across spec, plan, and tasks. **Never auto-invoke.**
- [`.agents/skills/speckit.checklist/`](.agents/skills/speckit.checklist/) - **User-only** (`/speckit.checklist`). Generate custom checklist for current feature based on requirements. **Never auto-invoke.**
- [`.agents/skills/speckit.constitution/`](.agents/skills/speckit.constitution/) - **User-only** (`/speckit.constitution`). Create or update project constitution from principle inputs. **Never auto-invoke.**

### Code Area → Rules/Skills Mapping

When working in a specific code area, apply these rules and skills:

| Code Area | Rules | Skills |
|-----------|-------|--------|
| `cmd/invowk/` | go-patterns, testing, licensing, commands | cli, d2-diagrams |
| `internal/app/execute/` | go-patterns, testing, licensing, package-design | cli |
| `internal/container/` | go-patterns, testing, windows, licensing | container |
| `internal/discovery/` | go-patterns, testing, licensing, package-design | discovery, d2-diagrams |
| `internal/runtime/` | go-patterns, testing, windows, licensing | shell (for virtual runtime), d2-diagrams |
| `internal/config/` | go-patterns, testing, cue-patterns, licensing | cue |
| `pkg/cueutil/` | go-patterns, testing, cue-patterns, licensing | cue |
| `internal/sshserver/` | go-patterns, testing, licensing | server |
| `internal/tuiserver/` | go-patterns, testing, licensing | server |
| `internal/tui/` | go-patterns, testing, licensing | testing, tui-testing, tmux-testing |
| `internal/issue/` | go-patterns, testing, licensing | — |
| `internal/provision/` | go-patterns, testing, windows, licensing | container |
| `pkg/invowkfile/` | go-patterns, testing, cue-patterns, licensing, package-design | cue, invowk-schema |
| `pkg/invowkmod/` | go-patterns, testing, cue-patterns, licensing, package-design | cue, invowk-schema |
| `website/` | general-rules | docs |
| `docs/architecture/` | general-rules | docs, d2-diagrams |
| `internal/uroot/` | go-patterns, testing, licensing | uroot |
| `internal/core/serverbase/` | go-patterns, testing, licensing | server |
| `internal/benchmark/` | go-patterns, testing, licensing, commands | — |
| `internal/watch/` | go-patterns, testing, licensing | — |
| `pkg/platform/` | go-patterns, testing, windows, licensing | — |
| `pkg/types/` | go-patterns, testing, licensing, package-design | — |
| `tests/cli/` | testing | testing, cli, invowk-schema |
| `tools/primitivelint/` | go-patterns, testing, licensing | — |

## Architecture Overview

```
invowkfile.cue -> CUE Parser -> pkg/invowkfile -> Runtime Selection -> Execution
                                                  |
                                  +---------------+---------------+
                                  |               |               |
                               Native         Virtual        Container
                            (host shell)    (mvdan/sh)    (Docker/Podman)
```

- **CUE Schemas**:
  - `pkg/invowkfile/invowkfile_schema.cue` defines `invowkfile` structure
  - `pkg/invowkmod/invowkmod_schema.cue` defines `invowkmod` structure
  - `internal/config/config_schema.cue` defines config structure
- **Runtime Interface**: All runtimes implement the same interface in `internal/runtime/`.
- **TUI Components**: Built with Charm libraries (bubbletea, huh, lipgloss).

## Directory Layout

- `cmd/invowk/` - CLI commands using Cobra.
- `internal/` - Private packages:
  - `app/execute/` - Execution orchestration (runtime resolution, execution context construction).
  - `config/` - Configuration management with CUE schema.
  - `container/` - Docker/Podman container engine abstraction.
  - `core/serverbase/` - Shared server state machine base type (used by sshserver, tuiserver).
  - `discovery/` - Module and command discovery.
  - `issue/` - Error handling with ActionableError type.
  - `runtime/` - Execution runtimes (native, virtual, container).
  - `sshserver/` - SSH server for remote execution.
  - `tui/` - Terminal UI components.
  - `tuiserver/` - TUI server for interactive sessions.
  - `uroot/` - u-root utility implementations for virtual shell built-ins.
  - `benchmark/` - Benchmarks for PGO profile generation.
  - `watch/` - File-watching with debounced re-execution for `--ivk-watch` mode.
  - `provision/` - Container provisioning (ephemeral layer attachment).
- `pkg/` - Public packages (cueutil, invowkmod, invowkfile, platform, types).
- `tests/cli/` - CLI integration tests using testscript (`.txtar` files in `testdata/`).
- `modules/` - Sample invowk modules for validation and reference.
- `scripts/` - Build, install, and release scripts (`install.sh` for Linux/macOS, `install.ps1` for Windows, `enhance-winget-manifest.sh` for WinGet CI automation).
- `tools/` - Development tools (separate Go modules):
  - `primitivelint/` - Custom `go/analysis` analyzer for DDD Value Type enforcement. Detects bare primitives in struct fields, function params, and returns. Also checks for missing `IsValid`/`String` methods, constructor existence/signatures, functional options patterns, and struct immutability. Run via `make check-types`. Full DDD audit via `make check-types-all`. Baseline regression gate via `make check-baseline`; update after type improvements with `make update-baseline`.
- `specs/` - Feature specifications, research, and implementation plans.
- `tasks/` - Pending analysis documents and planning notes (e.g., `tasks/next/` for items awaiting decision).

## Container Runtime Limitations

**The container runtime ONLY supports Linux containers.** This is a fundamental design limitation:

- **Supported images**: Debian-based images (e.g., `debian:stable-slim`).
- **NOT supported**: Alpine-based images (`alpine:*`) and Windows container images.

**Why no Alpine support:**
- There are many subtle gotchas in musl-based environments.
- We prioritize reliability over image size.

**Why no Windows container support:**
- They're rarely used and would introduce too much extra complexity to Invowk's auto-provisioning logic (which attaches an ephemeral image layer containing the `invowk` binary and the needed `invowkfiles` and `invowkmods` to the user-specified image/containerfile when the container runtime is used)

**When writing tests, documentation, or examples:**
- Always use `debian:stable-slim` as the reference container image.
- Never use `ubuntu:*` or other non-`debian:stable-slim` base images. Language-specific images (`golang:1.26`, `python:3-slim`) are allowed when demonstrating language-specific runtimes.
- Never use Alpine images.
- Never use Windows container images (e.g., `mcr.microsoft.com/windows/*`).

## Key Dependencies

- `github.com/spf13/cobra` - CLI framework.
- `github.com/spf13/viper` - Configuration management.
- `cuelang.org/go` - CUE language support for configuration/schema.
- `github.com/charmbracelet/*` - TUI components (lipgloss, bubbletea, huh).
- `mvdan.cc/sh/v3` - Virtual shell implementation.
- `github.com/rogpeppe/go-internal/testscript` - CLI integration tests.

See `go.mod` for exact versions. Schema sync tests verify Go struct tags match CUE schema fields at CI time.

