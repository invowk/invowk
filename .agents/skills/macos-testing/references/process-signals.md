# macOS Process Lifecycle and Signal Handling

Reference material for the `macos-testing` skill. Covers process creation,
Mach exception handling, POSIX signals, and `exec.CommandContext` behavior.

---

## Process Creation

### fork+exec vs posix_spawn

macOS supports two process creation models:

| Mechanism | Used By | Behavior |
|-----------|---------|----------|
| `fork+exec` | Go's `exec.Command`, C `fork()`/`exec()` | Duplicate process, then replace image |
| `posix_spawn` | `NSTask`, `system()`, `popen()` | Create process with new image directly |

Modern macOS (10.14+) strongly prefers `posix_spawn`. The kernel optimizes
`posix_spawn` with a fast path that avoids full address space duplication.
`fork` without `exec` is discouraged and can deadlock in multi-threaded
programs (Go programs are always multi-threaded).

**Go's behavior:** `exec.Command` uses `fork+exec` internally. Go's runtime
handles the pre-fork/post-fork thread synchronization correctly:

1. Before `fork`: all goroutines are suspended; only the forking thread runs.
2. `fork()` creates the child process (single-threaded copy).
3. Child immediately calls `exec()` to replace the process image.
4. Parent resumes all goroutines.

This is safe because the child never runs Go code -- it executes immediately.
The brief window between `fork` and `exec` runs only C code that sets up
file descriptors, working directory, and process group.

### Process Groups and Sessions

macOS follows POSIX process group semantics:

```
Session (sid)
  +-- Process Group (pgid)
        +-- Leader process
        +-- Child process 1
        +-- Child process 2
```

**Relevant for Go tests:**

```go
// Create a new process group for the child
cmd := exec.CommandContext(ctx, "long-running-tool")
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true,  // Child gets its own process group
}

// Send signal to the entire process group
syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
```

Setting `Setpgid: true` is important when the child may spawn its own
children. Without it, sending a signal to the parent process only kills
the direct child, leaving grandchildren as orphans.

---

## Mach Exception Handling

macOS has a two-layer fault handling mechanism:

```
Hardware fault
  +-- Mach exception (EXC_*)
        +-- Caught by Mach exception handler? -> handled
        +-- Not caught -> converted to POSIX signal (SIG*)
              +-- Caught by signal handler? -> handled
              +-- Not caught -> default action (terminate, core dump, etc.)
```

### Exception Types

| Mach Exception | POSIX Signal | Cause |
|---------------|-------------|-------|
| `EXC_BAD_ACCESS` | `SIGSEGV` / `SIGBUS` | Invalid memory access |
| `EXC_BAD_INSTRUCTION` | `SIGILL` | Illegal instruction |
| `EXC_BREAKPOINT` | `SIGTRAP` | Breakpoint / debug trap |
| `EXC_ARITHMETIC` | `SIGFPE` | Division by zero, overflow |
| `EXC_SOFTWARE` | Various | Software-generated exception |
| `EXC_CRASH` | `SIGABRT` | Process crashed |

### Impact on Testing

**Debuggers** (like Delve) intercept Mach exceptions via task exception ports.
This means:
- A test running under Delve may not see `SIGSEGV` in the normal signal path.
- Breakpoints work by inserting `EXC_BREAKPOINT` traps, not `SIGTRAP` signals.
- This is transparent for normal Go test execution -- it only matters when
  debugging tests interactively.

**Go's runtime** does not install Mach exception handlers. On darwin it
handles faults through its POSIX signal handlers: the kernel converts
`EXC_BAD_ACCESS` to `SIGSEGV`/`SIGBUS`, and the runtime turns that into a
recoverable panic (`runtime error: invalid memory address or nil pointer
dereference`). Stack overflow is different: the goroutine stack check in the
function prologue reports `goroutine stack exceeds limit` as a fatal runtime
error, which is not a panic and cannot be recovered. Mach exceptions matter
only to debuggers such as Delve (above).

---

## POSIX Signals on macOS

macOS supports the full POSIX signal set. Go's `os/signal` package works
identically on macOS and Linux for test-relevant signals.

### Signal Reference

| Signal | Number | Default | Blockable | Notes |
|--------|--------|---------|-----------|-------|
| `SIGHUP` | 1 | Terminate | Yes | Terminal hangup; Go exits by default, stays ignored if inherited as ignored (e.g. `nohup`) |
| `SIGINT` | 2 | Terminate | Yes | Ctrl+C; `os.Interrupt` |
| `SIGQUIT` | 3 | Core dump | Yes | Ctrl+\\; Go dumps goroutines |
| `SIGILL` | 4 | Core dump | Yes | Illegal instruction |
| `SIGTRAP` | 5 | Core dump | Yes | Debug trap |
| `SIGABRT` | 6 | Core dump | Yes | `abort()` / Go fatal |
| `SIGFPE` | 8 | Core dump | Yes | Arithmetic error |
| `SIGKILL` | 9 | Terminate | **No** | Unblockable kill |
| `SIGBUS` | 10 | Core dump | Yes | Bus error (alignment) |
| `SIGSEGV` | 11 | Core dump | Yes | Segmentation fault |
| `SIGPIPE` | 13 | Terminate | Yes | Broken pipe; fatal on fd 1/2, `EPIPE` elsewhere (see below) |
| `SIGALRM` | 14 | Terminate | Yes | Timer alarm |
| `SIGTERM` | 15 | Terminate | Yes | Graceful shutdown |
| `SIGSTOP` | 17 | Stop | **No** | Unblockable pause |
| `SIGTSTP` | 18 | Stop | Yes | Ctrl+Z; terminal stop |
| `SIGCONT` | 19 | Continue | Yes | Resume stopped process |
| `SIGCHLD` | 20 | Ignore | Yes | Child process status change |
| `SIGUSR1` | 30 | Terminate | Yes | User-defined |
| `SIGUSR2` | 31 | Terminate | Yes | User-defined |

**macOS-specific note:** Signal numbers differ from Linux for some signals.
`SIGBUS` is 10 on macOS (7 on Linux), `SIGSTOP` is 17 (19 on Linux),
`SIGCONT` is 19 (18 on Linux). Go's `syscall` package handles this --
always use `syscall.SIGTERM` not the raw number.

### Signal Handling in Go

Go installs signal handlers with `SA_ONSTACK` (alternate signal stack) and
`SA_RESTART` (restart interrupted system calls). Signals are delivered to a
dedicated signal-handling M (OS thread) and dispatched to goroutines via
`signal.Notify`.

Key behaviors:
- `signal.Notify(ch, sig)` -- channel must be buffered (at least 1). Signals
  that arrive when the channel is full are dropped.
- `signal.Stop(ch)` -- stop delivering signals to this channel.
- `signal.Ignore(sig)` -- ignore signal entirely (no delivery, no default).
- `signal.Reset(sig)` -- restore default signal behavior.

```go
// Correct signal handling in tests
ch := make(chan os.Signal, 1)
signal.Notify(ch, syscall.SIGTERM)
defer signal.Stop(ch)

// Send signal to self
syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

select {
case sig := <-ch:
    // sig == syscall.SIGTERM
case <-time.After(time.Second):
    t.Fatal("signal not received")
}
```

### SIGPIPE Behavior

Per the `os/signal` package documentation, when the program has not called
`signal.Notify` for `SIGPIPE`:

- a write to a broken pipe on file descriptor 1 or 2 (stdout/stderr) makes
  the program exit with `SIGPIPE`;
- a write to a broken pipe on any other descriptor (pipes, sockets) fails
  with `syscall.EPIPE` and the program keeps running.

So test code that pipes a subprocess's output gets an `EPIPE` error return,
while a Go binary whose own stdout is closed early (`invowk ... | head -1`)
can die by `SIGPIPE`. This has nothing to do with `SA_RESTART`.

**macOS-specific:** The `SIGPIPE` behavior is identical to Linux in Go
programs. However, C programs linked via cgo may see different default
`SIGPIPE` handling depending on whether the C runtime installs its own
handler.

---

## exec.CommandContext on macOS

`exec.CommandContext` cancels a running command when the context is cancelled.
On macOS, the default behavior sends `SIGKILL` (immediate, unblockable).

### Customizing Cancellation

Go 1.20+ supports `cmd.Cancel` and `cmd.WaitDelay` for graceful shutdown:

```go
ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
defer cancel()

cmd := exec.CommandContext(ctx, "long-running")

// Send SIGTERM first, then SIGKILL after grace period
cmd.Cancel = func() error {
    return cmd.Process.Signal(syscall.SIGTERM)
}
cmd.WaitDelay = 3 * time.Second // Wait 3s for graceful exit before SIGKILL
```

**Sequence on context cancellation:**
1. Context cancelled.
2. `cmd.Cancel()` called -- sends `SIGTERM`.
3. If process exits within `WaitDelay` -- `cmd.Wait()` returns.
4. If process still running after `WaitDelay` -- `SIGKILL` sent.
5. `cmd.Wait()` returns with the exit status.

### Process Group Cleanup

When a command spawns child processes, killing the parent may leave children
running. Use process group signals for clean shutdown:

```go
cmd := exec.CommandContext(ctx, "parent-process")
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

cmd.Cancel = func() error {
    // Signal the entire process group
    return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}
```

**macOS note:** Process group signaling works identically to Linux. The
negative PID convention (`-pid`) sends the signal to all processes in the
group.

### Exit Code Semantics

Shell and CI-log convention on macOS (and Linux):
- 0: success
- 1-125: application-defined failure
- 126: command found but not executable
- 127: command not found
- 128+N: killed by signal N (e.g., 137 = 128 + 9 = SIGKILL). Shells and
  GitHub Actions logs ("Process completed with exit code 137") report a
  signal death this way, which is why 137 routes to OOM triage.

Go callers see something different: for a signalled child,
`ProcessState.ExitCode()` returns -1, not 128+N. Detect signal death with
`WaitStatus.Signaled()` and `WaitStatus.Signal()`.

**Comparison with Windows:** On Windows, `exec.CommandContext` uses
`TerminateProcess` which sets exit code 1. This makes a context kill
indistinguishable from a normal failure. On Unix the signal is preserved in
the wait status, so tests can distinguish graceful exit, error exit, and
signal death.

```go
// On macOS/Linux: distinguish signal death from error exit
var exitErr *exec.ExitError
if errors.As(err, &exitErr) {
    status := exitErr.Sys().(syscall.WaitStatus)
    if status.Signaled() {
        sig := status.Signal() // e.g., syscall.SIGKILL
        // Handle signal death
    }
}
```

---

## invowk-Relevant Process Patterns

### Native Runtime Command Execution

invowk's native runtime (`internal/runtime/native.go`) uses `exec.CommandContext`
to run shell commands. On macOS and Linux the shell comes from `getShell`:
`$SHELL` when set, else `bash`, else `sh` from `PATH`. Signal delivery to the
child process follows standard POSIX semantics. The runtime does not put the
child in its own process group; context cancellation kills only the direct
child (`SIGKILL`), bounded by `cmd.WaitDelay`.

`promoteContextError()` (`native_helpers.go`) exists for Windows, where
`TerminateProcess` leaves a valid exit code 1 and no error. On Unix the path
differs, and this is a **known open gap pending a maintainer decision, not a
contract**: a context-killed child reports `ExitCode() == -1`,
`ExitCode.Validate` (`pkg/types/exit_code.go`) rejects negative values, so
`extractExitCode` already sets `result.Error` to an `InvalidExitCodeError`
and `promoteContextError` does not surface `context.DeadlineExceeded`. The
behavior has not been verified at run time. Do not write tests or code that
rely on either classification until the maintainer decides; report it if a
failure depends on it.

### Container Runtime on macOS

On macOS, Podman runs inside a Linux VM (`podman machine`). The host-side
invowk process communicates with the VM via the Podman API socket.
`exec.CommandContext` on the host controls the Podman CLI client, not the
container process directly. Signal delivery to the Podman client causes it
to forward the signal to the container via the API.

The flock fallback (`run_lock_other.go`) returns `errFlockUnavailable`
because host-side file locks cannot reach the VM filesystem; callers fall back
to `sync.Mutex`. See "flock on macOS" in `filesystem-kqueue.md`.
