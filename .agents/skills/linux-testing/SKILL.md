---
name: linux-testing
description: >-
  Deep Linux-specific testing knowledge for Go. Covers process lifecycle
  (clone/fork+exec, process groups via setpgid, PDEATHSIG), full POSIX signal
  handling (SIGKILL/SIGTERM/SIGINT), exec.CommandContext signal delivery and
  cmd.Cancel customization, ext4/XFS case-sensitivity, inotify watch limits
  (ENOSPC/EMFILE/ENFILE), file descriptor limits, cgroups and namespace
  isolation for container tests, OOM killer risks with -race (10x memory),
  and the full container test infrastructure (ContainerTestContext,
  ContainerSemaphore, flock-based cross-binary serialization).
  Use when debugging Linux-only failures, container test hangs, inotify errors,
  or understanding the 5-layer container test timeout strategy.
disable-model-invocation: false
---

# Linux Testing Skill

Linux is the **full-test platform** for this project. All container integration
tests run exclusively on Linux. This skill provides deep Linux OS primitive
knowledge needed to understand container test infrastructure, debug Linux-only
failures, and write correct platform-specific test code.

## Normative Precedence

1. `.agents/rules/testing.md` -- authoritative test policy (organization, parallelism, container timeout strategy).
2. `.agents/rules/go-patterns.md` -- context propagation, code style.
3. `.agents/skills/go-testing/SKILL.md` -- Go testing toolchain knowledge, decision frameworks.
4. This skill -- Linux OS primitives and the container test infrastructure.
5. `references/container-testing-deep.md` -- comprehensive container test infrastructure deep dive.
6. `references/process-namespaces.md` -- cgroups and namespace isolation for container understanding.
7. `references/filesystem-inotify.md` -- inotify API, watch limits, error handling.
8. `.agents/skills/testing/SKILL.md` -- invowk-specific test patterns, testscript, TUI/container.
9. `.agents/skills/testing/SKILL.md` § "Pre-Write Checklist" -- mandatory guardrails before writing any test code.

If this skill conflicts with a rule, follow the rule.

**Cross-references:**
- `go-testing` -- primary testing entry point; routes Linux symptoms here.
- `windows-testing` -- Windows OS primitives (CreateProcess, NTFS, timer resolution).
- `macos-testing` -- macOS OS primitives (APFS, kqueue coalescing, timer coalescing).
- `container` -- container engine abstraction, Docker/Podman patterns, Linux-only policy.
- `testing` -- invowk-specific test patterns including the container timeout layers.

---

## Process Lifecycle

Linux process creation uses the `clone` syscall, which is a superset of the
traditional `fork`. Go's runtime uses `clone` internally for `os/exec`, not raw
`fork`, because Go needs fine-grained control over which resources the child
inherits (memory, file descriptors, signal handlers, namespaces).

**Process groups and sessions:**

- **Process group** (`setpgid`): A leader process and its children share a
  process group ID (PGID). Signals sent to `-pgid` reach the entire group.
  Go sets `cmd.SysProcAttr.Setpgid = true` to put a child in a new group,
  enabling `syscall.Kill(-pid, sig)` to kill the child and all its descendants.
- **Session leader** (`setsid`): Creates a new session with a new process group.
  The session leader has no controlling terminal. Used by daemons; generally not
  needed for tests.
- **`SIGCHLD` and zombie prevention**: When a child exits, the kernel sends
  `SIGCHLD` to the parent. Go's runtime automatically reaps child processes
  (calls `waitpid`), so zombie processes are not a concern in normal Go code.
- **`prctl(PR_SET_PDEATHSIG, SIGKILL)`**: Ensures a child process receives a
  signal when its parent dies. Go does NOT set this by default, but it can be
  configured via `cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL`. Without this,
  if the parent (e.g., the test binary) is killed by the OOM killer or a
  timeout, children become orphans reparented to PID 1 (init/systemd).
- **Orphan processes**: If a parent dies without setting `PDEATHSIG`, the child
  is reparented to PID 1. In CI, orphaned container engine subprocesses can
  accumulate and exhaust resources. The container cleanup patterns (see
  `references/container-testing-deep.md`) are the defense against this.

**Implications for test subprocess cleanup:**

Test processes killed by `-timeout` receive `SIGKILL` (unblockable). Deferred
cleanup functions do not run. The multi-layer timeout strategy addresses this
by using `env.Defer` in testscript and `t.Cleanup` in Go tests, which the test
framework runs before the binary-level timeout fires.

---

## Signal Handling

Linux implements the full POSIX signal set. Key signals and their Go behavior:

| Signal | Number | Blockable | Go Default Behavior |
|--------|--------|-----------|---------------------|
| `SIGKILL` | 9 | No | Immediate kill, cannot be caught or ignored |
| `SIGTERM` | 15 | Yes | Catchable; used for graceful shutdown |
| `SIGINT` | 2 | Yes | Go converts to `os.Interrupt`; Ctrl+C |
| `SIGPIPE` | 13 | Yes | Go ignores by default (broken pipe) |
| `SIGSTOP` | 19 | No | Unblockable pause; `SIGCONT` resumes |
| `SIGCONT` | 18 | Yes | Resume after `SIGSTOP` |
| `SIGQUIT` | 3 | Yes | Go prints goroutine stack dump and exits |
| `SIGUSR1` | 10 | Yes | User-defined; no default Go behavior |
| `SIGUSR2` | 12 | Yes | User-defined; no default Go behavior |

**`exec.CommandContext` on Linux:**

- Default behavior: sends `SIGKILL` to the child process when the context is
  cancelled. This is immediate and unblockable -- no cleanup handlers run in
  the child.
- **`cmd.Cancel` customization**: Override the default kill behavior to send
  `SIGTERM` first (graceful), then rely on `cmd.WaitDelay` to send `SIGKILL`
  if the process does not exit in time:

```go
cmd := exec.CommandContext(ctx, "some-binary")
cmd.Cancel = func() error {
    return cmd.Process.Signal(syscall.SIGTERM)
}
cmd.WaitDelay = 10 * time.Second // SIGKILL after 10s if SIGTERM ignored
```

- **Process group kill**: To kill a child and all its descendants, use
  `syscall.Kill(-pid, sig)` (negative PID sends to the entire group). Set
  `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` to put the child
  in its own process group first.

**Why this matters for tests:**

Container engine subprocesses (Docker/Podman) may spawn additional child
processes (conmon, crun, runc). Killing only the top-level process may leave
these orphaned. The `cmd.WaitDelay = 10 * time.Second` on container
subprocesses in `internal/container/engine_base.go` ensures pipe cleanup
even when child processes linger after the parent is killed.

---

## File System

Linux filesystems have distinct characteristics that affect test behavior:

- **Case-sensitive**: ext4, XFS, btrfs, and tmpfs are all case-sensitive.
  `Foo.cue` and `foo.cue` are different files. This is the opposite of Windows
  (NTFS, case-insensitive) and macOS (APFS, case-preserving but insensitive).
  Tests that pass on Linux may fail on macOS/Windows due to case mismatches.
- **Inode-based**: Hard links share inodes. `os.SameFile()` compares inodes,
  not paths. Rename operations (`os.Rename`) are atomic on the same filesystem.
- **Symlinks**: `os.Symlink` works without privileges (unlike Windows, which
  requires `SeCreateSymbolicLinkPrivilege`). Tests creating symlinks need no
  special handling on Linux.
- **`/proc` pseudo-filesystem**: Per-process info at `/proc/self/status` (memory
  usage, threads), system limits at `/proc/sys/fs/` (inotify watches, file-max).
  The `go-testing` skill's race detector memory analysis references `/proc`.
- **`/dev/shm`**: Shared memory backed by tmpfs (RAM-based, fast). Available on
  all modern Linux distros.
- **`/tmp`**: Usually tmpfs on modern distros (Fedora, Ubuntu 22.04+), meaning
  it is RAM-backed, fast, and auto-cleared on reboot. `t.TempDir()` creates
  directories under `/tmp` by default.

---

## File Locking

Linux provides two main file locking mechanisms. Both are **advisory** -- they
do not prevent other processes from reading or writing; both sides must
cooperate by acquiring the lock before accessing the shared resource.

- **`flock`**: Advisory lock on an open file description (NOT file descriptor).
  Per-open-file-description means that `flock` locks are NOT inherited across
  `fork`/`clone` -- the child gets a new file description. This differs from
  macOS where `flock` IS inherited across `fork`.
- **`fcntl` locks (POSIX)**: Per-process, inherited across `fork`. More
  granular (byte-range locks), but more complex semantics. Not used by this
  project.
- **`O_EXCL` with `O_CREAT`**: Atomic file creation. The `open` call fails if
  the file already exists. Useful for lock-file-as-existence patterns.

**Project usage of `flock`:**

1. **`internal/runtime/run_lock_linux.go`**: Podman serialization lock on
   `$XDG_RUNTIME_DIR/invowk-podman.lock`. Prevents the rootless Podman
   `ping_group_range` race between concurrent invowk processes. Falls back to
   `os.TempDir()` when `XDG_RUNTIME_DIR` is unset.
2. **`internal/testutil/container_suite_lock_linux.go`**: Cross-binary test
   serialization via `AcquireContainerSuiteLock()`. Uses flock on
   `$XDG_RUNTIME_DIR/invowk-container-suite.lock` (or `/tmp/` fallback).
   Only used by `tests/cli` container tests -- `internal/runtime` tests use
   the in-process semaphore instead.
3. **`run_lock_linux_test.go`**: Tests flock contention using `atomic.Bool` and
   `time.Sleep(50ms)` to verify that goroutine B blocks while goroutine A
   holds the lock, plus a serialized-counter test with 5 goroutines.

**Key flock behaviors on Linux:**
- Blocking: `unix.Flock(fd, unix.LOCK_EX)` blocks until the lock is available.
- Auto-release: The kernel releases the lock when the fd is closed, including
  on process crash.
- `Release()` is idempotent: calling `LOCK_UN` + `Close()` multiple times is
  safe (nil-receiver guard pattern used in both production and test code).

---

## inotify

inotify is the Linux-specific file watching API. It is fundamentally different
from macOS's kqueue and Windows's `ReadDirectoryChangesW`. Full deep dive in
`references/filesystem-inotify.md`.

**Key facts for test code:**

- **Per-user watch limits**: `/proc/sys/fs/inotify/max_user_watches` controls
  how many inotify watches a single user can hold. Default varies by distro
  (typically 8192-524288). Exhaustion causes `ENOSPC`.
- **`ENOSPC` when watches exhausted**: The most common inotify-related test
  failure. CI runners sharing many watchers across concurrent jobs can exhaust
  the limit. Temporary fix: `sysctl -w fs.inotify.max_user_watches=524288`.
- **Recursive watch requires manual directory walking**: Unlike macOS's FSEvents
  or Windows's `ReadDirectoryChangesW`, inotify watches individual directories,
  not trees. Go's `fsnotify` library handles recursive walking internally.
- **Event coalescing**: inotify coalesces consecutive identical events on the
  same watch descriptor. This is LESS aggressive than kqueue -- individual write
  events are typically preserved. Tests relying on exact event counts should
  still use polling/debouncing rather than exact assertions.
- **`inotify_init1` flags**: `IN_NONBLOCK` (non-blocking reads) and
  `IN_CLOEXEC` (close-on-exec, prevents fd leak to child processes).

**The `watcher_fatal_unix_test.go` test** in `internal/watch/` validates error
handling for `ENOSPC` (inotify exhausted), `EMFILE` (process fd limit), and
`ENFILE` (system fd limit). Build-tagged `//go:build !windows`.

---

## File Descriptor Limits

- **Soft limit**: Historically 1024 on Linux. Go's runtime raises the soft
  limit to the hard limit at startup (since Go 1.19). Most modern distros set
  the hard limit to 1048576.
- **System-wide limit**: `/proc/sys/fs/file-max`. Exhaustion causes `ENFILE`.
- **Per-user limit**: `/etc/security/limits.conf` (PAM-based).
- **`EMFILE`**: Per-process file descriptor limit reached. Different from
  inotify limits -- `EMFILE` means the process has too many open files overall,
  not specifically inotify watches.
- **`ENFILE`**: System-wide file table full. Rare in normal CI but possible
  under extreme load (many parallel test binaries with container operations).

CI impact: Each container operation opens multiple file descriptors (socket to
daemon, pipe to subprocess, lock files). The container semaphore
(`INVOWK_TEST_CONTAINER_PARALLEL=2`) prevents fd exhaustion by limiting
concurrent container operations.

---

## Container Test Infrastructure

**This is the most critical section** -- Linux is where all container
integration tests run. The container test infrastructure uses a 5-layer timeout
strategy to prevent indefinite hangs. Full deep dive in
`references/container-testing-deep.md`.

### The 5-Layer Timeout Strategy

| Layer | Mechanism | Scope | Timeout |
|-------|-----------|-------|---------|
| 1 | `testutil.ContainerTestContext(t, timeout)` | Per-test | 5 min |
| 2 | `env.Defer` cleanup | Per-testscript | — (cleanup) |
| 3 | Go `-timeout` flag | Per-binary | 15 min |
| 4 | `testutil.ContainerSemaphore()` | Per-process | parallel=2 |
| 5 | GitHub `timeout-minutes` | Per-job | 30 min |

**Critical rule**: EVERY container test calling `Execute()` or `ExecuteCapture()`
MUST use `ContainerTestContext`, not bare `t.Context()`. Bare `t.Context()` has
NO deadline — if the daemon hangs, the subprocess blocks indefinitely.

```go
ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
result, err := engine.Execute(ctx, cmd)
```

### Supporting Infrastructure

- **Engine health probes**: `probeEngineHealthBeforeTest()` runs a 10s `<engine> version` check before each test. Fails fast instead of waiting for 3-min deadline.
- **Image pre-pull**: CI pulls `debian:stable-slim` before tests — removes network dependency from test timing.
- **Engine masking**: `sudo mv /usr/bin/docker /usr/bin/docker.disabled` ensures `AutoDetectEngine()` picks the right engine per CI matrix entry.
- **Suite lock vs semaphore**: `AcquireContainerSuiteLock()` (flock, cross-binary) for `tests/cli`; `ContainerSemaphore()` (in-process channel, cap 2) for `internal/runtime`. Do NOT add suite locks to `internal/runtime` tests.
- **`cmd.WaitDelay = 10s`**: On container subprocesses (`engine_base.go`). Prevents indefinite hang when killed processes leave pipes open.

Full deep dive with code examples: `references/container-testing-deep.md`.

---

## Cgroups and Namespaces

Container tests run inside Linux namespaces and cgroups. Understanding these
is essential for diagnosing container test failures. Full deep dive in
`references/process-namespaces.md`.

**Cgroup v2 unified hierarchy** (default on Ubuntu 22.04+, Fedora 31+):
- Single tree hierarchy (unlike v1's multiple trees per controller).
- Resource controllers: `cpu` (CFS bandwidth, weight), `memory` (limit, swap,
  OOM control), `io` (bandwidth, weight), `pids` (fork bomb protection).
- Docker and Podman create per-container cgroups for resource isolation.

**Linux namespaces** (7 types):
- **User**: UID/GID mapping. Rootless Podman maps host UID 1000 to container
  UID 0.
- **PID**: Container sees its own PID space (PID 1 in container is not PID 1
  on host).
- **Mount**: Isolated filesystem view. Overlay filesystem for container layers.
- **Network**: Virtual interfaces, bridge (Docker) or slirp4netns (rootless
  Podman).
- **UTS**: Isolated hostname.
- **IPC**: Isolated System V IPC and POSIX message queues.
- **Cgroup**: Isolates cgroup view (container sees itself as root of cgroup
  tree).

**How namespaces affect tests:**
- PID 1 signal handling: In containers, PID 1 does not receive signals that
  have their default action as "terminate" unless the process installs signal
  handlers. This is why `docker stop` sends SIGTERM, waits a grace period,
  then SIGKILL.
- Network isolation: Container processes cannot reach host services unless
  explicit port mapping (`-p host:container`) or host networking is configured.
- Filesystem isolation: Only explicitly mounted paths are visible inside the
  container. The invowk binary is auto-provisioned via ephemeral layer.

**Rootless Podman specifics:**
- Uses user namespaces without real root: `subuid`/`subgid` mapping in
  `/etc/subuid` and `/etc/subgid`.
- The `ping_group_range` sysctl race: concurrent rootless Podman invocations
  can race on writing to `/proc/sys/net/ipv4/ping_group_range`. The project
  addresses this with (1) `CONTAINERS_CONF_OVERRIDE` disabling `default_sysctls`
  via `podman_sysctl_linux.go`, and (2) flock serialization via
  `run_lock_linux.go`.

---

## OOM Killer

The Linux OOM (Out Of Memory) killer can terminate test processes under memory
pressure. This is particularly relevant for container tests with the race
detector.

**Why tests are vulnerable:**
- **Race detector**: The `-race` flag increases memory usage by approximately
  10x. Each goroutine's memory overhead grows from ~8KB to ~80KB+.
- **CI runners**: `ubuntu-latest` GitHub Actions runners have ~7 GB RAM.
  Multiple parallel container tests with `-race` can exceed this.
- **Container cgroup limits**: OOM can come from the host cgroup limit, not
  just the container's own limit.

**Symptoms:**
- Test process killed with no output, exit code 137 (128 + SIGKILL = 9).
- `dmesg | grep -i oom` or `journalctl -k | grep -i oom` shows the kill.
- All tests in the binary show `(unknown)` status (binary killed mid-run).

**Mitigation:**
- Limit parallel container tests (the semaphore: `INVOWK_TEST_CONTAINER_PARALLEL=2`).
- Avoid `-race` on memory-intensive benchmarks (`make pgo-profile` uses
  `-pgo=off`, not `-race`).
- If a test binary is OOM-killed in CI, check the CI logs for `exit code 137`
  and correlate with the memory pressure of concurrent jobs.

---

## Seccomp

Docker and Podman apply seccomp (Secure Computing Mode) profiles that restrict
which syscalls container processes can make.

- **Docker default**: Blocks approximately 44 syscalls including `clone` with
  `CLONE_NEWUSER`, `mount`, `ptrace`, `reboot`, `keyctl`, and others.
- **Podman default**: Similar profile; slightly different based on version.
- **Impact on tests**: Generally transparent for invowk tests since the invowk
  binary runs as a normal process inside the container. The blocked syscalls
  are primarily kernel-level operations that invowk does not use.
- **Custom profiles**: Can be specified via `--security-opt seccomp=profile.json`
  if a test ever needs a blocked syscall (unlikely for this project).
- **Diagnosis**: If a test fails with `EPERM` or `operation not permitted`
  inside a container for a syscall that works on the host, seccomp is the likely
  cause. Check `docker inspect` for the applied security options.

---

## CI Configuration

### Runners and Matrix

Linux CI runs on two runner variants with two container engines each:

| Runner | Engine | Mode | Purpose |
|--------|--------|------|---------|
| `ubuntu-24.04` | Docker | full | Stable baseline |
| `ubuntu-24.04` | Podman | full | Rootless container testing |
| `ubuntu-latest` | Docker | full | Rolling forward compatibility |
| `ubuntu-latest` | Podman | full | Rolling + rootless |

Total: 4 Linux CI jobs, each with `timeout-minutes: 30`.

### Three Test Steps

Full-mode Linux jobs split tests into three separate `gotestsum` invocations:

1. **All packages except `tests/cli` and `internal/runtime`**: The bulk of
   unit and integration tests. Excludes the two packages that need special
   container handling.
2. **`internal/runtime` isolated**: Container runtime tests with the in-process
   semaphore. Isolated to prevent interference with CLI tests.
3. **CLI integration tests** (`tests/cli/...`): Testscript-based container tests
   with `AcquireContainerSuiteLock`. Uses `INVOWK_TEST_CONTAINER_ENGINE` to
   pin the engine.

### Test Runner Configuration

All steps use gotestsum with `--rerun-fails --rerun-fails-max-failures 5 -race -timeout 15m`. Rerun reports produce `::warning::` annotations; JUnit XML with `flaky_summary: true` enables PR flaky test summaries. See `go-testing` skill for full gotestsum flag reference.

---

## Invowk-Specific Linux Patterns

### Build Tags

- `//go:build linux` -- strict Linux-only code. Used for:
  - `run_lock_linux.go` (flock-based Podman serialization)
  - `container_suite_lock_linux.go` (flock-based test serialization)
  - `podman_sysctl_linux.go` (sysctl override via temp file)
- `//go:build !windows` -- Unix code (Linux + macOS). Used for:
  - `watcher_fatal_unix_test.go` (inotify error handling test)
  - Tests that use POSIX signals or Unix-specific filesystem behavior

### Key Linux-Specific Source Files

| File | Purpose |
|------|---------|
| `internal/runtime/run_lock_linux.go` | flock on `$XDG_RUNTIME_DIR/invowk-podman.lock` |
| `internal/runtime/run_lock_linux_test.go` | flock contention tests with `atomic.Bool` |
| `internal/testutil/container_suite_lock_linux.go` | Cross-binary test serialization |
| `internal/testutil/container_context.go` | `ContainerTestContext` with 5-min deadline |
| `internal/testutil/container_semaphore.go` | `ContainerSemaphore` (buffered channel, cap 2) |
| `internal/container/podman_sysctl_linux.go` | Sysctl override temp file for Podman |
| `internal/container/podman_sysctl_linux_test.go` | Tests for sysctl override logic |
| `internal/watch/watcher_fatal_unix_test.go` | ENOSPC/EMFILE/ENFILE error handling |
| `internal/container/engine_base.go` | `cmdWaitDelay = 10 * time.Second` |

### Container Integration Txtar Tests

CLI container tests live in `tests/cli/testdata/container_*.txtar`. They are
gated by `[!container-available] skip` and run serially within the
`TestContainerCLI` suite under `AcquireContainerSuiteLock`.

Container stderr in exit-code tests: container commands may produce incidental
stderr (shell prompt `#`). Do NOT add `! stderr .` to container error-path
txtar tests.

---

## Common Linux Test Failure Matrix

| Symptom | Likely Cause | Diagnosis | Fix |
|---------|-------------|-----------|-----|
| Exit code 137, no output | OOM killer | `dmesg \| grep -i oom` | Reduce parallel tests, check `-race` memory |
| `ENOSPC` from fsnotify | inotify watch limit | `cat /proc/sys/fs/inotify/max_user_watches` | `sysctl -w fs.inotify.max_user_watches=524288` |
| `EMFILE` too many open files | Process fd limit | `ulimit -n` | Increase soft limit or reduce concurrency |
| Container test hangs 3+ min | Daemon unresponsive | Check `probeEngineHealthBeforeTest` output | Restart engine, check cgroup deadlocks |
| `flock` test flaky | Timing-sensitive sleep | Check `time.Sleep(50ms)` margin | Increase sleep or use channel-based sync |
| `EPERM` in container | Seccomp blocking syscall | `docker inspect --format='{{.HostConfig.SecurityOpt}}'` | Custom seccomp profile (rare) |
| Podman race on `ping_group_range` | Concurrent rootless Podman | Check sysctl override active | Verify `CONTAINERS_CONF_OVERRIDE` set |
| All tests `(unknown)` | Binary killed (timeout or OOM) | Check exit code and `dmesg` | Increase `-timeout` or reduce memory |
| Container cleanup failure | Orphaned containers | `docker ps -a \| grep invowk-test` | Manual cleanup, check `env.Defer` |
| `context deadline exceeded` | `ContainerTestContext` expired | Check if 5-min timeout is sufficient | Increase timeout for slow CI runners |
| Test passes locally, fails in CI | Missing image pre-pull | Check if `debian:stable-slim` cached | Verify pre-pull step runs before tests |
| Wrong engine selected | Engine masking failed | `which docker && which podman` | Verify mask step ran, check `INVOWK_TEST_CONTAINER_ENGINE` |

---

## Related Skills

| Skill | When to Consult |
|---|---|
| `go-testing` | Go test execution model, all flags, race detector, vet analyzers, benchmark/fuzz APIs |
| `windows-testing` | Windows process lifecycle, TerminateProcess, NTFS pitfalls, timer resolution |
| `macos-testing` | APFS case-insensitivity, kqueue coalescing, timer coalescing, /tmp symlink |
| `container` | Container engine abstraction, Docker/Podman patterns, Linux-only container runtime policy |
| `testing` | Invowk-specific test patterns, testscript, TUI testing, container runtime testing |
| `review-tests` | Test suite audit with 102-item checklist across 8 surfaces |
