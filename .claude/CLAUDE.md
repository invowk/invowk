# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules, which are filesystem directories named as `<module-id>.invkmod` (preferably using the RDNS convention) that contain:
  - an `invkmod.cue` file
  - an `invkfile.cue` file

  Modules can require other modules as dependencies, which is how Invowk effectively provides modularity and `cmd` re-use for users. Additionally, modules also serve as a means to bundle scripts and ad-hoc files required for `cmd` execution.

  The only guarantee Invowk provides about cross `cmd`/module visibility is that `cmds` from a given module (e.g: `module foo`) that requires another module (e.g.: `module bar`) will be able to see/call `cmds` from the required module -- or, in other words, even though transitive dependencies are supported, only first-level dependencies are effectively exposed to the caller (e.g.: `cmds` from `module foo` will be able to see/call `cmds` from `module bar`, but not from the dependencies of `module bar`).

## Rules for Agents (Critical)

**CRITICAL:** Whenever possible and appropriate, multiple Tasks and Subagents must be used.

**CRITICAL:** The files in `.claude/rules/` define the authoritative rules for agents. EVERYTIME there is ANY change to files/rules inside `.claude/rules` (new file, file rename, file removed, etc.), the index/sync map in this file MUST be updated accordingly.

**Rules Index / Sync Map (must match `.claude/rules/`):**
- [`.claude/rules/checklist.md`](.claude/rules/checklist.md) - Pre-completion verification steps.
- [`.claude/rules/commands.md`](.claude/rules/commands.md) - Build, test, and release commands.
- [`.claude/rules/cue-patterns.md`](.claude/rules/cue-patterns.md) - CUE schema patterns, string validation, common pitfalls.
- [`.claude/rules/general-rules.md`](.claude/rules/general-rules.md) - Instruction priority, code quality, documentation.
- [`.claude/rules/git.md`](.claude/rules/git.md) - Commit signing, squash merge, message format.
- [`.claude/rules/go-patterns.md`](.claude/rules/go-patterns.md) - Go style, naming, errors, interfaces.
- [`.claude/rules/licensing.md`](.claude/rules/licensing.md) - SPDX headers and MPL-2.0 rules.
- [`.claude/rules/package-design.md`](.claude/rules/package-design.md) - Package boundaries and module design.
- [`.claude/rules/testing.md`](.claude/rules/testing.md) - Test patterns, cross-platform testing, skipOnWindows.
- [`.claude/rules/windows.md`](.claude/rules/windows.md) - Windows-specific constraints and guidance.

**Skills Index (`.claude/skills/`):**

Skills provide domain-specific procedural guidance. They are invoked when working on specific components.

- [`.claude/skills/cue/`](.claude/skills/cue/) - CUE schema parsing, 3-step parse flow, validation matrix, schema sync tests.
- [`.claude/skills/docs/`](.claude/skills/docs/) - Documentation workflow and Docusaurus website development.
- [`.claude/skills/invowk-schema/`](.claude/skills/invowk-schema/) - Invkfile/invkmod schema guidelines, cross-platform runtime patterns.
- [`.claude/skills/server/`](.claude/skills/server/) - Server state machine pattern for SSH and TUI servers.
- [`.claude/skills/shell/`](.claude/skills/shell/) - Shell runtime rules for mvdan/sh virtual shell.
- [`.claude/skills/testing/`](.claude/skills/testing/) - Testing patterns, testscript CLI tests, race conditions, TUI/container testing.
- [`.claude/skills/uroot/`](.claude/skills/uroot/) - u-root utility implementation patterns.

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
- `github.com/rogpeppe/go-internal/testscript` - CLI integration tests.

See `go.mod` for exact versions. Schema sync tests verify Go struct tags match CUE schema fields at CI time.
