# C4 Container Diagram (C2)

This diagram zooms into Invowk to show its internal containers - the major applications, components, and data stores that make up the system.

> **Note**: In C4 terminology, "container" refers to a separately runnable/deployable unit (not Docker containers). Since Invowk is a single CLI binary, we show the major internal components as logical containers.

## Diagram

![C4 Container Diagram](../diagrams/rendered/c4/container.svg)

## Internal Components

### Entry Points

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **CLI Commands** | Go/Cobra | Entry points for all user interactions: `cmd`, `init`, `config`, `module`, `tui`, `completion` subcommands |

### Core Engine

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **Discovery Engine** | Go | Finds `invkfile.cue` and `*.invkmod` directories with precedence ordering. Builds unified command tree. |
| **Configuration Manager** | Go/Viper+CUE | Loads config from `~/.config/invowk/config.cue`. Validates against CUE schema. |
| **CUE Parser** | Go/cuelang | Implements 3-step parsing: compile schema → unify with data → decode to Go structs. Provides rich error messages. |
| **Module Resolver** | Go | Resolves Git-based dependencies. Manages cache at `~/.cache/invowk/modules/`. Handles lock files for reproducibility. |

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
| **Invkfiles** | CUE | `./invkfile.cue`, configured includes | Command definitions with implementations |
| **Modules** | Directories | `*.invkmod/` | Packaged commands with `invkmod.cue` metadata |
| **Module Cache** | Filesystem | `~/.cache/invowk/modules/` | Cached Git-fetched remote modules |

## Component Interactions

### Command Execution Flow

1. User invokes `invowk cmd <name>`
2. **CLI Commands** receives request
3. **Discovery Engine** finds all available commands
4. **CUE Parser** parses `invkfile.cue` and module files
5. Command is matched, runtime is selected
6. Appropriate **Runtime** executes the command
7. For containers: **Image Provisioner** prepares the environment

### Configuration Loading

1. **Configuration Manager** checks for config file
2. **CUE Parser** validates against schema
3. Config values influence runtime selection and behavior

### Module Resolution

1. **Discovery Engine** finds module requirements
2. **Module Resolver** checks cache, fetches from Git if needed
3. Dependencies are resolved transitively
4. Commands from required modules become available

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
