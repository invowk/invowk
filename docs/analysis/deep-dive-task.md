# Comprehensive Analysis: go-task/task Codebase

> **Date**: 2026-02-26
> **Repository**: https://github.com/go-task/task
> **Purpose**: Detailed codebase analysis for comparison with Invowk and Just

## Project Overview

| Metric | Value |
|--------|-------|
| Language | Go 1.24+ (toolchain go1.26.0) |
| License | MIT |
| Module path | `github.com/go-task/task/v3` |
| Open Issues | ~143 |
| Open PRs | ~73 |
| Root-level Go files | ~15 (task.go, executor.go, compiler.go, variables.go, setup.go, etc.) |
| `internal/` packages | 21 packages (deepcopy, editors, env, execext, filepathext, fingerprint, flags, fsext, fsnotifyext, goext, hash, input, logger, output, slicesext, sort, summary, sysinfo, templater, term, version) |
| `taskfile/` packages | 2 packages (taskfile, taskfile/ast) |
| Other packages | args, bin, cmd/task, cmd/release, cmd/sleepit, completion, errors, experiments, taskrc |
| Total packages (est.) | ~30+ |
| Root test files | task_test.go (70KB), executor_test.go (26KB), formatter_test.go, init_test.go, signals_test.go, watch_test.go |
| Estimated Go LoC | ~15,000-20,000 (excluding testdata, website) |
| Test-to-code ratio | Moderate (~0.5-0.7x based on file sizes) |
| Release cadence | Regular; latest v3.48.0; conventional commits used |

---

## 1. Code Quality

### 1.1 Code Organization and Package Structure

**Root package (`task`):** The core logic lives directly in the root package (`package task`), NOT in `internal/` or `cmd/`. This is somewhat unusual for a Go project of this size. Key root-level files:

- `task.go` (16KB) -- `RunTask`, `runCommand`, `runDeps`, `runDeferred`, `startExecution`, `FindMatchingTasks`, `GetTask`
- `executor.go` (15KB) -- `Executor` struct definition + ~30 functional option types
- `setup.go` (7.5KB) -- `Setup()` orchestration, temp dir, logger, output, compiler initialization
- `compiler.go` (6.2KB) -- Template variable compilation
- `variables.go` (14KB) -- `CompiledTask`, `compiledTask`, `itemsFromFor`, `product`
- `call.go` (240B) -- `Call` struct
- `concurrency.go` (436B) -- Semaphore acquire/release
- `hash.go`, `help.go`, `init.go`, `precondition.go`, `requires.go`, `signals.go`, `status.go`, `watch.go`, `completion.go`

**Strengths:**
- Flat root structure makes the entry point easy to find.
- `internal/` packages are small, focused utility packages (e.g., `deepcopy`, `filepathext`, `slicesext`, `templater`).
- `taskfile/ast` is a clean separation of the YAML schema types from parsing logic.
- `errors/` package centralizes all error types with numeric exit codes.

**Weaknesses:**
- The root `task` package is a "God package" -- it holds the `Executor` struct (30+ fields), task execution, compilation, variable resolution, setup, watch mode, help, init, and completion. This is ~80KB of Go code in a single package with no sub-package decomposition.
- `variables.go` at 14KB contains both compilation logic AND the `itemsFromFor`/`product` utilities -- these concerns are mixed.
- No `internal/execution/` or `internal/runtime/` decomposition -- everything is a method on `*Executor`.

### 1.2 Naming Conventions and Consistency

**Strengths:**
- Consistent naming: `New*` constructors, `With*` option functions, `*Error` for error types.
- File names map to their primary content (`task.go` -> task execution, `setup.go` -> setup).
- Import grouping configured: standard, default, `github.com/go-task`, local module.

**Weaknesses:**
- The `Executor` struct mixes public and private fields freely -- exported fields like `Dir`, `Force`, `Verbose` alongside unexported `fuzzyModel`, `concurrencySemaphore`, `taskCallCount`. This blurs the public API surface.
- Inconsistent comment quality: `WithVersionCheck` and `WithFailfast` both have identical doc comments `"tells the [Executor] whether or not to check the version of"` -- the `WithFailfast` comment is a copy-paste bug (incomplete sentence).
- `FastCompiledTask` name is unclear -- it means "compiled without evaluating shell variables" but the name suggests performance optimization.

### 1.3 Code Duplication / DRY Adherence

**Concerning duplication:**
- `CompiledTask` vs `FastCompiledTask` vs `CompiledTaskForTaskList` in `variables.go` -- three nearly identical functions that copy Task struct fields. `CompiledTaskForTaskList` is 50 lines of field-by-field struct copying that duplicates `compiledTask`.
- The functional option pattern in `executor.go` creates massive boilerplate: each option requires a `With*` function, a private `*Option` struct, and an `ApplyToExecutor` method. For ~28 options, this is ~500+ lines of pure boilerplate. The same pattern is duplicated in `reader.go` for `Reader` options (~200+ lines).
- `runCommand` in `task.go` has inline error-handling logic for `IgnoreError` that's duplicated between the `cmd.Task != ""` and `cmd.Cmd != ""` branches.

### 1.4 Idiomatic Go Patterns

**Strengths:**
- Uses `errgroup.Group` for concurrent dependency execution.
- Proper `context.Context` threading through execution chains.
- `sync.Mutex` used for shared state (`dynamicCache`, `executionHashes`).
- Channel-based semaphore for concurrency limiting (idiomatic Go pattern).
- `sync.Once` for lazy fuzzy model initialization.

**Weaknesses:**
- `Executor` is a mutable struct with 30+ fields set via functional options, but many fields are also directly set after construction (e.g., in `setup.go`: `e.Logger`, `e.Compiler`, `e.Output`). This creates a two-phase initialization anti-pattern.
- Uses `atomic.AddInt32` for task call counting but stores the atomics in a `map[string]*int32` -- mixing atomic operations with map access creates subtle correctness concerns (though the map itself is initialized once in `setupConcurrencyState`).
- All types are bare primitives (`string`, `bool`, `int`, `time.Duration`). No domain types whatsoever -- `Dir` is just `string`, `Task` name is `string`, etc.

### 1.5 Linting Configuration

**File:** `.golangci.yml` (version "2" format, golangci-lint v2)

**Enabled formatters:** gofmt, gofumpt, goimports, gci
**Enabled linters:** depguard, mirror, misspell, noctx, paralleltest, thelper, tparallel, usetesting

**Notable configuration:**
- `depguard` rule: Forces use of `github.com/go-task/task/v3/errors` instead of standard `errors` package (except in tests and `errors/*.go`).
- Uses preset exclusions: `comments`, `common-false-positives`, `legacy`, `std-error-handling`.

**Gaps compared to Invowk:**
- No `exhaustive` linter (enum completeness).
- No `wrapcheck` (error wrapping enforcement).
- No `errname` (error naming conventions).
- No `goconst` (magic string detection).
- No `revive` or `decorder` (declaration ordering).
- No custom analyzers (Invowk has `goplint` for DDD type enforcement).
- The linting configuration is relatively **permissive** -- only 7 linters beyond formatters.

### 1.6 Comment Quality and Documentation

- Root-level exported types and functions have doc comments, but quality varies. Some are excellent (`FindMatchingTasks` has a thorough docstring explaining matching behavior). Others are minimal or buggy (`WithFailfast` copy-paste error).
- Internal packages generally lack doc comments on functions.
- No semantic commenting convention (no `HACK:`, `NOTE:`, `TODO:` standards).
- The `README.md` is minimal (1.5KB) -- most documentation lives on the Docusaurus website.

---

## 2. Abstractions and Patterns

### 2.1 Key Architectural Patterns

**Central Abstraction -- The Executor:**

The `Executor` struct is the core of the entire codebase. It is both the configuration holder AND the execution engine. Every operation (run, list, watch, init, completion) is a method on `*Executor`. There is no separation between:
- Configuration/flags
- Task resolution/lookup
- Task compilation (template variable evaluation)
- Task execution (shell command dispatch)
- Output formatting
- Fingerprinting/up-to-date checking

This is a **monolithic executor** pattern, in contrast to Invowk's decomposed pipeline (discovery -> config -> runtime selection -> execution).

**Functional Options Pattern:**

Both `Executor` and `Reader` use the functional options pattern with explicit interface types:

```go
type ExecutorOption interface {
    ApplyToExecutor(*Executor)
}
```

Each option is a separate struct:
```go
type dirOption struct { dir string }
func (o *dirOption) ApplyToExecutor(e *Executor) { e.Dir = o.dir }
```

This is verbose but provides good API ergonomics for callers. **~730 lines** of `executor.go` are pure option boilerplate.

**Node Pattern (Taskfile Sources):**

The `Node` interface in `taskfile/node.go` is a clean abstraction for Taskfile source locations:
```go
type Node interface {
    Read() ([]byte, error)
    Parent() Node
    Location() string
    Dir() string
    Checksum() string
    Verify(checksum string) bool
    ResolveEntrypoint(entrypoint string) (string, error)
    ResolveDir(dir string) (string, error)
}
```

Implementations: `FileNode`, `HTTPNode`, `GitNode`, `StdinNode`, `CacheNode`. This is the strongest interface design in the codebase.

**Graph-Based Taskfile Includes:**

Uses `github.com/dominikbraun/graph` for DAG representation of included Taskfiles. `reader.go` recursively reads Taskfiles and builds vertices/edges with cycle detection. This is architecturally sound.

### 2.2 Task Execution Design

The execution flow is:

1. `cmd/task/task.go` -> `main()` -> `run()` -> parses flags with `pflag`
2. Creates `Executor` with `NewExecutor(opts...)`
3. `e.Setup()` -> reads Taskfile, sets up compiler/logger/output
4. `e.Run(ctx, calls...)` -> validates tasks exist, splits watch/regular, runs via `errgroup`
5. `e.RunTask(ctx, call)` -> compiles task, checks platform/preconditions/fingerprints, runs deps, runs commands
6. `e.runCommand(ctx, t, call, i)` -> dispatches to either subtask call or shell execution via `execext.RunCommand`
7. Shell execution uses `mvdan.cc/sh/v3` (same as Invowk) via `mvdan.cc/sh/moreinterp`

**Key difference from Invowk:** Task does NOT have multiple runtime modes (native/virtual/container). All execution goes through `mvdan/sh`. The `moreinterp` variant provides enhanced interpreter features.

**No CLI framework:** Task uses raw `pflag` instead of Cobra. The `cmd/task/task.go` main function is a single `run()` function with direct flag checking (`flags.Version`, `flags.Help`, `flags.Init`, etc.).

### 2.3 Configuration/Schema Management (YAML)

- Uses YAML (via `go.yaml.in/yaml/v3`, not standard `gopkg.in/yaml.v3`) for Taskfile format.
- Schema is defined implicitly through Go struct types in `taskfile/ast/` with custom `UnmarshalYAML` methods.
- No formal schema validation (no JSON Schema, no CUE, no OpenAPI).
- Schema version checking: Taskfile `version` field must be `>= 3` and `<= current binary version`.
- Supports shortcut syntax (scalar string -> single command, sequence -> list of commands).

### 2.4 Package Boundary Design

**Exported public API:** The root `task` package IS the public Go API. `Executor`, `Call`, `MatchingTask`, `NewExecutor`, `With*` options are all exported. This makes the library usable as a Go package (not just a CLI tool).

**Internal packages are utility-focused:**
- `internal/deepcopy` -- Generic deep copy helpers using `DeepCopier` interface
- `internal/execext` -- Shell command execution wrapper
- `internal/fingerprint` -- Sources/status checking for up-to-date detection
- `internal/output` -- Output formatting (interleaved, grouped, prefixed)
- `internal/templater` -- Go template compilation for variable substitution
- `internal/logger` -- Colorized logging

**Boundary violations:**
- `taskfile/ast` depends on `errors` and `internal/deepcopy` -- the AST package importing internal utilities creates an indirect coupling.
- The `Compiler` struct in the root package accesses `logger.Logger` directly and writes to `Stderr` during variable resolution -- mixing logging concerns into compilation.

### 2.5 Type System Usage

Task uses **no domain types** beyond raw Go primitives. Everything is `string`:
- Task names: `string`
- File paths: `string`
- Directory paths: `string`
- Environment variables: `string`
- Shell commands: `string`
- Method types: `string` (compared against `"checksum"`, `"timestamp"`, `"none"`)
- Run modes: `string` (compared against `"always"`, etc.)

The `ast.Var` type is a grab-bag: `Value any`, `Sh *string`, `Dir string`, `Ref string`, `Live any`. This uses `any` (interface{}) extensively, losing type safety.

There are no enums, no validated types, no `IsValid()` methods. This is in stark contrast to Invowk's DDD value type system.

### 2.6 Plugin/Extensibility Architecture

- **Experiments system:** `experiments/` package provides feature flags via environment variables (`TASK_X_*`) or `.env` files. Current experiments: `GentleForce`, `RemoteTaskfiles`, `EnvPrecedence`. Inactive: `AnyVariables`, `MapVariables`. Each experiment has version gates.
- **No plugin system:** Task is monolithic with no plugin hooks, middleware, or extension points.
- **Remote Taskfiles:** HTTP, Git, and stdin-based Taskfile loading is the primary extensibility mechanism.
- **Task RC:** `taskrc/` package provides user-level configuration (`.taskrc.yml`).

---

## 3. Error Handling

### 3.1 Error Wrapping Patterns

- Uses `fmt.Errorf("...: %w", err)` in some places (e.g., `compiler.go`: `fmt.Errorf("task: failed to get variables: %w", err)`).
- Custom error types implement `Unwrap() error` when wrapping inner errors (e.g., `TaskRunError`, `TaskfileDecodeError`).
- The custom `errors` package re-exports `errors.New`, `errors.Is`, `errors.As`, `errors.Unwrap` from the standard library to avoid import aliasing.

### 3.2 Custom Error Types

Centralized in `errors/` package with three categories:

1. **Taskfile errors** (`errors_taskfile.go`): `TaskfileNotFoundError`, `TaskfileAlreadyExistsError`, `TaskfileInvalidError`, `TaskfileFetchFailedError`, `TaskfileNotTrustedError`, `TaskfileNotSecureError`, `TaskfileCacheNotFoundError`, `TaskfileVersionCheckError`, `TaskfileNetworkTimeoutError`, `TaskfileCycleError`, `TaskfileDoesNotMatchChecksum`

2. **Task errors** (`errors_task.go`): `TaskNotFoundError`, `TaskRunError`, `TaskInternalError`, `TaskNameConflictError`, `TaskNameFlattenConflictError`, `TaskCalledTooManyTimesError`, `TaskCancelledByUserError`, `TaskCancelledNoTerminalError`, `TaskMissingRequiredVarsError`, `TaskNotAllowedVarsError`

3. **Decode errors** (`error_taskfile_decode.go`): `TaskfileDecodeError` with line/column/tag/snippet for rich YAML error reporting.

All custom error types implement the `TaskError` interface:
```go
type TaskError interface {
    error
    Code() int
}
```

Exit codes are organized by range: 0-1 general, 50-59 TaskRC, 100-111 Taskfile, 200-207 Task.

### 3.3 User-Facing Error Presentation

- Error messages are prefixed with `task:` (e.g., `task: Task "foo" does not exist`).
- `TaskfileDecodeError` includes colorized output with file location, line/column, and code snippet (via the `snippet.go` implementation).
- `TaskNotFoundError` includes "Did you mean?" fuzzy matching suggestion.
- GitHub Actions CI error annotations: `::error title=Task 'name' failed::message` (in `cmd/task/task.go`).
- Exit codes enable programmatic error handling by callers.

### 3.4 Error Recovery and Graceful Degradation

- `IgnoreError` flag on tasks/commands -- execution continues despite non-zero exit codes.
- Deferred commands always run (errors are logged but not propagated): `"task: ignored error in deferred cmd: %s"`.
- Remote Taskfile caching: If network fails but cache exists (even expired), falls back to cached version.
- Optional includes: `include.Optional` silently skips missing Taskfiles.
- `context.DeadlineExceeded` is wrapped into `TaskfileNetworkTimeoutError` with the timeout duration.

### 3.5 Sentinel Errors vs Typed Errors

Task uses **typed errors exclusively** -- no sentinel errors (no `var ErrFoo = errors.New(...)`). The one exception is `ErrIncludedTaskfilesCantHaveDotenvs` in `taskfile/ast/taskfile.go`.

Error checking is done via type assertions:
```go
if _, ok := err.(*errors.TaskNotFoundError); ok { ... }
```

And `errors.As`:
```go
var exitCode interp.ExitStatus
if errors.As(err, &exitCode) { ... }
```

---

## 4. Reliability

### 4.1 Concurrency Patterns

**Parallel task execution:** Uses `errgroup.Group` for concurrent dependency and parallel task execution.

**Concurrency limiting:** Channel-based semaphore pattern:
```go
func (e *Executor) acquireConcurrencyLimit() func() {
    e.concurrencySemaphore <- struct{}{}
    return func() { <-e.concurrencySemaphore }
}
```

The `releaseConcurrencyLimit` counterpart is used during dependency execution to avoid deadlocks -- a task releases its slot while waiting for deps, then reacquires afterward.

**Execution deduplication:** `startExecution` uses a hash-based mutex map (`executionHashes`) to prevent the same task from executing concurrently. Waiting tasks block on a `context.Context.Done()` channel.

**Task call counting:** `atomic.AddInt32` with a per-task counter to detect infinite loops (limit: 1000 calls).

**Concerns:**
- `executionHashes` uses a manual `sync.Mutex` for the map, while `taskCallCount` uses atomics -- inconsistent synchronization strategies.
- The `Compiler.dynamicCache` uses a `sync.Mutex` (not `sync.Map` or `xsync.Map`) which could be a bottleneck under high parallelism.

### 4.2 Resource Cleanup

- `defer release()` for concurrency semaphore -- correctly paired acquire/release.
- `defer cancel()` for context cancellation in `startExecution` and `runDeferred`.
- `defer cf()` for timeout context in `readTaskfile`.
- `defer CleanGitCache()` in `Reader.Read`.
- Signal handling: `InterceptInterruptSignals()` in `signals.go` (behind build tag `signals`).
- Output writer cleanup: `closer(err)` pattern with `CloseFunc` type.

### 4.3 Input Validation and Sanitization

- Task existence check before execution (`GetTask` -> `TaskNotFoundError`).
- Internal task check (prevents running internal tasks from CLI).
- Platform filtering (`shouldRunOnCurrentPlatform`).
- Required variable validation (`areTaskRequiredVarsSet`, `areTaskRequiredVarsAllowedValuesSet`).
- Taskfile schema version check (`doVersionChecks`).
- Remote Taskfile checksum verification (`Verify` method on nodes).
- Trust prompts for remote Taskfiles.

**Gaps:**
- No input sanitization on shell commands -- commands are passed directly to `mvdan/sh` (expected behavior for a task runner, but worth noting).
- Task names are not validated for format -- any string is accepted.

### 4.4 Defensive Programming Patterns

- Nil checks throughout: `if t == nil { ... }`, `if call == nil { ... }`, `if call.Vars == nil { call.Vars = ast.NewVars() }`.
- `DeepCopy()` methods on all AST types to prevent mutation of shared state during compilation.
- Maximum task call limit (1000) to prevent infinite recursion.
- `sync.Once` for lazy initialization (`fuzzyModelOnce`).
- Graceful handling of development builds where version is "devel" (skips version comparison).

---

## 5. Tests

### 5.1 Test Coverage and Organization

**Main test files:**
- `task_test.go` (70KB, ~26KB compiled) -- The primary integration test file. Tests task execution, dependencies, variables, preconditions, status, dry run, and more. Uses extensive `testdata/` fixtures.
- `executor_test.go` (26KB) -- Tests the `Executor` struct configuration and behavior.
- `formatter_test.go` (5.5KB) -- Tests output formatting.
- `init_test.go` (1KB) -- Tests `task --init`.
- `signals_test.go` (7.5KB) -- Tests signal handling (build-tagged `signals`).
- `watch_test.go` (2KB) -- Tests file watching (build-tagged `watch`).
- `taskfile/node_git_test.go` (8KB), `node_http_test.go` (8KB) -- Node resolution tests.
- `taskfile/snippet_test.go` (9.6KB) -- YAML snippet extraction tests.
- `taskfile/ast/platforms_test.go`, `precondition_test.go`, `taskfile_test.go` -- AST unit tests.
- `internal/fingerprint/task_test.go` (5.4KB) -- Fingerprint tests with mocks.
- `internal/output/output_test.go` (4.6KB) -- Output wrapper tests.
- `experiments/experiment_test.go` (3KB) -- Experiment flag tests.

**Test data:** `testdata/` directory at root contains YAML Taskfile fixtures for integration tests.

### 5.2 Types of Tests

- **Integration tests (dominant):** Most tests create an `Executor`, call `Setup()`, and then `Run()`, asserting on stdout/stderr output or error types. These are full pipeline tests from Taskfile reading through execution.
- **Unit tests (limited):** Present for AST types, node parsing, snippet extraction, output formatting, fingerprint logic. Coverage of `internal/` packages is uneven.
- **E2E tests (limited):** Signal and watch tests use build tags and appear to spawn actual processes. The `cmd/sleepit` helper is built specifically for signal testing.
- **No testscript/txtar tests:** Unlike Invowk, Task does not use `rogpeppe/go-internal/testscript`.
- **Golden file testing:** Uses `github.com/sebdah/goldie/v2` for golden file comparison (`generate:fixtures` task regenerates them).
- **Mock generation:** Uses `github.com/vektra/mockery` (v3.2.2) for generating mock files. `internal/fingerprint/checker_mock.go` (9.2KB) is a generated mock.

### 5.3 Test Infrastructure and Helpers

- `goldie/v2` for golden file assertions.
- `stretchr/testify` for assertions (`assert`, `require`).
- `mockery` for interface mock generation.
- `gotestsum` for test execution (not pinned -- `@latest` in Taskfile!).
- No custom test helper framework.
- Build tags: `signals`, `watch` for platform-specific test isolation.

### 5.4 Test Quality

**Strengths:**
- Extensive integration coverage for the happy path -- many Taskfile configurations are tested.
- Golden files reduce assertion boilerplate for complex output.
- Mock-based fingerprint testing allows isolated checker testing.
- Table-driven tests used in `node_git_test.go`, `node_http_test.go`, `platforms_test.go`.
- `golangci-lint` enforces `paralleltest`, `tparallel`, `thelper`, `usetesting`.

**Weaknesses:**
- `task_test.go` at 70KB is a monolithic test file -- not decomposed by feature area.
- Tests are tightly coupled to Taskfile fixtures in `testdata/` -- changes to the YAML format require updating many fixtures.
- Limited error-path testing -- most tests verify the happy path.
- No fuzzing tests.
- No benchmark tests (no `_bench_test.go` files).
- `gotestsum` installed at `@latest` -- not version-pinned. This violates reproducibility.

### 5.5 CI/CD Testing Pipeline

**Workflows:**
- `test.yml`: Tests on matrix (Go 1.24.x, 1.25.x) x (ubuntu, macos, windows). Uses Task itself to run tests (`./bin/task test`). Bootstrapping: builds Task first, then uses it to run the test suite.
- `lint.yml`: Runs `golangci-lint`.
- `pr-build.yml`: GoReleaser build verification on PRs.
- `release.yml`, `release-nightly.yml`: GoReleaser-based releases.
- Issue management workflows (triage, experiments, awaiting response).

**Self-hosting:** Task uses itself to run its own test suite ("dogfooding"). The CI builds Task, then invokes `./bin/task test` which runs `gotestsum ./...`.

---

## Summary Scorecard

| Dimension | Rating | Notes |
|-----------|--------|-------|
| Code Organization | 5/10 | God package problem. Core logic lacks decomposition. |
| Naming Consistency | 7/10 | Generally good but with copy-paste bugs and misleading names. |
| DRY Adherence | 5/10 | Three duplicated compilation functions. 930 lines of option boilerplate. |
| Idiomatic Go | 6/10 | Uses errgroup/context well, but two-phase initialization and bare primitives. |
| Linting | 5/10 | Only 7 linters beyond formatters. No wrapcheck, exhaustive, errname. |
| Comment Quality | 5/10 | Variable quality. Some copy-paste bugs. No semantic commenting. |
| Error Types | 7/10 | Well-organized centralized error package with exit code ranges. |
| Error Messages | 7/10 | TaskfileDecodeError with snippets is good. Fuzzy suggestions present. |
| Concurrency | 7/10 | Channel semaphore with deadlock avoidance. Execution deduplication. |
| Resource Cleanup | 6/10 | Defer-based cleanup present but no named-return error aggregation. |
| Input Validation | 6/10 | Basic validation present. No format validation on task names. |
| Test Organization | 5/10 | 70KB monolithic test file. Integration-heavy, limited unit tests. |
| Test Quality | 6/10 | Good happy-path coverage. Limited error-path testing. No fuzzing. |
| CI Pipeline | 7/10 | Multi-platform, self-dogfooding. Unpinned gotestsum. |
| Extensibility | 6/10 | Experiments system, remote Taskfiles, TaskRC. No plugins. |
| **Overall** | **6.0/10** | **Mature and widely used, but showing signs of organic growth without periodic architectural investment.** |

---

## Key Strengths

1. **Node interface** -- Clean abstraction for Taskfile sources (file/HTTP/Git/stdin/cache)
2. **Error system with exit code ranges** -- Well-organized `errors/` package
3. **Remote Taskfile support** -- Sophisticated caching, checksum verification, offline fallback
4. **Concurrency design** -- Channel semaphore with deadlock avoidance
5. **Self-dogfooding** -- Uses Task to build and test itself
6. **Experiments system** -- Clean feature flags for gradual rollout
7. **DAG-based include resolution** -- Graph library with cycle detection
8. **Public Go API** -- Library-usable (not just CLI)

## Key Weaknesses

1. **God package** -- ~80KB of code in root package mixing all concerns
2. **No domain types** -- All bare primitives, `any` in AST
3. **Massive option boilerplate** -- ~930 lines of functional option scaffolding
4. **Monolithic test file** -- `task_test.go` at 70KB
5. **Unpinned tool versions** -- `gotestsum@latest`
6. **Minimal linting** -- 7 linters vs Invowk's 40+
7. **Two-phase initialization** -- `Executor` created then mutated by `Setup()`
8. **Duplicated compilation functions** -- Three nearly identical functions
9. **No CLI framework** -- Raw pflag loses Cobra's discoverability
10. **No formal schema** -- YAML parsed via custom `UnmarshalYAML`, no JSON Schema/CUE
