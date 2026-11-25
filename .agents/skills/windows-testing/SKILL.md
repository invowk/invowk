---
name: windows-testing
description: Invowk Windows testing guidance for Go and testscript code. Use for Windows process lifecycle, os/exec and TerminateProcess, NTFS/MAX_PATH/sharing issues (ERROR_SHARING_VIOLATION, Access is denied), PATH/PATHEXT, USERPROFILE/os.UserHomeDir (SetHomeDir), PowerShell CRLF output, ConPTY hangs, timer resolution, race detector overhead, windows-latest CI, proactive path-touching refactors (`filepath.IsAbs`, `filepath.FromSlash`, `filepath.Join`, `filepath.ToSlash`), CUE-fed path value types, volume mounts, and platform-split testscript coverage.
---

# Windows Testing Skill

Use this skill when a change or failure touches Windows-specific behavior in
Go tests, CLI testscript, path handling, process lifecycle, or TUI execution.
This skill routes the work; detailed background lives in `references/`.

## Precedence

1. `.agents/rules/windows.md` - authoritative for path handling and Windows limits.
2. `.agents/rules/testing.md` - authoritative for test organization, parallelism, cross-platform patterns, and PowerShell rules for native tests.
3. This skill - Windows triage checklist and repo-specific failure modes.
4. `references/process-lifecycle.md` - CreateProcess, TerminateProcess, Job Objects, `cmd.WaitDelay`, `cmd.Cancel`.
5. `references/filesystem-pitfalls.md` - NTFS, MAX_PATH, sharing violations, reserved names.
6. `references/powershell-testing.md` - PowerShell testscript reference.

Before writing any test code, apply the "Pre-Write Checks" in
`.agents/skills/testing/references/pre-write-and-go-patterns.md`.

## First Checks

Run the smallest useful Windows-oriented checks for the touched surface:

```bash
make check-windows-build
go test -run '<target>' ./...
go test -race -short -run '<target>' ./...
```

`make check-windows-build` is compile/vet coverage only. It does not prove
Windows runtime path semantics; path regressions are covered by pathmatrix tests
and goplint/baseline checks.

For CLI/testscript changes, include a Windows-path pass locally where possible
and preserve platform split coverage in `.txtar` fixtures.

## Path-Touching Refactors

When a change touches path parsing, validation, or volume-mount composition:

1. Identify whether the path is a host path, virtual path, container path, or
   CUE-fed value type (`FilesystemPath`, `WorkDir`, `SubdirectoryPath`,
   `ScriptPath`, etc.).
2. Use `internal/testutil/pathmatrix` for validators/resolvers where possible.
3. Register a new path-contract validator or resolver in the
   `pathMatrixCoverage` map in `tests/coverage/pathmatrix_coverage_test.go`
   (or add a justified exemption there); the guardrail fails otherwise. Its AST
   discovery also catches small unexported helpers extracted from an existing
   path function (for example a `filepath.Base`/`strings.HasPrefix` target
   classifier), so register those too.
4. Run focused tests, then `make check-baseline` for goplint path regressions.

## Process Lifecycle

Windows has no POSIX `fork` or Unix signal model. Go `os/exec` uses
`CreateProcess`, and cancellation generally terminates a process with
`TerminateProcess(pid, 1)`.

Checklist:

- Set `cmd.WaitDelay` for commands with captured stdout/stderr.
- Use `cmd.Cancel` only when graceful console interruption is intentionally
  needed.
- Treat exit code `1` after context cancellation as ambiguous until the context
  error has been checked.
- Keep `promoteContextError` in the native runtime after `extractExitCode` so
  Windows timeouts surface as `context.DeadlineExceeded` or `context.Canceled`.
- Watch for orphaned subprocesses; Windows does not kill process trees unless
  they are grouped explicitly, such as with Job Objects or a new process group.

Deep reference: `references/process-lifecycle.md`.

## Filesystem And Paths

Windows filesystem failures often come from path syntax, open handles, or
case-insensitive lookup.

Checklist (full tables: `.agents/rules/windows.md` "Container Path Patterns"
and "Reserved Names and Limits"; that rule is path-scoped and may not be
loaded):

- Container paths: check absoluteness with a leading `/`
  (`isContainerAbsolutePath`), never `filepath.IsAbs` (`/app` is not absolute
  on Windows); join segments with `path.Join`, never `filepath.Join`;
  `filepath.ToSlash` host paths before composing volume mount strings.
- Reserved names (`CON`, `NUL`, `COM1`, ...): check with
  `platform.IsWindowsReservedName`.
- Use `.bat`, `.cmd`, or `.exe` for executable fixtures; executable permission
  bits alone do not make a file runnable on Windows.
- Use `${:}` and `${/}` in testscript PATH/path construction.
- For tests that exercise the process home, `config.CommandsDir`, or global
  module discovery, use `internal/testutil.SetHomeDir(t, dir)`. Direct
  `t.Setenv("HOME", dir)` is Unix-only and leaves Windows `USERPROFILE`
  unchanged. Prefer injecting the home instead: production code reads it only
  through `internal/basedir` (`basedir.New(goos, getenv, userHomeDir)`), so a
  test can pass a resolver without touching the process environment.
- `filepath.IsAbs` follows the host OS: `\Users\me` and `C:Users\me` are
  relative on Windows, and `/home/user` is relative there too. Table tests
  with an injected `goos` must use host-absolute fixtures (`t.TempDir()`
  children); Windows-only path forms belong in `_windows_test.go` files.
- Account for NTFS case-insensitivity and 8.3 short-path aliases in string
  assertions. If only identity matters, compare `os.Stat` results with
  `os.SameFile`.

Deep reference: `references/filesystem-pitfalls.md`.

## PowerShell Testscript

Canonical rules: `.agents/rules/testing.md` "Native tests". In short:
compatible with 5.1 and 7+ (CI runs native txtar under `pwsh` only, so check
5.1 by hand); `$env:VAR`; `Write-Output`, not `echo`;
`$ErrorActionPreference = 'Stop'`; `\r?$` on anchored regex assertions
(CRLF output); identical assertions on every platform.

- Prefer platform-split implementations when Bash and PowerShell behavior would
  otherwise distort the test.

Deep reference: `references/powershell-testing.md`.

## Timing And Race Detector

Windows timer granularity is commonly about 15.6ms, and the race detector has
larger overhead than Linux/macOS.

Checklist:

- Prefer event-based synchronization over `time.Sleep`.
- Use at least a 50ms margin for timing assertions.
- Expect `-race` to amplify memory and scheduling pressure.
- Do not add TUI `TestMain` pre-warming for lipgloss/bubbletea races (history:
  `.agents/memorials/skills-platform-testing-history.md`). Treat Windows TUI
  `-race` failures (timeouts, crashes, `(unknown)` results) as memory or
  scheduling pressure unless evidence shows a real data race.
- A `WARNING: DATA RACE` report with two access stacks is that evidence:
  reproduce it and inspect both stacks
  (`.agents/skills/go-testing/references/race-detector-guide.md`) instead of
  dismissing it as pressure.

## ConPTY And TUI

Windows ConPTY can exist in headless CI, so tests that would fail early on
Linux/macOS can instead block inside `ReadConsoleInput`.

Checklist:

- Skip Windows tests that call interactive adapter paths such as
  `internal/app/commandadapters/interactive_session.go` `runInteractiveCmd`, or
  run a Bubble Tea program against a ConPTY.
- Model-level tests (`model.Update`, `model.View`) are safe.
- `tea.WithContext(ctx)` is still correct production code, but it is not enough
  to make headless ConPTY tests reliable on Windows.
- Durable TUI E2E coverage belongs in `tmux-testing`; tmux itself is not a
  Windows CI assumption.
- Never wrap a standard handle in `os.NewFile` to call a console API: the
  wrapper's finalizer closes the process's real stdin/stdout/stderr handle once
  the GC collects it. Pass `windows.Handle(fd)` directly
  (`internal/tui/interactive_windows.go`).
- `conpty.ConPty.Write` has no guard against a closed handle: a goroutine that
  writes to a ConPTY must stop before the adapter's `Close`
  (`emulatedScreen.close` in `internal/tui/interactive_screen.go`).

## Current CI Shape

Windows CI runs on `windows-latest` with no container engine. The job runs short
unit tests and CLI integration tests as separate steps in `.github/workflows/ci.yml`.

Implications:

- Container runtime tests must skip on Windows.
- `-race` is on in both Windows steps: "Run tests (short)" and "Run CLI
  integration tests".
- CLI integration tests must use platform-portable testscript syntax.
- A Go package timeout can cascade into many `(unknown)` test results; inspect
  the first hung package or stack dump, not only the summary.

## Invowk-Specific Hot Spots

- `internal/runtime/native_helpers.go` - `promoteContextError`.
- `internal/runtime/` - virtual path resolver and bridge path assertions.
- `internal/container/` and `internal/containerplan/` - host/container path
  composition and Linux-only container policy.
- `internal/app/deps/` - dependency path validation.
- `internal/app/commandadapters/interactive_session.go`, `internal/tui/`, and
  `internal/tuiserver/` - TUI/ConPTY-sensitive execution paths.
- `pkg/invowkmod/` - module paths, archives, and lock-file fixtures.
- `tests/cli/testdata/*.txtar` - platform-split CLI fixtures.

## Failure Matrix

| Symptom | Likely Cause | First Fix |
|---|---|---|
| Timeout becomes exit code 1 | `TerminateProcess` masked context error | Check `promoteContextError` call order |
| `ERROR_SHARING_VIOLATION` or `Access is denied` on cleanup | Defender scan or open handle | Close handles, retry, use `t.TempDir()` |
| Regex line anchor fails | CRLF output | Use `\r?$` |
| `/app` not detected as absolute | Windows host path semantics | Check Unix-style container paths before `filepath.IsAbs` |
| Expected `~/.invowk/cmds` missing after setting `HOME` | Windows `os.UserHomeDir()` reads `USERPROFILE` | Use `testutil.SetHomeDir(t, dir)` and keep the test serial |
| Fixture will not execute | Missing PATHEXT extension | Use `.bat`, `.cmd`, or `.exe` |
| `time.Sleep(1ms)` flaky | 15.6ms timer granularity | Use events or wider tolerance |
| TUI test hangs in CI | ConPTY blocks in headless runner | Skip PTY-dependent path on Windows |
| PATH fixture fails | Literal `:` separator | Use `${:}` in txtar |

## Related Skills

- `.agents/skills/go-testing/` - primary testing workflow.
- `.agents/skills/testing/` - Invowk testscript and repo test patterns.
- `.agents/skills/tmux-testing/` - durable interactive TUI tests.
- `.agents/skills/tui-testing/` - ad hoc VHS visual debugging.
