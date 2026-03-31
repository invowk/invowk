# Test Pattern Catalog

Consolidated testing patterns for subagent reference (SA-2, SA-3, SA-4, SA-8).

## 1. Required Patterns

| Pattern | When Required | Example |
|---------|---------------|---------|
| **Table-driven tests** | Functions with 3+ test cases must use `tests := []struct{...}` with `t.Run(tt.name, ...)`. Each case needs `name string`. Error cases need `wantErr bool` and verification of both error presence AND content. | `tests := []struct{ name string; input string; wantErr bool }{...}; for _, tt := range tests { t.Run(tt.name, func(t *testing.T) { ... }) }` |
| **`t.Parallel()`** | First call in every `Test*` function (before `t.Skip()`). All table-driven subtests inside parallel parents must also call `t.Parallel()`. Exceptions: global state mutation (`os.Chdir`, `os.Setenv`, `t.Setenv`, `SetHomeDir`), CUE subtests (thread-unsafe), SSH host key collision. | `func TestFoo(t *testing.T) { t.Parallel(); ... }` |
| **`t.Context()`** | Default context in test functions (Go 1.24+). `b.Context()` for benchmarks. | `ctx := t.Context()` |
| **`testing.Short()` gating** | Required for any test needing external resources (container engine, network). Message: `"skipping integration test in short mode"`. | `if testing.Short() { t.Skip("skipping integration test in short mode") }` |
| **`t.Helper()`** | Required in all test helper functions. | `func assertNoError(t *testing.T, err error) { t.Helper(); ... }` |
| **`t.TempDir()`** | Preferred over `os.MkdirTemp` + manual cleanup. Lifecycle managed by testing framework. | `dir := t.TempDir()` |
| **Error assertions** | Use `errors.Is`/`errors.As` on sentinel errors and typed error structs, not string matching on `err.Error()`. | `if !errors.Is(err, ErrNotFound) { t.Errorf(...) }` |
| **Import alias** | When both `runtime.GOOS` and internal `runtime` package are needed. | `goruntime "runtime"` |

## 2. Anti-Patterns

| Anti-Pattern | Why It's Bad | Fix |
|--------------|--------------|-----|
| **`time.Sleep()` in tests** | Creates flaky tests dependent on system load. | Channel synchronization, `testutil.NewFakeClock()` with `Advance()`. |
| **`reflect.DeepEqual` on typed slices** | Not type-safe. | `slices.Equal`. |
| **Hardcoded Unix paths** (`/foo/bar`) in assertions | Fails on Windows. | `filepath.Join()` or `skipOnWindows`. |
| **Shared `MockCommandRecorder` across parallel subtests** | Race condition. | Per-test recorder instance. |
| **`context.Background()` in test functions** | Should use `t.Context()`. Only allowed in `TestMain`, `env.Defer()` callbacks, package-level init. | `t.Context()`. |
| **`os.MkdirTemp` + `defer os.RemoveAll` with parallel subtests** | Data race -- parent `defer` fires while subtests still running. | `t.TempDir()`. |
| **`[GOOS:windows]` in txtar** | NOT a valid condition (falls through to `commonCondition` error). | Use built-in `[windows]`. |
| **`tt := tt` loop-variable rebinding** | Redundant in Go 1.22+, flagged by `modernize`. | Remove. |
| **Circular/trivial tests** | Testing constant == literal, zero-value == zero. | Test behavioral contracts instead. |
| **Discarding `Validate()` return** | `_ = x.Validate()` or bare `x.Validate()`. | Always check and propagate error. |

## 3. Container Test Patterns

Five-layer timeout strategy:

1. **Per-test deadline** -- catches individual test hangs.
   ```go
   testscript.Params{Deadline: time.Now().Add(3 * time.Minute)}
   ```

2. **Container cleanup** -- prevents orphaned containers.
   ```go
   env.Defer(func() { cleanupTestContainers(containerPrefix) })
   ```

3. **CI runner timeout** -- catches catastrophic failures.
   ```bash
   # CI
   gotestsum ... -timeout 15m
   # Local (make test-cli)
   go test -timeout 10m
   ```

4. **Semaphore** -- prevents Podman resource exhaustion. Place AFTER `t.Parallel()` and `testing.Short()` skip, BEFORE container operations. Default: `min(GOMAXPROCS, 2)`, override via `INVOWK_TEST_CONTAINER_PARALLEL`.
   ```go
   sem := testutil.ContainerSemaphore()
   sem <- struct{}{}
   defer func() { <-sem }()
   ```

5. **Bounded context** -- 5-minute deadline for `Execute()`/`ExecuteCapture()` calls. Bare `t.Context()` has NO deadline.
   ```go
   ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
   ```

Additional container patterns:

- `probeEngineHealthBeforeTest()` -- 10s engine liveness check before each CLI test.
- `HOME` must be set to `env.WorkDir` in `Setup` (avoids `mkdir /no-home: permission denied`).
- Container image must be `debian:stable-slim` (unless language-specific runtime demo).
- `internal/runtime` tests do NOT use `AcquireContainerSuiteLock` (semaphore alone suffices).

## 4. Testscript/txtar Patterns

### CUE Correctness

- All `invowkfile.cue` implementations must declare `platforms:` with all applicable platforms.
- Virtual runtime: `[{name: "linux"}, {name: "macos"}, {name: "windows"}]`.
- Native runtime: platform-split with separate Linux/macOS and Windows implementations.
- Container runtime: `platforms: [{name: "linux"}]` only.
- `runtimes` and `platforms` are struct lists, not string arrays.

### Workspace and Isolation

- ALL file entries under `$WORK` -- root entries pollute other tests.
- Tests needing broken fixtures must isolate into subdirectories (`cd $WORK/subdir`).
- Use `cd $WORK` for inline CUE tests, `cd $PROJECT_ROOT` only for dogfooding.
- No `env.Cd` in test `Setup` function.

### Skip Guards

- `[!container-available] skip` for container tests.
- `[in-sandbox] skip` for sandbox-sensitive tests.
- `[windows]`, `[linux]`, `[darwin]` are built-in conditions.
- `[GOOS:windows]` is NOT valid (use `[windows]`).
- Windows testscript setup must set `APPDATA=WorkDir/appdata` and `USERPROFILE=WorkDir`.

### Assertion Patterns

- `stdout`/`stderr` patterns are Go regex -- escape parentheses: `\(s\)` not `(s)`.
- CLI error tests must check BOTH stdout (styled handler output) AND stderr (Cobra error rendering).
- `! stderr .` on happy-path tests to verify no error output.
- No placeholder env vars in setup (only production-used vars).

### Line Endings

- `.gitattributes` enforces `*.txtar text eol=lf`.
- SHA hash mismatches on Windows if `core.autocrlf=true` converts line endings.

## 5. Flakiness Signatures

1. **`time.Sleep` in assertions** -- Detect: grep for `time.Sleep` in `_test.go` files.
2. **Shared mock across parallel subtests** -- Detect: `MockCommandRecorder` or similar mock + `Reset()` in parallel loop.
3. **`os.MkdirTemp` + parallel subtests** -- Detect: `os.MkdirTemp` in parent with `t.Parallel()` subtests.
4. **Missing container semaphore** -- Detect: container `Execute()`/`ExecuteCapture()` without `ContainerSemaphore()`.
5. **Bare `t.Context()` in container tests** -- Detect: `t.Context()` passed to container `Execute()` without `ContainerTestContext()`.
6. **Time-based uniqueness** -- Detect: `time.Now()` for unique IDs without atomic counter fallback.
7. **Port collision** -- Detect: hardcoded ports without `net.Listen(":0")` pattern.
8. **Missing `t.Cleanup()` for goroutines** -- Detect: goroutine started in test without cleanup/cancellation.
9. **Race on shared map** -- Detect: map accessed from both test goroutine and spawned goroutine without mutex.
10. **Unguarded `XDG_CONFIG_HOME`** -- Detect: tests relying on XDG fallback without `unset XDG_CONFIG_HOME`.
11. **Txtar workspace contamination** -- Detect: broken fixture files at `$WORK` root affecting other tests.
12. **Container daemon stall** -- Detect: container test without bounded `ContainerTestContext()` or `Deadline`.

## 6. Lint Directive Patterns

### nolintlint Directive Lifecycle

The `nolintlint` linter (enabled in `.golangci.toml` with `require-specific = true`) validates
all `//nolint:` directives. This is the most common source of "fix creates new problem" failures:
the test logic is correct, but CI goes red because of a stale lint suppression.

**Lifecycle rules:**

| Rule | What Happens If Violated |
|------|--------------------------|
| Always name the linter: `//nolint:tparallel` not `//nolint` | `nolintlint` reports "should mention specific linter" |
| Remove directive when the underlying issue is fixed | `nolintlint` reports "directive is unused" (stale suppression) |
| Add justification comment: `//nolint:tparallel // CUE not thread-safe` | Required by project convention |
| Directive must be on the correct line | Misplaced directives suppress nothing and become stale |

**Common failure pattern**: A fix removes the code that triggered a linter warning but leaves
the `//nolint:` directive in place. The next `make lint` run fails with `nolintlint` reporting
the directive as unused/stale. Detection: `grep -rn '//nolint:' --include='*_test.go'` in
recently modified files, then verify each directive is still needed with `make lint`.

**When adding `//nolint:`**: Run `make lint` without it first. Only add if lint fails.
**When removing code near `//nolint:`**: Run `make lint` after removal to confirm directive is still needed.

### t.Helper() Semantics

`t.Helper()` marks the calling function as a test helper. When a helper's assertion fails,
Go reports the caller's file:line rather than the helper's. Missing `t.Helper()` causes
confusing failure output — the CI log points to the wrong file and line.

**When to add `t.Helper()`:**
- Any function accepting `*testing.T` (or `testing.TB`) that calls `t.Error`, `t.Fatal`,
  `t.Errorf`, `t.Fatalf`, or other assertion helpers
- Nested helpers: if helper A calls helper B which calls `t.Fatal`, BOTH A and B need
  `t.Helper()` for the stack trace to correctly point to the test call site
- Detection: `grep -rn 'func.*\*testing\.T' --include='*_test.go'` then check each
  unexported function for `t.Helper()` presence

**When NOT to add `t.Helper()`:**
- Functions passed directly to `t.Run()` as subtests — these ARE the test, not helpers.
  Adding `t.Helper()` would hide the actual failure line.
- Functions that only use `t.Log` or `t.Skip` (no failure reporting)
- Functions that return errors to the caller instead of calling `t.Error`/`t.Fatal`

### t.Fatal vs t.Error Before Dereferences

Use `t.Fatalf` (not `t.Errorf`) when the next line would dereference the result. `t.Errorf`
continues execution, causing a nil-pointer panic that masks the actual test failure.

Detection: look for patterns where `t.Error`/`t.Errorf` checks a nil value and the next
statement dereferences it (e.g., `result.Foo` after `if result == nil { t.Error(...) }`).
