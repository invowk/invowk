# Comprehensive Analysis: Invowk Codebase

> **Date**: 2026-02-26
> **Repository**: https://github.com/invowk/invowk
> **Purpose**: Detailed codebase analysis for comparison with Just and Task

## Quantitative Metrics

| Metric | Value |
|--------|-------|
| Language | Go 1.26+ |
| License | MPL-2.0 |
| Total Go files | 511 |
| Production Go files | 285 |
| Test Go files | 226 |
| Total lines of Go code | 122,553 |
| Production code lines | 47,235 |
| Test code lines | 75,318 |
| Test-to-production LOC ratio | ~1.59:1 |
| CLI integration tests (txtar) | 113 |
| Packages (internal + pkg + cmd) | ~20 distinct packages |
| doc.go files | 20 |
| CUE schema lines | 685 (3 files) |

The test-to-production ratio of **1.59:1** is unusually high and signals a deep testing culture. The test code base is actually larger than the production codebase, which is rare.

---

## 1. Code Quality

### 1.1 Overall Organization and Structure

The codebase follows the standard Go project layout with disciplined enforcement of boundaries:

```
main.go          (11 lines, pure composition root)
cmd/invowk/      (CLI layer: Cobra handlers, no domain logic)
internal/        (private: runtime, discovery, config, container, TUI, SSH, etc.)
pkg/             (public: invowkfile, invowkmod, cueutil, platform, types)
tests/cli/       (end-to-end CLI tests via testscript)
tools/goplint/   (separate Go module: custom DDD analyzer)
```

The `main.go` is 11 lines -- a pure entry point that delegates to `cmd.Execute()`:

```go
package main

import (
    cmd "github.com/invowk/invowk/cmd/invowk"
)

func main() {
    cmd.Execute()
}
```

This is textbook separation. The `cmd/invowk/app.go` file is the composition root, and even there, business logic is expressed through interfaces (`CommandService`, `DiscoveryService`, `ConfigProvider`, `DiagnosticRenderer`), not concretions.

### 1.2 Naming Conventions

Naming is highly consistent and follows Go conventions strictly:

- Package names: all lowercase, single-word (`runtime`, `discovery`, `invowkfile`, `cueutil`)
- Types: PascalCase exported, camelCase unexported
- Errors: sentinel variables prefixed with `Err` (`ErrRuntimeNotAvailable`), types suffixed with `Error` (`InvalidRuntimeTypeError`)
- Interfaces: action-oriented (`Runtime`, `EnvBuilder`, `CommandService`, `CapturingRuntime`, `InteractiveRuntime`)
- Constants: PascalCase for exported with descriptive prefixes (`RuntimeTypeNative`, `PlatformLinux`)

There are no obvious naming inconsistencies across packages.

### 1.3 Code Duplication / DRY

The codebase has thoughtful DRY discipline. For example, `NativeRuntime` has unified `executeShellCommon` and `executeInterpreterCommon` helpers that serve both streaming and capturing modes. The `EnvBuilder` interface with `DefaultEnvBuilder` cleanly separates env construction from runtime execution, reused by both native and virtual runtimes. The `serverbase.Base` type provides shared lifecycle infrastructure for both the SSH and TUI servers, avoiding duplication of atomic state machine logic.

### 1.4 Idiomatic Go Patterns

Strongly idiomatic. Notable exemplary patterns:

**Functional options everywhere:**
```go
func WithShell(shell invowkfile.ShellPath) NativeRuntimeOption {
    return func(r *NativeRuntime) { r.shell = shell }
}

func NewNativeRuntime(opts ...NativeRuntimeOption) *NativeRuntime { ... }
```

**`strings.Cut` instead of `strings.Split`:**
```go
name, _, ok := strings.Cut(e, "=")
```

**Correct `defer` resource cleanup with named returns:**
```go
defer func() {
    if closeErr := f.Close(); closeErr != nil && err == nil {
        err = closeErr
    }
}()
```

**`slices.Clone` for defensive copies:**
```go
func (e *ActionableError) Suggestions() []string { return slices.Clone(e.suggestions) }
```

**Go 1.26+ features used**: `strings.SplitSeq`, `for range N` patterns, `slices.Collect(maps.Values(...))`.

### 1.5 Linting and Static Analysis

The `.golangci.toml` configuration enables approximately **40+ linters** beyond the default set:

Key enabled linters include: `modernize`, `goconst`, `decorder`, `funcorder`, `copyloopvar`, `iotamixing`, `mirror`, `exhaustive`, `gosec`, `wrapcheck`, `errname`, `nilerr`, `nilnil`, `predeclared`, `reassign`, `tagliatelle`, `thelper`, `tparallel`, `unconvert`, `wastedassign`, `gocritic`, `errcheck`, `errorlint`, `forbidigo`, `revive`, `staticcheck`.

The configuration includes detailed inline comments for every linter explaining why it is enabled. Particular rules enforced:
- `decorder` + `funcorder`: Declaration order (const/var/type/func, exported before unexported)
- `errname`: sentinel errors prefixed with `Err`, error types suffixed with `Error`
- `wrapcheck`: all external errors must be wrapped
- `exhaustive`: all enum switch cases must be explicit
- `forbidigo`: specific forbidden functions (e.g., all `fmt.Print*` variants, `pflag.Get*` raw calls)

There are 78 total `nolint` directives across 46 files -- a low rate for a codebase of this size, indicating real lint compliance rather than suppression. Every `nolint` directive has a comment explaining why.

Additionally, the project has a **custom `go/analysis` analyzer** (`tools/goplint/`) in a separate Go module that enforces DDD Value Type compliance, including detecting bare primitives in struct fields, missing `IsValid()`/`String()` methods, missing constructors, and improper functional options.

### 1.6 Comment Quality and Documentation

Exceptional. The project enforces "semantic commenting" -- comments explain intent, behavior, invariants, and design rationale, not just what the code does.

From `env_builder.go`:
```go
// EnvBuilder builds environment variables for command execution.
// It applies a 10-level precedence hierarchy (higher number wins):
//
//  1. Host environment (filtered by inherit mode)
//  2. Root-level env.files
//  ...
//  10. RuntimeEnvVars (--ivk-env-var flag) - HIGHEST priority
```

From `runtime.go` (the `Runtime` interface):
```go
// Execute may return a non-zero ExitCode without an Error -- this represents a
// normal process exit with non-zero status. Error is reserved for infrastructure
// failures (binary not found, container failed to start, etc.). Callers should
// check both fields: ExitCode for process-level success and Error for runtime
// failures.
```

20 packages have dedicated `doc.go` files. Package comments are uniformly present and substantive.

---

## 2. Abstractions and Patterns

### 2.1 Key Architectural Patterns

**Composition Root + Dependency Injection**

`cmd/invowk/app.go` is a textbook composition root:

```go
type App struct {
    Config      ConfigProvider
    Discovery   DiscoveryService
    Commands    CommandService
    Diagnostics DiagnosticRenderer
    stdout      io.Writer
    stderr      io.Writer
}

func NewApp(deps Dependencies) (*App, error) {
    if deps.Config == nil { deps.Config = config.NewProvider() }
    // ...
}
```

All services are injected as interfaces, production defaults are provided but can be overridden in tests.

**Request-Scoped Value Objects**

`ExecuteRequest` is an immutable value object capturing all CLI inputs:
```go
type ExecuteRequest struct {
    Name         string
    Args         []string
    Runtime      invowkfile.RuntimeMode
    Interactive  bool
    Verbose      bool
    ForceRebuild bool
    // ...22 fields total, all immutable after construction
}
```

### 2.2 Runtime Abstraction Layer

The runtime layer is a cleanly designed capability-based interface hierarchy:

```go
// Base capability - all runtimes
type Runtime interface {
    Name() string
    Execute(ctx *ExecutionContext) *Result
    Available() bool
    Validate(ctx *ExecutionContext) error
}

// Extended capability - runtimes that support output capture
type CapturingRuntime interface {
    ExecuteCapture(ctx *ExecutionContext) *Result
}

// Extended capability - runtimes with PTY attachment
type InteractiveRuntime interface {
    Runtime
    SupportsInteractive() bool
    PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error)
}
```

Type-assertion + capability-check helpers:
```go
func GetInteractiveRuntime(rt Runtime) InteractiveRuntime {
    if ir, ok := rt.(InteractiveRuntime); ok && ir.SupportsInteractive() {
        return ir
    }
    return nil
}
```

The `Registry` pattern allows runtime lookup by type, with `Available()` filtering and atomic `ExecutionID` generation.

### 2.3 Configuration and CUE Schema Management

Configuration uses a layered approach:
1. **CUE schemas** (`invowkfile_schema.cue`, `invowkmod_schema.cue`, `config_schema.cue`) -- embedded in the binary via `//go:embed`
2. **Generic `ParseAndDecode[T]`** in `pkg/cueutil/parse.go` -- a 3-step parse flow (compile schema, compile user data + unify, validate + decode)
3. **Schema sync tests** -- `pkg/invowkfile/sync_test.go` uses reflection to verify that every CUE schema field has a matching Go struct JSON tag and vice versa, catching drift at CI time

### 2.4 Package Boundary Design

The `internal/` vs `pkg/` distinction is used correctly:
- `pkg/` contains types that could be used by external consumers
- `internal/` contains implementation details

Cross-cutting types that appear in multiple domain packages live in `pkg/types/` (a leaf package with stdlib-only dependencies): `FilesystemPath`, `ExitCode`, `ListenPort`, `DescriptionText`.

### 2.5 DDD Value Types

This is the most distinctive architectural feature. The project applies Domain-Driven Design value type discipline at the Go type level. Instead of passing raw `string` or `int`, every domain concept has a named type:

```go
type RuntimeMode string
type EnvInheritMode string
type PlatformType string
type ContainerImage string
type ExecutionID string
type RuntimeType string
type ExitCode int
```

Every named type has:
- `IsValid() (bool, []error)` -- returns validation errors as a slice
- `String() string` -- implements Stringer
- A typed `Invalid*Error` that wraps a sentinel `Err*` for `errors.Is()` compatibility
- A `New*` constructor for types backed by structs

The custom `goplint` analyzer enforces this pattern with a baseline of 262 accepted findings (down from 840), with a regression gate via `make check-baseline`.

### 2.6 Plugin/Extensibility Architecture

User extensibility is achieved through CUE-format files rather than a traditional plugin system:
- `invowkfile.cue` for project-level command definitions
- `*.invowkmod` directories for reusable modules
- Modules can declare `requires` dependencies on other modules (first-level only)

This is a deliberate design choice: extensibility via data (CUE schemas) rather than code plugins.

---

## 3. Error Handling

### 3.1 Error Wrapping Patterns

176 files use `errors.Is`/`errors.As`/`errors.Unwrap`. 89 files use `fmt.Errorf("...: %w", err)`. The `wrapcheck` linter enforces that all errors from external packages must be wrapped.

Every sentinel error is wrapped in a typed error struct:
```go
var ErrRuntimeNotAvailable = errors.New("runtime not available")

type InvalidRuntimeTypeError struct {
    Value RuntimeType
}

func (e *InvalidRuntimeTypeError) Error() string {
    return fmt.Sprintf("invalid runtime type %q (valid: native, virtual, container)", e.Value)
}

func (e *InvalidRuntimeTypeError) Unwrap() error {
    return ErrInvalidRuntimeType
}
```

This allows callers to use both `errors.Is(err, ErrInvalidRuntimeType)` for broad matching and `errors.As(err, &InvalidRuntimeTypeError{})` for detailed inspection.

### 3.2 Custom Error Types

Three layers of error sophistication:

**1. DDD domain errors** -- per-type validation errors

**2. ActionableError** -- rich contextual errors:
```go
type ActionableError struct {
    operation   string
    resource    string
    suggestions []string
    cause       error
}

err := issue.NewErrorContext().
    WithOperation("find shell").
    WithResource("shells attempted: bash, sh").
    WithSuggestion("Set the SHELL environment variable").
    Wrap(originalErr).
    BuildError()
```

**3. Issue templates** -- 15 markdown templates embedded in the binary for rich TUI rendering.

### 3.3 User-Facing Error Presentation

Errors are rendered differently based on context:
- Non-verbose mode: `failed to <operation>: <resource>: <cause>`
- Verbose mode: full error chain with numbered depth
- TUI mode: rendered via `glamour.Render()` for rich markdown display

Platform-specific suggestions:
```go
func (r *NativeRuntime) shellNotFoundError(attempted []string) error {
    ctx := issue.NewErrorContext().
        WithOperation("find shell").
        WithResource("shells attempted: " + strings.Join(attempted, ", "))

    switch runtime.GOOS {
    case platform.Windows:
        ctx.WithSuggestion("Install PowerShell Core (pwsh)")
    case "darwin":
        ctx.WithSuggestion("Set the SHELL environment variable")
    }
    return ctx.Wrap(fmt.Errorf("no shell found in PATH")).BuildError()
}
```

---

## 4. Reliability

### 4.1 Concurrency Patterns and Safety

**Atomic operations for server lifecycle**: `serverbase.Base` uses `atomic.Int32` for state transitions with a CAS retry loop.

**Atomic execution ID generation**: `Registry` uses `atomic.Uint64` for monotonic counters.

**Cross-process serialization**: On Linux, `internal/runtime/run_lock_linux.go` uses `unix.Flock()` to serialize container engine calls, preventing rootless Podman race conditions.

**Context propagation**: 82 files use `context.Context`. The execution pipeline correctly propagates context through `exec.CommandContext`.

**Discovery cache with mutex**: Per-request discovery cache uses `sync.Mutex` to prevent redundant filesystem scans.

### 4.2 Resource Cleanup

39 files use `defer func()` patterns (named-return error aggregation). The project enforces the named-return defer pattern for resource cleanup. `PreparedCommand.Cleanup func()` for interactive mode ensures temp files are cleaned up even when execution is managed by the PTY layer.

### 4.3 Input Validation and Sanitization

Input validation is applied at two layers:

**CUE schema level**: Constraints at parse time (e.g., regex patterns for env var names, `strings.MaxRunes(32)` for duration strings).

**Go type level**: Every named type has `IsValid() (bool, []error)` called at domain boundaries. CUE file size is bounded by `CheckFileSize` in `cueutil` to prevent OOM attacks.

### 4.4 Defensive Programming

- Build tag isolation for platform-specific code
- Early context cancellation checks
- Non-blocking error channel sends
- Platform constant guards using `pkg/platform`
- Cross-platform stubs for platform-specific features

---

## 5. Tests

### 5.1 Types of Tests

**Unit tests**: 226 test files, table-driven with `t.Parallel()` throughout (219 files use `t.Parallel()` or `t.Run()`).

**Schema sync tests**: 1,158 lines verifying CUE schema ↔ Go struct JSON tag alignment via reflection.

**CLI integration tests**: 113 `.txtar` testscript files executing the real binary in isolated workspaces.

**Integration tests**: Container runtime tests against real Docker/Podman.

**DDD compliance tests**: Every named type's test file covers valid, invalid, edge cases, sentinel error wrapping via `errors.Is`.

**Coverage guardrail**: `TestBuiltinCommandTxtarCoverage` (424 lines) enforces that every leaf CLI command has a corresponding `.txtar` test.

**Runtime mirror test**: Verifies native_*.txtar exists for each virtual_*.txtar.

### 5.2 Test Infrastructure and Helpers

- Mock types at package boundaries
- `AllPlatformConfigs()` test helper for portable fixtures
- Testscript helpers for binary building and container smoke testing
- `t.Context()` (Go 1.26) instead of `context.Background()`

### 5.3 Test Quality

- `errors.Is()` verified for every sentinel error
- Container tests with exponential backoff retry and transient classification
- `t.TempDir()` everywhere for cleanup
- `tparallel` linter enforces parallel test discipline

### 5.4 CI/CD Pipeline

CI runs 6 matrix configurations:
- Ubuntu 24.04 with Docker/Podman (full test suite)
- Ubuntu latest with Docker/Podman (full test suite)
- Windows (short mode)
- macOS (short mode)

Additional workflows: lint, website test, diagram validation, release with GoReleaser.

CI uses `gotestsum` with `--rerun-fails` for transient container flakiness. Test results reported as JUnit XML.

---

## Summary Scorecard

| Dimension | Rating | Notes |
|-----------|--------|-------|
| Code Organization | 9.5/10 | Clean layer separation, composition root, no circular dependencies |
| Naming Consistency | 9.5/10 | Zero violations observed across 285 production files |
| DRY Adherence | 9/10 | Shared helpers, interface reuse, serverbase pattern |
| Idiomatic Go | 9.5/10 | Functional options, generics, modern stdlib |
| Linting | 10/10 | 40+ linters, custom analyzer with DDD enforcement |
| Comment Quality | 9.5/10 | Semantic comments, design rationale, 20 doc.go files |
| Runtime Abstraction | 9.5/10 | Capability interface hierarchy, Registry pattern |
| Config Management | 9/10 | CUE schemas with sync tests, 3-step parse flow |
| Package Design | 9/10 | Clean internal/pkg boundary, cross-cutting types in pkg/types/ |
| DDD Value Types | 10/10 | 90+ named types, all with IsValid/String/constructors, custom analyzer |
| Error Handling | 9.5/10 | Sentinel + typed + actionable + issue templates, full chain |
| Concurrency Safety | 8.5/10 | Atomic state machines, flock, context propagation |
| Resource Cleanup | 8.5/10 | Named return defer, PreparedCommand.Cleanup |
| Input Validation | 9/10 | CUE + Go type IsValid + size limits |
| Test Coverage | 9.5/10 | 1.59:1 ratio, 226 test files, 113 txtar E2E tests |
| Test Quality | 9.5/10 | Table-driven, parallel, schema sync, coverage guardrail |
| CI/CD | 8.5/10 | 6-platform matrix, gotestsum retries, separate workflows |
| **Overall** | **9.2/10** | **Exceptionally disciplined Go project with DDD enforcement.** |

---

## Key Strengths

1. **DDD Value Type system** with custom `go/analysis` analyzer enforcement
2. **Three-tier error presentation** (technical, actionable, rich issue templates)
3. **CUE as configuration schema** with CI-verified sync tests
4. **1.59:1 test-to-code ratio** with comprehensive test types
5. **40+ linters** with inline rationale comments
6. **Capability-based runtime hierarchy** (Native/Virtual/Container)
7. **Composition root with dependency injection**
8. **Coverage guardrails** enforcing test completeness

## Key Weaknesses / Risks

1. **Potential over-engineering** -- 90+ named types, custom analyzer, test suite larger than production code
2. **Pre-1.0 maturity** -- hasn't faced scale of community contributions and production incidents
3. **CUE learning curve** -- less familiar than YAML/TOML for new contributors
4. **Contributor friction** -- 40+ linters and DDD enforcement may deter casual contributions
5. **Linux-only containers** -- fundamental design limitation
