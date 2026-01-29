# Invowk Architecture

## Directory Structure

```
cmd/invowk/          - CLI commands (Cobra-based)
  cmd.go             - Main command implementation (~96KB)
  module.go          - Module commands
  root.go            - Root command setup
  tui_*.go           - TUI helper commands

internal/            - Private packages
  config/            - Viper + CUE configuration
  container/         - Docker/Podman runtime
  discovery/         - Multi-source command discovery
  issue/             - Issue tracking utilities
  platform/          - Platform detection
  runtime/           - Runtime interface + implementations
  sshserver/         - SSH server for container access
  testutil/          - Test utilities
  tui/               - Bubbletea TUI components
  tuiserver/         - TUI server mode

pkg/                 - Public packages
  invkfile/          - invkfile.cue schema + parsing
  invkmod/           - invkmod.cue schema + parsing

modules/             - Sample invkmod modules for validation
tests/               - Integration tests (testscript)
vhs/                 - VHS demo recordings
website/             - Documentation website
```

## Data Flow

```
invkfile.cue → CUE Parser → pkg/invkfile → Runtime Selection → Execution
                                              |
                              +---------------+---------------+
                              |               |               |
                           Native         Virtual        Container
                        (host shell)    (mvdan/sh)    (Docker/Podman)
```

## Schema Locations
- `pkg/invkfile/invkfile_schema.cue` - Command definitions
- `pkg/invkmod/invkmod_schema.cue` - Module metadata
- `internal/config/config_schema.cue` - App configuration

## Server Pattern
Long-running components follow a state machine pattern:
`Created → Starting → Running → Stopping → Stopped` (or `Failed`)
Reference implementation: `internal/sshserver/server.go`
