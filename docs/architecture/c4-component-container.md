# C3 Component Diagram - Container Package

This diagram zooms into the **Container Engine Abstraction** container from the [C2 Container diagram](./c4-container.md) to show the internal components of the `internal/container` package. It reveals how Invowk provides a unified interface over Docker and Podman CLIs, handles Podman-specific concerns (SELinux, rootless), and transparently adapts to sandboxed environments (Flatpak, Snap).

## Diagram

![C3 Component Diagram - Container](../diagrams/rendered/c4/component-container.svg)

## Interface

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **Engine** | Go interface | 10-method contract for container operations: `Name`, `Available`, `Version`, `Build`, `Run`, `Remove`, `ImageExists`, `RemoveImage`, `BinaryPath`, `BuildRunArgs`. All factory functions return this type. |

## Implementations

| Component | Technology | Responsibility |
|-----------|------------|----------------|
| **BaseCLIEngine** | Go struct | Shared base for CLI-based engines. Holds `binaryPath`, `execCommand`, `volumeFormatter`, `runArgsTransformer`. Provides argument builders (`BuildArgs`, `RunArgs`, `ExecArgs`, `RemoveArgs`, `RemoveImageArgs`) and command execution (`RunCommand`, `RunCommandCombined`, `RunCommandStatus`, `RunCommandWithOutput`, `CreateCommand`). Constructed via `NewBaseCLIEngine(binaryPath, ...BaseCLIEngineOption)`. |
| **DockerEngine** | Go struct | Embeds `*BaseCLIEngine`. Implements `Engine`. Locates the `docker` binary via `exec.LookPath`. Delegates argument building and execution to the embedded base. Created via `NewDockerEngine()`. |
| **PodmanEngine** | Go struct | Embeds `*BaseCLIEngine`. Implements `Engine`. Searches for `podman` or `podman-remote` (fallback for immutable distros). Injects SELinux volume labels (`:z`/`:Z`) via `VolumeFormatFunc` and rootless user namespace (`--userns=keep-id`) via `RunArgsTransformer`. Created via `NewPodmanEngine()`. |
| **SandboxAwareEngine** | Go struct (decorator) | Wraps any `Engine`. Detects Flatpak/Snap sandboxes via `platform.DetectSandbox()` and prefixes commands with `flatpak-spawn --host` or `snap run --shell` so container CLI invocations target the host, not the sandbox. Returns the unwrapped engine when no sandbox is detected (zero overhead). |

## Functional Options

| Type | Signature | Purpose |
|------|-----------|---------|
| **BaseCLIEngineOption** | `func(*BaseCLIEngine)` | Functional option type for `NewBaseCLIEngine`. Enables constructor customization without parameter explosion. |
| **ExecCommandFunc** | `func(ctx, name, arg...) *exec.Cmd` | Injection point for `exec.CommandContext`. Allows tests to mock command execution without a real container engine. |
| **VolumeFormatFunc** | `func(volume string) string` | Transforms volume strings before passing to `-v`. Podman uses this to append SELinux labels (`:z`) on Linux. Identity function by default. |
| **RunArgsTransformer** | `func(args []string) []string` | Post-processes the `run` argument slice. Podman uses this to inject `--userns=keep-id` before the image name for rootless compatibility. Identity function by default. |
| **SELinuxCheckFunc** | `func() bool` | Determines whether SELinux labeling should be applied. Injected into Podman's volume formatter. Defaults to checking `/sys/fs/selinux` existence. |

## Request/Response Types

| Type | Fields | Purpose |
|------|--------|---------|
| **BuildOptions** | `ContextDir`, `Dockerfile`, `Tag`, `BuildArgs`, `NoCache`, `Stdout`, `Stderr` | Input for `Engine.Build()`. Dockerfile paths are resolved relative to ContextDir with path traversal protection. |
| **RunOptions** | `Image`, `Command`, `WorkDir`, `Env`, `Volumes`, `Ports`, `Remove`, `Name`, `Stdin`, `Stdout`, `Stderr`, `Interactive`, `TTY`, `ExtraHosts` | Input for `Engine.Run()` and `Engine.BuildRunArgs()`. Volumes are processed through `VolumeFormatFunc` before use. |
| **RunResult** | `ContainerID`, `ExitCode`, `Error` | Output from `Engine.Run()`. Non-zero exit codes from the container process are captured in `ExitCode`; only infrastructure failures populate `Error`. |
| **VolumeMount** | `HostPath`, `ContainerPath`, `ReadOnly`, `SELinux` | Structured volume mount specification. Parsed from and formatted to the `host:container[:options]` string format used by Docker/Podman. |
| **PortMapping** | `HostPort`, `ContainerPort`, `Protocol` | Structured port mapping specification. Protocol defaults to `tcp`. |

## Error Types

| Type | Purpose |
|------|---------|
| **EngineNotAvailableError** | Returned when a container engine cannot be found or contacted. Wraps `ErrNoEngineAvailable` sentinel for `errors.Is` compatibility. Includes engine name and reason string for actionable diagnostics. |
| **ErrNoEngineAvailable** | Sentinel error. Callers use `errors.Is(err, ErrNoEngineAvailable)` to detect "no engine" conditions regardless of which engine was tried. |

## Factory Functions

| Function | Signature | Behavior |
|----------|-----------|----------|
| **NewEngine** | `NewEngine(preferredType EngineType) (Engine, error)` | Creates the preferred engine (Docker or Podman). If unavailable, falls back to the alternative. Wraps the result with `SandboxAwareEngine`. Returns `EngineNotAvailableError` if neither engine is available. |
| **AutoDetectEngine** | `AutoDetectEngine() (Engine, error)` | Tries Podman first (rootless-friendly default), then Docker. Wraps with `SandboxAwareEngine`. Returns `EngineNotAvailableError` if neither is found. |

## External Dependencies

| Dependency | Package | Usage |
|------------|---------|-------|
| **platform.DetectSandbox()** | `pkg/platform` | Returns `SandboxType` (None, Flatpak, Snap) used by `SandboxAwareEngine` to decide whether to prefix commands with host spawn. |
| **issue.NewErrorContext()** | `internal/issue` | Creates actionable error contexts with operation name, resource, and user-facing suggestions for build/run failures. |
| **os/exec** | stdlib | Underlying CLI execution. `exec.LookPath` finds engine binaries; `exec.CommandContext` runs them. Injected via `ExecCommandFunc` for testing. |

## Key Patterns

### Composition via Embedding

`DockerEngine` and `PodmanEngine` both embed `*BaseCLIEngine`. The base provides all argument building (`BuildArgs`, `RunArgs`, `ExecArgs`) and command execution (`RunCommand`, `CreateCommand`, etc.), while concrete engines only implement the `Engine` interface methods and engine-specific construction. This avoids code duplication without requiring inheritance.

### Decorator Pattern

`SandboxAwareEngine` wraps any `Engine` to handle Flatpak/Snap sandboxes. It intercepts every method to optionally prefix CLI invocations with `flatpak-spawn --host` or `snap run --shell`. When no sandbox is detected, `NewSandboxAwareEngine` returns the unwrapped engine directly, adding zero overhead in the common case.

### Functional Options

`BaseCLIEngineOption` follows the [Dave Cheney functional options pattern](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis). The constructor sets sensible defaults (real `exec.CommandContext`, identity volume formatter, identity args transformer), and options override them. This powers two key use cases:
- **Testing**: Inject a mock `ExecCommandFunc` to verify argument construction without running containers.
- **Engine-specific behavior**: Podman's constructor prepends `WithVolumeFormatter` (SELinux labels) and `WithRunArgsTransformer` (rootless userns) before any user-supplied options.

### Auto-Detection with Fallback

Both `NewEngine` and `AutoDetectEngine` follow a try-preferred-then-fallback strategy. `AutoDetectEngine` defaults to Podman first because it is more commonly available in rootless setups (Fedora, immutable distros). Both factory functions always wrap the result with `SandboxAwareEngine` to ensure sandbox compatibility.

## Design Rationale

### Why embedding over traditional interfaces?

Docker and Podman share identical argument formats for most operations (`build`, `run`, `rm`, `rmi`). Embedding `*BaseCLIEngine` lets concrete engines reuse argument building and command execution without boilerplate delegation methods. Engine-specific behavior (Podman SELinux labels, rootless userns) is injected via functional options rather than method overrides, keeping the base engine generic.

### Why a decorator for sandbox handling?

Sandbox awareness is a cross-cutting concern orthogonal to engine type. A decorator keeps the `DockerEngine` and `PodmanEngine` implementations clean -- they never need to know about Flatpak or Snap. The decorator also short-circuits when no sandbox is detected, avoiding any runtime cost for the majority of users.

### Why functional options instead of config structs?

Options like `ExecCommandFunc` exist primarily for test injection. A config struct would expose these internals in the public API and require callers to understand fields they never set. Functional options keep the constructor clean (`NewPodmanEngine()` with no arguments for production use) while allowing fine-grained control when needed.

### Why Podman-first in AutoDetectEngine?

Podman is the default container engine on Fedora and other Red Hat-based distributions. It runs rootless by default, requires no daemon, and works on immutable distros (Silverblue/Kinoite) via `podman-remote`. Trying Podman first reduces friction for users on these common Linux configurations.

## Related Diagrams

- [C4 Context (C1)](./c4-context.md) - System boundaries and external actors
- [C4 Container (C2)](./c4-container.md) - Internal containers of the Invowk system
- [C4 Component: Runtime (C3)](./c4-component-runtime.md) - Runtime package internals
- [Command Execution Sequence](./sequence-execution.md) - Temporal flow of command execution
- [Discovery Precedence Flowchart](./flowchart-discovery.md) - How commands are discovered
