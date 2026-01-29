# Invowk Project Overview

## What is Invowk?

Invowk is a **dynamically extensible command runner** (similar to `just`, `task`, and `mise`) written in **Go 1.25+**. It provides a flexible way to define and execute commands with multiple runtime options.

## Core Concepts

### Execution Runtimes
- **Native shell** - Uses host system shell (bash on Unix, PowerShell on Windows)
- **Virtual shell** - Uses `mvdan/sh`, a pure Go POSIX shell interpreter (cross-platform)
- **Container** - Docker/Podman execution (Linux containers only, Debian-based images)

### User-Defined Commands (`cmds`)
- Defined in `invkfile.cue` files using CUE format
- Available under the `invowk cmd` namespace
- Support platforms, runtimes, args, flags, dependencies, and environment variables

### User-Defined Modules
- Directories named `<module-id>.invkmod` (RDNS convention preferred)
- Contain `invkmod.cue` (metadata) and `invkfile.cue` (commands)
- Support dependencies on other modules (first-level only exposed to callers)
- Bundle scripts and files for command execution

## Key Dependencies
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `cuelang.org/go` - CUE language for config/schema
- `github.com/charmbracelet/*` - TUI components (lipgloss, bubbletea, huh)
- `mvdan.cc/sh/v3` - Virtual shell implementation

## Container Runtime Limitations
- **Only Linux containers supported**
- Use `debian:stable-slim` as reference image
- **No Alpine support** (musl gotchas)
- **No Windows containers** (complexity for auto-provisioning)
