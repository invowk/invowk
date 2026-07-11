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
---

# Go Testing Skill

Comprehensive reference for the Go testing toolchain and cross-platform test
engineering. This skill covers Go-level concerns; for OS-level primitives that
affect test behavior, consult the platform skills listed below.

## Normative Precedence

1. `.agents/rules/testing.md` — authoritative test policy (organization, parallelism rules, cross-platform patterns).
2. `.agents/skills/go/SKILL.md` — authoritative context propagation and code style.
3. This skill — Go testing toolchain knowledge, decision frameworks, reference material.
4. `.agents/skills/testing/SKILL.md` — invowk-specific test patterns, testscript, TUI/container testing.

Do NOT duplicate content from rules or the Go skill; cross-reference instead.

## Platform Skill Router

When a test failure is platform-specific, consult the right platform skill:

| Symptom / Area | Skill | Key Topics |
|---|---|---|
| `TerminateProcess` exit code masking | `windows-testing` | No POSIX signals; exit code 1 indistinguishable from normal failure |
| `ERROR_SHARING_VIOLATION` on temp files | `windows-testing` | Antivirus scanning, mandatory file locking |
| Timer-based test flakiness (Windows) | `windows-testing` | Default 15.6ms timer resolution |
| Race detector timeout on large suites | `windows-testing` | Higher overhead; `-timeout 15m` pattern |
| Windows TUI rendering/race symptoms | `windows-testing` | Current TUI race guidance and known obsolete pre-warm patterns |
| APFS case-insensitive filename collision | `macos-testing` | Case-preserving but case-insensitive FS |
| `/tmp` path comparison mismatch | `macos-testing` | `/tmp` → `/private/tmp` symlink |
| `/var` vs `/private/var` path comparison mismatch | `macos-testing` | macOS runner symlink resolution |
| Watcher missing rapid file events | `macos-testing` | `kqueue` aggressive event coalescing |
| Timer-based test flakiness (macOS) | `macos-testing` | Power-saving timer coalescing |
| gotestsum false-FAIL on parallel subtests | `macos-testing` | Missing `-v` flag; macOS-specific gotestsum issue |
| `RUNNER~1` vs long temp path mismatch | `windows-testing` | 8.3 short path aliases |
| Virtual runtime env/path assertion mismatch | `macos-testing`, `windows-testing` | Assert against virtual resolver outputs |
| Container test hangs indefinitely | `linux-testing` | Missing `WaitDelay`, unbounded context |
| `inotify: too many watches` (ENOSPC) | `linux-testing` | Per-user inotify watch limits |
| Container OOM with `-race` | `linux-testing` | Workload-dependent instrumentation overhead + cgroup limits |
| Podman rootless permission errors | `linux-testing` | User namespace mapping, cgroup v2 |
| flock contention in CI | `linux-testing` | Cross-binary test serialization |
| ARM64 memory ordering differences | `macos-testing` | Apple Silicon relaxed ordering |

## Failure Triage Workflow

1. Capture the exact failing command, package, test name, platform, and whether
   the result came from CI, local `make test`, or a focused `go test`.
2. Reproduce narrowly with `go test -count=1 -run '<TestName>' ./path/...` before
   widening the scope. Add `-race` when the failure involves concurrency or CI
   race lanes.
3. Classify the failure as Go/toolchain, testscript, container, TUI, or
   platform-specific; then load the matching Invowk skill.
4. After the fix, verify with the narrow reproduction and the repo target that
   owns that surface.

## Toolchain Reference Router

Use the smallest reference that owns the question:

- Read [references/test-flags-matrix.md](references/test-flags-matrix.md) for
  execution, caching, flag interactions, package-binary timeouts, and current CI
  combinations. `-timeout` applies independently to each package's test binary;
  it is not one shared deadline for every package selected by `go test`.
- Read [references/race-detector-guide.md](references/race-detector-guide.md)
  for race report interpretation and common race patterns. Treat a report as
  strong evidence and inspect both access stacks; do not promise an absolute
  zero-false-positive guarantee around `unsafe`, cgo, or instrumentation edges.
- Read [references/go-vet-analyzers.md](references/go-vet-analyzers.md) for test-
  relevant vet analyzers.
- Read [references/benchmark-fuzzing-guide.md](references/benchmark-fuzzing-guide.md)
  for benchmark and fuzz workflows.

Use `-count=1` to bypass successful test caching during reproduction. Use
`-race` for executed concurrent paths, not benchmarks, and remember that its
overhead can expose latent scheduling assumptions.

## Cross-Platform Path Contract Tests

When a test asserts the output of a resolver-backed API, expected values should
come from that resolver rather than from separate host path normalization. This
matters most for virtual runtime bridges and environment variables:
`invowk.path(...)`, `INVOWK_ANCHOR_*`, and `INVOWK_PATH_*`.

Use the same helper path as production code whenever possible. In
`internal/runtime` tests, that usually means `newVirtualPathResolver`,
`newVirtualPathResolverForInteractiveConfig`, `resolver.anchors`,
`resolver.paths`, and `resolver.resolveBridgePath`.

Avoid deriving these expected strings with `filepath.Join`,
`normalizeExistingOrParent`, `filepath.EvalSymlinks`, or raw
`ctx.EffectiveWorkDir()`. Those helpers can produce platform-specific aliases:
`/var` vs `/private/var` on macOS and `RUNNER~1` vs a long user profile path on
Windows. If the behavior under test is filesystem identity rather than API text,
assert identity with `os.Stat` and `os.SameFile` instead of string equality.

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

Cross-ref: `.agents/skills/go/SKILL.md` § "Context And Processes" for production context rules.
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

## Test API Guardrails

Follow `.agents/rules/testing.md` for `t.Parallel()` placement and exceptions.
That placement is an Invowk lint/policy contract, not a standard-library API
requirement. Keep `t.Setenv`, `t.Chdir`, and other process-global mutations out
of parallel tests. Call `t.Helper()` first in assertion helpers (but not in a
`t.Run` body), and report worker goroutine failures back to the test goroutine
instead of calling `t.Fatal` or `t.FailNow` there. Read
`references/go-vet-analyzers.md` for the analyzers that enforce these patterns.

## nolintlint Directive Lifecycle

The project enables `nolintlint` with `require-specific = true`. Every `//nolint:` directive
must name the suppressed linter and include a justification comment. Stale directives (where
the suppressed issue no longer exists) cause lint failures — this is the most common "fix
creates new problem" pattern in this codebase.

**Key rule**: When removing or moving code near a `//nolint:` directive, always run
`make lint` afterward to verify the directive is still needed.

See `.agents/skills/review-tests/references/pattern-catalog.md` § "nolintlint Directive Lifecycle"
for the full lifecycle rules and common failure patterns.

## Build Tags for Tests

- **`_test.go` suffix**: automatically excluded from production builds; compiled only during `go test`.
- **`*_<GOOS>_test.go`**: platform-constrained test file (e.g., `run_lock_linux_test.go`).
- **`//go:build <expr>`**: explicit build constraint (e.g., `//go:build !windows`).
- **`-tags` flag**: `go test -tags=integration ./...` — enable custom build tags.

Discover current platform-specific test files with:

```bash
rg --files -g '*_linux_test.go' -g '*_windows_test.go' -g '*_darwin_test.go'
```

## Benchmarks and Fuzzing

Read `references/benchmark-fuzzing-guide.md`. Prefer `b.Loop()` on the pinned
Go toolchain, use `b.Context()`, keep race instrumentation out of performance
measurements, and commit useful fuzz seeds as regression inputs.

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
problematic on macOS CI runners. Always use `-v` with `--rerun-fails` in new or
edited gotestsum invocations, and audit workflow changes with
`rg -n 'gotestsum|--rerun-fails|-v' .github Makefile scripts`.

## Coverage

Use `references/test-flags-matrix.md` for coverage flag interactions. CLI test
coverage builds an instrumented binary, sets `GOCOVERDIR`, and merges data with
`go tool covdata`; use `make test-cli-cover` rather than reconstructing it.

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
