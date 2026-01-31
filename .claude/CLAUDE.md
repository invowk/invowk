# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules, which are filesystem directories named as `<module-id>.invkmod` (preferably using the RDNS convention) that contain:
  - an `invkmod.cue` file
  - an `invkfile.cue` file

  Modules can require other modules as dependencies, which is how Invowk effectively provides modularity and `cmd` re-use for users. Additionally, modules also serve as a means to bundle scripts and ad-hoc files required for `cmd` execution.

  The only guarantee Invowk provides about cross `cmd`/module visibility is that `cmds` from a given module (e.g: `module foo`) that requires another module (e.g.: `module bar`) will be able to see/call `cmds` from the required module -- or, in other words, even though transitive dependencies are supported, only first-level dependencies are effectively exposed to the caller (e.g.: `cmds` from `module foo` will be able to see/call `cmds` from `module bar`, but not from the dependencies of `module bar`).

## Agentic Context Discipline & Subagent Policy

**CRITICAL:** Subagents are the **PRIMARY MECHANISM** for complex and/or long exploration and implementation work.

### Use Subagents (model=opus) by default for/when:

- Multi-file reads/edits (≥3 files) or cross-surface edits (e.g.: "frontend" + "backend" + schemas + data etc.), *ALWAYS* launching 1 subagent per file to be read/edited
- Research-heavy tasks (code exploration, code reviews, audits, schema analysis, migration planning)
- Any step that might consume >20% of context budget *UNLESS* it's orchestration/planning work.

### Do NOT use Subagents by default for/when:

- Orchestration, planning, or any decision-making activity/step. Instead, the main agent must perform those activities/steps based on the information gathered by the subagents.

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
- `pkg/` - Public packages (invkmod, invkfile).
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

## Active Technologies
- N/A (no persistent storage required) (005-uroot-utils)
- Go 1.25+ + CUE v0.15.3, Cobra, Viper, Bubble Tea, mvdan/sh (006-go-codebase-audit)
- N/A (no persistent storage) (006-go-codebase-audit)
- Go 1.25+ with CUE v0.15.3 + Cobra, Viper, Bubble Tea, Lip Gloss, mvdan/sh (007-pkg-structure-audit)
- N/A (file-based CUE configuration only) (007-pkg-structure-audit)

**Core Stack**:
- Go 1.25+ with `cuelang.org/go v0.15.3` (CUE schemas and validation)
- `github.com/spf13/cobra` (CLI framework)
- `github.com/spf13/viper` (configuration management)
- `github.com/charmbracelet/*` (TUI: lipgloss, bubbletea, huh)
- `mvdan.cc/sh/v3` (virtual shell runtime)

**Testing**:
- Go stdlib `testing`
- `github.com/rogpeppe/go-internal/testscript` (CLI integration tests)

**Configuration**:
- File-based CUE configuration files (`invkfile.cue`, `invkmod.cue`, `config.cue`)
- Schema sync tests verify Go struct tags match CUE schema fields at CI time

## Recent Changes

- 006-go-codebase-audit: Large file splits (>800 lines → <600 lines), extracted `internal/core/serverbase/` and `internal/cueutil/` packages, container engine base abstraction, ActionableError type, CUE schema validation constraints
- 004-cue-lib-optimization: CUE library usage patterns documented in `.claude/rules/cue.md`, schema sync tests added for all CUE-parsed types, file size guards added to parse functions
- 003-test-suite-audit: Comprehensive test suite improvements, testscript CLI tests
- 002-codebase-cleanup-audit: Initial codebase cleanup and standardization
