# Overview

Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). From the user perspective, Invowk offers two key extensibility primitives:
- User-defined commands (called `cmds`), which are defined in `invkfile.cue` files using CUE format. `cmds` are made available under the reserved `invowk cmd` built-in command/namespace.
- User-defined modules (called `packs`), which are filesystem directories named as `<pack-id>.invkpack` (preferably using the RDNS convention) that contain: 
  - an `invkpack.cue` file 
  - an `invkfile.cue` file
  
  `packs` can require other `packs` as dependencies, which is how Invowk effectively provides modularity and `cmd` re-use for users. Additionally, `packs` also serve as a means to bundle scripts and ad-hoc files required for `cmd` execution.

  The only guarantee Invowk provides about cross `cmd`/`pack` visibility is that `cmds` from a given `pack` (e.g: `pack foo`) that requires another `pack` (e.g.: `pack bar`) will be able to see/call `cmds` from the required `pack` -- or, in other words, even though transitive dependencies are supported, only first-level dependencies are effectively exposed to the caller (e.g.: `cmds` from `pack foo` will be able to see/call `cmds` from `pack bar`, but not from the dependencies of `pack bar`).

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
  - `pkg/invkfile/invkpack_schema.cue` defines `invkpack` structure
  - `internal/config/config_schema.cue` defines config structure
- **Runtime Interface**: All runtimes implement the same interface in `internal/runtime/`.
- **TUI Components**: Built with Charm libraries (bubbletea, huh, lipgloss).

## Directory Layout

- `cmd/invowk/` - CLI commands using Cobra.
- `internal/` - Private packages (config, container, discovery, issue, runtime, sshserver, tui, tuiserver).
- `pkg/` - Public packages (pack, invkfile).
- `packs/` - Sample invowk packs for validation and reference.

## Container Runtime Limitations

**The container runtime ONLY supports Linux containers.** This is a fundamental design limitation:

- **Supported images**: Debian-based images (e.g., `debian:stable-slim`).
- **NOT supported**: Alpine-based images (`alpine:*`) and Windows container images.

**Why no Alpine support:**
- There are many subtle gotchas in musl-based environments.
- We prioritize reliability over image size.

**Why no Windows container support:**
- They're rarely used and would introduce too much extra complexity to Invowk's auto-provisioning logic (which attaches an ephemeral image layer containing the `invowk` binary and the needed `invkfiles` and `invkpacks` to the user-specified image/containerfile when the container runtime is used)

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
