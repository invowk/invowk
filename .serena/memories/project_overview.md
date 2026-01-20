# Project Overview: Invowk

## Purpose
Invowk is a dynamically extensible command runner (similar to `just`, `task`, and `mise`) written in Go 1.25+. It supports multiple execution runtimes:
- **Native shell**: Execute on host shell
- **Virtual shell**: Using mvdan/sh for portable execution
- **Containerized execution**: Docker/Podman support

## Key Concepts
- **Commands (`cmds`)**: User-defined commands in `invkfile.cue` files (CUE format)
- **Modules (`modules`)**: Modular directories named `<module-id>.invkmod` containing:
  - `invkmod.cue` - metadata
  - `invkfile.cue` - commands
  - Scripts and ad-hoc files for cmd execution

## Tech Stack
- **Language**: Go 1.25+
- **CLI Framework**: Cobra
- **Configuration**: Viper + CUE schemas
- **TUI**: Charm libraries (bubbletea, huh, lipgloss, glamour)
- **Virtual Shell**: mvdan/sh v3
- **Container**: Docker/Podman (Linux containers only, Debian-based)

## Key Dependencies
- `github.com/spf13/cobra` - CLI
- `github.com/spf13/viper` - Config
- `cuelang.org/go` - CUE language
- `github.com/charmbracelet/*` - TUI components
- `mvdan.cc/sh/v3` - Virtual shell
- `github.com/go-git/go-git/v5` - Git operations

## License
Eclipse Public License 2.0 (EPL-2.0)
