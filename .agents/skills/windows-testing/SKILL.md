---
name: windows-testing
description: Windows testing guidance for Invowk Go code. Use for Windows process lifecycle, os/exec, NTFS/MAX_PATH/sharing issues, PATH/PATHEXT, timer resolution, race detector overhead, windows-latest CI, proactive path-touching refactors (`filepath.IsAbs`, `filepath.FromSlash`, `filepath.Join`, `filepath.ToSlash`), CUE-fed path value types, volume mounts, and platform-split testscript coverage.
---

# Windows Testing Skill

Use this skill when a change or failure touches Windows-specific behavior in
Go tests, CLI testscript, path handling, process lifecycle, or TUI execution.
This skill routes the work; detailed background lives in `references/`.

## Precedence

1. `.agents/rules/windows.md` - authoritative for path handling, CI config, and PowerShell patterns.
2. `.agents/rules/testing.md` - authoritative for test organization, parallelism, and cross-platform patterns.
3. This skill - Windows triage checklist and repo-specific failure modes.
4. `references/process-lifecycle.md` - CreateProcess, TerminateProcess, Job Objects, `cmd.WaitDelay`, `cmd.Cancel`.
5. `references/filesystem-pitfalls.md` - NTFS, MAX_PATH, sharing violations, reserved names.
6. `references/powershell-testing.md` - PowerShell testscript reference.

Before writing any test code, follow `.agents/skills/testing/SKILL.md` §
"Pre-Write Checklist".

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
3. Add or update the pathmatrix coverage guardrail when a new path-bearing type
   participates in cross-platform semantics.
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

Checklist:

- Reserved names (`CON`, `PRN`, `AUX`, `NUL`, `COM1`-`COM9`, `LPT1`-`LPT9`)
  need Windows-specific handling.
- Use `.bat`, `.cmd`, or `.exe` for executable fixtures; executable permission
  bits alone do not make a file runnable on Windows.
- Use `${:}` and `${/}` in testscript PATH/path construction.
- Do not rely on `filepath.IsAbs("/app")`; Unix-style container paths are not
  absolute Windows host paths.
- Use `path.Join` or forward-slash strings for container paths, not
  `filepath.Join`.
- Convert Windows host paths to slash form before composing Docker/Podman volume
  mount strings.
- Account for NTFS case-insensitivity and 8.3 short-path aliases in string
  assertions. If only identity matters, compare `os.Stat` results with
  `os.SameFile`.

Deep reference: `references/filesystem-pitfalls.md`.

## PowerShell Testscript

PowerShell fixtures must work on Windows PowerShell 5.1 and PowerShell 7+.

Checklist:

- Use `$env:VAR` for environment variables.
- Use `Write-Output` for reliable string output.
- Use `$ErrorActionPreference = 'Stop'`.
- Use `\r?$` in anchored regex assertions because PowerShell emits CRLF.
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
- Do not add TUI `TestMain` pre-warming for lipgloss/bubbletea races. The old
  pre-warm pattern was a no-op; current guidance is to treat future Windows TUI
  race-detector failures as memory-pressure problems unless new evidence shows a
  real data race.

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

## Current CI Shape

Windows CI runs on `windows-latest` with no container engine. The job runs short
unit tests and CLI integration tests as separate steps in `.github/workflows/ci.yml`.

Implications:

- Container runtime tests must skip on Windows.
- Race detector is enabled for the Windows short-test step.
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
| `ERROR_SHARING_VIOLATION` | Defender scan or open handle | Close handles, retry, use `t.TempDir()` |
| Regex line anchor fails | CRLF output | Use `\r?$` |
| `/app` not detected as absolute | Windows host path semantics | Check Unix-style container paths before `filepath.IsAbs` |
| Fixture will not execute | Missing PATHEXT extension | Use `.bat`, `.cmd`, or `.exe` |
| `time.Sleep(1ms)` flaky | 15.6ms timer granularity | Use events or wider tolerance |
| TUI test hangs in CI | ConPTY blocks in headless runner | Skip PTY-dependent path on Windows |
| PATH fixture fails | Literal `:` separator | Use `${:}` in txtar |

## Related Skills

- `.agents/skills/go-testing/` - primary testing workflow.
- `.agents/skills/testing/` - Invowk testscript and repo test patterns.
- `.agents/skills/tmux-testing/` - durable interactive TUI tests.
- `.agents/skills/tui-testing/` - ad hoc VHS visual debugging.
