# Podman Parallel Test Flakiness

## Issue Summary

Container CLI tests may exhibit sporadic failures when run in parallel on systems using rootless Podman. The failures manifest as:

```
Error: preparing container <id> for attach: crun: write to `/proc/sys/net/ipv4/ping_group_range`
(are all the IDs mapped in the user namespace?): Invalid argument: OCI runtime error
```

This is a **known Podman/crun issue**, not a bug in invowk code.

## Solution (Implemented)

The infrastructure uses a layered defense to handle transient Podman errors while enabling full parallel execution:

### Layer 0: Sysctl Override (Prevention)

On **Linux with local Podman**, `NewPodmanEngine()` creates a temporary file containing a
`containers.conf` override that sets `default_sysctls = []`. The file is written to `os.TempDir()`
and referenced via `CONTAINERS_CONF_OVERRIDE=<path>` in every Podman subprocess's environment.
This prevents crun from writing `net.ipv4.ping_group_range` in new namespaces — eliminating the
race condition at its source.

The temp file is cleaned up when the engine is closed via `BaseCLIEngine.Close()`, which is called
through the `CloseEngine()` helper when the container runtime is released.

When the temp file override **cannot** work, `runWithRetry()` serializes container runs to prevent
the ping_group_range race. The serialization strategy depends on the platform:

- **Linux (podman-remote)**: `acquireRunLock()` uses `flock(2)` on a well-known file
  (`$XDG_RUNTIME_DIR/invowk-podman.lock`, fallback: `os.TempDir()`). This provides **cross-process**
  serialization — all invowk processes on the same machine share the flock, so concurrent testscript
  tests and parallel terminal invocations don't race. Detected via binary name + symlink resolution
  at engine creation time.
- **macOS/Windows**: flock is unavailable (Podman runs in a VM, host-side locks don't reach it).
  `acquireRunLock()` returns an error, and the code falls back to `sync.Mutex` for intra-process
  protection only. The VM boundary provides some natural isolation.
- **Docker**: not affected by the race — Docker never implements `SysctlOverrideChecker`, so
  neither flock nor mutex is acquired.

The `SysctlOverrideChecker` interface allows the runtime layer to query whether the override is active
on a specific engine instance. Build operations (`ensureImage`) are NOT serialized.

| Platform | Prevention | Recovery |
|----------|------------|----------|
| Linux (local Podman) | Temp file `CONTAINERS_CONF_OVERRIDE` | `runWithRetry()` (transient errors) |
| Linux (podman-remote) | **flock** (cross-process) | `runWithRetry()` (transient errors) |
| macOS/Windows | `sync.Mutex` (intra-process) | `runWithRetry()` (transient errors) |
| Docker (any platform) | N/A (no issue) | N/A |
| Temp file unavailable | **flock** / `sync.Mutex` fallback | `runWithRetry()` (transient errors) |

### Stderr Buffering

`runWithRetry()` buffers stderr per-attempt. Transient errors from the container engine (written by
crun/runc directly to the inherited stderr fd) are discarded on retry. Only the final attempt's stderr
is flushed to the caller's original writer. This prevents `ping_group_range` error messages from
leaking to the user's terminal even when retries succeed. When all retries are exhausted, stderr
from the final attempt is flushed so the user receives diagnostic output.

**Tradeoff**: For the non-interactive `Execute()` path, stderr from the user's command is delayed until
the command finishes (no streaming). This is acceptable because container commands in invowk are
short-lived scripts. Interactive mode (`PrepareCommand()`) returns a `PreparedCommand` attached to a
PTY and does not use `runWithRetry()`. `ExecuteCapture()` uses `runWithRetry()` with its own stderr
buffer — the retry stderr buffer captures engine-level errors while the capture buffer collects
the user command's output.

**Key files:**
- `internal/container/podman_sysctl_linux.go` — `createSysctlOverrideTempFile()`, `isRemotePodman()`, `sysctlOverrideOpts()`
- `internal/container/podman_sysctl_other.go` — no-op stub for non-Linux
- `internal/container/engine_base.go` — `CmdCustomizer`, `SysctlOverrideChecker`, `WithCmdEnvOverride()`, `WithSysctlOverridePath()`, `WithSysctlOverrideActive()`, `Close()`
- `internal/container/engine.go` — `CloseEngine()` helper
- `internal/container/podman.go` — `Close()`, `SysctlOverrideActive()` methods on `PodmanEngine`
- `internal/container/sandbox_engine.go` — `Close()`, `SysctlOverrideActive()` forwarding
- `internal/runtime/container.go` — `ContainerRuntime.Close()` (forwards to `CloseEngine()`)
- `internal/runtime/container_exec.go` — `containerRunMu` (fallback mutex), `runWithRetry()` (flock + stderr buffering)
- `internal/runtime/run_lock_linux.go` — `acquireRunLock()`, `runLock` (flock-based cross-process lock)
- `internal/runtime/run_lock_other.go` — no-op stub, forces fallback to `sync.Mutex`

### Layer 1: Production Run-Level Retry

`runWithRetry()` in `internal/runtime/container_exec.go` wraps `engine.Run()` with retry logic that mirrors the existing `ensureImage()` build retry pattern. Transient errors (classified by `container.IsTransientError()`) are retried up to 5 times with exponential backoff (1s, 2s, 4s, 8s). This benefits both production users and test execution.

### Layer 2: Vestigial Global State Removal

The package-level `execCommand` variable in `internal/container/engine.go` was moved to test-only scope in `engine_mock_test.go`. This removed the forced sequential execution of container unit tests, enabling safe `t.Parallel()` across all mock tests.

### Layer 3: Parallel Test Execution

All container tests now run in parallel:

1. **Unit tests** (`internal/container/`): All mock tests use `t.Parallel()` with instance-injected `NewMockCommandRecorder()`.
2. **Integration tests** (`internal/runtime/`): All container integration tests use `t.Parallel()` with independent resources (`t.TempDir()`, unique runtime instances).
3. **CLI tests** (`tests/cli/`): `TestContainerCLI` runs `container_*.txtar` tests in parallel with per-test deadlines and cleanup handlers.
4. **Non-container tests** (`tests/cli/`): `TestCLI` runs all other tests in parallel.
5. **Smoke test retry**: The container availability check includes retry logic with exponential backoff.

### Test Execution

```bash
# Run all tests - all container tests run in parallel
make test

# Run only container CLI tests (parallel)
go test -v -run "TestContainerCLI" ./tests/cli/...

# Run only non-container CLI tests (parallel)
go test -v -run "TestCLI$" ./tests/cli/...

# Run container unit tests (parallel)
go test -v -race ./internal/container/...

# Run container integration tests (parallel, requires container engine)
go test -v -race ./internal/runtime/...

# Skip container tests (short mode)
go test -v -short ./tests/cli/...
```

## Root Cause

When multiple rootless Podman containers start simultaneously, they may race to configure user namespace settings. The `ping_group_range` sysctl is particularly prone to this issue because:

1. Each container attempts to map the setting into its user namespace
2. Concurrent writes to `/proc/sys/net/ipv4/ping_group_range` can fail
3. The crun runtime surfaces this as an "Invalid argument" error

## Affected Environments

- **Fedora Silverblue/Kinoite** (uses `podman-remote` by default)
- **Rootless Podman** on any Linux distribution
- **CI environments** running parallel container tests

## Manual Workarounds (Legacy)

The following workarounds are **no longer needed** since the layered retry + parallel solution is implemented. They are documented for reference only:

### 1. Sequential Execution (superseded by run-level retry)

Previously, container tests were forced sequential to avoid the race. Now `runWithRetry()` absorbs transient errors automatically.

### 2. Manual Retry

Previously required re-running tests manually. Now automatic via `runWithRetry()`.

### 3. Reduced Parallelism

Previously used `-parallel 1` to limit concurrency. No longer needed.

## Verification

To verify whether a failure is this known issue vs. an actual bug:

1. Check the error message contains `ping_group_range` and `OCI runtime error`
2. Re-run the specific failing test - if it passes, it was this issue
3. Run the test sequentially - if it passes consistently, it was this issue

## Related Issues

- Podman issue tracker: Search for "ping_group_range" and "parallel"
- crun issue tracker: User namespace race conditions

## Not Affected

- **Docker** (uses different namespace handling)
- **Rootful Podman** (doesn't use user namespaces)
- **Unit tests with mocks** (no real container operations)
- **Individual test runs** (always pass)
