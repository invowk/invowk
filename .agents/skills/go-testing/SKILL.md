---
name: go-testing
description: >-
  Go test toolchain entry point for Go test code: `go test` flags, execution
  model and caching, per-package timeouts, race detector reports and
  overhead, `go vet` analyzers, context patterns, parallelism safety
  (`t.Parallel()`), resolver-backed path assertions (`invowk.path`,
  `INVOWK_PATH_*`/`INVOWK_ANCHOR_*`, `/private/var`, `RUNNER~1`), testing
  package API, benchmark/fuzz APIs, gotestsum, and coverage. Use when writing
  or editing Go tests, debugging test failures, flakes or race reports, or
  tuning CI performance. OS issues: windows-testing, macos-testing,
  linux-testing; Invowk conventions and testscript: `testing`.
---

# Go Testing Skill

Go testing toolchain knowledge and decision frameworks. For OS-level primitives
that affect test behavior, use the platform skills in the router below.

`.agents/rules/testing.md` and `.agents/skills/go/SKILL.md` win over this skill
on conflict (AGENTS.md "Governance precedence"). Between skills: this one owns
the Go toolchain; `testing` owns Invowk conventions, testscript, TUI and
container surfaces. Do NOT duplicate content from rules or the Go skill;
cross-reference instead.

## Platform Skill Router

When a test failure is platform-specific, consult the right platform skill:

| Symptom / Area | Skill | Key Topics |
|---|---|---|
| `TerminateProcess` exit code masking | `windows-testing` | No POSIX signals; exit code 1 indistinguishable from normal failure |
| `ERROR_SHARING_VIOLATION` on temp files | `windows-testing` | Antivirus scanning, mandatory file locking |
| Timer-based test flakiness (Windows) | `windows-testing` | Default 15.6ms timer resolution |
| Race detector timeout on large suites | `windows-testing` | Higher `-race` overhead; one package timeout turns every gotestsum result of that package `(unknown)` |
| Windows TUI rendering/race symptoms | `windows-testing` | Current TUI race guidance and known obsolete pre-warm patterns |
| APFS case-insensitive filename collision | `macos-testing` | Case-preserving but case-insensitive FS |
| `/tmp` path comparison mismatch | `macos-testing` | `/tmp` → `/private/tmp` symlink |
| `/var` vs `/private/var` path comparison mismatch | `macos-testing` | macOS runner symlink resolution |
| Watcher missing rapid file events | `macos-testing` | `kqueue` aggressive event coalescing |
| Timer-based test flakiness (macOS) | `macos-testing` | Power-saving timer coalescing |
| gotestsum false-FAIL on parallel subtests | this skill, "gotestsum Integration" | Missing `-v` with `--rerun-fails`; seen most on macOS runners |
| `RUNNER~1` vs long temp path mismatch | `windows-testing` | 8.3 short path aliases |
| Virtual runtime env/path assertion mismatch | `macos-testing`, `windows-testing` | Assert against virtual resolver outputs (see "Cross-Platform Path Contract Tests") |
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

goplint findings are not a test-toolchain concern: see the goplint row in
`.agents/rules/commands.md` and `docs/goplint/README.md`.

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
  relevant vet analyzers and the golangci-lint test linters.
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
│       └── SPECIAL: structurally forced → context.Background() with explicit timeout
│           (see the list below)
```

`context.Background()` outside `TestMain` is allowed only when it is
structurally forced (rule "Test Context Usage"):

- testscript `Env.Setup` and `Env.Defer` hooks: no `*testing.T` in scope.
- `t.Cleanup` functions: `t.Context()` is already cancelled when they run.
- rapid property callbacks: `rapid.T` has no `Context()`.

Bound it with `context.WithTimeout`, add a comment saying why, and use
`//nolint:usetesting // <reason>` when the linter fires. The rule currently
names only `TestMain` and `Env.Defer`; the other cases follow existing repo
usage (`tests/cli/cmd_container_test.go` Setup probe, `tests/cli/tui_tmux_test.go`
`kill`, `internal/app/modulesync/tidy_formal_test.go`).

Container tests that call `Execute()`/`ExecuteCapture()` use the project helper
(CRITICAL rule item in "Integration and Container Tests"):

```go
func TestContainer(t *testing.T) {
    ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
    result, err := engine.Execute(ctx, cmd)
    // ...
}

// TestMain: context.Background(): *testing.M has no Context() method
```

Cross-ref: `.agents/skills/go/SKILL.md` § "Context And Processes" for production context rules.
Cross-ref: `.agents/rules/testing.md` § "Test Context Usage" for test-specific policy.

## Parallelism Decision Framework

Use `t.Parallel()` by default and omit it only for shared mutable state; the
policy and placement rules live in `.agents/rules/testing.md` § "Test Parallelism".
Use this matrix to decide whether a resource is safe.

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
| `t.Chdir()` | **NO** | Process-global working directory (test-scoped restore only) | Omit `t.Parallel()` |
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

Loop variables: no `tt := tt` rebinding (`modernize` `forvar`), and use
`maps.Copy` for maps shared with parallel subtests (rule "Test Parallelism").

Cross-ref: `.agents/skills/review-tests/references/known-exceptions.md` for legitimate exceptions.

## Test API Guardrails

`t.Parallel()`-first placement is an Invowk lint/policy contract (rule "Test
Parallelism"), not a standard-library API requirement. `t.Helper()` placement
and reporting worker-goroutine failures to the test goroutine are covered in
`.agents/skills/testing/references/pre-write-and-go-patterns.md` § "Pre-Write
Checks" and § "Deterministic Tests". Read `references/go-vet-analyzers.md` for
the analyzers that enforce these patterns.

## nolintlint Directive Lifecycle

`.golangci.toml` configures `nolintlint` with `allow-unused = false`,
`require-explanation = true` and `require-specific = true`. Every `//nolint:`
directive must name the suppressed linter and carry a justification (both
lint-enforced), and a directive whose issue no longer exists fails the gate —
the most common "fix creates new problem" pattern in this codebase.

**Key rule**: When removing or moving code near a `//nolint:` directive, always run
`make lint` afterward to verify the directive is still needed.

See `.agents/skills/review-tests/references/pattern-catalog.md` § "nolintlint Directive Lifecycle"
for the full lifecycle rules and common failure patterns.

## Build Tags for Tests

- **`_test.go` suffix**: automatically excluded from production builds; compiled only during `go test`.
- **`*_<GOOS>_test.go`**: platform-constrained test file (e.g., `run_lock_linux_test.go`).
- **`//go:build <expr>`**: explicit build constraint (e.g., `//go:build !windows`).
- The repo uses only GOOS constraints in tests (`linux`, `windows`, `!windows`);
  there are no custom test tags such as `integration`. Integration gating uses
  `testing.Short()` (rule "Integration and Container Tests").

Discover current platform-specific test files with:

```bash
rg --files -g '*_linux_test.go' -g '*_windows_test.go' -g '*_darwin_test.go'
rg -l '^//go:build' -g '*_test.go'
```

## Benchmarks and Fuzzing

Read `references/benchmark-fuzzing-guide.md`. Prefer `b.Loop()` on the pinned
Go toolchain, use `b.Context()`, keep race instrumentation out of performance
measurements, and commit useful fuzz seeds as regression inputs.

## gotestsum Integration

The project uses `gotestsum` for enhanced test output and flaky test detection.
Install the pinned version (`.agents/rules/version-pinning.md`):
`go install gotest.tools/gotestsum@v1.13.0`.

| Flag | Purpose | Notes |
|------|---------|-------|
| `--rerun-fails` | Re-run only failing tests | **Requires `-v`** for parallel subtest tracking |
| `--rerun-fails-max-failures N` | Skip reruns if > N failures | Real regression, not flakiness |
| `--rerun-fails-report FILE` | Log which tests needed reruns | Flake signal |
| `--format testdox` | Human-readable output | Test names as sentences |
| `--junitfile FILE` | JUnit XML output | For CI test reporting |
| `--packages ./...` | Package list | Passed before `--` separator |

```bash
# Like `make test` (which also filters the package list), plus CI-style report
# flags; go test flags such as -v go after `--`
gotestsum --format testdox --rerun-fails --rerun-fails-max-failures 5 \
  --rerun-fails-report rerun-report.txt --junitfile test-results.xml \
  --packages ./... -- -v
```

**Critical**: Without `-v`, gotestsum cannot reconcile parallel subtest statuses —
it may report a parent test as FAIL even when all subtests pass. This is especially
problematic on macOS CI runners. Always use `-v` (after `--`) with `--rerun-fails`
in new or edited gotestsum invocations. Some existing CI lanes do not follow this
yet; do not copy them as templates. Audit workflow changes with
`rg -n -e '--rerun-fails' -e 'gotestsum' .github Makefile scripts`.

## Coverage

Use `references/test-flags-matrix.md` for coverage flag interactions. CLI test
coverage builds an instrumented binary, sets `GOCOVERDIR`, and merges data with
`go tool covdata`; use `make test-cli-cover` rather than reconstructing it.

## Related Skills

`testing` (Invowk patterns, testscript, TUI and container surfaces),
`review-tests` (read-only suite audits), `tmux-testing` (durable interactive TUI
tests), `tui-testing` (VHS visual debugging), plus the platform skills above.
