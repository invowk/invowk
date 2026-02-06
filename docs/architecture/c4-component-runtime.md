# C4 Component Diagram: Runtime (C3)

This diagram zooms into the **Runtime** container from the [C2 Container Diagram](./c4-container.md) to show its internal components -- the interfaces, concrete implementations, and supporting types that make up Invowk's command execution layer.

> **Note**: The runtime package (`internal/runtime/`) is responsible for executing user-defined commands. It provides three interchangeable execution backends behind a common interface hierarchy, a registry for runtime dispatch, and a structured execution context that decouples I/O, environment, and TUI concerns.

## Diagram

![C3 Component Diagram - Runtime](../diagrams/rendered/c4/component-runtime.svg)

## Interfaces

| Interface | Methods | Role |
|-----------|---------|------|
| **Runtime** | `Name()`, `Execute()`, `Available()`, `Validate()` | Core execution contract. All runtimes implement this. `Execute` returns a `*Result` with both exit code and error -- non-zero exit code without error is a normal process exit; error indicates infrastructure failure. |
| **CapturingRuntime** | `ExecuteCapture()` | Optional output capture capability. Returns a `*Result` with `Output` and `ErrOutput` fields populated. Does not embed `Runtime`. |
| **InteractiveRuntime** | `SupportsInteractive()`, `PrepareInteractive()` | PTY attachment capability. Embeds `Runtime`. Returns a `PreparedCommand` with an `exec.Cmd` ready for PTY attachment and a cleanup function. |
| **EnvBuilder** | `Build()` | Environment variable construction following a 10-level precedence hierarchy (host env at level 1 through `--env-var` CLI flags at level 10). |

## Implementations

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **NativeRuntime** | Go | Executes commands via the host shell (`bash`/`sh` on Unix, `PowerShell` on Windows). Fastest option. Configurable shell override. Implements Runtime, CapturingRuntime, and InteractiveRuntime. |
| **VirtualRuntime** | Go/mvdan-sh | Embedded POSIX shell interpreter with optional u-root built-in utilities. No host shell dependency. Spawns a subprocess of itself for PTY-based interactive mode. Implements Runtime, CapturingRuntime, and InteractiveRuntime. |
| **ContainerRuntime** | Go | Executes commands inside Docker/Podman containers. Depends on `container.Engine`, `provision.LayerProvisioner`, `sshserver.Server`, and `config.Config`. Linux containers only. Implements Runtime, CapturingRuntime, and InteractiveRuntime. |
| **DefaultEnvBuilder** | Go | Standard 10-level precedence implementation: host env (filtered) -> root/command/impl env files -> root/command/impl env vars -> ExtraEnv -> runtime env files -> runtime env vars. |
| **MockEnvBuilder** | Go | Test helper that returns a fixed environment map. Enables testing runtimes in isolation without real file system access or env loading. |

## Supporting Types

| Type | Role |
|------|------|
| **Registry** | Map-based runtime dispatcher. Stores `RuntimeType -> Runtime` mappings. Provides `Get()`, `GetForContext()`, `Available()`, and `Execute()` (which chains validate-then-execute). |
| **RuntimeType** | String type identifying runtime variants: `"native"`, `"virtual"`, `"container"`. |
| **ExecutionContext** | Primary data structure for command execution. Composed of `IOContext`, `EnvContext`, and `TUIContext` sub-types plus command metadata, selected runtime/implementation, and execution ID. |
| **IOContext** | Groups I/O streams: `Stdout` (`io.Writer`), `Stderr` (`io.Writer`), `Stdin` (`io.Reader`). Factory functions `DefaultIO()` and `CaptureIO()` provide common configurations. |
| **EnvContext** | Groups environment configuration: `ExtraEnv` (INVOWK_FLAG_*, INVOWK_ARG_*), `RuntimeEnvVars` (--env-var), `RuntimeEnvFiles` (--env-file), and inheritance overrides (mode, allow, deny). |
| **TUIContext** | Groups TUI server connection details: `ServerURL` and `ServerToken`. Used to pass `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN` into command environments. |
| **Result** | Execution result: `ExitCode`, `Error`, `Output` (captured stdout), `ErrOutput` (captured stderr). `Success()` returns true when both exit code is 0 and error is nil. |
| **PreparedCommand** | Returned by `PrepareInteractive()`: contains an `exec.Cmd` ready for PTY attachment and an optional `Cleanup` function. |

## External Dependencies

| Dependency | Used By | Purpose |
|------------|---------|---------|
| `container.Engine` | ContainerRuntime | Unified Docker/Podman container engine abstraction |
| `provision.LayerProvisioner` | ContainerRuntime | Creates ephemeral image layers with invowk binary and modules |
| `sshserver.Server` | ContainerRuntime | Token-based SSH server for container-to-host callbacks |
| `config.Config` | ContainerRuntime | Application configuration (container engine preference, etc.) |
| `mvdan.cc/sh/v3` | VirtualRuntime | Embedded POSIX shell interpreter (syntax, interp, expand) |
| `internal/uroot` | VirtualRuntime | Built-in utilities for the virtual shell (cp, mv, cat, etc.) |
| `pkg/invkfile` | ExecutionContext | Command and Invkfile types, RuntimeMode, EnvInheritMode |

## Key Relationships

### Interface Segregation

The runtime package uses interface segregation to let callers depend only on the capabilities they need:

- **Runtime** is the base contract -- any caller that just needs to run a command accepts `Runtime`.
- **CapturingRuntime** is a standalone interface for output capture. Callers that need captured output can type-assert to it.
- **InteractiveRuntime** embeds `Runtime` and adds PTY support. The helper function `GetInteractiveRuntime()` combines type assertion with `SupportsInteractive()` capability check, returning nil if either fails.

All three concrete runtimes (Native, Virtual, Container) implement all three interfaces. This is verified at compile time with interface satisfaction checks in `container.go`.

### Registry Dispatch

The `Registry` decouples runtime selection from execution:

1. CLI layer resolves the `RuntimeType` from command defaults or `--runtime` flag.
2. `Registry.GetForContext()` looks up the matching `Runtime` from its internal map.
3. `Registry.Execute()` chains: get runtime -> check availability -> validate -> execute.

This pattern means the execution pipeline never needs to know which concrete runtime is in use.

### ExecutionContext Composition

`ExecutionContext` uses composition of three focused sub-types rather than a flat struct:

- **IOContext** -- I/O streams, easily swapped between real (`DefaultIO()`) and capture (`CaptureIO()`) modes.
- **EnvContext** -- Environment variable configuration, including inheritance overrides from CLI flags.
- **TUIContext** -- TUI server connection details, zero-value means "not configured".

This separation enables focused testing: you can test environment building independently of I/O handling, or test TUI integration without constructing a full execution context.

### EnvBuilder 10-Level Precedence

The `EnvBuilder` interface abstracts environment construction. `DefaultEnvBuilder` applies a strict 10-level precedence (higher number wins):

| Level | Source | Example |
|-------|--------|---------|
| 1 | Host environment (filtered by inherit mode) | `PATH`, `HOME` |
| 2 | Root-level `env.files` | `invkfile.cue` top-level dotenv |
| 3 | Command-level `env.files` | Per-command dotenv files |
| 4 | Implementation-level `env.files` | Platform-specific dotenv |
| 5 | Root-level `env.vars` | `invkfile.cue` top-level vars |
| 6 | Command-level `env.vars` | Per-command inline vars |
| 7 | Implementation-level `env.vars` | Platform-specific vars |
| 8 | ExtraEnv | `INVOWK_FLAG_*`, `INVOWK_ARG_*`, `ARGC`, `ARGn` |
| 9 | `--env-file` flag | CLI-specified dotenv files |
| 10 | `--env-var` flag | CLI-specified key=value pairs (highest priority) |

Host environment filtering itself has a 3-level precedence chain: default mode -> invkfile runtime config overrides -> CLI flag overrides. Three modes are supported: `none` (empty), `allow` (allowlist), and `all` (everything minus denylist).

## Design Rationale

### Why Interface Segregation?

Callers depend only on the capabilities they actually use. A dependency resolver that just checks availability calls `Runtime.Available()`. The output capture system asserts `CapturingRuntime` only when capture is needed. The TUI system checks for `InteractiveRuntime` only for interactive commands. This prevents unnecessary coupling and makes the package easier to extend with new runtimes that may not support all capabilities.

### Why Composition for ExecutionContext?

A flat struct with 15+ fields would be harder to test and harder to read. By grouping into `IOContext`, `EnvContext`, and `TUIContext`, each sub-type can be constructed and tested independently. Factory functions like `DefaultIO()`, `CaptureIO()`, and `DefaultEnv()` provide common configurations without requiring callers to set every field.

### Why an EnvBuilder Interface?

Environment building involves file system access (loading dotenv files), host environment inspection, and a complex precedence hierarchy. `MockEnvBuilder` lets tests focus on runtime execution logic by providing a fixed environment map, eliminating flaky tests from file system state and simplifying test setup. It also opens the door for alternative strategies (e.g., a caching env builder) without modifying runtime implementations.

### Why a Registry?

Direct construction of runtimes would couple the CLI layer to each concrete type's constructor and dependencies (e.g., ContainerRuntime needs `config.Config`, `container.Engine`, `provision.LayerProvisioner`). The Registry pattern lets the application bootstrap wire all runtimes once at startup, and the execution pipeline simply asks for a runtime by type. Adding a new runtime means registering it -- no changes to the execution pipeline.

## Related Diagrams

- [C4 Context Diagram (C1)](./c4-context.md) - System boundaries and external actors
- [C4 Container Diagram (C2)](./c4-container.md) - Major internal components
- [C4 Component: Container (C3)](./c4-component-container.md) - Container engine internals
- [Command Execution Sequence](./sequence-execution.md) - Temporal flow of command execution
- [Runtime Selection Flowchart](./flowchart-runtime-selection.md) - How runtimes are chosen
