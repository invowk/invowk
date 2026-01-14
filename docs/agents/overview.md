# Overview

Invowk is a dynamically extensible command runner (like `just`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). Commands are defined in `invkfile` files using CUE format.

## Architecture Overview

```
invkfile.cue -> CUE Parser -> pkg/invkfile -> Runtime Selection -> Execution
                                                  |
                                  +---------------+---------------+
                                  |               |               |
                               Native         Virtual        Container
                            (host shell)    (mvdan/sh)    (Docker/Podman)
```

- **CUE Schemas**: `pkg/invkfile/invkfile_schema.cue` defines invkfile structure, `internal/config/config_schema.cue` defines config.
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
- The container runtime executes scripts using `/bin/sh` which invokes `dash` or `bash`.
- Alpine uses BusyBox's `ash` shell which has subtle incompatibilities.
- Alpine lacks many standard GNU utilities that scripts may depend on.
- We prioritize reliability over image size.

**Why no Windows container support:**
- Scripts are executed with `/bin/sh -c` which doesn't exist in Windows containers.
- Windows containers use PowerShell or cmd.exe, not POSIX shells.
- The runtime architecture assumes a POSIX-compatible environment.

**When writing tests or examples:**
- Always use `debian:stable-slim` as the reference container image.
- Never use Alpine images in tests, examples, or documentation.
- Never use Windows container images (e.g., `mcr.microsoft.com/windows/*`).

## Key Dependencies

- `github.com/spf13/cobra` - CLI framework.
- `github.com/spf13/viper` - Configuration management.
- `cuelang.org/go` - CUE language support for configuration/schema.
- `github.com/charmbracelet/*` - TUI components (lipgloss, bubbletea, huh).
- `mvdan.cc/sh/v3` - Virtual shell implementation.
