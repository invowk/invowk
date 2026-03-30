---
name: windows-testing
description: >-
  Deep Windows-specific testing knowledge for Go. Covers process lifecycle
  (CreateProcess, no fork, Job Objects, TerminateProcess), signal handling
  differences (no POSIX signals, only os.Interrupt/os.Kill), exec.CommandContext
  Windows behavior (exit code 1 indistinguishable from normal failure),
  file system pitfalls (NTFS case-insensitivity, MAX_PATH, sharing violations,
  Defender scanning), timer resolution (15.6ms default causes timing flakiness),
  race detector overhead, and CI patterns (windows-latest, -timeout 15m).
  Use when debugging Windows-only test failures, writing platform-split tests,
  or understanding why tests flake on windows-latest CI runners.
disable-model-invocation: false
---

# Windows Testing Skill

Deep reference for Windows OS primitives that affect Go test behavior. This
skill complements `.agents/rules/windows.md` (which covers path handling, CI
config, and PowerShell patterns) by providing OS-level knowledge needed to
diagnose and prevent Windows-specific test failures.

## Normative Precedence

1. `.agents/rules/windows.md` -- authoritative for path handling, CI config, PowerShell patterns.
2. `.agents/rules/testing.md` -- authoritative for test organization, parallelism, cross-platform patterns.
3. This skill -- Windows OS primitives, process lifecycle, filesystem behavior, timer resolution.
4. `references/process-lifecycle.md` -- deep dive on CreateProcess, TerminateProcess, Job Objects.
5. `references/filesystem-pitfalls.md` -- deep dive on NTFS, MAX_PATH, sharing violations.
6. `references/powershell-testing.md` -- PowerShell reference for testscript tests.

Do NOT duplicate content from rules; cross-reference instead.

## Process Lifecycle

Windows does not have `fork()`. All process creation goes through the
`CreateProcess` Win32 API, which creates both a process and its initial thread
in a single call. This has several implications for Go test code:

- **No fork-exec**: Go's `os/exec` uses `CreateProcess` directly, not
  `fork` + `exec`. There is no parent-child process tree in the Unix sense.
- **Job Objects**: Windows groups processes using Job Objects. Go's
  `syscall.SysProcAttr` can set `CREATE_NEW_PROCESS_GROUP` to put a child
  process in a new group. Without Job Objects, killing a parent does NOT kill
  its children -- orphan processes are common on Windows.
- **TerminateProcess**: The only reliable way to kill a process on Windows.
  It is immediate, runs no cleanup handlers, and sets the exit code to
  whatever DWORD value the caller passes (typically 1). There is no graceful
  signal equivalent.
- **GenerateConsoleCtrlEvent**: The closest Windows equivalent to sending a
  signal. Can deliver `CTRL_C_EVENT` or `CTRL_BREAK_EVENT` to a process group.
  Only works for console applications in the same console session or a
  separate process group created with `CREATE_NEW_PROCESS_GROUP`.

For the full deep dive on process management, including `cmd.WaitDelay`,
`cmd.Cancel`, and the `promoteContextError` pattern, see
`references/process-lifecycle.md`.

## Signal Handling Differences

Windows has no POSIX signals. Go's `os/signal` package supports only two
signals on Windows:

| Signal | Windows Behavior |
|--------|-----------------|
| `os.Interrupt` | Maps to `CTRL_C_EVENT` via `GenerateConsoleCtrlEvent`. Only works for console processes in the same or designated process group. |
| `os.Kill` | Maps to `TerminateProcess(pid, 1)`. Immediate, no cleanup. |

All other signals (`syscall.SIGTERM`, `syscall.SIGHUP`, etc.) are not
available on Windows. Code that calls `signal.Notify` with Unix-only signals
will compile but never receive those signals.

### exec.CommandContext on Windows

When `exec.CommandContext` detects context cancellation, it kills the process
via `TerminateProcess(pid, 1)`. This sets exit code 1, which is
indistinguishable from a normal non-zero exit. The result:

1. `cmd.ProcessState.ExitCode()` returns `1` (valid exit code).
2. `cmd.Run()` returns an `*exec.ExitError` with code 1.
3. The context error (`context.DeadlineExceeded` or `context.Canceled`) is
   silently dropped.

This is why invowk's `promoteContextError()` exists in
`internal/runtime/native_helpers.go:96-103`. After `extractExitCode` runs, it
checks `ctx.Context.Err()` and promotes the context error when `result.Error`
is nil. Without this, timeouts on Windows produce no diagnostic output.

### cmd.WaitDelay and cmd.Cancel

Go 1.20+ added two fields to `exec.Cmd` that are critical for Windows:

- **`cmd.WaitDelay`**: After the process exits, Go waits this duration for
  I/O goroutines (reading stdout/stderr) to finish. Without it, a killed
  process with open pipes can cause `cmd.Wait()` to hang indefinitely.
  Always set this in production and test code.
- **`cmd.Cancel`**: Custom function called instead of `TerminateProcess` when
  the context is cancelled. Can be used to send `CTRL_C_EVENT` via
  `GenerateConsoleCtrlEvent` for graceful shutdown instead of hard kill.

## File System Decision Tree

When a test fails on Windows with a file system error, use this decision tree:

```
Is the error about file names?
  YES --> Is it a reserved name (CON, PRN, AUX, NUL, COM1-9, LPT1-9)?
    YES --> Use platform.IsReservedWindowsName() to detect and skip/adapt.
    NO  --> Does it contain illegal characters (< > : " | ? *)?
      YES --> Sanitize or skip the test case on Windows.
      NO  --> Is the path > 260 characters?
        YES --> Enable long path support or shorten the path.
        NO  --> Continue to next check.

Is the error about file access?
  YES --> Is it ERROR_SHARING_VIOLATION (0x80070020)?
    YES --> Another process has the file open (antivirus, indexer, or test
            subprocess). Add retry logic or use t.TempDir() for cleanup.
    NO  --> Is it ERROR_ACCESS_DENIED?
      YES --> Check Windows Defender scanning delay on new temp files.
             Or check if the file is read-only / still open.
      NO  --> Is it about case-sensitivity?
        YES --> NTFS is case-preserving but case-insensitive.
               Creating "Foo.txt" and "foo.txt" references the same file.
        NO  --> Check references/filesystem-pitfalls.md for more.
```

For the full filesystem reference, see `references/filesystem-pitfalls.md`.

## Path Handling

Path handling (host vs container paths, `filepath.ToSlash`, `filepath.IsAbs`
behavior) is authoritatively covered in `.agents/rules/windows.md` -- Path
Handling. Key points:

- `filepath.IsAbs("/app")` returns FALSE on Windows.
- Container paths always use forward slashes; use `path.Join()` or
  `filepath.ToSlash()`.
- The `skipOnWindows` table-driven pattern handles Unix-only path test cases.

Do not duplicate the path handling rules here. Consult `windows.md` directly.

## Line Endings

PowerShell's `Write-Output` (and most Windows command-line tools) emit CRLF
(`\r\n`) line endings. This affects testscript assertions:

- Go's `(?m)$` in regex matches before `\n`, NOT before `\r`.
- `stdout '^done$'` will fail for PowerShell output because the actual line is
  `done\r\n` and `$` anchors at the `\n`.
- **Fix**: Use `\r?$` in anchored regex assertions: `stdout '^done\r?$'`.
- Non-anchored substring matching (`stdout 'done'`) is not affected because
  testscript normalizes for substring matching.

## PATHEXT and Executable Resolution

Windows determines whether a file is executable by its extension, not by
permission bits. The `PATHEXT` environment variable lists executable
extensions (default: `.COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC;.PS1`).

- `exec.LookPath` on Windows appends each PATHEXT extension when searching.
- `pkg/platform`'s `IsExecutable()` checks file extension against PATHEXT,
  NOT Unix permission bits.
- Tests that create executable files on Windows must use `.bat` or `.exe`
  extensions. A file with `0755` permissions but no recognized extension is
  NOT executable on Windows.

## Timer Resolution

Windows has a default timer resolution of 15.625ms (1/64 second). This is the
granularity of `time.Sleep`, `time.After`, `time.Tick`, and all
time-dependent operations.

| Platform | Default Timer Resolution | Notes |
|----------|------------------------|-------|
| Linux | ~1ms (high-resolution timers) | `CONFIG_HIGH_RES_TIMERS` enabled by default |
| macOS | ~1ms (but with timer coalescing) | Power management may delay wakeups |
| Windows | 15.625ms | Can be reduced to ~1ms via `timeBeginPeriod(1)` but Go does not call this |

**Impact on tests**:
- `time.Sleep(1 * time.Millisecond)` actually sleeps ~16ms on Windows.
- Tests that assert "operation completed in < 10ms" will fail on Windows.
- Timing-based uniqueness (e.g., "two events 1ms apart have different
  timestamps") can collide on Windows.

**Mitigation**: Use generous tolerances in timing assertions (at least 50ms
margin on Windows). Prefer event-based synchronization (channels, sync
primitives) over `time.Sleep` in tests.

## Temp Directory

Windows temp directory behavior differs from Unix in ways that affect tests:

- `os.TempDir()` returns `%TEMP%` or `%TMP%` (typically
  `C:\Users\<user>\AppData\Local\Temp`).
- **Windows Defender real-time scanning**: New files in temp directories
  trigger Defender scans. This can cause:
  - `ACCESS_DENIED` errors when trying to open a file that Defender is still
    scanning.
  - Noticeable delays (50-200ms) on first file access.
  - `ERROR_SHARING_VIOLATION` if the test opens a file while Defender has it
    locked.
- **`t.TempDir()` cleanup failures**: If a test subprocess or Defender still
  has a file handle open, `os.RemoveAll` in `t.TempDir()` cleanup will fail
  with `ERROR_SHARING_VIOLATION`. The Go testing framework logs this as a
  test error.
- **Path length**: Deep nesting in temp directories can exceed MAX_PATH
  (260 chars). `t.TempDir()` names include the test name, which can be long
  for table-driven subtests.

**Mitigation**: Use `t.TempDir()` (lifecycle-managed) instead of
`os.MkdirTemp` + `defer os.RemoveAll`. Add small retry delays when
`ERROR_SHARING_VIOLATION` errors occur. Keep temp directory paths short.

## Race Detector on Windows

The Go race detector has higher overhead on Windows than on Linux. The
combination of `-race` + many parallel tests + Windows can cause timeouts
that do not occur on other platforms.

**Overhead factors**:
- Race instrumentation adds ~5-10x CPU overhead per goroutine operation.
- Windows thread creation/scheduling is heavier than Linux (no `clone(2)`).
- `CreateProcess` startup time adds latency to every subprocess-spawning test.

### Charm Library Thread Safety (lipgloss, bubbletea, termenv)

Source-level analysis of the Charm library stack reveals that the TUI test
rendering path is fully thread-safe. The `GOMAXPROCS(1)` restriction in
`internal/tui/testmain_windows_race_test.go` exists solely for memory
pressure under the race detector, NOT for thread safety.

**Library-by-library findings** (verified against actual source):

| Library | Version | Thread Safety | Key Evidence |
|---------|---------|---------------|-------------|
| lipgloss v2 | v2.0.2 | `Style` is a pure value type. `Render()` uses only local variables — zero global state access. | `style.go:142-195`: struct fields are all values (ints, colors, rune). No `sync` primitives in core. |
| bubbletea v2 | latest | All sync primitives on `Program`/renderer, not `Model`/`View`. Tests never run a `Program`. | `tea.go:76-80`: `NewView()` is a plain struct constructor. |
| colorprofile | v0.4.3 | Global color conversion cache uses `sync.RWMutex` — properly synchronized. | `profile.go:49-55` |
| termenv | v0.16.0 | `sync.Once` guards for color caching in `Output`. `go-isatty` resolves `LazyDLL` at package init. | `output.go:32-35`, `isatty_windows.go:29-33` |

**Terminal detection lifecycle**: lipgloss v2 runs terminal detection exactly
once, at package init time (`var Writer = colorprofile.NewWriter(os.Stdout,
os.Environ())`), before any test executes. `NewStyle().Render()` does NOT
trigger terminal detection — it is purely computational. The old "pre-warm"
pattern (`_ = lipgloss.NewStyle().Render("")`) was a no-op that hit the
`props == 0` fast path without touching any Console API.

**Proof from CI**: Linux and macOS run 396 TUI tests with `-race` AND full
parallelism (`t.Parallel()`) — same rendering code path, no failures. This
confirms the rendering path has no data races.

**Current approach**: No `TestMain` or `GOMAXPROCS` restriction exists for
Windows TUI tests. All TUI tests run with `-race` and full parallelism on
`windows-latest`. If memory pressure under the race detector becomes a
problem in the future, a build-tagged `TestMain` with `GOMAXPROCS(1)` can
be reintroduced (the rendering path is confirmed thread-safe; the
restriction would be purely a memory budget workaround).

## ConPTY (Windows Pseudo-Console)

Windows ConPTY (`CreatePseudoConsole`) is a kernel-level pseudo-terminal API
available since Windows 10 1809. Unlike Unix PTYs, ConPTY is **always available
in headless environments** (CI runners, SSH sessions, containers). This creates
a platform-specific divergence that is invisible during local development.

### The Headless CI Problem

On Linux/macOS CI (headless), PTY creation via `xpty.NewPty()` **fails
immediately** because there is no terminal device. Code that calls
`tea.NewProgram(m).Run()` after PTY creation never executes — the function
returns early with a PTY error.

On Windows CI (headless), `xpty.NewPty()` calls `NewConPty()` which uses the
ConPTY API. This **succeeds unconditionally**, even without a physical terminal.
The PTY is backed by `CONIN$`/`CONOUT$` console handles. After PTY creation,
`tea.NewProgram(m).Run()` enters its event loop and calls `ReadConsoleInput` on
the ConPTY input handle.

`ReadConsoleInput` is a **blocking Windows kernel syscall** that does not
reliably unblock when:
- The console handle is closed from another goroutine
- A context is cancelled via `tea.WithContext(ctx)`
- The Go runtime attempts to interrupt it

This means any Bubble Tea program connected to a ConPTY in headless CI will
**block indefinitely** waiting for keyboard input that never arrives.

### Why `tea.WithContext(ctx)` Doesn't Help on Windows

`tea.WithContext(ctx)` works by sending a kill signal through the message
channel when the context expires. On Windows with ConPTY, this is insufficient
for **both** context timing scenarios:

- **Pre-cancelled context**: **NOT reliably immediate.** There is a race between
  Bubble Tea checking `ctx.Done()` in its startup path and the ConPTY subsystem
  spawning the `ReadConsoleInput` goroutine. If ConPTY init wins the race, the
  goroutine enters the blocking kernel syscall before the cancellation is
  processed. This is non-deterministic — the same test can pass in one CI run
  and hang in the next.
- **Active context that expires later**: Does NOT work. The ConPTY input reader
  goroutine is already blocked in `ReadConsoleInput`. The kill signal is sent to
  the message channel, but the main event loop may be waiting for the input
  reader goroutine to yield. The program hangs until the Go test binary timeout
  kills the entire process.

### Impact on Test Suites

When a test calling `RunInteractiveCmd()` or `tea.Program.Run()` hangs on
Windows ConPTY, the Go `-timeout` flag eventually kills the test binary. This
produces a **timeout cascade**: every test in the package that hadn't completed
is reported as `(unknown)` by gotestsum. A single hanging test can mask 400+
test results.

### Mitigation

Skip **ALL** tests on Windows that call `RunInteractiveCmd()` or start a
`tea.Program` connected to a ConPTY — regardless of context state:

```go
if goruntime.GOOS == "windows" {
    t.Skip("skipping: ConPTY blocks on CONIN$ in headless CI; neither WithContext nor pre-cancellation reliably prevents the hang")
}
```

**Safe on Windows** (no skip needed):
- Tests that exercise the model layer directly (`model.Update()`, `model.View()`)
- Tests that use `NewInteractive().Command(cmd)` without calling `.Run()`
- Tests that call `RunInteractiveCmd` indirectly via mocks

**Unsafe on Windows** (must skip):
- Any test that calls `RunInteractiveCmd()` directly
- Any test that creates a `tea.NewProgram` with ConPTY I/O
- Even with `tea.WithContext(ctx)` and a pre-cancelled context

The production code should still use `tea.WithContext(ctx)` — it is the correct
pattern for real terminals and handles non-ConPTY cancellation on all platforms.

## Named Pipes

Windows uses named pipes (`\\.\pipe\<name>`) instead of Unix domain sockets
for local IPC. Key differences:

- Created via `CreateNamedPipe` Win32 API, not the filesystem.
- Path format: `\\.\pipe\<name>` (the `\\.\pipe\` prefix is mandatory).
- Go's `net.Listen("unix", path)` does NOT work on Windows (prior to
  Windows 10 1803 with AF_UNIX sockets). Use `net.Listen("tcp", "127.0.0.1:0")`
  as a portable alternative for tests.
- Named pipes support overlapped I/O but have different semantics for
  connection lifecycle (ConnectNamedPipe/DisconnectNamedPipe).

**For invowk**: The SSH server and TUI server use TCP listeners, which work
identically on all platforms. Named pipes are not used in the codebase.

## CI Configuration

The invowk CI pipeline runs Windows tests on `windows-latest` with specific
accommodations:

```yaml
# From .github/workflows/ci.yml
- os: windows
  runner: windows-latest
  engine: ""         # No container engine on Windows CI
  test-mode: short   # Unit tests only
```

**Key CI decisions**:

1. **Short mode only**: Windows CI runs `-short`, skipping container
   integration tests (container runtime is Linux-only by design).
2. **Single unified test step**: All packages (including `internal/tui`) run
   in one `gotestsum --packages ./... -- -race -short -v` invocation with
   Go's default 10-minute per-binary timeout.
3. **No container engine**: The `engine: ""` matrix value means no Docker or
   Podman is available. Container tests auto-skip via `[!container-available]`
   guards in testscript.
4. **Race detector**: Enabled for all packages including TUI. TUI is also
   race-checked on Linux (full mode) and macOS (short mode).
5. **Job timeout**: `timeout-minutes: 30` at the job level provides the
   safety net if the Go-level timeout fails to fire.
6. **ConPTY test skips**: Tests that exercise the `xpty.NewPty()` →
   `tea.Program.Run()` path must be skipped on Windows. ConPTY always
   succeeds in headless CI, and the Bubble Tea event loop blocks
   indefinitely on `ReadConsoleInput`. A single hanging test cascades
   to all remaining TUI tests as `(unknown)`. See the ConPTY section above.

## PowerShell Testing

PowerShell testing in testscript files is covered by two references:

- `.agents/rules/windows.md` -- PowerShell Script Testing in Testscript
  (authoritative for CI patterns, version compatibility, common pitfalls).
- `references/powershell-testing.md` -- Complete PowerShell reference
  (operators, error handling, Bash translation table, invowk patterns).

Key points (see the references for full details):
- Scripts must be compatible with both PowerShell 5.1 and 7+.
- Use `$env:VAR` for environment variables, not `$VAR`.
- Use `Write-Output` instead of `echo` for reliable string output.
- CRLF line endings from PowerShell require `\r?$` in regex assertions.
- `$ErrorActionPreference = 'Stop'` is the PowerShell equivalent of `set -e`.

## Invowk-Specific Patterns

### promoteContextError

Location: `internal/runtime/native_helpers.go:96-103`

Surfaces context deadline/cancellation errors that Windows's TerminateProcess
silently masks. Called after `extractExitCode` in both `Execute` and
`ExecuteCapture` paths of the native runtime. See the Signal Handling section
above for the full explanation.

### skipOnWindows Table-Driven Pattern

Used in `internal/runtime/runtime_native_test.go` and
`internal/runtime/runtime_virtual_test.go` for test cases with Unix-style
absolute paths that are semantically meaningless on Windows. The field and
its usage are documented in `.agents/rules/testing.md` -- Cross-Platform
Testing.

### Config Directory Isolation

In `tests/cli/cmd_test.go`, the `commonSetup` function sets
`APPDATA` and `USERPROFILE` on Windows to test-scoped paths:

```go
if runtime.GOOS == platform.Windows {
    env.Setenv("APPDATA", filepath.Join(env.WorkDir, "appdata"))
    env.Setenv("USERPROFILE", env.WorkDir)
}
```

Without this, all Windows tests share the real `%APPDATA%\invowk` config
directory, causing cross-test contamination (one test's `config init` affects
another).

### ConPTY Test Skips

The `internal/tui/interactive_exec_test.go` file skips
`TestRunInteractiveCmd_PTYCreationFailure` on Windows because ConPTY always
succeeds in headless CI and `tea.Program.Run()` blocks on `ReadConsoleInput`.
See the ConPTY section above for the full explanation.

## Common Windows Test Failure Matrix

When a test fails only on Windows, use this matrix to diagnose the cause:

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Exit code 1 but no error reported | `TerminateProcess` masked context timeout | Check if `promoteContextError` is called after `extractExitCode` |
| `ERROR_SHARING_VIOLATION` on temp files | Windows Defender scanning or open file handle | Use `t.TempDir()`, add retry logic, close handles before cleanup |
| `ACCESS_DENIED` on newly created file | Defender real-time scan locking the file | Add small delay or retry; avoid immediate open-after-create |
| Test timeout on CI but passes locally | Race detector overhead (5-10x) + 15.6ms timer | Increase `-timeout`, check `GOMAXPROCS`, use `-short` |
| Bubble Tea test hangs, 400+ `(unknown)` cascade | ConPTY `ReadConsoleInput` blocks in headless CI | Skip PTY-dependent tests on Windows (see ConPTY section) |
| lipgloss race detector crash | Concurrent Console API access | Add `TestMain` pre-warm pattern |
| Regex assertion fails for line match | CRLF line endings from PowerShell | Use `\r?$` instead of `$` in anchored assertions |
| `filepath.IsAbs("/app")` returns false | `/app` is not absolute on Windows | Use `t.TempDir()` + `filepath.Join()` for host paths |
| File with `0755` perms not executable | Windows uses PATHEXT, not permission bits | Use `.bat`/`.exe` extension for executable test files |
| `t.TempDir()` cleanup failure | Subprocess still holds file handle | Ensure subprocesses are killed before test returns; set `cmd.WaitDelay` |
| `time.Sleep(1ms)` takes 16ms | 15.6ms default timer resolution | Use event-based sync; add >= 50ms tolerance |
| Config contamination across tests | Missing `APPDATA`/`USERPROFILE` isolation | Set both to test-scoped paths in testscript `Setup` |
| Test subprocess orphaned after kill | No Job Object grouping | Use `CREATE_NEW_PROCESS_GROUP` in `SysProcAttr` |
| Named pipe `net.Listen("unix",...)` fails | AF_UNIX not available on older Windows | Use `net.Listen("tcp", "127.0.0.1:0")` |
| SHA hash mismatch in txtar fixtures | Git `core.autocrlf` converting `\n` to `\r\n` | Ensure `.gitattributes` has `*.txtar text eol=lf` |

## Related Skills

- `.agents/skills/go-testing/` -- Primary testing skill; routes to this skill for Windows-specific issues.
- `.agents/skills/testing/` -- Invowk-specific test patterns, testscript, TUI/container testing.
- `.agents/skills/tmux-testing/` -- tmux-based TUI testing (Linux-only).
- `.agents/skills/tui-testing/` -- VHS-based TUI testing workflow.
- `.agents/rules/windows.md` -- Path handling, CI config, PowerShell (authoritative).
- `.agents/rules/testing.md` -- Test organization, parallelism, cross-platform patterns (authoritative).
