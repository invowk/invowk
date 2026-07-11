---
name: linux-testing
description: >-
  Invowk Linux and Ubuntu CI testing guidance for process cleanup, POSIX
  signals, inotify failures, file-descriptor exhaustion, cgroups/namespaces,
  OOM kills, and the container test infrastructure. Use when debugging
  Linux-only failures, exit code 137, ping_group_range races, context deadline
  exceeded, `[!container-available]`, container test hangs, inotify
  ENOSPC/EMFILE/ENFILE errors, or Linux CI behavior.
---

# Linux Testing

Treat Linux as Invowk's full container-test platform. Start with current source
and CI evidence, then load only the reference for the failing primitive.

## Precedence and Routing

1. Follow `.agents/rules/testing.md` for mandatory test policy.
2. Use `.agents/skills/go-testing/SKILL.md` for Go test flags, contexts,
   parallelism, race reports, and gotestsum behavior.
3. Use `.agents/skills/testing/SKILL.md` for Invowk testscript and component
   patterns.
4. Use `.agents/skills/container/SKILL.md` for engine/runtime implementation.
5. Apply this skill for Linux-specific evidence and failure routing.

Load references conditionally:

- Read [references/container-testing-deep.md](references/container-testing-deep.md)
  for deadlines, engine health probes, semaphores, suite locks, and cleanup.
- Read [references/process-namespaces.md](references/process-namespaces.md) for
  process groups, signals, cgroups, namespaces, OOM, and seccomp.
- Read [references/filesystem-inotify.md](references/filesystem-inotify.md) for
  filesystem, flock, inotify, and descriptor-limit details.

## First Checks

1. Capture the failing test, package, runner, engine, exit code, timeout layer,
   and whether race instrumentation was enabled.
2. Read the current workflow and helpers instead of copying runner capacities,
   timeouts, or job counts from this skill:

   ```bash
   rg -n 'ubuntu|docker|podman|timeout|rerun-fails' .github/workflows/ci.yml
   rg -n 'ContainerTestContext|AcquireContainer(SuiteLock|Semaphore)|WaitDelay' internal tests
   ```

3. Reproduce the smallest affected package or txtar case with `-count=1`.
4. For a container failure, verify engine availability and health before
   increasing any timeout.
5. For resource failures, collect live evidence (`ulimit -n`, relevant `/proc`
   limits, cgroup memory state, and kernel logs when permitted).

## Process and Signal Contract

- A child started with `os/exec.Cmd` is reaped by `Cmd.Wait` or a helper such
  as `Cmd.Run`; the Go runtime does not reap arbitrary children. Always wait.
- Killing only the direct child may leave descendants. When production code
  owns a process tree, use the existing process-group pattern and test cleanup
  rather than adding sleeps.
- `exec.CommandContext` cancellation is immediate by default. When graceful
  termination matters, configure `cmd.Cancel` and a bounded `cmd.WaitDelay`.
- Binary-level `go test -timeout` termination cannot be relied on to run
  deferred cleanup. Keep operation and testscript deadlines inside that outer
  deadline.

Do not generalize Linux signal behavior to Windows. Consult `windows-testing`
when changing cross-platform process code.

## Filesystem and Watch Contract

- Linux filesystems used in CI are normally case-sensitive; tests that differ
  only by case can still fail on macOS or Windows.
- Do not assume `/tmp` is tmpfs or has a particular persistence policy. Use
  `t.TempDir()` and avoid performance assumptions about its backing store.
- inotify watches individual directories. `fsnotify` does not add recursive
  watching; Invowk must register every directory it needs and handle newly
  created subdirectories.
- Distinguish `ENOSPC` watch exhaustion from `EMFILE` per-process descriptor
  exhaustion and `ENFILE` system-wide exhaustion before proposing a fix.
- Use the existing flock helpers for cross-process container serialization;
  do not substitute an in-process mutex for cross-binary coordination.

## Container Test Invariants

Real engine calls in Go tests must use:

```go
ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
```

Keep the timeout and serialization layers distinct:

| Layer | Owner | Purpose |
|---|---|---|
| Operation context | `ContainerTestContext` | Bound one real-engine operation/test |
| Testscript deadline and `env.Defer` | CLI harness | Bound and clean one txtar case |
| In-process semaphore | Go package tests | Limit concurrent real-engine work |
| Cross-binary suite lock | CLI container suite | Serialize competing test binaries |
| Go/workflow timeout | test binary and CI | Outer last-resort termination |

Do not add the suite lock to ordinary Go package tests. Do not use a bare
`t.Context()` for an operation that can hang in a container daemon.

The durable Linux-specific mitigations are:

- health-check the selected engine before expensive integration work;
- pre-pull the policy-approved image in CI;
- preserve the Podman sysctl override and flock serialization for the
  rootless `ping_group_range` race;
- keep `cmd.WaitDelay` bounded for engine subprocess pipe cleanup;
- gate CLI container fixtures with the existing container-availability
  conditions and cleanup hooks.

## Failure Matrix

| Symptom | Evidence to collect | Likely route |
|---|---|---|
| Exit 137 or abruptly missing output | cgroup state and permitted kernel logs | OOM section in `process-namespaces.md` |
| inotify `ENOSPC` | `/proc/sys/fs/inotify/max_user_watches` and active watch count | `filesystem-inotify.md` |
| `EMFILE` / `ENFILE` | `ulimit -n`, process fd count, system file table | `filesystem-inotify.md` |
| Container test deadline | health probe, daemon logs, active containers, timeout layer | `container-testing-deep.md` |
| Rootless Podman `ping_group_range` failure | override env/file and lock path | container skill + Linux lock sources |
| Cleanup leaves containers/processes | wait path, cleanup registration, process group | process/container references |
| `EPERM` only inside container | engine security options and syscall evidence | namespace/seccomp reference |
| Wrong engine selected | current PATH, explicit engine env, workflow matrix | CI workflow and engine factory |
| Linux passes but macOS/Windows fails | path case, separators, signals, timing assumptions | matching platform skill |

Avoid treating higher timeouts, larger sysctl limits, or reduced parallelism as
the root fix until evidence identifies the exhausted or blocked layer.

## Source Hotspots

- `internal/testutil/container_context.go`
- `internal/testutil/container_semaphore.go`
- `internal/testutil/container_suite_lock_linux.go`
- `internal/container/run_lock_linux.go`
- `internal/container/podman_sysctl_linux.go`
- `internal/container/engine_base.go`
- `internal/watch/watcher_fatal_unix_test.go`
- `tests/cli/cmd_container_test.go`
- `.github/workflows/ci.yml`

## Verification

Run the smallest failing surface first, then the repository gates required by
the changed area. Typical focused commands are:

```bash
go test -count=1 -race ./internal/watch/...
go test -count=1 -race ./internal/container/... ./internal/testutil/...
go test -count=1 -race ./internal/runtime/... -run Container
go test -count=1 -race ./tests/cli/... -run Container
```

Use `make test` for final verification. Expected skips caused by a genuinely
unavailable container engine are not failures; unexpected skips or silently
reduced matrix coverage are findings.

## Related Skills

| Skill | Use for |
|---|---|
| `go-testing` | Go test execution, race, context, benchmark, and coverage behavior |
| `testing` | Invowk testscript, TUI, and component test patterns |
| `container` | Docker/Podman runtime and lifecycle implementation |
| `windows-testing` | Windows process, path, and filesystem behavior |
| `macos-testing` | APFS, kqueue, timer, and Darwin process behavior |
