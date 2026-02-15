# Architecture Enhancement Proposal: Stateless Composition and Typed Contracts

## Problem Statement

The current architecture has four coupled design problems that increase hidden behavior, test fragility, and runtime ambiguity:

1. CLI execution state is spread across mutable package globals in `cmd/invowk/root.go`, `cmd/invowk/cmd.go`, and `cmd/invowk/cmd_execute*.go`.
2. Discovery emits terminal output from `internal/discovery/discovery_commands.go` (`fmt.Fprintf(os.Stderr, ...)`) instead of returning diagnostics.
3. Production execution paths read mutable global config state via `internal/config/global.go` (`Get`, overrides, cache).
4. Module command storage uses `any` in `pkg/invowkmod/invowkmod.go`, requiring cast-based recovery in `pkg/invowkfile/parse.go`.

This proposal defines a single final architecture that removes these patterns without compatibility shims.

## Goals

- Remove mutable package-level runtime state from command execution.
- Introduce one explicit composition root that owns service lifecycle.
- Make Cobra handlers thin adapters that build requests and delegate.
- Make discovery side-effect-free for terminal output and return structured diagnostics.
- Replace implicit global config reads with an injected provider contract.
- Replace `Module.Commands any` with a typed interface declared in `pkg/invowkmod`.
- Keep user-visible CLI outcomes behaviorally equivalent after the rewrite.

## Non-Goals

- No compatibility layer, adapters, dual interfaces, or deprecation window.
- No feature expansion beyond architecture refactoring.
- No user-facing command grammar redesign unless required for equivalence.
- No partial migration; this is a coordinated, atomic internal rewrite.

## Breaking Change Policy (No Backward Compatibility)

This refactor is intentionally non-backward-compatible for internal architecture.

- Legacy globals and singleton-style command wiring are removed, not preserved.
- Old signatures are deleted once callers are moved.
- Tests are rewritten to target final contracts only.
- Any path that still depends on global config/discovery side effects is considered incomplete.

## Current Findings and Impact

- **CLI statefulness/orchestration concentration**
  - `cmd/invowk/root.go`: mutable `verbose`, `interactive`, root init config mutation.
  - `cmd/invowk/cmd.go`: mutable `runtimeOverride`, `fromSource`, `forceRebuild`, plus `sshServerInstance` singleton.
  - `cmd/invowk/cmd_execute.go`: orchestration hotspot `runCommandWithFlags(runCommandOptions)` owns discovery, config access, runtime creation, and execution branching.
- **Discovery side effects**
  - `internal/discovery/discovery_commands.go`: warning output is printed directly to stderr inside discovery.
- **Global mutable config state**
  - `internal/config/global.go`: process-wide cache/override state (`globalConfig`, `configFilePathOverride`, `Reset`, `SetConfigFilePathOverride`).
  - `internal/config/config.go`: `Load`/`Save` mutate global cache.
- **Untyped module command storage**
  - `pkg/invowkmod/invowkmod.go`: `Module.Commands any`.
  - `pkg/invowkfile/parse.go`: `GetModuleCommands` performs runtime cast from `any` to `*Invowkfile`.

Impact:

- Hidden shared state causes order-dependent behavior and brittle tests.
- Discovery cannot be reused safely in non-CLI contexts without spurious output.
- Config source and lifecycle are implicit instead of declared at boundaries.
- Type safety is deferred to runtime instead of compile-time.

## Target Architecture

Final architecture centers on one composition root and explicit injected services.

- **Composition root**: `cmd/invowk/root.go` builds `App` once at startup.
- **Command handlers**: Cobra `RunE` functions only map flags/args into typed requests and call services.
- **Service ownership**:
  - `CommandService` owns orchestration and runtime execution.
  - `DiscoveryService` owns discovery and diagnostics creation.
  - `ConfigProvider` owns config load lifecycle and source selection.
  - `DiagnosticRenderer` is CLI-owned and is the only terminal writer.
- **Server lifecycle**: SSH server lifecycle moves out of package globals into service-owned dependency lifecycle.
- **Dynamic command registration and completion**:
  - Dynamic leaf commands are constructed during `NewRootCommand(app)` (not in package `init()`).
  - Shell completion queries `DiscoveryService` at completion time and renders nothing unless caller chooses to.

Call direction:

`cobra RunE -> request builder -> app service -> discovery/runtime/config dependencies -> result + diagnostics -> CLI renderer`

No discovery/config/runtime package writes directly to stdout/stderr.

## Interface and Type Redesign

### 1) CLI statefulness/orchestration

Final target signatures/types:

```go
package cmd

import (
    "context"
    "io"

    "github.com/invowk/invowk/internal/config"
    "github.com/invowk/invowk/internal/discovery"

    "github.com/spf13/cobra"
)

type App struct {
    Config      ConfigProvider
    Discovery   DiscoveryService
    Commands    CommandService
    Diagnostics DiagnosticRenderer
}

type Dependencies struct {
    Config      ConfigProvider
    Discovery   DiscoveryService
    Commands    CommandService
    Diagnostics DiagnosticRenderer
    Stdout      io.Writer
    Stderr      io.Writer
}

func NewApp(deps Dependencies) (*App, error)
func NewRootCommand(app *App) *cobra.Command

type ExecuteRequest struct {
    Name          string
    Args          []string
    SourceFilter  string
    Runtime       string
    Interactive   bool
    Verbose       bool
    FromSource    string
    ForceRebuild  bool
    Workdir       string
    EnvFiles      []string
    EnvVars       map[string]string
    ConfigPath    string
}

type ExecuteResult struct {
    ExitCode int
}

type CommandService interface {
    Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error)
}

type DiscoveryService interface {
    DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
    DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
    GetCommand(ctx context.Context, name string) (discovery.LookupResult, error)
}

type DiagnosticRenderer interface {
    Render(ctx context.Context, diags []discovery.Diagnostic, stderr io.Writer)
}

type ConfigProvider interface {
    Load(ctx context.Context, opts config.LoadOptions) (*config.Config, error)
}
```

Ownership and call direction:

- `root.go` owns `App` construction and injects dependencies once.
- `cmd.go`, `cmd_discovery.go`, `cmd_execute.go` own request building only.
- `CommandService` owns orchestration previously concentrated in `runCommandWithFlags`.

Error/diagnostic contract:

- Services return domain errors and structured diagnostics.
- CLI decides rendering and exit behavior.

Caller responsibilities:

- Build `ExecuteRequest` from Cobra args/flags.
- Invoke one service method.
- Render diagnostics and map errors to CLI output/exit semantics.

Replaced-by mapping:

- `runCommandWithFlags(runCommandOptions)` -> `CommandService.Execute(context.Context, ExecuteRequest)`.
- Package globals (`verbose`, `interactive`, `runtimeOverride`, `fromSource`, `forceRebuild`) -> per-request fields in `ExecuteRequest`.
- `sshServerInstance` command-package singleton wiring -> service-owned dependency lifecycle in `Dependencies`.

### 2) Discovery side effects

Final target signatures/types:

```go
package discovery

import "context"

type Severity string

const (
    SeverityWarning Severity = "warning"
    SeverityError   Severity = "error"
)

type Diagnostic struct {
    Severity Severity
    Code     string
    Message  string
    Path     string
    Cause    error
}

type CommandSetResult struct {
    Set         *DiscoveredCommandSet
    Diagnostics []Diagnostic
}

type LookupResult struct {
    Command     *CommandInfo
    Diagnostics []Diagnostic
}

func New(cfg *config.Config) *Discovery
func (d *Discovery) DiscoverCommandSet(ctx context.Context) (CommandSetResult, error)
func (d *Discovery) DiscoverAndValidateCommandSet(ctx context.Context) (CommandSetResult, error)
func (d *Discovery) GetCommand(ctx context.Context, name string) (LookupResult, error)
```

Ownership and call direction:

- `internal/discovery` computes diagnostics but never writes terminal output.
- CLI owns all output policy through `DiagnosticRenderer`.

Error/diagnostic contract:

- Fatal conditions return `error`.
- Recoverable conditions (for example parse-skip warnings) are returned in `Diagnostics`.

Caller responsibilities:

- Always process returned diagnostics.
- Decide warning visibility for normal run, completion, and list contexts.

Replaced-by mapping:

- `DiscoverCommandSet() (*DiscoveredCommandSet, error)` -> `DiscoverCommandSet(ctx) (CommandSetResult, error)`.
- Discovery-layer `fmt.Fprintf(os.Stderr, ...)` -> append `Diagnostic` and return.
- `GetCommand(name) (*CommandInfo, error)` -> `GetCommand(ctx, name) (LookupResult, error)`.

### 3) Global mutable config state

Final target signatures/types:

```go
package config

import "context"

type LoadOptions struct {
    ConfigFilePath string
}

type Provider interface {
    Load(ctx context.Context, opts LoadOptions) (*Config, error)
}

func NewProvider() Provider
```

Composition-root usage:

```go
cfg, err := app.Config.Load(ctx, config.LoadOptions{ConfigFilePath: req.ConfigPath})
```

Ownership and call direction:

- Composition root owns provider instance lifecycle.
- Execution/list/validation/completion paths consume injected provider only.

Error/diagnostic contract:

- Load failures return `error` only.
- Any user-facing rendering remains in CLI.

Caller responsibilities:

- Pass `--config` value explicitly through `LoadOptions`.
- Do not mutate global overrides during production execution.

Replaced-by mapping:

- `config.Get()` in execution paths -> `Provider.Load(ctx, LoadOptions)`.
- `config.SetConfigFilePathOverride()` -> explicit `LoadOptions.ConfigFilePath`.
- Runtime dependency on `globalConfig` cache/override vars -> provider-owned state only.

### 4) Typed module-command storage (`any` removal)

Final target signatures/types:

```go
package invowkmod

type ModuleCommands interface {
    GetModule() string
    ListCommands() []string
}

type Module struct {
    Metadata      *Invowkmod
    Commands      ModuleCommands
    Path          string
    IsLibraryOnly bool
}
```

```go
package invowkfile

var _ invowkmod.ModuleCommands = (*Invowkfile)(nil)

func ParseModule(modulePath string) (*Module, error)
```

Ownership and call direction:

- `pkg/invowkmod` owns the interface contract.
- `pkg/invowkfile` implements the contract and assigns it to `Module.Commands`.

Error/diagnostic contract:

- Parsing errors remain parse-function errors.
- No runtime type assertion path remains for module commands.

Caller responsibilities:

- Use `Module.Commands` as typed interface.
- Treat `nil` commands as library-only module.

Replaced-by mapping:

- `Module.Commands any` -> `Module.Commands ModuleCommands`.
- `GetModuleCommands(m *Module) *Invowkfile` cast helper -> direct interface usage via `m.Commands`.
- Runtime cast from `any` in `pkg/invowkfile/parse.go` -> compile-time assertion in `pkg/invowkfile`.

## Execution Plan (Atomic, No Compatibility Layer)

1. Introduce final interfaces/types.
   - Add `App` dependency contracts, discovery diagnostic result types, config provider contract, and `invowkmod.ModuleCommands`.
2. Rewire composition root and CLI command execution.
   - Build `App` once in `root.go`; convert Cobra handlers to request builders; remove init-time orchestration side effects.
3. Refactor discovery return contracts and caller handling.
   - Change discovery APIs to return diagnostics; update all discovery callers (`cmd_discovery`, execution helpers, validation/completion paths).
4. Replace config global path with injected provider.
   - Replace production-path `config.Get()`/override usage with provider calls using explicit `LoadOptions`.
5. Apply typed module-command contract migration.
   - Change `Module.Commands` type, add compile-time assertion in `pkg/invowkfile`, remove cast helper usage.
6. Remove obsolete code paths immediately.
   - Delete legacy globals/singletons and old signatures in same migration branch.
7. Update tests to target final architecture only.
   - Rewrite unit/integration tests for injected services and diagnostic-returning discovery.

No dual-path support, no adapters, and no deprecation windows are allowed.

## Testing and Verification Criteria

Verification scenarios are required for each architecture objective:

1. **Absence of command-runtime package globals**
   - Static check confirms command behavior is not driven by mutable globals in `cmd/invowk/root.go`, `cmd/invowk/cmd.go`, and execution helpers.
   - Service tests prove isolated behavior across multiple app instances.
2. **Discovery has zero terminal-write side effects**
   - Discovery tests capture stdout/stderr and assert empty output while diagnostics are returned.
   - Code check confirms no `fmt.Print*`, `os.Stdout`, `os.Stderr`, or logger writes in `internal/discovery` execution paths.
3. **Config is consumed through injection only in execution paths**
   - No production execution/list/validation/completion path calls `config.Get()`.
   - Tests use fake provider to assert config comes only from injected dependency.
4. **`invowkmod` command storage is fully typed**
   - `pkg/invowkmod/invowkmod.go` no longer contains `Commands any`.
   - Compile-time assertion exists: `var _ invowkmod.ModuleCommands = (*invowkfile.Invowkfile)(nil)`.
   - No cast-based module command helper remains in `pkg/invowkfile/parse.go`.
5. **Behavioral equivalence of CLI outcomes**
   - Integration coverage for `invowk cmd --list`, ambiguity handling, disambiguated source execution (`@source`, `--from`), runtime selection, and exit-code propagation.
   - Pre/post refactor outcomes match for exit code and key user-visible output semantics.

## Risks and Mitigations

- Risk: one-shot migration creates temporary compile instability.
  - Mitigation: apply migration in strict sequence with compile checks after each numbered step.
- Risk: diagnostics order differences break brittle tests.
  - Mitigation: normalize diagnostic ordering before rendering and update assertions to stable order.
- Risk: command registration/completion behavior regresses when removing `init()` registration.
  - Mitigation: move dynamic command construction to `NewRootCommand(app)` and add explicit completion regression tests.
- Risk: hidden config globals remain in secondary execution paths.
  - Mitigation: enforce grep/static gates and fail CI on prohibited global access in production command paths.

## Acceptance Criteria

- The proposal is implementation-ready and leaves no major architecture decisions open.
- The proposal explicitly enforces no backward compatibility.
- Final interfaces and types are concrete and compile-oriented.
- Migration order is single-pass and atomic with immediate obsolete-path removal.
- Command runtime behavior no longer depends on mutable package globals.
- Discovery returns diagnostics and performs no terminal writes.
- Production execution paths consume config only through injected provider contract.
- `pkg/invowkmod.Module.Commands` is typed and free of runtime cast-based access patterns.
- Verification criteria are measurable and directly mapped to items 1-4.

## File-Level Change Map

| File | Final change |
| --- | --- |
| `cmd/invowk/root.go` | Build composition root (`App`) once; inject providers/services; remove global behavior wiring. |
| `cmd/invowk/cmd.go` | Keep Cobra shape; remove mutable command globals; delegate execution to service. |
| `cmd/invowk/cmd_discovery.go` | Consume `discovery.CommandSetResult`; render diagnostics in CLI only. |
| `cmd/invowk/cmd_execute.go` | Replace orchestration-heavy path with `CommandService.Execute`; request mapping only in handler. |
| `cmd/invowk/cmd_execute_helpers.go` | Remove package-owned SSH singleton lifecycle; convert to service dependency usage. |
| `internal/discovery/discovery_commands.go` | Return structured diagnostics; remove stderr writes; context-aware APIs. |
| `internal/discovery/discovery_files.go` | Preserve discovery traversal, but report recoverable issues via diagnostics. |
| `internal/config/global.go` | Remove production reliance on global cache/override state. |
| `internal/config/config.go` | Expose provider-backed load contract using explicit options. |
| `pkg/invowkmod/invowkmod.go` | Change `Module.Commands` from `any` to `ModuleCommands` interface. |
| `pkg/invowkfile/parse.go` | Assign typed command contract in `ParseModule`; remove cast-based helper path. |
