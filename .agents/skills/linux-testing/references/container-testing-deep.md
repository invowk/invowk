# Container Test Infrastructure Deep Dive

Comprehensive reference for the container test infrastructure that runs
exclusively on Linux. This is the most complex testing subsystem in the project,
with 5 layers of timeout defense, cross-binary serialization, engine health
probes, and container-specific cleanup patterns.

---

## The 5-Layer Timeout Strategy

Container tests are the most failure-prone tests in the project because they
depend on external daemon processes (Docker/Podman) that can hang indefinitely.
The 5-layer strategy ensures that every hang is caught at the narrowest possible
scope.

### Layer 1: Per-Test Context Deadline (ContainerTestContext)

**Source**: `internal/testutil/container_context.go`

```go
const DefaultContainerTestTimeout = 5 * time.Minute

func ContainerTestContext(t testing.TB, timeout time.Duration) context.Context {
    t.Helper()
    ctx, cancel := context.WithTimeout(t.Context(), timeout)
    t.Cleanup(cancel)
    return ctx
}
```

**How it works:**

1. Creates `context.WithTimeout(t.Context(), 5*time.Minute)`.
2. The context propagates through `exec.CommandContext(ctx, ...)` to the
   container engine subprocess.
3. When the deadline expires, Go's `exec` package sends SIGKILL to the
   subprocess.
4. `t.Cleanup(cancel)` ensures the context is cancelled even if the test
   returns early.

**Why bare `t.Context()` is forbidden for container tests:**

`t.Context()` has no deadline. If the container daemon becomes unresponsive
(e.g., Podman stuck in a cgroup operation), the subprocess blocks indefinitely:
- `cmd.Run()` blocks waiting for the process to exit.
- `cmd.Wait()` blocks waiting for I/O pipes to close.
- The test binary eventually gets killed by the binary-level `-timeout 15m`,
  but that produces a confusing panic with no useful error message.

With `ContainerTestContext`, the deadline fires after 5 minutes, SIGKILL is
sent, and the test fails with a clear `context deadline exceeded` error.

**Usage pattern (Go integration tests):**

```go
func TestContainerExec(t *testing.T) {
    t.Parallel()
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    sem := testutil.ContainerSemaphore()
    sem <- struct{}{}
    defer func() { <-sem }()

    ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
    execCtx := NewExecutionContext(ctx, cmd, inv)
    result := rt.Execute(execCtx)
    // ... assertions ...
}
```

**When NOT to use:**
- Tests that only call `Validate()` or perform type assertions (no daemon
  interaction).
- Tests with mocked container engines.

### Layer 2: Cleanup via env.Defer (Testscript Container Tests)

**Source**: `tests/cli/cmd_container_test.go`

```go
func containerSetup(env *testscript.Env) error {
    // ... common setup ...

    testSuffix := generateTestSuffix(env.WorkDir)
    containerPrefix := containerNamePrefix + testSuffix

    env.Defer(func() {
        cleanupTestContainersForHarness(containerPrefix, currentContainerHarness())
    })
    return nil
}
```

**How it works:**

1. Each container test gets a unique name prefix: `invowk-test-<hash>`.
2. `env.Defer()` registers a cleanup function that runs regardless of test
   outcome: pass, fail, timeout, or panic.
3. Cleanup lists running containers matching the prefix and force-removes them.

**Why this matters:**

When a test is terminated by the testscript deadline (Layer 3) or the binary
timeout (Layer 3b), deferred Go `defer` statements do NOT run. But
`env.Defer()` is called by the testscript framework as part of its cleanup
phase, which runs before the binary is killed.

For Go tests (not testscript), use `t.Cleanup()` instead:

```go
t.Cleanup(func() {
    // Remove orphaned containers
})
```

### Layer 3: CI Runner Binary Timeout

```yaml
-- -race -timeout 15m -coverprofile=coverage.out
```

The `-timeout 15m` flag on the `go test` command (passed through `gotestsum`)
kills the entire test binary after 15 minutes. This is the safety net for:
- Catastrophic daemon failures where even `ContainerTestContext` cannot kill the
  subprocess (rare, but possible with kernel-level blocks).
- Accumulated latency from many slow container tests.

**Relationship to Layer 1:**

The binary timeout must always exceed the sum of all sequential per-test
deadlines. With `DefaultContainerTestTimeout = 5 * time.Minute` and tests
running serially within `TestContainerCLI`, 3 tests would require 15 minutes.
The current configuration has headroom for this.

### Layer 4: Container Semaphore (ContainerSemaphore)

**Source**: `internal/testutil/container_semaphore.go`

```go
var ContainerSemaphore = sync.OnceValue(func() chan struct{} {
    n := containerParallelism()
    return make(chan struct{}, n)
})

func containerParallelism() int {
    if v := os.Getenv("INVOWK_TEST_CONTAINER_PARALLEL"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return n
        }
    }
    return min(runtime.GOMAXPROCS(0), 2)
}
```

**How it works:**

1. `sync.OnceValue` ensures the semaphore is created exactly once per process.
2. Default capacity: `min(GOMAXPROCS, 2)`. CI sets
   `INVOWK_TEST_CONTAINER_PARALLEL=2`.
3. Tests acquire a slot before container operations and release after:

```go
sem := testutil.ContainerSemaphore()
sem <- struct{}{}        // Acquire (blocks if at capacity)
defer func() { <-sem }() // Release
```

**Why cap at 2:**

Podman (especially rootless) has resource limits that manifest as hangs rather
than errors when too many concurrent operations run:
- cgroup controller contention (memory/cpu/pids).
- conmon process table pressure.
- Storage driver lock contention (overlay/fuse-overlayfs).

Capping at 2 prevents these issues while still allowing some parallelism.

**When to use the semaphore:**
- Integration tests that call `Execute()`, `ExecuteCapture()`, or `Build()`.
- CLI testscript tests that invoke container commands.

**When NOT to use:**
- Unit tests with mocked engines.
- Validation-only tests.
- Error-path tests that fail before reaching container operations.

### Layer 5: Job-Level Timeout

```yaml
jobs:
  test:
    timeout-minutes: 30
```

GitHub Actions kills the entire runner after 30 minutes. This is the absolute
last resort when all other layers fail. Without it, hung jobs burn up to
GitHub's 6-hour default.

---

## Engine Health Probes

**Source**: `tests/cli/cmd_container_test.go`

```go
const containerHealthProbeTimeout = 10 * time.Second

func probeEngineHealthBeforeTest() error {
    harness := currentContainerHarness()
    if harness.status != containerHarnessStatusReady || harness.binaryPath == "" {
        return nil
    }

    ctx, cancel := context.WithTimeout(context.Background(), containerHealthProbeTimeout)
    defer cancel()

    out, err := exec.CommandContext(ctx, harness.binaryPath, "version").CombinedOutput()
    if err != nil {
        return fmt.Errorf("container engine health re-check failed: %w\noutput: %s", err, out)
    }
    return nil
}
```

**Purpose:**

The container harness runs a one-time probe at init (`sync.OnceValue`), but the
engine can degrade mid-suite. Common degradation scenarios:
- Podman daemon stuck in cgroup operations after a previous test.
- conmon zombie processes consuming the process table.
- Docker daemon out of disk space after accumulated image layers.

The per-test health re-check (10-second `<engine> version`) catches these issues
early with a clear error message, instead of letting the test run for 3 minutes
before the testscript deadline fires.

**Why `context.Background()`:**

`probeEngineHealthBeforeTest()` is called inside `containerSetup()`, which is a
`testscript.Env.Setup` function. There is no `*testing.T` in scope (only
`*testscript.Env`), so there is no `t.Context()` available.

---

## Image Pre-Pull Strategy

```yaml
- name: Pre-pull container test images
  if: matrix.engine != '' && startsWith(matrix.runner, 'ubuntu')
  run: ${{ matrix.engine }} pull debian:stable-slim
```

**Rationale:**

Container tests that build images include `FROM debian:stable-slim`. Without
pre-pulling, the first test to run triggers a network-dependent image pull that:
- Adds variable latency (seconds to minutes depending on network).
- Can fail entirely on network issues, producing confusing errors.
- May time out on slow CI runners, consuming the per-test deadline.

Pre-pulling moves this dependency to a dedicated step that:
- Fails fast with a clear error if the registry is unreachable.
- Does not consume any test's deadline.
- Benefits all subsequent tests via the local cache.

---

## Engine Masking

Ubuntu CI runners have both Docker and Podman pre-installed. Without masking,
`AutoDetectEngine()` picks whichever engine it finds first, which may not be
the one specified by the CI matrix.

```yaml
- name: Mask Docker for Podman tests
  if: matrix.engine == 'podman' && startsWith(matrix.runner, 'ubuntu')
  run: sudo mv /usr/bin/docker /usr/bin/docker.disabled || true

- name: Mask Podman for Docker tests
  if: matrix.engine == 'docker' && startsWith(matrix.runner, 'ubuntu')
  run: sudo mv /usr/bin/podman /usr/bin/podman.disabled || true
```

**Why `|| true`:**

On rare occasions, one engine may not be installed (e.g., a new runner image).
The `|| true` prevents the mask step from failing the entire job.

**The `INVOWK_TEST_CONTAINER_ENGINE` env var:**

Set by CI to specify which engine to use explicitly, bypassing auto-detection:

```yaml
env:
  INVOWK_TEST_CONTAINER_ENGINE: ${{ matrix.engine }}
```

The container harness in `tests/cli/container_harness.go` reads this variable
and validates it. Invalid values cause a hard failure (not a skip).

---

## Container Cleanup Patterns

### Testscript Tests (env.Defer)

```go
env.Defer(func() {
    cleanupTestContainersForHarness(containerPrefix, currentContainerHarness())
})
```

The cleanup function:
1. Lists all containers matching the `invowk-test-<suffix>` prefix.
2. Force-removes running containers (`docker rm -f` / `podman rm -f`).
3. Logs any cleanup failures but does not fail the test.

### Go Integration Tests (t.Cleanup)

```go
t.Cleanup(func() {
    // Container-specific cleanup
})
```

`t.Cleanup` functions run in LIFO order after the test completes, including
on `t.Fatal`, but NOT after binary-level timeout kills.

---

## WaitDelay on Container Subprocesses

**Source**: `internal/container/engine_base.go`

```go
const cmdWaitDelay = 10 * time.Second

cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
cmd.WaitDelay = cmdWaitDelay
```

**The problem without WaitDelay:**

When a container process is killed (context cancellation sends SIGKILL):
1. The container engine process dies immediately.
2. BUT: child processes spawned by the engine (conmon, crun, runc) may keep
   stdout/stderr pipes open.
3. `cmd.Wait()` blocks waiting for pipe EOF, which never comes.
4. The test appears to hang even though the engine is dead.

**The fix:**

`cmd.WaitDelay = 10 * time.Second` tells Go's `exec` package: "After context
cancellation kills the process, wait up to 10 seconds for I/O pipes to close.
If they are still open after 10 seconds, forcefully close them."

This produces a clean `exec.ErrWaitDelay` error instead of an indefinite hang.

---

## Podman-Specific Issues

### Rootless Mode

Rootless Podman runs without real root privileges using user namespaces:
- Host UID 1000 maps to container UID 0 via `subuid`/`subgid`.
- Requires `newuidmap`/`newgidmap` (shadow-utils).
- Cgroup v2 required for resource limiting.
- Slower startup than Docker due to user namespace setup.

### ping_group_range Race

Concurrent rootless Podman invocations race on writing to
`/proc/sys/net/ipv4/ping_group_range`. The project addresses this at two
levels:

1. **`podman_sysctl_linux.go`**: Creates a temp file with
   `CONTAINERS_CONF_OVERRIDE` that sets `default_sysctls = []`, preventing
   Podman from attempting the sysctl write at all. Does not work for
   podman-remote (the override only affects the client, not the service).
2. **`run_lock_linux.go`**: flock-based serialization of container run calls.
   Blocks concurrent invocations so only one Podman run at a time.

### containers.conf

Podman reads configuration from `~/.config/containers/containers.conf`.
In CI, the testscript library sets `HOME=/no-home`, which breaks this.
The `containerSetup` function sets `HOME` to `env.WorkDir` to fix this.

---

## Docker-Specific Issues

### Daemon Mode

Docker requires a running daemon (`dockerd`). The daemon must be started before
tests run. CI runners have Docker pre-configured and running.

### Socket Permissions

Docker communicates via `/var/run/docker.sock`. The user running tests must be
in the `docker` group or use rootless Docker. CI runners handle this
automatically.

### BuildKit Backend

Docker uses BuildKit for image builds by default (since Docker 23). BuildKit
has different caching and error behavior than the legacy builder. Tests should
not assume legacy builder semantics.

---

## Container Stderr in Exit-Code Tests

Container commands may produce incidental stderr output that is not part of the
test contract:
- Shell prompt characters (`#`) from the container's `/bin/sh`.
- Podman informational messages about cgroup configuration.
- Docker BuildKit progress output.

**Rule:** Do NOT add `! stderr .` to container error-path txtar tests. This
assertion is too strict and will break when container engines add or change
their informational output. Instead, assert on specific error text with
`stderr 'expected error message'`.

---

## Suite Lock vs Semaphore Decision Guide

| Feature | Suite Lock (`AcquireContainerSuiteLock`) | Semaphore (`ContainerSemaphore`) |
|---------|------------------------------------------|-----------------------------------|
| Scope | Cross-binary (via flock) | In-process (buffered channel) |
| Location | `testutil/container_suite_lock_linux.go` | `testutil/container_semaphore.go` |
| Used by | `tests/cli` container tests | `internal/runtime` container tests |
| Concurrency | Serializes across test binaries | Limits within one binary (cap 2) |
| Platform | Linux only (`//go:build linux`) | All platforms (noop on non-Linux) |
| Overhead | Filesystem flock (kernel) | Channel send/receive (userspace) |

**Why `internal/runtime` does not use the suite lock:**

The in-process semaphore provides sufficient concurrency control for tests
within a single binary. Adding flock would make all tests fully sequential,
reducing throughput without benefit. The suite lock is only needed when two
separate test binaries (`internal/runtime` and `tests/cli`) might run container
operations concurrently during `make test`.

---

## Environment Variables Reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `INVOWK_TEST_CONTAINER_PARALLEL` | `min(GOMAXPROCS, 2)` | Max concurrent container ops |
| `INVOWK_TEST_CONTAINER_ENGINE` | (auto-detect) | Force specific engine (docker/podman) |
| `XDG_RUNTIME_DIR` | (system default) | Lock file directory for flock |
| `HOME` | (overridden to `env.WorkDir`) | Container config directory |
| `CONTAINERS_CONF_OVERRIDE` | (set by sysctl override) | Podman config override path |
