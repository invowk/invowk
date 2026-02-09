# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules, which are filesystem directories named as `<module-id>.invkmod` (preferably using the RDNS convention) that contain:
  - an `invkmod.cue` file
  - an `invkfile.cue` file

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

**CRITICAL:** The files in `.claude/rules/` define the authoritative rules for agents. EVERYTIME there is ANY change to files/rules inside `.claude/rules` (new file, file rename, file removed, etc.), the index/sync map in this file MUST be updated accordingly.

**Rules Index / Sync Map (must match `.claude/rules/`):**
- [`.claude/rules/checklist.md`](.claude/rules/checklist.md) - Pre-completion verification steps.
- [`.claude/rules/commands.md`](.claude/rules/commands.md) - Build, test, and release commands.
- [`.claude/rules/cue-patterns.md`](.claude/rules/cue-patterns.md) - CUE schema patterns, string validation, common pitfalls.
- [`.claude/rules/general-rules.md`](.claude/rules/general-rules.md) - Instruction priority, code quality, documentation.
- [`.claude/rules/git.md`](.claude/rules/git.md) - Commit signing, squash merge, message format.
- [`.claude/rules/go-patterns.md`](.claude/rules/go-patterns.md) - Go style, naming, errors, interfaces, comments.
- [`.claude/rules/licensing.md`](.claude/rules/licensing.md) - SPDX headers and MPL-2.0 rules.
- [`.claude/rules/package-design.md`](.claude/rules/package-design.md) - Package boundaries and module design.
- [`.claude/rules/testing.md`](.claude/rules/testing.md) - Test patterns, cross-platform testing, skipOnWindows.
- [`.claude/rules/windows.md`](.claude/rules/windows.md) - Windows-specific constraints and guidance.

### Agents

**Agents Index (`.claude/agents/`):**

Agents are specialized reviewers and generators that can be spawned as subagents for focused tasks.

- [`.claude/agents/code-reviewer.md`](.claude/agents/code-reviewer.md) - Go code review: decorder, sentinel errors, wrapcheck, SPDX headers, guardrail compliance.
- [`.claude/agents/cue-schema-agent.md`](.claude/agents/cue-schema-agent.md) - CUE schema specialist: 3-step parse flow, sync tests, validation matrix.
- [`.claude/agents/doc-updater.md`](.claude/agents/doc-updater.md) - Documentation sync: code→doc sync map, MDX snippets, i18n mirrors, diagram updates.
- [`.claude/agents/performance-analyzer.md`](.claude/agents/performance-analyzer.md) - Benchmark-aware reviewer: CUE hot path, discovery traversal, PGO profile maintenance.
- [`.claude/agents/security-reviewer.md`](.claude/agents/security-reviewer.md) - Security reviewer: SSH auth, container injection, gosec exclusions, env var handling.
- [`.claude/agents/test-writer.md`](.claude/agents/test-writer.md) - Testscript generator: virtual/native txtar pairs, platform-split CUE, exemption rules.

### Skills

**Skills Index (`.claude/skills/`):**

Skills provide domain-specific procedural guidance. They are invoked when working on specific components.

- [`.claude/skills/cli/`](.claude/skills/cli/) - CLI command structure, Cobra patterns, execution flow, hidden internal commands.
- [`.claude/skills/container/`](.claude/skills/container/) - Container engine abstraction, Docker/Podman patterns, path handling, Linux-only policy.
- [`.claude/skills/cue/`](.claude/skills/cue/) - CUE schema parsing, 3-step parse flow, validation matrix, schema sync tests.
- [`.claude/skills/d2-diagrams/`](.claude/skills/d2-diagrams/) - Agent-optimized D2 diagram generation with TALA layout, validation pipeline, deterministic output. **DEFAULT for new diagrams.**
- [`.claude/skills/discovery/`](.claude/skills/discovery/) - Module/command discovery, precedence order, collision detection, source tracking.
- [`.claude/skills/docs/`](.claude/skills/docs/) - Documentation workflow and Docusaurus website development.
- [`.claude/skills/invowk-schema/`](.claude/skills/invowk-schema/) - Invkfile/invkmod schema guidelines, cross-platform runtime patterns.
- [`.claude/skills/native-mirror/`](.claude/skills/native-mirror/) - User-invokable (`/native-mirror`). Generate native_*.txtar mirrors from virtual tests with platform-split CUE.
- [`.claude/skills/schema-sync-check/`](.claude/skills/schema-sync-check/) - User-invokable (`/schema-sync-check`). Validate CUE schema ↔ Go struct JSON tag alignment.
- [`.claude/skills/server/`](.claude/skills/server/) - Server state machine pattern for SSH and TUI servers.
- [`.claude/skills/shell/`](.claude/skills/shell/) - Shell runtime rules for mvdan/sh virtual shell.
- [`.claude/skills/testing/`](.claude/skills/testing/) - Testing patterns, testscript CLI tests, race conditions, TUI/container testing.
- [`.claude/skills/tmux-testing/`](.claude/skills/tmux-testing/) - tmux-based TUI testing for fast, CI-friendly text and ANSI verification.
- [`.claude/skills/tui-testing/`](.claude/skills/tui-testing/) - VHS-based TUI testing workflow for autonomous visual analysis.
- [`.claude/skills/uroot/`](.claude/skills/uroot/) - u-root utility implementation patterns.

### Code Area → Rules/Skills Mapping

When working in a specific code area, apply these rules and skills:

| Code Area | Rules | Skills |
|-----------|-------|--------|
| `cmd/invowk/` | go-patterns, testing, licensing, commands | cli, d2-diagrams |
| `internal/container/` | go-patterns, testing, windows, licensing | container |
| `internal/discovery/` | go-patterns, testing, licensing, package-design | discovery, d2-diagrams |
| `internal/runtime/` | go-patterns, testing, windows, licensing | shell (for virtual runtime), d2-diagrams |
| `internal/config/` | go-patterns, testing, cue-patterns, licensing | cue |
| `internal/cueutil/` | go-patterns, testing, cue-patterns, licensing | cue |
| `internal/sshserver/` | go-patterns, testing, licensing | server |
| `internal/tuiserver/` | go-patterns, testing, licensing | server |
| `internal/tui/` | go-patterns, testing, licensing | testing, tui-testing, tmux-testing |
| `internal/issue/` | go-patterns, testing, licensing | — |
| `internal/provision/` | go-patterns, testing, windows, licensing | container |
| `pkg/invkfile/` | go-patterns, testing, cue-patterns, licensing, package-design | cue, invowk-schema |
| `pkg/invkmod/` | go-patterns, testing, cue-patterns, licensing, package-design | cue, invowk-schema |
| `website/` | general-rules | docs |
| `docs/architecture/` | general-rules | docs, d2-diagrams |
| `internal/uroot/` | go-patterns, testing, licensing | uroot |
| `internal/core/serverbase/` | go-patterns, testing, licensing | server |
| `internal/benchmark/` | go-patterns, testing, licensing, commands | — |
| `pkg/platform/` | go-patterns, testing, windows, licensing | — |
| `tests/cli/` | testing | testing, cli, invowk-schema |

## Architecture Overview

```
invkfile.cue -> CUE Parser -> pkg/invkfile -> Runtime Selection -> Execution
                                                  |
                                  +---------------+---------------+
                                  |               |               |
                               Native         Virtual        Container
                            (host shell)    (mvdan/sh)    (Docker/Podman)
```

- **CUE Schemas**:
  - `pkg/invkfile/invkfile_schema.cue` defines `invkfile` structure
  - `pkg/invkmod/invkmod_schema.cue` defines `invkmod` structure
  - `internal/config/config_schema.cue` defines config structure
- **Runtime Interface**: All runtimes implement the same interface in `internal/runtime/`.
- **TUI Components**: Built with Charm libraries (bubbletea, huh, lipgloss).

## Directory Layout

- `cmd/invowk/` - CLI commands using Cobra.
- `internal/` - Private packages:
  - `config/` - Configuration management with CUE schema.
  - `container/` - Docker/Podman container engine abstraction.
  - `core/serverbase/` - Shared server state machine base type (used by sshserver, tuiserver).
  - `cueutil/` - Shared CUE parsing utilities (3-step parse pattern, error formatting).
  - `discovery/` - Module and command discovery.
  - `issue/` - Error handling with ActionableError type.
  - `runtime/` - Execution runtimes (native, virtual, container).
  - `sshserver/` - SSH server for remote execution.
  - `tui/` - Terminal UI components.
  - `tuiserver/` - TUI server for interactive sessions.
  - `uroot/` - u-root utility implementations for virtual shell built-ins.
  - `benchmark/` - Benchmarks for PGO profile generation.
  - `provision/` - Container provisioning (ephemeral layer attachment).
- `pkg/` - Public packages (invkmod, invkfile, platform).
- `modules/` - Sample invowk modules for validation and reference.

## Container Runtime Limitations

**The container runtime ONLY supports Linux containers.** This is a fundamental design limitation:

- **Supported images**: Debian-based images (e.g., `debian:stable-slim`).
- **NOT supported**: Alpine-based images (`alpine:*`) and Windows container images.

**Why no Alpine support:**
- There are many subtle gotchas in musl-based environments.
- We prioritize reliability over image size.

**Why no Windows container support:**
- They're rarely used and would introduce too much extra complexity to Invowk's auto-provisioning logic (which attaches an ephemeral image layer containing the `invowk` binary and the needed `invkfiles` and `invkmods` to the user-specified image/containerfile when the container runtime is used)

**When writing tests, documentation, or examples:**
- Always use `debian:stable-slim` as the reference container image.
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
