# Failure Pattern Catalog

Comprehensive catalog of failure patterns seen in the invowk project. Each entry
includes symptom, root cause, fix template, prevention measure, and platform skill
cross-reference. Where another document owns the full explanation, the entry keeps
the symptom and the one-line fix or directive and points there.

Use this catalog during Phase 2 (Diagnosis) of the fixer workflow to match observed
symptoms against known patterns. If a match is found, apply the documented fix
rather than investigating from scratch.

---

## CI Workflow / Scheduled Quality Gates

### CI-1: Step-scoped GitHub Actions env reused in a later step

**Symptom**: A GitHub Actions shell step fails under `set -u` with an error such
as `BENCH_REGEX: unbound variable`, often after the preceding build, test, or
benchmark command succeeded.

**Root cause**: The variable is defined only in an earlier step's `env:`;
step-level `env:` is not visible to later steps.

**Fix**: Promote the shared variable to job-level `env:` (or repeat it on every
step that reads it). Under `set -u`, check every `$VARIABLE` reference in the
workflow against the step, job, workflow and built-in environments. For
benchmark workflows, check the `bencher` skill before changing shared variables.

**Canonical home**: `.agents/rules/ci-workflows.md`; `bencher` SKILL.md;
`.agents/runbooks/quality-gates.md` (CI workflows).

---

### CI-2: Tracked mutation report trips the mutation dirty-worktree guard

**Symptom**: A manual Mutation Testing workflow or local mutation run fails
before running mutants with:

```text
Mutation testing rewrites package sources while it runs.
Commit, stash, or move these tracked changes before mutating:
  report.json
```

**Root cause**: A root-level generated mutation report is tracked; the wrapper
deletes stale reports first, which leaves a tracked deletion, and the guard
correctly aborts.

**Fix**: Untrack the generated reports and keep reports under the ignored
`artifacts/mutation/<profile>/<module>/`. Check with:

```bash
git ls-files report.json go-mutesting-summary.json go-mutesting-agentic.json \
  go-mutesting-gitlab.json go-mutesting-report.html
```

Never commit root-level go-mutesting output unless it is deliberately a docs or
release artifact with an explicit path and owner; run `make test-scripts` after
changing mutation tooling. If a tool starts writing a new root-level report name,
add collection/cleanup coverage and ignore/tracking checks in the same patch.

**Canonical home**: `.agents/rules/ci-workflows.md`,
`.agents/runbooks/mutation-testing.md`; `testing` SKILL.md for script-test updates.

---

## Race Conditions

### RC-1: Windows TUI `-race` failure (formerly attributed to lipgloss `sync.Once`)

**Symptom**: Race detector fires on `lipgloss` internal state during parallel TUI
tests on `windows-latest`, or Windows TUI tests under `-race` time out, crash, or
leave `(unknown)` results.

**Guardrail**: Do not assume lipgloss terminal detection and do not reintroduce a
`TestMain` pre-warm; it was a no-op in this repo (history:
`.agents/memorials/skills-platform-testing-history.md`). Treat the failure as
memory or scheduling pressure under `-race` unless a `DATA RACE` report with two
access stacks proves a real race.

**Prevention**: Prove the race with `-race` and inspect both stacks; prefer
package-local serialization or non-parallel tests around first render; add a
`TestMain` warm-up only when the package already has a verified pattern for it.

**Platform skill**: `windows-testing` SKILL.md ("Timing And Race Detector"),
`go-testing/references/race-detector-guide.md` ("Platform-Specific Behavior").

---

### RC-2: CUE cue.Value / cue.Context concurrent access

**Symptom**: Race detector fires on CUE internal state. Typically during
`BehavioralSync` tests or any test that calls `cue.Value.Unify()`,
`cue.Context.CompileString()`, or iterates CUE fields in parallel.

**Root cause**: `cue.Value` and `*cue.Context` are NOT safe for concurrent use.
`Unify()` and `CompileString()` mutate internal state. Sharing a CUE context
across parallel subtests causes data races.

**Fix template**: Run CUE-touching subtests serially. Add `//nolint:tparallel`
with a justification comment:

```go
//nolint:tparallel // CUE cue.Value is not safe for concurrent use
func TestBehavioralSync_SomeField(t *testing.T) {
    t.Parallel() // parent can be parallel
    // subtests must NOT call t.Parallel()
    for name, tc := range cases {
        t.Run(name, func(t *testing.T) {
            // no t.Parallel() here — serial execution required
        })
    }
}
```

**Prevention**: Never share `cue.Value` or `*cue.Context` across goroutines.
Create a new context per subtest if parallelism is needed.

**Platform skill**: `go-testing` SKILL.md ("Parallelism Decision Framework").

---

### RC-3: SSH host key file collision (RESOLVED)

**Symptom**: `permission denied` / `file exists` on `id_ed25519`, or a race
report, when SSH server tests run in parallel.

**Guardrail**: `sshserver.Server` always sets a host key through
`hostKeyOption()` (`internal/sshserver/server_lifecycle.go`), so this should not
recur. If it reappears, a new server construction path is bypassing
`hostKeyOption()` — fix that, do not serialize tests. History:
`.agents/memorials/skills-platform-testing-history.md`.

**Platform skill**: `go-testing` SKILL.md ("Parallelism Decision Framework").

---

### RC-4: os.Stdin replacement in parallel tests

**Symptom**: Race detector fires on `os.Stdin` access. One goroutine replaces
`os.Stdin` for a TUI test; another goroutine reads the original `os.Stdin`.

**Root cause**: `os.Stdin` is a package-level global. Replacing it in one test
while another test reads it is a data race. This affects tests in `internal/tui/`
and `cmd/invowk/` that simulate piped input.

**Fix template**: Do not call `t.Parallel()` on tests that replace `os.Stdin`.
Use the `isInputPiped()` and `readInputAll()` helpers from `cmd/invowk/tui_stdin.go`
instead of direct `os.Stdin` manipulation where possible.

**Prevention**: Tests that touch `os.Stdin` must omit `t.Parallel()`. Document the
reason in a comment: `// no t.Parallel(): replaces os.Stdin`.

**Platform skill**: `go-testing` SKILL.md ("Parallelism Decision Framework",
"Resource Safety Matrix").

---

### RC-5: Shared MockCommandRecorder across subtests

**Symptom**: Race detector fires on `MockCommandRecorder` fields. Multiple parallel
subtests write to the same mock recorder simultaneously.

**Root cause**: A single `MockCommandRecorder` instance is created in the parent test
and shared across parallel subtests. The functions returned by its `CommandFunc` and
`ContextCommandFunc` append to the exported `Invocations` slice without a lock.

**Fix template**: Create a fresh mock recorder per subtest:

```go
for name, tc := range cases {
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        recorder := &MockCommandRecorder{} // per-subtest instance
        // ... use recorder ...
    })
}
```

**Prevention**: Never share mock recorders across parallel subtests. Each subtest
must have its own instance.

**Platform skill**: `go-testing` SKILL.md ("Parallelism Decision Framework").

**Codebase locations**: `internal/container/engine_mock_test.go`,
`internal/container/engine_docker_mock_test.go`,
`internal/container/engine_podman_mock_test.go`.

---

## Timing / Flakiness

### TF-1: Watcher missing events on macOS (kqueue coalescing)

**Symptom**: Watcher tests fail on `macos-15` with missed file events. A file is
created or modified but the watcher callback is never invoked within the deadline.

**Root cause**: macOS uses `kqueue` for filesystem watching. kqueue coalesces rapid
events more aggressively than Linux's `inotify`. Multiple rapid file changes may
produce a single event, or the event may be delayed by macOS timer coalescing.

**Fix template**: Increase debounce intervals and use poll loops with generous
deadlines instead of expecting immediate events:

```go
// Bad: tight timing
time.Sleep(50 * time.Millisecond)
assert(eventReceived)

// Good: poll with deadline
deadline := time.After(2 * time.Second)
for {
    select {
    case <-eventCh:
        return // success
    case <-deadline:
        t.Fatal("timed out waiting for watcher event")
    }
}
```

**Prevention**: All watcher tests must use event-based synchronization (channels,
poll loops) instead of `time.Sleep`. Minimum poll deadline: 2 seconds for macOS.

**Platform skill**: `macos-testing` SKILL.md ("Watcher and Timing Contract"),
`macos-testing/references/filesystem-kqueue.md` ("kqueue Event Notification",
"Timer Coalescing").

---

### TF-2: tmux TUI test timing

**Symptom**: tmux-based TUI tests fail intermittently with assertion mismatches.
The terminal output does not contain the expected text when the assertion runs.

**Root cause**: tmux pane capture happens before the TUI has finished rendering.
The `time.Sleep` between sending input and capturing output is too short, or the
TUI render cycle takes longer than expected under CI load.

**Fix template**: Use poll loops with deadline instead of fixed sleeps. Query tmux
pane content repeatedly until the expected text appears or the deadline expires.

**Prevention**: All tmux test assertions must use polling. Document the expected
render time in a comment for calibration.

**Platform skill**: `macos-testing` SKILL.md ("Watcher and Timing Contract";
detail in `references/filesystem-kqueue.md` "Timer Coalescing"),
`tmux-testing` SKILL.md.

---

### TF-3: Windows timer resolution (15.6ms)

**Symptom**: Timer-based tests flake on `windows-latest`. A test expects an operation
to complete within a tight deadline, but Windows timer ticks only every 15.6ms by
default. A 10ms sleep actually sleeps for 15.6ms (or 31.2ms under load).

**Root cause**: Windows uses a 15.625ms default timer interrupt period. `time.Sleep`,
`time.After`, and `time.Tick` have minimum resolution of one timer tick. Under CI
load, timer granularity can effectively double.

**Fix template**: Use generous timeouts on Windows (at least 100ms for sub-second
operations). Avoid assertions that depend on sub-15ms timing precision.

**Prevention**: Never assert exact timing on Windows. Use `>=` assertions with wide
margins. For cross-platform tests, use the largest timeout needed by any platform.

**Platform skill**: `windows-testing` SKILL.md ("Timing And Race Detector").

---

### TF-4: macOS timer coalescing

**Symptom**: A `time.Sleep(100 * time.Millisecond)` takes 150-200ms on macOS.
Tests that depend on the sleep completing within a tight window fail intermittently.

**Root cause**: macOS power management coalesces timers to reduce wake-ups.
Short sleeps may be delayed to align with other scheduled timer events. This is
most pronounced on battery power (laptops) but also affects CI runners.

**Fix template**: Use event-based synchronization instead of sleep-based timing.
When sleep is unavoidable, allow 2x the expected duration on macOS.

**Prevention**: Replace `time.Sleep` with channel-based synchronization wherever
possible. Document macOS timer coalescing as the reason when wider timeouts are
needed.

**Platform skill**: `macos-testing` SKILL.md ("Watcher and Timing Contract"),
`macos-testing/references/filesystem-kqueue.md` ("Timer Coalescing").

---

### TF-5: Container test slow start

**Symptom**: Container integration tests fail on first run or under CI load with
`connection refused` or `context deadline exceeded` during container startup.

**Root cause**: The container engine (Docker or Podman) takes variable time to start
a container. Under CI load with limited resources, startup can take several seconds.
Without an engine health probe, tests may attempt to interact with a container before
it is ready.

**Fix template**: Use the engine health probe and pre-pull patterns from CI:

```go
// Use testutil.ContainerTestContext for bounded context
ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)

// CI pre-pulls the base image in the "Pre-pull container test images" step
// of .github/workflows/ci.yml: ${{ matrix.engine }} pull debian:stable-slim
```

**Prevention**: All container tests must use `testutil.ContainerTestContext()` with
an explicit timeout, never bare `t.Context()`. CI pre-pulls `debian:stable-slim` to
remove network latency from the critical path.

**Platform skill**: `linux-testing` SKILL.md ("Container Test Invariants"),
`linux-testing/references/container-testing-deep.md` ("Engine Health Probes",
"Image Pre-Pull Strategy").

---

## Platform-Specific

### PS-1: Windows CRLF in regex assertions

**Symptom**: Testscript `stdout` or `stderr` regex assertions fail on Windows even
though the output looks correct. The regex matches on Linux/macOS but not Windows.

**Root cause**: PowerShell and many Windows tools emit `\r\n`; `$` matches before
`\n`, so the `\r` stays in the line.

**Fix**: Use `\r?$` on every anchored assertion (`stdout '^done\r?$'`); it also
passes on Unix. Substring assertions need nothing. Do not split assertions by
platform for this.

**Canonical home**: `.agents/rules/testing.md` "Native tests";
`windows-testing/references/powershell-testing.md`.

---

### PS-2: Windows TerminateProcess exit code masking

**Symptom**: A test expects a specific non-zero exit code from a cancelled context,
but gets exit code 1 on Windows. The error message does not mention context cancellation.

**Root cause**: `exec.CommandContext` on Windows calls `TerminateProcess`, which sets
exit code 1. This is indistinguishable from a normal failure. The `ctx.Err()` is
silently dropped because the exit code looks like a valid non-context error.

**Fix template**: Call `promoteContextError()` from `internal/runtime/native_helpers.go`
after `extractExitCode`:

```go
// In native runtime execution path:
result := extractExitCode(err, captured)
promoteContextError(ctx, result)
```

**Prevention**: Always check `ctx.Err()` after process exit on Windows when the
context may have been cancelled.

On Unix the same helper does not surface the context error (the killed child's
exit code -1 fails validation first); that is an open gap pending a maintainer
decision (`macos-testing/references/process-signals.md` "Native Runtime Command
Execution").

**Platform skill**: `windows-testing` SKILL.md ("Process Lifecycle"),
`windows-testing/references/process-lifecycle.md` ("The promoteContextError Pattern").

**Codebase locations**: `internal/runtime/native_helpers.go`, `internal/runtime/native.go`.

---

### PS-3: macOS /tmp and /var path aliases (/private/tmp, /private/var)

**Symptom**: Path comparison assertion fails on macOS: one side shows `/tmp/...`
or `/var/folders/...`, the other `/private/tmp/...` or `/private/var/...`.

**Root cause**: `/tmp` and `/var` are symlinks into `/private`. `os.TempDir()`
returns `$TMPDIR` (normally `/var/folders/.../T`), and `t.TempDir()` builds on it
without resolving symlinks, while `filepath.EvalSymlinks`, `os.Getwd` after
`chdir`, or a child process report the resolved `/private/...` path.

**Fix template**: Build both sides from the same `t.TempDir()` base; for
resolver-backed APIs derive the expectation through the same production
resolver; when only identity matters compare `os.Stat` results with
`os.SameFile`. Do not replace an API string contract with ad hoc
`filepath.EvalSymlinks` on both sides.

**Prevention**: Never hardcode `/tmp` in tests. Always use `t.TempDir()` for
temporary directories.

**Platform skill**: `macos-testing` SKILL.md ("Filesystem Contract"),
`macos-testing/references/filesystem-kqueue.md` ("The `/tmp` -> `/private/tmp`
Symlink").

---

### PS-4: Windows ERROR_SHARING_VIOLATION on temp files

**Symptom**: `Access is denied` or `The process cannot access the file because it is
being used by another process` during test cleanup on Windows.

**Root cause**: Windows uses mandatory file locking. If a file is opened by one process
(including Windows Defender real-time scanning), another process cannot delete or
rename it. Test cleanup that deletes temp files races with Defender scans.

**Fix template**: Close all file handles explicitly before cleanup. For stubborn cases,
retry deletion with a short backoff:

```go
t.Cleanup(func() {
    // Close handle first
    f.Close()
    // Retry removal (Defender may hold a brief lock)
    for range 3 {
        if err := os.Remove(f.Name()); err == nil {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
})
```

**Prevention**: Always close file handles before calling `os.Remove` or relying on
`t.TempDir()` cleanup. Use `t.TempDir()` which handles retry internally.

**Platform skill**: `windows-testing` SKILL.md ("Failure Matrix"),
`windows-testing/references/filesystem-pitfalls.md` ("ERROR_SHARING_VIOLATION").

---

### PS-5: Linux ENOSPC from inotify

**Symptom**: `inotify: too many watches` or `no space left on device` error from
`fsnotify.NewWatcher()` or `watcher.Add()` on Linux.

**Root cause**: The per-user inotify watch limit
(`/proc/sys/fs/inotify/max_user_watches`; defaults vary by distro, see
`linux-testing/references/filesystem-inotify.md` "Per-User Watch Limits") is
exhausted, often by parallel tests each creating watchers or by leaked watchers.

**Fix template**: Evidence first. Read the live limit and count active watches,
and distinguish `ENOSPC` from `EMFILE`/`ENFILE`. Then close leaked watchers
(`t.Cleanup`), reduce watched directories, or serialize watcher-heavy tests. Do
not treat a larger sysctl limit as the root fix without that evidence; CI does
not raise inotify limits today. A local `sudo sysctl -w
fs.inotify.max_user_watches=...` is a diagnostic aid, not a fix.

**Prevention**: Watcher tests clean up watchers in `t.Cleanup()`.

**Platform skill**: `linux-testing` SKILL.md ("Filesystem and Watch Contract",
"Failure Matrix"), `linux-testing/references/filesystem-inotify.md`
("Error Handling").

---

### PS-6: Windows PATHEXT affecting exec.LookPath

**Symptom**: `exec.LookPath` finds (or fails to find) an executable on Windows
when it works on Linux/macOS. Or it finds a `.bat` file instead of the expected
binary.

**Root cause**: Windows `exec.LookPath` searches `PATHEXT` extensions (`.COM`,
`.EXE`, `.BAT`, `.CMD`, etc.) in order. A `foo.bat` in the PATH will be found
before `foo.exe` if `.BAT` precedes `.EXE` in `PATHEXT`. Tests creating executable
files must use `.bat` or `.exe` extensions on Windows.

**Fix template**: Use `[windows]`/`[!windows]` conditionals in testscript tests
to create platform-appropriate executables. In Go tests, use
`goruntime.GOOS == platform.Windows` (constants in `pkg/platform/os.go`) to
choose the correct extension.

**Prevention**: Tests that create or look up executables must account for PATHEXT
on Windows. Use `pkg/platform` constants for OS detection.

**Platform skill**: `windows-testing` SKILL.md ("Filesystem And Paths",
"Failure Matrix").

---

## Container

### CT-1: Container test hangs indefinitely

**Symptom**: A container integration test hangs forever on Linux CI. The test
never completes, and the CI job is eventually killed by the 30-minute job timeout.
No output is produced after the container command starts.

**Root cause**: The test uses `t.Context()` (which has no deadline) or
`context.Background()` instead of a bounded context. The container process blocks
in an uninterruptible kernel state (Podman cgroup operations), and no timeout
fires to cancel it.

**Fix template**: Use `testutil.ContainerTestContext()` with an explicit timeout:

```go
ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
execCtx := NewExecutionContext(ctx, cmd, inv)
result := rt.Execute(execCtx)
```

**Prevention**: All container tests that reach a real engine must use
`testutil.ContainerTestContext()`. Defense in depth comes from the timeout and
serialization layers in `linux-testing` SKILL.md "Container Test Invariants"
(operation context, testscript deadline and `env.Defer`, in-process semaphore,
cross-binary suite lock, Go/workflow timeout), plus the per-test engine health
probe.

**Platform skill**: `linux-testing` SKILL.md ("Container Test Invariants"),
`linux-testing/references/container-testing-deep.md` ("Timeout and
Serialization Layers").

**Codebase locations**: `internal/testutil/container_context.go`,
`internal/runtime/container_integration_validation_test.go`.

---

### CT-2: OOM with -race (exit code 137)

**Symptom**: Container tests exit with code 137 on Linux CI. The race detector
is enabled. Multiple container tests run in parallel.

**Root cause**: The Go race detector adds roughly 5-10x memory overhead. Combined with
container operations (which fork processes and allocate cgroup memory), parallel
container tests can exceed the runner's memory limit. The Linux OOM killer
terminates the process with SIGKILL (exit code 137).

**Fix template**: Limit container test parallelism with the semaphore:

```go
// Set via environment variable in CI
// INVOWK_TEST_CONTAINER_PARALLEL=2
```

The CI workflow sets `INVOWK_TEST_CONTAINER_PARALLEL: "2"` for all full-mode runs.

**Prevention**: Never increase `INVOWK_TEST_CONTAINER_PARALLEL` above 2 for race-enabled
builds. Monitor CI memory usage if new container tests are added.

**Platform skill**: `linux-testing/references/process-namespaces.md`
("OOM Killer"), `linux-testing/references/container-testing-deep.md`.

---

### CT-3: Podman ping_group_range race

**Symptom**: Podman container tests fail intermittently with permission errors or
`newuidmap`/`newgidmap` failures on Linux CI. Multiple Podman tests start
containers simultaneously.

**Root cause**: Rootless Podman's OCI runtime (crun) writes
`/proc/sys/net/ipv4/ping_group_range` from Podman's `default_sysctls` when a
container starts; concurrent invocations race on that sysctl write.
`newuidmap`/`newgidmap` invocations can also collide when several containers
start at once.

**Fix template**: Keep both existing mitigations: `CONTAINERS_CONF_OVERRIDE`
with `default_sysctls = []` (`internal/container/podman_sysctl_linux.go`; it
cannot help podman-remote, where the service performs the write) and the flock
run serialization (`internal/container/run_lock_linux.go`).

**Prevention**: The container test semaphore and
`INVOWK_TEST_CONTAINER_PARALLEL` limit in-process concurrency; `flock` provides
cross-binary serialization when several test binaries run at once.

**Platform skill**: `linux-testing` SKILL.md ("Container Test Invariants",
"Failure Matrix"), `linux-testing/references/container-testing-deep.md`
("Podman-Specific Issues").

**Codebase locations**: `internal/container/podman_sysctl_linux_test.go`,
`internal/container/run_lock_linux_test.go`.

---

### CT-4: Engine masking failure in CI

**Symptom**: A "Mask Docker for Podman tests" / "Mask Podman for Docker tests"
step fails with `::error::<engine> is still on PATH after masking`, or container
tests use the wrong engine.

**Root cause**: The steps loop over `command -v`, rename each `PATH` install of
the non-tested engine to `.disabled`, and fail loudly if it is still found.
Runner images move engine binaries (between `/usr/bin` and `/usr/local/bin`) or
add installs.

**Fix template**: Read the mask step output and the step in
`.github/workflows/ci.yml`; adjust the masking loop to the new install layout.

**Prevention**: Keep masking fail-loud; never add `|| true`.

**Platform skill**: `linux-testing/references/container-testing-deep.md`
("Engine Masking"); `.github/workflows/ci.yml` mask steps.

---

### CT-5: Container stderr in exit-code tests

**Symptom**: A testscript container error-path test fails because `! stderr .`
matches unexpected stderr output. The container command produced incidental
stderr (e.g., shell prompt `#`, container runtime warnings).

**Fix**: Assert specific stderr text; never use `! stderr .` in container
testscript tests:

```
! exec invowk cmd broken-cmd
stderr 'expected error message'
```

**Canonical home**: `linux-testing/references/container-testing-deep.md`
("Container Stderr in Exit-Code Tests"); `testing/references/testscript-and-mirrors.md`
("Testscript Contract").

---

## Assertion / Logic

### AL-1: Path separator in assertions

**Symptom**: A test assertion comparing file paths fails on Windows. The expected
path uses `/` separators but the actual path uses `\`.

**Fix**: Never hardcode path separators for host paths: build expected values
with `filepath.Join()` (in testscript, with `${/}`). Container paths are always
`/` and may stay literal.

```go
expected := filepath.Join("internal", "config", "config.go")
```

**Canonical home**: `.agents/rules/testing.md` "Cross-Platform Testing";
`windows-testing` SKILL.md ("Filesystem And Paths").

---

### AL-2: Case-sensitive filename collision on macOS

**Symptom**: A test creates two files with names differing only in case (e.g.,
`Config.go` and `config.go`). On macOS, only one file exists after creation.

**Root cause**: APFS (the default macOS filesystem) is case-preserving but
case-insensitive. `Config.go` and `config.go` refer to the same file. Creating
the second silently overwrites the first.

**Fix template**: Use unique filenames that differ in more than just case. For
tests that specifically test case sensitivity, skip on macOS:

```go
if goruntime.GOOS == platform.Darwin {
    t.Skip("macOS APFS is case-insensitive")
}
```

**Prevention**: Never rely on case-sensitive filenames in cross-platform tests.
Use `t.Skip` with explanation for tests that specifically exercise case sensitivity.

**Platform skill**: `macos-testing` SKILL.md ("Filesystem Contract"),
`macos-testing/references/filesystem-kqueue.md` ("APFS (Apple File System)").

---

### AL-3: gotestsum false-FAIL on parallel subtests

**Symptom**: The parent test shows `FAIL` in gotestsum output even though all
subtests passed. The CI JUnit report marks the parent as failed. This occurs
primarily on macOS CI.

**Root cause**: Without the `-v` flag, `gotestsum` does not receive per-subtest
PASS/FAIL lines from `go test`. It cannot reconcile the parent test status when
subtests run in parallel and complete in non-deterministic order. The `testdox`
format requires `-v` to correctly attribute subtest outcomes to their parents.

**Fix template**: Ensure `-v` is passed to `go test` when using `gotestsum` with
parallel subtests:

```bash
gotestsum --format testdox --packages ./... -- -v -race
```

The short-mode unit step ("Run tests (short)") passes `-v`. The full-mode
`--rerun-fails` steps ("Run tests (full, non-CLI)", "Run internal/runtime tests
(full)") currently do not, so a confusing parent FAIL there may be this artifact;
changing the workflow is a CI change outside this catalog.

**Prevention**: Always pass `-v` on the `go test` side of new or edited
`gotestsum --rerun-fails` invocations (`go-testing` SKILL.md "gotestsum
Integration").

**Platform skill**: `go-testing` SKILL.md ("gotestsum Integration");
`macos-testing` SKILL.md ("Failure Matrix", gotestsum row).

---

### AL-4: Testscript dead commands after file marker

**Symptom**: A testscript command appears to be silently ignored. No error, no
output — the command simply does not execute.

**Root cause**: Commands placed after a `-- filename --` marker in a `.txtar` file
are treated as file content, not as testscript commands. The testscript parser
considers everything after the first `-- filename --` line as part of the file
section.

**Fix template**: Move the command above the file section, or add it as a separate
command block before the file markers:

```
# Commands go here, before file markers
exec invowk cmd my-command
stdout 'expected output'

-- invowkfile.cue --
{file content}
```

**Prevention**: Review testscript files to ensure all commands appear before the
first `-- filename --` marker.

**Platform skill**: `testing/references/testscript-and-mirrors.md` ("Testscript Contract").

---

### AL-5: Native txtar assertions mirroring virtual assertions verbatim

**Symptom**: A `native_*.txtar` test fails because its assertions were copied
verbatim from the corresponding `virtual_*.txtar` test. The native test has
different scripts per platform (bash vs PowerShell), but the assertions assume
identical output format.

**Root cause**: Native tests run platform-specific shell scripts (`$SHELL`/bash on
Linux/macOS, PowerShell on Windows). Script output format may differ from the
virtual shell, and PowerShell emits CRLF.

**Fix template**: Make both platform implementations print the same text and
write one platform-agnostic assertion; `.agents/rules/testing.md` "Native tests"
requires identical `stdout`/`stderr` assertions across platforms. Anchored
assertions use `\r?$`, which passes on every platform:

```
stdout 'result: ok'
stdout '^done\r?$'
```

**Prevention**: When generating native mirrors from virtual tests (using the
`native-mirror` skill), review all assertions for platform-specific assumptions.
Do not copy virtual assertions verbatim.

**Platform skill**: `testing/references/testscript-and-mirrors.md` ("Cross-Platform Implementations"),
`native-mirror` SKILL.md.
