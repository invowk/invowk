# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules, which are filesystem directories named as `<module-id>.invkmod` (preferably using the RDNS convention) that contain:
  - an `invkmod.cue` file
  - an `invkfile.cue` file

  Modules can require other modules as dependencies, which is how Invowk effectively provides modularity and `cmd` re-use for users. Additionally, modules also serve as a means to bundle scripts and ad-hoc files required for `cmd` execution.

  The only guarantee Invowk provides about cross `cmd`/module visibility is that `cmds` from a given module (e.g: `module foo`) that requires another module (e.g.: `module bar`) will be able to see/call `cmds` from the required module -- or, in other words, even though transitive dependencies are supported, only first-level dependencies are effectively exposed to the caller (e.g.: `cmds` from `module foo` will be able to see/call `cmds` from `module bar`, but not from the dependencies of `module bar`).

## Agentic Context Discipline & Subagent Policy

**CRITICAL:** Subagents are **MANDATORY** for ALL code reads/explorations AND ALL code edits. The main agent must NEVER directly read source code files or edit files—these operations MUST be delegated to subagents.

### Mandatory Subagent Usage

**ALL of the following operations MUST be delegated to subagents:**

1. **Code Reads/Explorations**: ANY file read for understanding code (even a single file)
2. **Code Edits**: ANY file modification (even a single file, even a single line)
3. **Research Tasks**: Code exploration, reviews, audits, schema analysis, migration planning
4. **Multi-Surface Work**: Cross-domain operations (e.g., "frontend" + "backend" + schemas)

**Parallelization Rule**: When multiple files need to be read or edited, launch **1 subagent per file** in parallel (single message, multiple Task tool calls).

### Sequential Task Handling

When tasks have dependencies and must be executed in order:

1. **STILL use subagents** for each task—sequential dependency does NOT exempt from subagent requirement
2. **Wait for completion** of each subagent before launching the next dependent one
3. **Main agent orchestrates** the sequence but NEVER performs the actual reads/edits directly

**Example (sequential edits where B depends on A):**
```
1. Launch subagent A to edit file X → wait for completion
2. Launch subagent B to edit file Y (depends on A's changes) → wait for completion
3. Main agent synthesizes results and reports to user
```

### gopls Requirement for All Code Operations

**CRITICAL:** All agents (main and subagents) MUST use `gopls` (Go Language Server via the `gopls-lsp` MCP plugin) for ALL code operations in this Go codebase:

| Operation | gopls Tool | NOT Allowed |
|-----------|------------|-------------|
| **Symbol lookup** | `gopls: definition`, `gopls: references` | Manual grep for function names |
| **Code exploration** | `gopls: hover`, `gopls: documentSymbol` | Reading files and scanning by eye |
| **Finding usages** | `gopls: references`, `gopls: implementation` | Grep for symbol names |
| **Rename/refactor** | `gopls: rename` | Find-and-replace across files |
| **Understanding types** | `gopls: hover`, `gopls: typeDefinition` | Inferring from context |
| **Finding call sites** | `gopls: callHierarchy` | Grep for function calls |

**Why gopls is mandatory:**
1. **Semantic accuracy** — gopls understands Go's type system, imports, and scopes; text search does not
2. **Refactor safety** — `gopls: rename` handles shadowing, package boundaries, and test files correctly
3. **Cross-package navigation** — gopls resolves imports and follows symbols across package boundaries
4. **Interface implementations** — gopls finds all implementations of an interface; grep cannot

**Exceptions (gopls NOT required):**
- Quick file discovery via `Glob` (e.g., "find all `*_test.go` files")
- Searching for string literals, comments, or non-Go content
- Reading configuration files (`.cue`, `.json`, `.yaml`, `.md`)

**Agent Selection:**
- For **code exploration tasks**, the `code-explorer` agent (`feature-dev:code-explorer`) MUST be used instead of the built-in `Task(Explore)` agent
- All subagents performing Go code operations MUST have access to and use gopls tools

### What the Main Agent Does Directly (NO subagents)

The main agent performs these activities directly—do NOT delegate:

- **Orchestration**: Coordinating subagent launches, sequencing, and result synthesis
- **Planning**: Designing implementation approaches, creating plans
- **Decision-Making**: Choosing between alternatives based on subagent findings
- **User Communication**: Explaining results, asking clarifying questions
- **Lightweight Lookups**: Quick glob/grep for file discovery (NOT for reading file contents)

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
