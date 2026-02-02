# Package Map: Go Package Structure & Organization Audit

**Date**: 2026-01-30
**Branch**: `007-pkg-structure-audit`

This document defines package responsibilities and boundaries for the restructured codebase.

---

## Package Hierarchy

```
invowk-cli/
├── cmd/invowk/          # CLI entry point and commands
├── internal/            # Private implementation packages
│   ├── config/          # Configuration management
│   ├── container/       # Container engine abstraction
│   ├── core/            # Shared infrastructure
│   │   └── serverbase/  # Server lifecycle state machine
│   ├── cueutil/         # CUE parsing utilities
│   ├── discovery/       # Module and command discovery
│   ├── issue/           # Actionable error handling
│   ├── runtime/         # Execution runtimes
│   ├── sshserver/       # SSH callback server
│   ├── testutil/        # Test utilities
│   │   └── invkfiletest/ # Invkfile-specific test helpers
│   ├── tui/             # Terminal UI components
│   ├── tuiserver/       # TUI HTTP server
│   └── uroot/           # u-root utilities integration
└── pkg/                 # Public API packages
    ├── invkfile/        # Invkfile types and parsing
    ├── invkmod/         # Module types and operations
    └── platform/        # Cross-platform utilities
```

---

## Package Responsibilities

### cmd/invowk

**Purpose**: CLI entry point and command definitions using Cobra.

**Responsibilities**:
- Define CLI commands and subcommands
- Parse command-line arguments and flags
- Coordinate between internal packages
- Format and display output to users

**Key Files After Restructure**:
| File | Responsibility |
|------|----------------|
| `root.go` | Root command, global flags, initialization |
| `styles.go` | **NEW**: Shared color palette and style definitions |
| `cmd_execute.go` | `invowk cmd` subcommand (will be split) |
| `cmd_discovery.go` | Discovery-related commands |
| `module.go` | `invowk module` subcommand |
| `config.go` | `invowk config` subcommand |

**Dependencies**: Uses internal/* packages; does not export types.

---

### internal/config

**Purpose**: Application configuration management using Viper with CUE format.

**Responsibilities**:
- Load configuration from CUE files
- Provide typed access to config values
- Manage config file location discovery
- Validate configuration against schema

**Dependencies**: cueutil, container (for engine config types)

---

### internal/container

**Purpose**: Abstraction layer for container runtimes (Docker/Podman).

**Responsibilities**:
- Detect available container engines
- Execute container operations (run, exec, build, remove)
- Handle image management
- Auto-provision invowk binary into containers

**Key Types**:
- `Engine` interface - common operations
- `BaseCLIEngine` - shared CLI argument building
- `DockerEngine`, `PodmanEngine` - engine-specific implementations

**Status**: Already well-consolidated; no changes needed.

---

### internal/core/serverbase

**Purpose**: Shared state machine for long-running server components.

**Responsibilities**:
- Manage server lifecycle states (Created → Starting → Running → Stopping → Stopped)
- Provide atomic state reads and mutex-protected transitions
- Track background goroutines with WaitGroup
- Handle context-based cancellation

**Used By**: sshserver, tuiserver

**Status**: Has implementation; needs doc.go only.

---

### internal/cueutil

**Purpose**: CUE parsing utilities implementing the 3-step parse pattern.

**Responsibilities**:
- Compile CUE schemas
- Validate user data against schemas
- Format CUE errors with JSON paths
- Handle file size guards

**Note**: Intentionally internal. `pkg/` packages use this for parsing implementation but don't expose it to external consumers.

---

### internal/discovery

**Purpose**: Find and load invkfiles and modules from various locations.

**Responsibilities**:
- Search for invkfile.cue in current directory and search paths
- Discover modules in module directories
- Build unified command tree from discovered files
- Handle command precedence and shadowing

**Key Files After Restructure**:
| File | Responsibility |
|------|----------------|
| `discovery.go` | Core discovery orchestration (~400 lines) |
| `discovery_files.go` | **NEW**: File-level discovery logic |
| `discovery_commands.go` | **NEW**: Command aggregation and tree building |

**Note**: Per `.claude/rules/package-design.md`, file discovery + command aggregation are tightly coupled and stay in one package with internal file separation.

---

### internal/issue

**Purpose**: Actionable error handling with user-friendly messages.

**Responsibilities**:
- Define `ActionableError` type with remediation steps
- Format errors with Markdown guidance
- Provide structured error information for TUI display

**Status**: Has implementation; needs doc.go only.

---

### internal/runtime

**Purpose**: Command execution runtime interface and implementations.

**Responsibilities**:
- Define `Runtime` interface
- Implement native, virtual, and container runtimes
- Handle interpreter resolution
- Manage execution context

**Key Files After Restructure**:
| File | Responsibility |
|------|----------------|
| `runtime.go` | Interface definitions, common types |
| `native.go` | Native shell runtime |
| `virtual.go` | mvdan/sh virtual shell runtime |
| `container.go` | Container runtime orchestration |
| `container_exec.go` | Container execution (refactored for deduplication) |
| `container_helpers.go` | **NEW**: Extracted `prepareContainerExecution()` helper |

---

### internal/sshserver

**Purpose**: SSH server for container callback authentication.

**Responsibilities**:
- Start SSH server on dynamic port
- Handle container-to-host authentication
- Manage SSH key pairs
- Track session lifecycle

**Key Files After Restructure**:
| File | Responsibility |
|------|----------------|
| `server.go` | Core server implementation (~400 lines) |
| `server_lifecycle.go` | **NEW**: Start/Stop/Wait methods |
| `server_auth.go` | **NEW**: Authentication and key management |

---

### internal/testutil

**Purpose**: Shared test utilities and helpers.

**Responsibilities**:
- Provide `MustChdir`, `MustSetenv`, etc. helpers
- Define `Clock` interface for time mocking
- Implement `FakeClock` for deterministic tests
- Provide resource cleanup helpers

**Subpackage - internal/testutil/invkfiletest**:
- `NewTestCommand()` builder for test commands
- Avoids import cycles (separate from invkfile package)

---

### internal/tui

**Purpose**: Terminal UI components using Charm libraries.

**Responsibilities**:
- Implement Bubble Tea models for interactive prompts
- Provide choose, confirm, input, filter components
- Handle keyboard navigation
- Render styled output with Lip Gloss

**Status**: Has implementation; needs doc.go only.

---

### internal/tuiserver

**Purpose**: HTTP server for child process TUI requests.

**Responsibilities**:
- Listen for TUI component requests from containers/subprocesses
- Forward requests to parent Bubble Tea program
- Handle overlay rendering
- Manage request/response lifecycle

**Status**: Has implementation; needs doc.go only.

---

### internal/uroot

**Purpose**: u-root utilities integration for virtual shell.

**Responsibilities**:
- Provide built-in implementations of common utilities
- Enable portable shell scripts without external dependencies
- Stream I/O for all file operations

**Status**: Has doc.go; no changes needed.

---

### pkg/invkfile

**Purpose**: Types and parsing for invkfile.cue command definitions.

**Responsibilities**:
- Define `Invkfile`, `Command`, `Implementation` types
- Parse invkfile.cue files with CUE validation
- Select implementations based on runtime/platform
- Validate command structure (leaf-only args, etc.)

**Key Files After Restructure**:
| File | Responsibility |
|------|----------------|
| `doc.go` | **NEW**: Package documentation |
| `invkfile.go` | Core types |
| `parse.go` | Parsing entry points |
| `validation.go` | Refactored: Core validation logic (~400 lines) |
| `validation_runtime.go` | **NEW**: Runtime-specific validation |
| `validation_deps.go` | **NEW**: Dependency validation |
| `invkfile_validation.go` | May merge with validation.go or become validation_invkfile.go |

**Note**: Uses internal/cueutil for parsing implementation.

---

### pkg/invkmod

**Purpose**: Module types, parsing, and operations.

**Responsibilities**:
- Define `Invkmod`, `ModuleRequirement` types
- Parse invkmod.cue files
- Resolve module dependencies
- Handle module validation, packaging, vendoring

**Key Files After Restructure**:
| File | Responsibility |
|------|----------------|
| `doc.go` | Package documentation (exists) |
| `invkmod.go` | Core types |
| `resolver.go` | Refactored: Resolution orchestration (~400 lines) |
| `resolver_deps.go` | **NEW**: Dependency resolution logic |
| `resolver_cache.go` | **NEW**: Cache management |
| `operations_*.go` | Module operations (validate, create, package, vendor) |

---

### pkg/platform

**Purpose**: Cross-platform compatibility utilities.

**Responsibilities**:
- Detect current platform (Linux, macOS, Windows)
- Provide platform-specific path handling
- Map platform names to Go GOOS values

**Status**: Has doc.go; no changes needed.

---

## Dependency Graph

```
                    cmd/invowk
                        │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
   internal/tui   internal/discovery  internal/runtime
        │               │               │
        │               │               ├─────────┐
        ▼               ▼               ▼         ▼
   internal/tuiserver  pkg/invkfile  internal/container
        │                   │               │
        │                   ▼               │
        ▼           internal/cueutil        │
   internal/core/serverbase                 │
        ▲                                   │
        │                                   ▼
   internal/sshserver ◄─────────────────────┘
        │
        ▼
   internal/config
        │
        ▼
   internal/cueutil

Legend:
─────► imports
◄───── uses (reverse for context)
```

**Key Constraints**:
1. No circular dependencies
2. `pkg/` packages may import `internal/cueutil` (documented design choice)
3. `internal/` packages do not import `cmd/invowk`
4. `testutil` is imported only by `*_test.go` files

---

## Interface Contracts

### Runtime Interface (internal/runtime)

```go
type Runtime interface {
    Name() string
    Execute(ctx *ExecutionContext) *Result
    ExecuteCapture(ctx *ExecutionContext) *Result
    Available() bool
    Validate(ctx *ExecutionContext) error
}
```

### Container Engine Interface (internal/container)

```go
type Engine interface {
    Name() string
    Available(ctx context.Context) (bool, error)
    Run(ctx context.Context, opts RunOptions) (*RunResult, error)
    Exec(ctx context.Context, containerID string, cmd []string, opts ExecOptions) (*ExecResult, error)
    Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)
    Remove(ctx context.Context, containerID string, opts RemoveOptions) error
    ImageExists(ctx context.Context, image string) (bool, error)
}
```

### Clock Interface (internal/testutil)

```go
type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
    Since(t time.Time) time.Duration
}
```

### Command Source Interface (pkg/invkmod - future)

Per `.claude/rules/package-design.md`, if `invkmod` needs command information from `invkfile`:

```go
// CommandSource provides command definitions to a module.
// Implemented by invkfile.Invkfile.
type CommandSource interface {
    Commands() []CommandInfo
}

type CommandInfo struct {
    Name        string
    Description string
}
```

---

## File Size Targets

| Package | Current Max | Target Max |
|---------|-------------|------------|
| pkg/invkfile | 753 lines | <600 lines |
| pkg/invkmod | 726 lines | <600 lines |
| internal/discovery | 715 lines | <600 lines |
| cmd/invowk | 643 lines | <600 lines |
| internal/sshserver | 627 lines | <600 lines |
