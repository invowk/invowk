---
name: go-testing
description: >-
  Comprehensive Go 1.22+ testing knowledge. Covers all go test flags and their
  interactions, execution model (compilation, caching, parallel scheduling),
  race detector internals and platform-specific overhead, go vet analyzers,
  context patterns decision tree, parallelism safety framework, testing package
  API, benchmark/fuzz APIs, gotestsum integration, and coverage tooling.
  Primary entry point for testing — references platform skills
  (windows-testing, macos-testing, linux-testing) for OS-level issues.
  Use this whenever working on test code, debugging test failures, or
  optimizing CI performance.
disable-model-invocation: false
---

# Go Testing Skill

Comprehensive reference for the Go testing toolchain and cross-platform test
engineering. This skill covers Go-level concerns; for OS-level primitives that
affect test behavior, consult the platform skills listed below.

## Normative Precedence

1. `.agents/rules/testing.md` — authoritative test policy (organization, parallelism rules, cross-platform patterns).
2. `.agents/rules/go-patterns.md` — authoritative context propagation and code style.
3. This skill — Go testing toolchain knowledge, decision frameworks, reference material.
4. `.agents/skills/testing/SKILL.md` — invowk-specific test patterns, testscript, TUI/container testing.

Do NOT duplicate content from rules; cross-reference instead.

## Platform Skill Router

When a test failure is platform-specific, consult the right platform skill:

| Symptom / Area | Skill | Key Topics |
|---|---|---|
| `TerminateProcess` exit code masking | `windows-testing` | No POSIX signals; exit code 1 indistinguishable from normal failure |
| `ERROR_SHARING_VIOLATION` on temp files | `windows-testing` | Antivirus scanning, mandatory file locking |
| Timer-based test flakiness (Windows) | `windows-testing` | Default 15.6ms timer resolution |
| Race detector timeout on large suites | `windows-testing` | Higher overhead; `-timeout 15m` pattern |
| lipgloss `sync.Once` race on Windows | `windows-testing` | `TestMain` pre-warm pattern |
| APFS case-insensitive filename collision | `macos-testing` | Case-preserving but case-insensitive FS |
| `/tmp` path comparison mismatch | `macos-testing` | `/tmp` → `/private/tmp` symlink |
| Watcher missing rapid file events | `macos-testing` | `kqueue` aggressive event coalescing |
| Timer-based test flakiness (macOS) | `macos-testing` | Power-saving timer coalescing |
| gotestsum false-FAIL on parallel subtests | `macos-testing` | Missing `-v` flag; macOS-specific gotestsum issue |
| Container test hangs indefinitely | `linux-testing` | Missing `WaitDelay`, unbounded context |
| `inotify: too many watches` (ENOSPC) | `linux-testing` | Per-user inotify watch limits |
| Container OOM with `-race` | `linux-testing` | 10x memory overhead + cgroup limits |
| Podman rootless permission errors | `linux-testing` | User namespace mapping, cgroup v2 |
| flock contention in CI | `linux-testing` | Cross-binary test serialization |
| ARM64 memory ordering differences | `macos-testing` | Apple Silicon relaxed ordering |

## Go Test Execution Model

### Compilation and Caching

`go test` compiles each package into a separate test binary. Results are cached in
`$GOCACHE` based on: package source, test source, build flags, environment variables
read by tests, and files read via `os.Open` (heuristic).

Cache invalidation:
- `-count=1` — forces re-execution (bypasses cache entirely)
- `-race` — different build; cached separately from non-race builds
- Any flag change — `-short`, `-v`, `-timeout`, etc. create distinct cache entries
- File changes — any `.go` file in the package or its dependencies

### Parallel Scheduling

Go's test parallelism operates at two levels:

1. **Inter-package**: packages are compiled and tested in parallel (bounded by `GOMAXPROCS`).
   Each package's tests run in their own process.
2. **Intra-package**: tests calling `t.Parallel()` run concurrently within the package,
   bounded by `-parallel` (defaults to `GOMAXPROCS`).

The `-parallel` flag controls ONLY intra-package parallelism. There is no flag to
control inter-package parallelism directly (`-p` controls build parallelism, not
test execution).

### Binary Mode vs List Mode

- **List mode** (default): `go test ./...` — compiles and runs tests per package.
- **Binary mode**: `go test -c -o test.exe && ./test.exe` — compile once, run the
  binary directly. Useful for profiling, debugging, and custom test execution.

## Test Flags Quick Reference

| Flag | Purpose | Key Interactions |
|------|---------|-----------------|
| `-race` | Enable race detector | 5-10x mem, 2-20x CPU; cached separately; see `references/race-detector-guide.md` |
| `-count N` | Run each test N times | `-count=1` bypasses cache; `-count=0` means "use cache" |
| `-parallel N` | Max concurrent `t.Parallel()` tests | Defaults to `GOMAXPROCS`; only intra-package |
| `-timeout D` | Kill test binary after duration | Default 10m; affects ALL packages in the run |
| `-short` | Set `testing.Short()` to true | Convention: skip slow/integration tests |
| `-v` | Verbose output | **Required** for gotestsum `--rerun-fails` with parallel subtests |
| `-run R` | Run only tests matching regex | Applied per-subtest: `-run TestFoo/case_one` |
| `-skip R` | Skip tests matching regex (Go 1.22+) | Applied after `-run`; does NOT affect benchmarks |
| `-failfast` | Stop on first failure | Useful for quick CI feedback; does NOT stop other packages |
| `-shuffle on\|off\|N` | Randomize test execution order | Detects order-dependent tests; N is seed |
| `-cover` | Enable coverage | `-coverprofile=c.out` for file output; `-covermode=atomic` with `-race` |
| `-json` | JSON output | Machine-readable; used by gotestsum |
| `-list R` | List tests matching regex | No execution; useful for discovery |

Full flag matrix with all interactions: see `references/test-flags-matrix.md`.

## Race Detector

The Go race detector is built on Google's ThreadSanitizer (TSan). It instruments
memory accesses at compile time and tracks happens-before relationships at runtime.

**Key characteristics:**
- **Memory overhead**: 5-10x — each 8-byte memory access gets a shadow word
- **CPU overhead**: 2-20x — instrumentation on every read/write
- **Coverage**: only detects races on *executed* code paths (not static analysis)
- **No false positives**: if it reports a race, it is a real race (barring `unsafe.Pointer` tricks)
- **Platform-specific behavior**: see `references/race-detector-guide.md`

**When to use `-race`:**
- Always in CI (every test run)
- During local development when touching concurrent code
- NOT in benchmarks (overhead distorts measurements)

**Interaction with `-parallel`:** The race detector adds synchronization overhead that
changes timing. A test that passes without `-race` may fail with it due to different
goroutine scheduling. This is by design — the race detector exposes latent races.

See `references/race-detector-guide.md` for: ThreadSanitizer internals, platform-specific
behavior (Windows sync primitives, lipgloss pre-warm pattern), common race patterns.

## Context Patterns Decision Tree

Use this decision tree to select the right context for test code:

```
Is this a TestMain / *testing.M scope?
├── YES → context.Background()  (no *testing.T available)
├── NO → Is this a benchmark?
│   ├── YES → b.Context()  (cancelled when benchmark ends)
│   └── NO → Is there a time-bounded operation? (network, subprocess, etc.)
│       ├── YES → context.WithTimeout(t.Context(), duration)
│       │   └── For container tests: testutil.ContainerTestContext(t, timeout)
│       ├── NO → Do you need manual cancellation control?
│       │   ├── YES → context.WithCancel(t.Context())
│       │   └── NO → t.Context()  (DEFAULT — cancelled when test ends)
│       └── SPECIAL: testscript env.Setup? → context.Background() with explicit timeout
│           (no *testing.T available in Setup)
```

**Code patterns:**

```go
// DEFAULT — use for most unit tests
func TestFoo(t *testing.T) {
    ctx := t.Context()
    result, err := myFunc(ctx, input)
    // ...
}

// BOUNDED — use for operations that may hang (network, subprocess)
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
    defer cancel()
    result, err := client.Call(ctx)
    // ...
}

// CONTAINER — use the project's ContainerTestContext helper
func TestContainer(t *testing.T) {
    ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
    result, err := engine.Execute(ctx, cmd)
    // ...
}

// MANUAL CANCEL — use when you need to cancel mid-test
func TestCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(t.Context())
    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel()
    }()
    err := longOperation(ctx)
    require.ErrorIs(t, err, context.Canceled)
}

// TestMain — no *testing.T available
func TestMain(m *testing.M) {
    ctx := context.Background()
    // setup with ctx...
    os.Exit(m.Run())
}
```

Cross-ref: `.agents/rules/go-patterns.md` § "Context Usage" for production context rules.
Cross-ref: `.agents/rules/testing.md` § "Test Context Usage" for test-specific policy.

## Parallelism Decision Framework

Use `t.Parallel()` by default. Only omit it when the test touches shared mutable state.

### Resource Safety Matrix

| Resource | Safe with `t.Parallel()`? | Reason | Mitigation |
|----------|--------------------------|--------|------------|
| Pure computation | YES | No shared state | — |
| `t.TempDir()` | YES | Each call returns unique dir | — |
| `t.Context()` | YES | Each test gets independent context | — |
| `net.Listen(":0")` | YES | OS assigns unique port | — |
| Read-only package vars | YES | No mutation | — |
| `t.Setenv()` | **NO** | Modifies process-global `os.Environ` | Omit `t.Parallel()` |
| `os.Setenv()` | **NO** | Same as `t.Setenv()` but without test-scoping | Omit `t.Parallel()` |
| `os.Chdir()` | **NO** | Process-global working directory | Omit `t.Parallel()` |
| `os.Stdin` replacement | **NO** | Process-global file descriptor | Omit `t.Parallel()` |
| Shared `sync.Mutex`-protected struct | DEPENDS | Safe if all accesses hold the lock | Verify lock discipline |
| `cue.Value` / `*cue.Context` | **NO** | CUE is not thread-safe (internal mutation) | Serial subtests; `//nolint:tparallel` |
| Shared filesystem path (e.g., SSH host keys) | **NO** | Write conflicts across tests | Serial subtests or unique paths |
| Shared channel (e.g., `RequestChannel()`) | **NO** | Multiplexed reads/sends race | Serial subtests; `//nolint:tparallel` |
| `exec.Command` with inherited env | DEPENDS | Safe if no `t.Setenv`; process gets snapshot | — |
| HTTP test server (`httptest.NewServer`) | YES | Each call creates independent server | — |
| lipgloss `Style` / `Render()` | YES | Pure value type; zero global state in rendering path. Terminal detection runs once at package init. | — |
| bubbletea `Model.Init()` / `View()` | YES | No shared state outside `Program`. Tests don't run `Program`. | — |
| colorprofile color conversion | YES | Global cache protected by `sync.RWMutex` | — |

### Go 1.22+ Loop Variable Rule

Go 1.22 changed loop variable semantics — each iteration gets a fresh copy. The old
`tt := tt` rebinding pattern is no longer needed and should be removed:

```go
// Go 1.22+ — CORRECT (no rebinding needed)
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // tt is already a fresh copy per iteration
    })
}
```

For maps in parallel subtests, use `maps.Copy` to avoid concurrent map read/write.

Cross-ref: `.agents/rules/testing.md` § "Test Parallelism" for canonical parallel patterns.
Cross-ref: `.agents/skills/review-tests/references/known-exceptions.md` for legitimate exceptions.

## `testing` Package API Summary

### Lifecycle

| Method | Purpose | Notes |
|--------|---------|-------|
| `t.Cleanup(f)` | Register cleanup (LIFO order) | Runs after test + subtests complete |
| `t.TempDir()` | Auto-cleaned temp directory | Unique per call; cleaned after test |
| `t.Setenv(k, v)` | Set env var, restore after test | **Panics** if called after `t.Parallel()` |
| `t.Context()` | Context cancelled when test ends | Go 1.21+; default choice for test contexts |
| `t.Chdir(dir)` | Change working dir, restore after test | Go 1.24+; **panics** after `t.Parallel()` |

### Control Flow

| Method | Purpose | Notes |
|--------|---------|-------|
| `t.Run(name, f)` | Create subtest | Subtests inherit parent's timeout |
| `t.Parallel()` | Mark test for parallel execution | Must be first call in subtest body |
| `t.Skip(args...)` / `t.Skipf(fmt, ...)` | Skip test | Use for platform/env guards |
| `t.SkipNow()` | Skip immediately | Calls `runtime.Goexit()` |
| `t.Deadline()` | Returns test timeout deadline | `(time.Time, bool)` — false if no `-timeout` |

### Assertions

| Method | Behavior |
|--------|----------|
| `t.Error(args...)` / `t.Errorf(fmt, ...)` | Record failure; continue |
| `t.Fatal(args...)` / `t.Fatalf(fmt, ...)` | Record failure; stop test |
| `t.FailNow()` | Fail immediately (calls `runtime.Goexit()`) |
| `t.Log(args...)` / `t.Logf(fmt, ...)` | Log (shown only on failure or with `-v`) |
| `t.Helper()` | Mark as helper (skip in failure stack trace) |

**`t.Helper()` rule**: Call `t.Helper()` as the first statement in any function that
accepts `*testing.T` and calls assertion methods (`t.Error`, `t.Fatal`, etc.). This
includes transitively — if helper A calls helper B which calls `t.Fatal`, both A and B
need `t.Helper()`. Functions passed to `t.Run()` as subtests do NOT need `t.Helper()`.
See `review-tests/references/pattern-catalog.md` § "t.Helper() Semantics".

**Critical**: `t.Fatal` / `t.FailNow` inside a goroutine will panic — they call
`runtime.Goexit()` which only exits the current goroutine, not the test. Use
`t.Error` + return in goroutines, or communicate failures via channels.

### Package-Level Functions

| Function | Purpose |
|----------|---------|
| `testing.Short()` | True if `-short` flag set |
| `testing.Verbose()` | True if `-v` flag set |
| `testing.Testing()` | True if running under `go test` (Go 1.21+) |

## `go vet` Analyzers

Key analyzers affecting test code (full catalog: `references/go-vet-analyzers.md`):

| Analyzer | What It Catches | Test Relevance |
|----------|----------------|----------------|
| `testinggoroutine` | `t.Fatal`/`t.FailNow` called from non-test goroutine | **Critical** — common bug in concurrent tests |
| `copylocks` | Copying a value containing a `sync.Mutex` | Catches lock copy in test helpers |
| `lostcancel` | `context.WithCancel` without calling cancel | Goroutine leak in tests |
| `loopclosure` | Closure captures loop variable | Mitigated in Go 1.22+; still relevant for `go` statements |
| `waitgroup` | `sync.WaitGroup.Add` called inside goroutine | Common test synchronization bug |
| `printf` | Format string mismatches | Catches `t.Errorf`/`t.Fatalf` format bugs |

## nolintlint Directive Lifecycle

The project enables `nolintlint` with `require-specific = true`. Every `//nolint:` directive
must name the suppressed linter and include a justification comment. Stale directives (where
the suppressed issue no longer exists) cause lint failures — this is the most common "fix
creates new problem" pattern in this codebase.

**Key rule**: When removing or moving code near a `//nolint:` directive, always run
`make lint` afterward to verify the directive is still needed.

See `review-tests/references/pattern-catalog.md` § "nolintlint Directive Lifecycle"
for the full lifecycle rules and common failure patterns.

## Build Tags for Tests

- **`_test.go` suffix**: automatically excluded from production builds; compiled only during `go test`.
- **`*_<GOOS>_test.go`**: platform-constrained test file (e.g., `run_lock_linux_test.go`).
- **`//go:build <expr>`**: explicit build constraint (e.g., `//go:build !windows`).
- **`-tags` flag**: `go test -tags=integration ./...` — enable custom build tags.

Platform-specific test files in this project:
- `internal/tui/testmain_windows_test.go` — `//go:build windows`
- `internal/watch/watcher_fatal_unix_test.go` — `//go:build !windows`
- `internal/watch/watcher_fatal_windows_test.go` — `//go:build windows`
- `internal/container/podman_sysctl_linux_test.go` — `//go:build linux`
- `internal/runtime/run_lock_linux_test.go` — `//go:build linux`

## Standard Library Test Helpers

- **`testing/fstest`**: `MapFS` — in-memory filesystem for testing `fs.FS` implementations.
- **`testing/iotest`**: `ErrReader`, `HalfReader`, `DataErrReader`, `OneByteReader`, `TimeoutReader` — simulate I/O edge cases.
- **`testing/slogtest`**: `TestHandler` — verify `slog.Handler` implementations (Go 1.22+).

## Benchmark API

| Method | Purpose | Notes |
|--------|---------|-------|
| `b.Loop()` | Preferred iteration (Go 1.24+) | Compiler cannot optimize away; replaces `b.N` loop |
| `b.N` | Legacy iteration count | `for i := 0; i < b.N; i++ { ... }` |
| `b.ReportAllocs()` | Include alloc stats | Call once before loop |
| `b.ResetTimer()` | Reset after expensive setup | |
| `b.StopTimer()` / `b.StartTimer()` | Pause timing | |
| `b.RunParallel(body)` | Parallel benchmark | `body` receives `*testing.PB` with `pb.Next()` loop |
| `b.ReportMetric(n, unit)` | Custom metrics | e.g., `b.ReportMetric(float64(size), "bytes/op")` |
| `b.Context()` | Benchmark context | Cancelled when benchmark ends |

Full guide: `references/benchmark-fuzzing-guide.md`.

## Fuzzing API

| Method | Purpose |
|--------|---------|
| `f.Fuzz(func(t *testing.T, args...))` | Define fuzz target |
| `f.Add(args...)` | Add seed corpus entry |
| `f.Skip(args...)` | Skip (in fuzz function) |

Run with `go test -fuzz=FuzzName -fuzztime=30s`.

Corpus stored in `testdata/fuzz/FuzzName/`. Committed corpus entries run as
regular tests (regression testing). Full guide: `references/benchmark-fuzzing-guide.md`.

## gotestsum Integration

The project uses `gotestsum` for enhanced test output and flaky test detection.

| Flag | Purpose | Notes |
|------|---------|-------|
| `--rerun-fails` | Re-run only failing tests | **Requires `-v`** for parallel subtest tracking |
| `--rerun-fails-max-failures N` | Skip reruns if > N failures | Real regression, not flakiness |
| `--rerun-fails-report FILE` | Log which tests needed reruns | Flake signal |
| `--format testdox` | Human-readable output | Test names as sentences |
| `--junitfile FILE` | JUnit XML output | For CI test reporting |
| `--packages ./...` | Package list | Passed before `--` separator |

**Critical**: Without `-v`, gotestsum cannot reconcile parallel subtest statuses —
it may report a parent test as FAIL even when all subtests pass. This is especially
problematic on macOS CI runners. Always use `-v` with `--rerun-fails`.

## Coverage Tooling

| Flag | Purpose | Notes |
|------|---------|-------|
| `-cover` | Enable coverage | Shows per-package % |
| `-coverprofile=c.out` | Write coverage data | Use `go tool cover -html=c.out` to view |
| `-coverpkg=pkg1,pkg2` | Measure coverage of non-test packages | Cross-package coverage |
| `-covermode=set\|count\|atomic` | Coverage mode | `atomic` required with `-race` |

CLI test coverage uses a special flow: build binary with `-cover`, set `GOCOVERDIR`,
merge per-test data with `go tool covdata textfmt`. See `make test-cli-cover`.

---

## Related Skills

| Skill | When to Use |
|---|---|
| `windows-testing` | Windows OS primitives: process lifecycle, signals, file system, timer resolution |
| `macos-testing` | macOS OS primitives: APFS, kqueue, timer coalescing, /tmp symlink, ARM64 |
| `linux-testing` | Linux OS primitives: container infrastructure, inotify, cgroups, OOM killer |
| `testing` | Invowk-specific: testscript, TUI component testing, container runtime testing |
| `review-tests` | Test suite audit with 102-item checklist across 8 surfaces |
| `tmux-testing` | TUI e2e testing with tmux |
| `tui-testing` | VHS-based TUI visual testing |
