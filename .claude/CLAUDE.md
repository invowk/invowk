# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules, which are filesystem directories named as `<module-id>.invkmod` (preferably using the RDNS convention) that contain:
  - an `invkmod.cue` file
  - an `invkfile.cue` file

  Modules can require other modules as dependencies, which is how Invowk effectively provides modularity and `cmd` re-use for users. Additionally, modules also serve as a means to bundle scripts and ad-hoc files required for `cmd` execution.

  The only guarantee Invowk provides about cross `cmd`/module visibility is that `cmds` from a given module (e.g: `module foo`) that requires another module (e.g.: `module bar`) will be able to see/call `cmds` from the required module -- or, in other words, even though transitive dependencies are supported, only first-level dependencies are effectively exposed to the caller (e.g.: `cmds` from `module foo` will be able to see/call `cmds` from `module bar`, but not from the dependencies of `module bar`).

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
- `internal/` - Private packages (config, container, discovery, issue, runtime, sshserver, tui, tuiserver).
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
- Go 1.25+ + Cobra (CLI), Viper (config), CUE (schemas), Charm libraries (TUI), mvdan/sh (virtual shell) (002-codebase-cleanup-audit)
- N/A (CLI tool, file-based configuration) (002-codebase-cleanup-audit)
- Go 1.25+ + stdlib `testing`, `github.com/charmbracelet/bubbletea` (TUI), `mvdan.cc/sh/v3` (virtual shell), `github.com/rogpeppe/go-internal/testscript` (CLI tests) (003-test-suite-audit)
- N/A (test infrastructure only) (003-test-suite-audit)

## Recent Changes
- 002-codebase-cleanup-audit: Added Go 1.25+ + Cobra (CLI), Viper (config), CUE (schemas), Charm libraries (TUI), mvdan/sh (virtual shell)
