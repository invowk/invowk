# C4 Container Diagram (C2)

This diagram zooms into Invowk to show its internal containers - the major applications, components, and data stores that make up the system.

> **Note**: In C4 terminology, "container" refers to a separately runnable/deployable unit (not Docker containers). Since Invowk is a single CLI binary, we show the major internal components as logical containers.

## Diagram

![C4 Container Diagram](../diagrams/rendered/c4/container.svg)

## Internal Components

### Entry Points

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **CLI Commands** | Go/Cobra | Entry points for all user interactions: `cmd`, `init`, `config`, `module`, `tui`, `validate`, `completion`, `audit`, `agent` subcommands |

### Core Engine

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **Command Service** | Go | Hexagonal domain service (`internal/app/commandsvc/`) orchestrating command execution. Receives requests from CLI, coordinates discovery, validation, SSH lifecycle, and runtime dispatch. Returns typed results/errors; CLI adapter applies rendering. |
| **Dependency Validator** | Go | Dependency validation domain (`internal/app/deps/`). Checks root, command, and implementation dependencies on the host, plus selected container runtime dependencies inside the container. |
| **Execution Context Builder** | Go | Runtime selection and execution context construction (`internal/app/execute/`). Produces the selected runtime and `runtime.ExecutionContext`; `internal/app/commandsvc` dispatches through `runtime.Registry`. |
| **Discovery Engine** | Go | Finds `invowkfile.cue` and `*.invowkmod` directories with precedence ordering. Builds unified command tree. |
| **Configuration Manager** | Go/CUE | Loads config from `~/.config/invowk/config.cue`. Validates against CUE schema. |
| **CUE Parser** | Go/cuelang | Implements 3-step parsing: compile schema → unify with data → decode to Go structs. Provides rich error messages. |
| **Module Resolver** | Go | Used by module commands such as `invowk module sync` to resolve Git-based dependencies. Manages cache at `~/.invowk/modules/`. Handles lock files for reproducibility. |
| **Watch Engine** | Go | Monitors file system for changes. Debounces change events and triggers command re-execution for `--ivk-watch` mode. |
| **Audit Scanner** | Go | Security scanning of module system (`internal/audit/`). Detects supply-chain risks, path traversal, symlink abuse, and environment variable injection. Supports `--llm` flag for LLM-powered analysis. |
| **Agent Command Authoring** | Go | LLM-assisted command authoring (`internal/agentcmd/`, `internal/llm/`). Renders agent prompts, resolves configured LLM backends, validates generated CUE, and patches `invowkfile.cue`. |

### Runtimes

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **Native Runtime** | Go | Executes commands via host shell (`bash`/`sh` on Unix, `PowerShell` on Windows). Fastest option. |
| **Virtual Runtime** | Go/mvdan-sh | Embedded POSIX shell interpreter. Includes u-root builtins for portability. No host shell dependency. |
| **Container Runtime** | Go | Executes commands inside Docker/Podman containers. Provides isolation and reproducibility. |

### Container Infrastructure

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **Container Engine Abstraction** | Go | Unified interface for Docker and Podman. Auto-detects available engine with fallback. |
| **Image Provisioner** | Go | Creates ephemeral image layers containing invowk binary and required modules. Enables seamless container execution. |

### Servers

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **SSH Server** | Go/Wish | Token-based SSH server for container-to-host callbacks. Enables `enable_host_ssh` feature. |
| **TUI Server** | Go/Bubble Tea | HTTP server handling TUI component requests from child processes. Enables interactive prompts in any runtime. |

## Data Stores

| Store | Format | Location | Purpose |
|-------|--------|----------|---------|
| **Config File** | CUE | `~/.config/invowk/config.cue` | User preferences: container engine, includes, etc. |
| **Invowkfiles** | CUE | `./invowkfile.cue`, configured includes | Command definitions with implementations |
| **Modules** | Directories | `*.invowkmod/` | Packaged commands with `invowkmod.cue` metadata |
| **Module Cache** | Filesystem | `~/.invowk/modules/` | Cached Git-fetched remote modules |

## Component Interactions

### Command Execution Flow

1. User invokes `invowk cmd <name>`
2. **CLI Commands** receives request, builds a service request
3. **Command Service** orchestrates the execution pipeline
4. **Discovery Engine** finds all available commands
5. **CUE Parser** parses `invowkfile.cue` and module files
6. **Command Service** matches command, selects runtime, validates dependencies
7. Appropriate **Runtime** executes the command
8. For containers: **Image Provisioner** prepares the environment

### Configuration Loading

1. **Configuration Manager** checks for config file
2. **CUE Parser** validates against schema
3. Config values influence runtime selection and behavior

### Module Resolution

1. **Discovery Engine** reads local, sibling, configured, and vendored module paths only
2. **Module Resolver** is used by module commands such as `invowk module sync` to check cache, fetch from Git when needed, and update the lock file
3. Dependencies use the explicit-only model (every transitive dep must be declared in root `invowkmod.cue`)
4. Commands from synchronized and discoverable modules become available

## Design Rationale

### Why Three Runtimes?

| Runtime | Use Case | Trade-off |
|---------|----------|-----------|
| Native | Speed, full shell features | Platform-dependent |
| Virtual | Portability, no shell dependency | Limited shell features |
| Container | Isolation, reproducibility | Overhead, Linux only |

### Why Separate Servers?

- **SSH Server**: Enables commands inside containers to call back to the host (e.g., for secrets management)
- **TUI Server**: Allows any subprocess (native, virtual, container) to request interactive UI components

### Why CUE for Configuration?

- Schema validation built-in
- Type checking before runtime
- Composable configurations
- Better error messages than YAML/JSON

## Related Diagrams

- [C4 Context Diagram (C1)](./c4-context.md) - System boundaries and external actors
- [Command Execution Sequence](./sequence-execution.md) - Temporal flow of command execution
- [Runtime Selection Flowchart](./flowchart-runtime-selection.md) - How runtimes are chosen
- [Discovery Precedence Flowchart](./flowchart-discovery.md) - How commands are discovered
