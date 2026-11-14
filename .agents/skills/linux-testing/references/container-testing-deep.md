# Container Test Infrastructure Deep Dive

Comprehensive reference for the container test infrastructure that runs
exclusively on Linux. This is the most complex testing subsystem in the project,
with layered timeouts and serialization, engine health probes, and
container-specific cleanup patterns.

---

## Timeout and Serialization Layers

Container tests are the most failure-prone tests in the project because they
depend on external daemon processes (Docker/Podman) that can hang indefinitely.
The layers below catch every hang at the narrowest possible scope. They match
the layer table in `linux-testing` SKILL.md "Container Test Invariants" (the
canonical enumeration):

| # | Layer | Owner | Value today |
|---|-------|-------|-------------|
| 1 | Operation context | `testutil.ContainerTestContext` | `DefaultContainerTestTimeout` = 5m |
| 2 | Testscript deadline and `env.Defer` cleanup | `tests/cli` harness | `containerTestTimeout` = 3m per txtar case |
| 3 | In-process semaphore | Go package tests | capacity `min(GOMAXPROCS, 2)` |
| 4 | Cross-binary suite lock | `tests/cli` container suite | flock (Linux) |
| 5 | Go `-timeout` and job `timeout-minutes` | test binary and CI | 10m (CLI) / 15m (non-CLI, runtime); job 30m |

Read the current values from source and `.github/workflows/ci.yml` before
relying on them.

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
- The test binary is eventually killed by the binary-level `-timeout`
  (Layer 5), but that produces a confusing panic with no useful error
  message, and no cleanup runs.

With `ContainerTestContext`, the deadline fires after 5 minutes, SIGKILL is
sent, and the test fails with a clear `context deadline exceeded` error.

**Usage pattern (Go integration tests):**

```go
func TestContainerExec(t *testing.T) {
    t.Parallel()
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    testutil.AcquireContainerSemaphore(t) // Layer 3; released via t.Cleanup

    ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
    execCtx := NewExecutionContext(ctx, cmd, inv)
    result := rt.Execute(execCtx)
    // ... assertions ...
}
```

When it is not needed (mocked engines, `Validate()`-only or type-assertion
tests): `.agents/rules/testing.md` "Integration and Container Tests".

### Layer 2: Testscript Deadline and env.Defer Cleanup (CLI Container Tests)

**Source**: `tests/cli/cmd_container_test.go`

`TestContainerCLI` runs each `container_*.txtar` file as a serial subtest
under the suite lock (Layer 4), and passes each `testscript.Run` a
`Deadline` of `time.Now().Add(containerTestTimeout)` (3 minutes).
`containerSetup` (which first calls `commonSetup`) registers cleanup:

```go
func containerSetup(env *testscript.Env) error {
    // ... commonSetup, engine pinning ...

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
2. `env.Defer()` registers a cleanup function that testscript runs when the
   case ends: pass, fail, or the testscript `Deadline` firing.
3. Cleanup lists running containers matching the prefix and force-removes them.

**Limits:** `env.Defer` (like `t.Cleanup` and Go `defer`) does NOT run when
the whole test binary is killed by `go test -timeout` (Layer 5). That is why
the testscript deadline must fire well before the binary timeout.

For Go tests (not testscript), use `t.Cleanup()` instead:

```go
t.Cleanup(func() {
    // Remove orphaned containers
})
```

### Layer 3: Container Semaphore (In-Process)

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
3. Tests acquire a slot once per test with
   `testutil.AcquireContainerSemaphore(t)`, after `t.Parallel()` and the
   Short skip and before any container operation; it releases through
   `t.Cleanup`. Nothing outside `internal/testutil` uses the raw channel.

**Why cap at 2:**

Podman (especially rootless) has resource limits that manifest as hangs rather
than errors when too many concurrent operations run:
- cgroup controller contention (memory/cpu/pids).
- conmon process table pressure.
- Storage driver lock contention (overlay/fuse-overlayfs).

Capping at 2 prevents these issues while still allowing some parallelism.

When to acquire it and when not to (mocked engines, validation-only tests,
error paths that fail before container operations):
`.agents/rules/testing.md` "Integration and Container Tests".

### Layer 4: Cross-Binary Suite Lock

**Source**: `internal/testutil/container_suite_lock_linux.go`

`AcquireContainerSuiteLock(t)` takes a flock so the `tests/cli` container
suite never runs real-container work concurrently with another test binary
during `make test`. Only `tests/cli` uses it. Comparison with the semaphore:
"Suite Lock vs Semaphore Decision Guide" below; lock semantics:
`filesystem-inotify.md` "File Locking (flock)".

### Layer 5: Go Binary Timeout and Job Timeout

```yaml
# .github/workflows/ci.yml (full mode)
-- -race -timeout 15m -coverprofile=coverage.out      # non-CLI and internal/runtime steps
-- -race -timeout 10m -coverprofile=cli-coverage.out  # CLI integration step

jobs:
  test:
    timeout-minutes: 30
```

The `-timeout` flag on `go test` (passed through `gotestsum`) kills the whole
test binary. It is the safety net for:
- Catastrophic daemon failures where even `ContainerTestContext` cannot kill
  the subprocess (rare, but possible with kernel-level blocks).
- Accumulated latency from many slow container tests.

It produces a `panic: test timed out` with goroutine dumps rather than a
useful assertion, and deferred cleanup does not run, so never lean on it.

**Invariant:** the binary `-timeout` must exceed the chain of serial
per-test deadlines with headroom. Today the CLI suite runs under
`-timeout 10m` against a 3m `containerTestTimeout` per serial txtar case, and
the non-CLI and `internal/runtime` steps run under `-timeout 15m` against the
5m `DefaultContainerTestTimeout`. When you add serial container cases or raise
a per-test deadline, re-check the step timeout.

`timeout-minutes: 30` on the job is the absolute last resort when every
other layer fails. Without it, hung jobs burn up to GitHub's 6-hour default.

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

The "Mask Docker for Podman tests" and "Mask Podman for Docker tests" steps in
`.github/workflows/ci.yml` resolve every `PATH` install of the other engine
with `command -v` (looping, because runner images move binaries between
`/usr/bin` and `/usr/local/bin`), rename each to `<path>.disabled`, and then
fail the job loudly if the engine is still on `PATH`. Read the steps there
rather than a copy. Keep the masking fail-loud; never reintroduce a
`|| true` that lets a job silently test the wrong engine (history:
`.agents/memorials/skills-platform-testing-history.md`).

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

Podman reads configuration from `~/.config/containers/containers.conf`, and
testscript's default `HOME=/no-home` breaks it. `commonSetup` (which
`containerSetup` calls first) sets `HOME` to `env.WorkDir`; details, including
the Windows `APPDATA`/`USERPROFILE` overrides: `.agents/rules/testing.md`
"Integration and Container Tests".

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

| Feature | Suite Lock (`AcquireContainerSuiteLock`) | Semaphore (`AcquireContainerSemaphore`) |
|---------|------------------------------------------|-----------------------------------|
| Scope | Cross-binary on Linux (via flock) | In-process (buffered channel) |
| Location | `testutil/container_suite_lock_linux.go` (+ `_other.go`) | `testutil/container_semaphore.go` |
| Used by | `tests/cli` container tests | Go package container tests (`internal/runtime`, `internal/container`, `internal/provision`) |
| Concurrency | Serializes across test binaries | Limits within one binary (cap 2 by default) |
| Platform | flock on Linux; in-process `sync.Mutex` elsewhere (`container_suite_lock_other.go`) | Same channel on every platform (no build tag) |
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
