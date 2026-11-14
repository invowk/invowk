# Linux Namespaces and Cgroups

Reference for the Linux namespace and cgroup primitives that underpin container
test isolation, plus process groups, signals and the OOM killer (sections at
the end). Understanding these is essential for diagnosing container test
failures, resource exhaustion, and permission errors.

---

## Linux Namespaces Overview

Namespaces provide process-level isolation of kernel resources. Each namespace
type isolates a specific global resource so that processes inside the namespace
see their own independent instance. Containers use all 7 namespace types
together to create the illusion of a separate system.

### User Namespace

**What it isolates:** User and group IDs (UID/GID mapping).

- Maps host UIDs/GIDs to container UIDs/GIDs. Rootless Podman uses this to map
  host UID 1000 to container UID 0 (root inside the container without real root
  on the host).
- Requires `subuid`/`subgid` configuration in `/etc/subuid` and `/etc/subgid`.
  Each user gets a range of subordinate UIDs (e.g., `user:100000:65536`).
- `newuidmap` and `newgidmap` (from shadow-utils) perform the mapping.

**Implications for file permissions in mounted volumes:**
- Files created inside the container as root (UID 0) are owned by the mapped
  host UID (e.g., 100000) outside the container.
- If tests mount host directories into the container, file ownership may appear
  wrong when inspected from the host.
- Invowk's ephemeral layer provisioning avoids this issue by copying files into
  the container image layer rather than bind-mounting.

### PID Namespace

**What it isolates:** Process ID space.

- The first process in a PID namespace becomes PID 1 (init) within that
  namespace. On the host, it has a different (higher) PID.
- `kill` inside the container only affects processes in the same PID namespace.
- PID 1 has special signal handling: it does NOT receive signals whose default
  action is "terminate" (SIGTERM, SIGINT) unless it explicitly installs signal
  handlers. This is why `docker stop` sends SIGTERM, waits a grace period, then
  SIGKILL.
- Zombie processes inside the container need PID 1 to reap them. Most container
  runtimes use a tiny init (tini, catatonit, or Docker's built-in
  `--init` flag) to handle this.

**Impact on tests:**
- Processes spawned inside the container cannot be killed from outside using
  their container-internal PID. The host must use the host-visible PID.
- Docker/Podman handle this transparently via `docker stop`/`podman stop`.

### Mount Namespace

**What it isolates:** Filesystem mount table.

- Each container sees its own filesystem tree, independent of the host.
- **Overlay filesystem**: Container images use a layered filesystem. The base
  image is the bottom layer; each subsequent layer adds, modifies, or deletes
  files. A writable top layer captures container changes.
- **Bind mounts**: `docker run -v /host/path:/container/path` makes a host
  directory visible inside the container. Changes are bidirectional.
- Invowk's auto-provisioning creates an ephemeral overlay layer containing the
  `invowk` binary and required invowkfiles/invowkmods, then attaches it to the
  user-specified image.

**Impact on tests:**
- Files created inside the container are only visible inside the container
  unless explicitly mounted or copied out.
- Testscript creates files under `$WORK`, which is NOT automatically visible
  inside the container. The provisioning layer handles this.

### Network Namespace

**What it isolates:** Network interfaces, routing tables, iptables rules, ports.

- Each container gets its own virtual network interface (veth pair).
- **Docker bridge networking**: Containers connect to a virtual bridge
  (`docker0`). NAT provides outbound connectivity. Port mapping (`-p
  host:container`) provides inbound access.
- **Rootless Podman** uses `slirp4netns` or `pasta` (newer) for userspace
  networking. Slower than Docker's kernel-level bridge but does not require root.
- **Host networking** (`--network=host`): Container shares the host's network
  namespace. No isolation, but eliminates port mapping overhead.

**Impact on tests:**
- Container processes cannot reach host services (e.g., a test-spawned server)
  without explicit port mapping or host networking.
- DNS resolution inside the container uses Docker/Podman-configured resolvers,
  not the host's `/etc/resolv.conf` directly.
- Network latency within the container is slightly higher than on the host due
  to the veth/bridge overhead.

### UTS Namespace

**What it isolates:** Hostname and NIS domain name.

- `hostname` inside the container returns the container ID or a custom name,
  not the host's hostname.
- Generally transparent for tests. Only relevant if test assertions check
  hostname output.

### IPC Namespace

**What it isolates:** System V IPC objects (shared memory, semaphores, message
queues) and POSIX message queues.

- Processes inside the container cannot access IPC objects created on the host
  or in other containers.
- Generally transparent for Go tests (Go rarely uses System V IPC directly).

### Cgroup Namespace

**What it isolates:** The cgroup filesystem view.

- The container sees itself as the root of its cgroup tree.
- `cat /proc/self/cgroup` inside the container shows paths relative to the
  container's cgroup, not the host's full path.
- Allows containers to read their own resource limits without exposing host
  cgroup structure.

---

## Cgroup v2 Unified Hierarchy

Cgroup v2 is the default on modern Linux distros (Ubuntu 22.04+, Fedora 31+,
RHEL 9+). It replaces cgroup v1's multiple independent trees with a single
unified hierarchy.

### Key Differences from v1

| Feature | Cgroup v1 | Cgroup v2 |
|---------|-----------|-----------|
| Hierarchy | Multiple trees (one per controller) | Single unified tree |
| Controller activation | Implicit (mount controller tree) | Explicit (`+cpu +memory` in `cgroup.subtree_control`) |
| Thread-level control | Per-thread cgroup membership | "threaded" mode (opt-in) |
| PSI (Pressure Stall Info) | Not available | Built-in (`cpu.pressure`, `memory.pressure`, `io.pressure`) |
| OOM handling | Kill one process | Kill entire cgroup (configurable) |

### Resource Controllers

**CPU controller:**
- `cpu.max`: Hard limit (CFS bandwidth). Format: `quota period` in
  microseconds. E.g., `100000 100000` = 100% of one CPU.
- `cpu.weight`: Proportional sharing (1-10000, default 100). Replaces v1's
  `cpu.shares`.
- CFS throttling: When a container exceeds its CPU quota, its processes are
  suspended until the next period. Symptoms: test slowdowns that appear random.

**Memory controller:**
- `memory.max`: Hard memory limit. Exceeding triggers OOM killer within the
  cgroup.
- `memory.high`: Soft limit. Exceeding triggers memory reclaim (swap, page
  cache eviction) but does not kill.
- `memory.swap.max`: Swap limit. Setting to 0 disables swap for the cgroup.
- `memory.oom.group`: When set to 1, OOM kills all processes in the cgroup
  (not just the largest).

**IO controller:**
- `io.max`: Bandwidth limit per device. Format: `MAJ:MIN rbps=X wbps=Y`.
- `io.weight`: Proportional sharing (1-10000, default 100).
- Disk-intensive container tests may be throttled by IO limits on CI runners.

**PIDs controller:**
- `pids.max`: Maximum number of processes in the cgroup.
- Fork bomb protection. Default for Docker containers: no limit (unless
  `--pids-limit` is set).
- CI runners may have system-wide limits that affect container process counts.

---

## Docker Namespace/Cgroup Creation

When `docker run` (or the equivalent API call) creates a container:

1. **dockerd** receives the run request and delegates to **containerd**.
2. **containerd** calls **runc** (or an alternative OCI runtime) to create the
   container.
3. **runc** uses `clone(2)` with namespace flags (`CLONE_NEWUSER`,
   `CLONE_NEWPID`, `CLONE_NEWNS`, `CLONE_NEWNET`, `CLONE_NEWUTS`,
   `CLONE_NEWIPC`, `CLONE_NEWCGROUP`) to create the namespaced process.
4. **runc** writes the container's cgroup configuration (resource limits) to
   the appropriate cgroup v2 control files.
5. **runc** sets up the overlay filesystem (mount namespace) and configures
   networking (network namespace).
6. **runc** executes the container's entrypoint as PID 1 within the new
   namespaces.

### Docker's Process Chain

```
dockerd
  └── containerd
       └── containerd-shim
            └── runc → [container PID 1]
```

The shim process remains running after runc exits. It:
- Holds the container's stdio pipes open.
- Waits for the container process to exit.
- Reports the exit code back to containerd.

---

## Rootless Podman

Rootless Podman runs entirely in user space without requiring root privileges
or a daemon process.

### Architecture

```
podman (CLI)
  └── conmon (container monitor)
       └── crun (OCI runtime) → [container PID 1]
```

**conmon** is Podman's equivalent of Docker's containerd-shim. It:
- Holds the container's stdio pipes.
- Logs container output.
- Reports the exit status.

### User Namespace Mapping

```
Host                    Container
UID 1000 (user)    →   UID 0 (root)
UID 100000-165535  →   UID 1-65535
```

This mapping is configured in `/etc/subuid`:
```
user:100000:65536
```

**Implications:**
- Container root (UID 0) has no real privileges on the host.
- Files created as container root are owned by host UID 100000.
- Some operations that require real root (like `mount` for certain filesystem
  types) fail in rootless mode.

### Limitations Relevant to Tests

- **No `--privileged`**: Rootless Podman cannot run truly privileged
  containers. Not relevant for invowk tests.
- **Limited network modes**: `slirp4netns`/`pasta` instead of kernel bridge.
  Slightly slower than Docker networking.
- **Storage driver**: Uses `fuse-overlayfs` or native overlay with certain
  kernel versions. `fuse-overlayfs` is slower than Docker's native overlay.
- **Startup time**: User namespace setup adds ~100-200ms to container start.
  This adds up across many container tests.

---

## How Namespaces Affect Tests

### PID 1 Signal Handling

The container's PID 1 process (typically `/bin/sh` or the invowk binary) has
special signal handling:
- SIGTERM: Ignored by default unless the process installs a handler.
- SIGINT: Ignored by default unless the process installs a handler.
- SIGKILL: Always works (not catchable).

When `docker stop` or `podman stop` is called:
1. SIGTERM is sent to PID 1.
2. After a grace period (default 10s), SIGKILL is sent.

This means graceful shutdown of container tests requires either:
- Using `docker kill` (sends SIGKILL directly).
- Using `--init` flag to install a real init process that handles signals.

### Network Isolation

Container processes cannot reach:
- Host-bound services (without port mapping or `--network=host`).
- Other container's services (without shared network or explicit links).

Tests that need to communicate between the host and container must use port
mapping (`-p host:container`) or volume-mounted Unix sockets.

### Filesystem Isolation

Only explicitly mounted or provisioned paths are visible:
- Invowk auto-provisions the binary and config files via ephemeral layer.
- Test fixtures in `$WORK` are NOT automatically visible unless the test
  framework provisions them.

### Resource Limits

OOM can come from:
1. **Container cgroup limit**: `--memory` flag on `docker run`.
2. **System cgroup limit**: CI runner's resource allocation.
3. **Host memory exhaustion**: All containers share host RAM.

The container semaphore (cap 2) helps prevent case 3 on CI runners.

---

## Seccomp Profiles

### Docker Default Profile

Docker's default seccomp profile blocks approximately 44 syscalls including:
- `clone` with `CLONE_NEWUSER` (prevents further namespace creation inside
  container).
- `mount` (prevents arbitrary mounts).
- `ptrace` (prevents debugging other processes).
- `reboot` (prevents container from rebooting the host).
- `keyctl` (prevents key management operations).
- Various kernel administration syscalls.

### Podman Default Profile

Similar to Docker's but may differ in edge cases depending on the Podman
version. Podman's seccomp support is provided by the OCI runtime (crun/runc).

### Custom Profiles

If a test needs a blocked syscall (unlikely for this project):

```bash
docker run --security-opt seccomp=custom-profile.json ...
```

### Diagnosing Seccomp Blocks

If a container test fails with `EPERM` or `operation not permitted` for an
operation that works on the host:
1. Check if the operation involves a syscall blocked by seccomp.
2. Use `docker inspect` to see the applied security options.
3. Test with `--security-opt seccomp=unconfined` to confirm seccomp is the
   cause (do not use this in production).

---

## Process Groups, Signals and Orphans

Go's `os/exec` starts children with `clone` (not a raw `fork`) so it controls
exactly which resources the child inherits.

**Process groups and sessions:**

- **Process group** (`setpgid`): signals sent to `-pgid` reach every process
  in the group. `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` puts
  the child in a new group, and `syscall.Kill(-cmd.Process.Pid, sig)` then
  signals the child and all descendants that stayed in that group. Invowk
  production code does not do this today (no `Setpgid`/`Setsid` in the
  tree); a change that must own a process tree adds it together with a
  cleanup test.
- **Session leader** (`setsid`): new session and process group, no
  controlling terminal. Used by daemons; generally not needed for tests.
- **Reaping**: a child becomes a zombie until its parent waits on it.
  `Cmd.Wait` (or `Cmd.Run`) reaps; the Go runtime does not reap arbitrary
  children. Always wait.
- **`Pdeathsig`**: `cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL` asks the
  kernel (`prctl(PR_SET_PDEATHSIG)`) to signal the child when its parent
  dies. Go does not set it by default. Per `prctl(2)` it fires when the
  parent *thread* that created the child exits, not only the whole process.
- **Orphans**: if the parent dies without `Pdeathsig`, the child is
  reparented to PID 1 (or the nearest subreaper) and keeps running. Orphaned
  engine subprocesses can accumulate in CI; the container cleanup patterns in
  `container-testing-deep.md` are the defense.

**Linux signal numbers** (always use the `syscall` constants; numbers differ
on macOS):

| Signal | Number | Blockable | Go behavior |
|--------|--------|-----------|-------------|
| `SIGKILL` | 9 | No | Immediate kill; cannot be caught or ignored |
| `SIGTERM` | 15 | Yes | Catchable; graceful shutdown |
| `SIGINT` | 2 | Yes | Delivered as `os.Interrupt`; Ctrl+C |
| `SIGPIPE` | 13 | Yes | Fatal on a broken pipe to fd 1/2; other fds get `EPIPE` (`os/signal` docs) |
| `SIGSTOP` | 19 | No | Unblockable pause; `SIGCONT` resumes |
| `SIGCONT` | 18 | Yes | Resume after `SIGSTOP` |
| `SIGQUIT` | 3 | Yes | Go prints goroutine stacks and exits |
| `SIGUSR1` | 10 | Yes | User-defined; no default Go handling |
| `SIGUSR2` | 12 | Yes | User-defined; no default Go handling |

**`exec.CommandContext` on Linux:**

- Default cancellation is `Process.Kill` (`SIGKILL`): immediate, no cleanup in
  the child.
- For graceful termination, set `cmd.Cancel` to send `SIGTERM` and a bounded
  `cmd.WaitDelay`, after which Go kills the process and closes the pipes:

  ```go
  cmd := exec.CommandContext(ctx, "some-binary")
  cmd.Cancel = func() error {
      return cmd.Process.Signal(syscall.SIGTERM)
  }
  cmd.WaitDelay = 10 * time.Second
  ```

- Container engine CLIs spawn helpers (conmon, crun, runc) that can keep
  stdout/stderr open after the engine process dies. `cmdWaitDelay` in
  `internal/container/engine_base.go` bounds that pipe wait; this, not a
  process group, is what keeps engine subprocess cleanup from hanging today.

**Test binary timeouts:** when `go test -timeout` fires, the test binary
panics and exits. Deferred functions, `t.Cleanup` and testscript `env.Defer`
do not run, and children are not killed. Keep operation and testscript
deadlines inside the binary timeout (`container-testing-deep.md` "Timeout and
Serialization Layers").

---

## OOM Killer

The Linux OOM killer can terminate test processes under memory pressure,
most often container tests running with `-race`.

**Why tests are vulnerable:**

- **Race detector**: the Go race detector documentation puts its cost at
  roughly 5-10x memory and 2-20x execution time, depending on the workload.
- **Runner memory**: hosted runners have a fixed memory budget shared by the
  test binaries, the container engine and every container. Read the current
  runner from the workflow; do not rely on a remembered capacity.
- **cgroup limits**: an OOM kill can come from the container's own cgroup
  (`--memory`), the runner's cgroup, or host exhaustion (see "Resource
  Limits" above).

**Symptoms and evidence:**

- The process dies with no further output; the shell or CI log reports exit
  code 137 (128 + `SIGKILL`), possibly after `signal: killed`.
- `dmesg | grep -i oom` or `journalctl -k | grep -i oom` shows the kill
  (when you have permission to read kernel logs).
- Every remaining test in that binary shows `(unknown)` in gotestsum/JUnit
  output, because the binary died mid-run.

**Mitigation:**

- Keep container concurrency bounded by the semaphore
  (`INVOWK_TEST_CONTAINER_PARALLEL=2` in CI). Never raise it above 2 for
  race-enabled runs without memory evidence.
- Do not add `-race` to memory-heavy benchmark or profiling runs
  (`make pgo-profile` runs benchmarks with `-pgo=off` and no `-race`).
- Correlate a 137 with what else ran concurrently in that job before
  changing timeouts or parallelism.
