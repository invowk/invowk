# Windows Process Lifecycle for Go Developers

Deep dive on Windows process management and how it affects Go test code.
This reference supports the parent skill (`SKILL.md`) with detailed
explanations of Win32 process APIs and their Go counterparts.

## CreateProcess API

Windows does not have `fork()`. All process creation uses the `CreateProcess`
Win32 API (or its variants: `CreateProcessAsUser`, `CreateProcessWithLogonW`,
`CreateProcessWithTokenW`). Key differences from Unix `fork`+`exec`:

- **Single-call creation**: `CreateProcess` simultaneously creates the new
  process and its primary thread. There is no intermediate state where the
  child shares the parent's address space.
- **No copy-on-write**: Since there is no fork, there is no COW memory
  sharing. The new process starts with its own address space loaded from the
  executable image.
- **Go's `os/exec` mapping**: `exec.Command` on Windows calls `CreateProcess`
  directly. The `syscall.SysProcAttr` struct exposes Windows-specific
  creation flags.

### Implications for Go subprocess management

- **Startup cost**: `CreateProcess` is heavier than `fork`+`exec` on Linux.
  Each `exec.Command().Run()` call in a test pays this cost. Tests that spawn
  many subprocesses will be measurably slower on Windows.
- **No process inheritance tree**: Unlike Unix where children form a tree
  under the parent PID, Windows processes are independent after creation.
  Killing the parent does NOT automatically kill children.
- **Environment block**: `CreateProcess` takes an `lpEnvironment` parameter
  which is a block of null-terminated `KEY=VALUE\0` strings, terminated by
  an extra `\0`. Go's `cmd.Env` builds this block. The block has a maximum
  size of 32,767 characters. `t.Setenv` in tests modifies the process-wide
  environment, which affects all subsequent `CreateProcess` calls.

## Job Objects

Job Objects are the Windows mechanism for grouping processes. They allow
operations on all processes in the group (terminate, set resource limits,
query statistics).

### How Go can use Job Objects

Go's `syscall.SysProcAttr` on Windows supports:

```go
cmd := exec.Command("child.exe")
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
}
```

`CREATE_NEW_PROCESS_GROUP` puts the child in a new process group, which is
necessary for `GenerateConsoleCtrlEvent` to target it specifically. However,
this is NOT the same as a Job Object -- it is a console process group.

For full Job Object control, you need `windows.CreateJobObject` and
`windows.AssignProcessToJobObject` from `golang.org/x/sys/windows`. This is
rarely needed in tests but is important for understanding why killing a
parent does not kill its children.

### Implications for killing process trees

When `exec.CommandContext` kills a process on Windows, it calls
`TerminateProcess` on the child PID only. If that child spawned
grandchildren, they become orphans. This differs from Unix where
`syscall.Kill(-pid, signal)` can signal the entire process group.

**In tests**: If your test starts a subprocess that itself starts children
(e.g., a shell script that launches background processes), killing the
parent via context cancellation will NOT kill the children. They will
continue running until they exit naturally or the test binary timeout kills
the entire process tree (Go's test runner creates a Job Object for itself
since Go 1.17+).

## TerminateProcess Deep Dive

`TerminateProcess` is the only reliable way to kill a process on Windows.

### API signature (conceptual)

```
BOOL TerminateProcess(HANDLE hProcess, UINT uExitCode);
```

### Behavior

- **Immediate**: The process is terminated immediately. No DLL detach
  notifications, no cleanup handlers, no destructors, no `atexit` callbacks.
- **No signals**: There is no signal delivery. The process does not get a
  chance to handle the termination.
- **Exit code**: The `uExitCode` parameter is stored as the process exit
  code. It is a DWORD (unsigned 32-bit integer, range 0-4294967295). When
  Go's `exec.CommandContext` calls `TerminateProcess`, it passes `1` as the
  exit code.
- **Pending I/O**: Outstanding I/O operations may or may not complete.
  Handles to the terminated process remain valid until closed.
- **Thread safety**: Can be called from any thread. Multiple calls are
  harmless (subsequent calls fail with ERROR_ACCESS_DENIED if the process
  has already terminated).

### Common exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic error (also used by TerminateProcess from exec.CommandContext) |
| 259 | `STILL_ACTIVE` -- reserved; never use as an actual exit code |
| 0xC0000005 (-1073741819) | `STATUS_ACCESS_VIOLATION` (NTSTATUS) |
| 0xC000013A (-1073741510) | `STATUS_CONTROL_C_EXIT` (Ctrl+C received) |
| 0xC0000142 (-1073741502) | `STATUS_DLL_INIT_FAILED` |

### Exit code type: DWORD not signal number

On Unix, a process can exit via a signal (SIGTERM, SIGKILL, etc.) and the
exit status encodes the signal number. On Windows, the exit code is always
a DWORD set by the process itself (`ExitProcess(code)`) or by
`TerminateProcess(handle, code)`. There are no signal numbers embedded in
the exit code.

Go's `os.ProcessState` on Windows:
- `ExitCode()` returns the DWORD exit code (or -1 if the process has not
  exited).
- `Sys()` returns `*syscall.WaitStatus` which on Windows only has the exit
  code.
- There is no `Signal()` method that returns a meaningful value on Windows.

## GenerateConsoleCtrlEvent

The closest Windows equivalent to sending a signal to a process.

### API signature (conceptual)

```
BOOL GenerateConsoleCtrlEvent(DWORD dwCtrlEvent, DWORD dwProcessGroupId);
```

### Behavior

- `dwCtrlEvent`: `CTRL_C_EVENT` (0) or `CTRL_BREAK_EVENT` (1).
- `dwProcessGroupId`: The process group to target. Use 0 for the current
  process's console group. Use a specific group ID for a child process
  created with `CREATE_NEW_PROCESS_GROUP`.
- Only works for console applications attached to a console.
- `CTRL_C_EVENT` can be caught by a `SetConsoleCtrlHandler` callback.
  `CTRL_BREAK_EVENT` can also be caught but is harder to suppress.

### Using cmd.Cancel for graceful shutdown

Go 1.20+ added `cmd.Cancel` to `exec.Cmd`. On Windows, you can use this
to send a Ctrl+C event instead of calling `TerminateProcess`:

```go
cmd := exec.CommandContext(ctx, "myapp.exe")
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
}
cmd.Cancel = func() error {
    // Send CTRL_BREAK_EVENT to the child's process group.
    return windows.GenerateConsoleCtrlEvent(
        windows.CTRL_BREAK_EVENT,
        uint32(cmd.Process.Pid),
    )
}
cmd.WaitDelay = 5 * time.Second  // Force kill after 5s if graceful fails
```

Note: `CTRL_C_EVENT` with a non-zero process group ID is unreliable on some
Windows versions. `CTRL_BREAK_EVENT` is more reliable for process group
targeting.

## How Go's exec.CommandContext Works on Windows

When the context is cancelled:

1. Go calls `cmd.Cancel` if set. If not set, it calls
   `cmd.Process.Kill()` which calls `TerminateProcess(pid, 1)`.
2. If `cmd.WaitDelay` is set, Go starts a timer. During this period:
   - The process exit is awaited.
   - I/O goroutines (reading from stdout/stderr pipes) continue draining.
3. If `cmd.WaitDelay` expires and I/O goroutines have not finished, Go
   forcefully closes the pipe handles and returns `exec.ErrWaitDelay`.
4. If `cmd.WaitDelay` is NOT set (zero value), Go waits indefinitely for
   I/O goroutines after the process is killed. If a grandchild process
   inherited the pipe handles and is still running, `cmd.Wait()` hangs
   forever.

**Critical**: Always set `cmd.WaitDelay` when using `exec.CommandContext` on
Windows. Without it, killed processes with inherited pipe handles can cause
indefinite hangs.

## Handle Inheritance

Windows process handles are inherited by child processes when:

1. The handle was created with `bInheritHandle = TRUE`.
2. `CreateProcess` was called with `bInheritHandles = TRUE`.

Go's `os/exec` sets `bInheritHandles = TRUE` for the stdin/stdout/stderr
pipe handles. This means:

- If a child process spawns a grandchild, the grandchild may inherit the
  pipe handles.
- Killing the child leaves the grandchild holding the pipe handles open.
- `cmd.Wait()` blocks until all readers of those pipes close (or
  `cmd.WaitDelay` fires).

**In tests**: Use `cmd.WaitDelay` to prevent hangs. Set it to a reasonable
value (e.g., 10 seconds) that gives legitimate I/O time to complete but
does not allow indefinite blocking.

## The promoteContextError Pattern

### Problem

On Windows, when `exec.CommandContext` kills a process due to context
cancellation:
1. `TerminateProcess(pid, 1)` sets exit code to 1.
2. `cmd.Run()` returns `*exec.ExitError` with code 1.
3. `extractExitCode` sees code 1, records it as a normal non-zero exit,
   and sets `result.Error = nil` (exit code 1 is a valid application exit).
4. The context error (`context.DeadlineExceeded`) is lost.

### Solution

`promoteContextError` in `internal/runtime/native_helpers.go:96-103` runs
after `extractExitCode`. It checks whether:
- `result.Error` is nil (meaning extractExitCode did not detect a hard error)
- `ctx.Context.Err()` is non-nil (meaning the context was cancelled/expired)

If both conditions are true, it promotes the context error to `result.Error`,
ensuring the error handler can classify it as a timeout and show a diagnostic.

```go
func promoteContextError(ctx *ExecutionContext, result *Result) {
    if result.Error != nil {
        return
    }
    if ctxErr := ctx.Context.Err(); ctxErr != nil {
        result.Error = ctxErr
    }
}
```

### Call sites

`promoteContextError` is called in both execution paths of the native runtime:
- `internal/runtime/native.go:218` (Execute)
- `internal/runtime/native.go:263` (ExecuteCapture)

### Why only native runtime

The virtual runtime (mvdan/sh) does not spawn OS processes, so
`TerminateProcess` is not involved. The container runtime handles process
lifecycle through the container engine, which has its own timeout/kill
semantics.

## Process Environment

### lpEnvironment block format

`CreateProcess` accepts an environment block as a single contiguous buffer
of null-terminated strings:

```
KEY1=VALUE1\0KEY2=VALUE2\0\0
```

The block is terminated by an extra null character. Maximum size is 32,767
characters.

### Implications for t.Setenv

`t.Setenv` in Go tests modifies the process-wide environment via
`os.Setenv`. This affects ALL subsequent subprocess launches in the same
test process (including concurrent tests in the same package). This is why
`t.Setenv` is not safe with `t.Parallel()` -- Go's testing framework
panics if you try.

For environment isolation in testscript tests, use `env.Setenv` which only
affects the testscript subprocess environment, not the host test process.
