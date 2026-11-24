# Design

## Context

See proposal.md, "Why". The current code, at base e14950aa:

- **The spin adapter.** `runTuiSpinWithRunner` (`cmd/invowk/tui_spin.go:79-132`):
  - It stores the command's result in a captured variable, `runResult`, which the action closure writes (`tui_spin.go:105-111`).
  - It returns a spinner error before it prints anything (`tui_spin.go:112-114`), so the command's output and exit status are lost.
  - It forwards `runResult.ExitCode` unchanged as `&ExitError{Code: …}` (`tui_spin.go:125-129`).
- **The spin runner.** `execTUISpinRunner.Run` uses `exec.CommandContext` with `WaitDelay` 10 s (`tui_spin.go:22`, `139-150`). A command killed by a signal yields `ExitCode() == -1`, which becomes `ExitCode: -1` (`tui_spin.go:143-147`).
- **`spinWithAction`** (`internal/tui/spin.go:312-333`):
  - On `ErrNoTerminal` it runs the action synchronously.
  - Otherwise it starts the action in a goroutine and runs the spinner. It waits on `doneCh` only when the spinner returns an error (`spin.go:328-331`).
  - The action takes no context (`spin.go:299`).
- **`spinActionModel.Update`** returns `tea.Quit` on `ctrl+c` (`spin.go:481-484`). `Program.Run` then returns nil (`bubbletea/v2@v2.1.1/tea.go:792-794`), so `spinWithAction` returns while the action still runs. That return is the N14 race.
- **Bubble Tea's signal handler.** Bubble Tea installs its own SIGINT/SIGTERM handler (`tea.go:665-705`). While the spinner runs, SIGINT turns into `InterruptMsg` and makes `Run` return `ErrInterrupted` (`tea.go:796-797`). SIGTERM turns into `QuitMsg` and makes `Run` return nil.
- **`Pager`** (`internal/tui/pager.go:166-186`) always resolves the terminal first. It writes the content to stdout only on `ErrNoTerminal`, so `pager | cat` opens the pager on `/dev/tty`.
- **`runTuiPager`** (`cmd/invowk/tui_pager.go:53-91`) delegates to the TUI server before it renders anything directly. It never checks stdout.
- **`Table`** (`internal/tui/table.go:151-176`) always runs a Bubble Tea program through `runComponentProgram`. There is no static render path.
- **`runTuiTable`** (`cmd/invowk/tui_table.go:85-109`) treats the non-selectable table like the selectable one; the only difference is that it prints no selection (`tui_table.go:105`). The server request carries `Print: !cfg.selectable` (`tui_table.go:239`), which the `-i` viewer maps to a non-selectable table (`internal/app/commandadapters/interactive_protocol.go:302`). That is not a static render either.
- **The terminal check.** `isTerminalFile` (`internal/tui/terminal.go:165-167`) is the `x/term` check the resolver uses.

### What a native `invowk cmd` does on Ctrl+C (brief item N14.1)

- **Probe, 2026-11-24, Fedora 45.** A native command with script `echo start; sleep 30` was started under `setsid`, and SIGINT was sent to its process group, which is what a terminal's Ctrl+C does. invowk exited **1**. stdout was `start`, and stderr held `Invalid exit code -1 (must be in range 0-255).`
- **The code path.**
  - The script dies of SIGINT, so `ExitCode()` is -1. `extractExitCode` rejects -1, sets `ExitCode = 1` and stores the validation error (`internal/runtime/native_helpers.go:73-81`).
  - `dispatchExecution` turns `result.Error` into a classified execution error (`internal/app/commandsvc/dispatch.go:79-81`).
  - `exitCodeForError` maps every error that is not an `ExitError` or a `UsageError` to 1 (`cmd/invowk/usage_error.go:175-186`).
  - On Windows, `promoteContextError` turns a cancelled context into the error (`native_helpers.go:91-103`). `classifyExecutionError` classifies it as cancelled (`internal/app/commandsvc/errors.go:50-51`), and the exit status is again 1.
- **The spec.** `openspec/specs/cli-exit-codes/spec.md:280`, "Non-usage failures exit with status 1", names "command timeout or cancellation" as status 1. The project defines no 130.
- **Conclusion.** A cancelled spin exits **1**, not 130.
- **Probe of the same date, `invowk tui spin` without a terminal.** `invowk tui spin -- sh -c 'echo start; sleep 30'` got SIGINT to its group under `setsid` and exited **255**, with `Exit status -1.`: the -1 from `tui_spin.go:146` reaches `os.Exit(-1)` through `root.go:109`. Native exits 1 in the same situation, so this change maps it to 1 (D3).

## Goals / Non-Goals

**Goals:**
- `SpinWithAction` never returns while the action still runs, except after the bounded wait, and then the adapter does not read the result.
- One rule for `pager` and the non-selectable `table` when stdout is not a terminal, applied before the server delegation.

**Non-Goals:**
- Termination signals in general (N1/S2): no new `signal.Notify`, no forwarding to the child, and no process groups. A cancelled command's grandchildren can outlive it, as they do after a native cancellation. See Open questions.
- The `-i` viewer (`internal/tui/interactive*.go`, `internal/app/commandadapters/interactive_session.go`): another change owns it.
- The message `Invalid exit code -1` that native prints on Ctrl+C. It is pre-existing, and native runtime code is out of scope.
- Value components and `table --selectable`: unchanged.

## Decisions

### D1. Status 1 on cancellation, matching native

A spin that the user cancels exits 1, with an error that wraps `context.Canceled` (for example `spin: command cancelled`). That error reaches `exitCodeForError` as an ordinary error, which gives 1. The status is 1 even when the command happened to finish during the bounded wait: the user asked to stop it, and native also reports 1 whenever the context ends first.
- *Alternative: 130 (128+SIGINT).* Rejected. The project defines no such code, a native `invowk cmd` gives 1, and the brief forbids inventing one.
- *Alternative: forward the killed command's status.* Rejected. `CommandContext` kills the command, so the status is -1 on Unix and 1 on Windows: it is not meaningful.

### D2. `SpinWithAction(ctx, opts, action func(context.Context))`, with a bounded cancel wait

- `SpinWithAction` derives `actionCtx, cancel := context.WithCancel(ctx)` and passes `actionCtx` to the action. Without a terminal, it runs `action(actionCtx)` synchronously, as today.
- With a terminal, the action runs in a goroutine that closes `doneCh` when it returns. The spinner returns in one of three ways:
  1. **`doneCh` closed:** normal completion; return nil.
  2. **The spinner returned before `doneCh` closed, and its error is nil or `tea.ErrInterrupted`:** the user's Ctrl+C, a SIGINT, or a Bubble Tea quit from its SIGTERM handler. `tea.ErrProgramKilled` (a panic in the program, since no external context is passed) is a drawing failure and falls under rule 3. Call `cancel()`, then wait on `doneCh` for at most `SpinCancelWait` (15 s).
     - If the action ended: return `ErrSpinCancelled`, which wraps `context.Canceled`.
     - If not: return `ErrSpinActionStillRunning`, and the goroutine is abandoned to process exit.
  3. **The spinner returned any other error:** for example the terminal could not enter raw mode. That is a drawing failure, not a cancellation, so the command keeps running and `SpinWithAction` waits on `doneCh` without a bound, as today (`spin.go:328-331`). Then it returns the spinner error.
- `spinActionModel` returns `tea.Interrupt` instead of `tea.Quit` on `ctrl+c`, so the key and SIGINT take the same path. Rule 2 still treats a nil return before `doneCh` as a cancellation, so every early return of the spinner is covered, whatever its cause.
- **The bound.** `SpinCancelWait` is an exported constant, and the adapter's `tuiSpinCommandWaitDelay` (10 s) must stay below it. A unit test asserts `tuiSpinCommandWaitDelay < tui.SpinCancelWait`. `exec.Cmd.WaitDelay` already bounds `Wait` after the kill: a grandchild that holds the output pipe is cut off after 10 s. So the 15 s bound is reached only when the process does not die at all.
- *Alternative: keep `func()` and let the adapter cancel its own context from outside.* Rejected. The adapter cannot know the spinner quit before `SpinWithAction` returns, and by then the race has happened.
- *Alternative: an unbounded wait after cancel.* Rejected by the brief ("bounded"). The bound is also a safety net if the kill does not take effect.
- `SpinBuilder.Action(func())` keeps its signature: `Run` wraps the function as `func(context.Context) { action() }`, and its context comes from `SpinBuilder.Context` or `context.Background()`. A builder action that ignores the context can reach the bound; the doc comment says so.

### D3. The adapter receives the result through a channel and maps every outcome

- The action sends `runner.Run(actionCtx, …)` into a buffered channel (capacity 1). After `SpinWithAction` returns, the adapter reads the channel without blocking.
- On every path where the action ended, the send happened before `close(doneCh)`, and `SpinWithAction` observed `doneCh`, so the result is there. On the `ErrSpinActionStillRunning` path the adapter does not read at all. Channel operations are race-free, so N14.3 holds by construction.
- The outcome table:

| `SpinWithAction` result | Result read | Output printed | Exit status | stderr |
|---|---|---|---|---|
| nil | yes | yes | the command's status (D3 mapping) | command error, if any |
| `ErrSpinCancelled` | yes | yes | 1 | `spin: command cancelled` |
| `ErrSpinActionStillRunning` | no | no | 1 | the command did not stop within 15 s and may still be running |
| other error, action ran | yes | yes | the command's status | the spinner error, as a warning line |
| resolve error, action not run | no | no | 1 | the resolve error (today's behavior) |

- **The exit mapping.**
  - A status in 1-255 is forwarded (`ExitError`).
  - 0 means success.
  - A status outside 0-255, which is -1 for a signal, becomes an error with exit status 1: `spin: command ended by a signal`. This mirrors `extractExitCode`.
  - If the cmd context was cancelled (a SIGINT through fang, `root.go:107`) and the command failed, the error wraps `context.Canceled`, and the status is again 1.
- **Telling "action ran" from "action not run".** Before this change the resolve error and the spinner error looked the same. The channel tells them apart: it is empty exactly when the action never ran or is still running, and the returned error says which.

### D4. The stdout check is done in the adapter, before the server delegation

- A helper, `stdoutIsTerminal(w io.Writer) bool`, is true only when `w` is an `*os.File` that `term.IsTerminal` accepts. An injected writer, such as a test buffer, counts as not a terminal.
- `runTuiPager` and `runTuiTable` run this check after they read their input and before `tuiclient.NewClientFromEnv()`. So "not a terminal" wins over both the TUI server and direct rendering, and the terminal resolver is never called.
- `tui.Pager` keeps its `ErrNoTerminal` fallback, for library callers and for the case where stdout is a terminal but no terminal can be opened.
- *Alternative: put the check inside `tui.Pager` and `tui.Table`.* Rejected for the server path. The adapter delegates to the server before it calls into `internal/tui`, and the rule must also hold under a server (the proposal's single rule).
- *Alternative: let the server render, and only skip direct rendering.* Rejected. `tui pager file | grep` inside an `-i` command would still show a modal and print nothing, which is the N15 defect.

### D5. The plain-text table writer lives in `internal/tui`

- `WriteTableText(w io.Writer, headers []string, rows [][]string) error` implements the format in the spec ("Plain-text table format"). Widths are measured with `lipgloss.Width`, so no new dependency is needed. Line ends are `\n` on every OS, like the rest of the tui output.
- The adapter calls it with the `headers` and `rows` that `splitHeadersAndData` already gives (`tui_table.go:210-216`).
- The format is the smallest that keeps columns aligned and loses no data in a pipe: no truncation, no `--height` window, no border. It is in the proposal for the maintainer to see.
- *Alternative: reuse the bubbles table `View()` with a borderless style.* Rejected. It truncates to the column widths and the height, and it emits styling sequences.
- *Alternative: write rows joined by the input separator, a CSV echo.* Rejected. The brief asks for aligned rows.

### D6. Tests

- **N14.3 race test, red first.**
  - In `internal/tui`, `TestSpinWithAction_CancelWaitsForAction` injects a resolver that returns a dummy terminal and a spinner that returns nil at once.
  - The action, which honors its context, writes a shared variable after it sees cancellation. The test reads that variable after `spinWithAction` returns.
  - The test is first written against the current `func()` signature, with an action that sleeps 50 ms and then writes. Run with `go test -race`, it must fail on the unfixed code: either a race report or the action not finished. The failing run is saved as a log.
  - Then the fix goes in, and the test is adapted to the context signature.
- **The other `internal/tui` units.** The bounded-wait path (a fake action that ignores cancellation, with a short bound injected through the internal function), and `ErrInterrupted` treated like nil.
- **`cmd/invowk` units.** One test per row of the D3 table, with a fake `tuiSpinAction`, plus the mapping of -1 and the `WaitDelay < SpinCancelWait` assertion. For pager and table: an injected non-terminal writer gets the content, and with `INVOWK_TUI_ADDR` pointing at a test server, no request reaches it (extending `startTableTestServer`). Plus `WriteTableText` golden cases taken from the spec scenarios.
- **testscript, all three OSes.** In `tests/cli/testdata/tui_no_terminal.txtar` (notty): the non-selectable table prints aligned rows and exits 0 (today it exits 1); the pager with `--title` and `--line-numbers` still passes the bytes through.
- **tmux, Linux and macOS.** In `tests/cli/tui_tmux_test.go`:
  - `pager file > out; echo INVOWK_EXIT:$?` and `table --file f | cat` finish with no key press, and the files hold the expected bytes. On the base these hang until the wait times out, and that failure is the calibration.
  - Ctrl+C during `spin -- sh -c 'echo $$ > pid; echo started; exec sleep 30'` gives `INVOWK_EXIT:1` within the timeout, the pane shows `started`, and `kill -0 $(cat pid)` fails afterwards.
  - `TestTUIExemptionTmuxCoverage` already finds `tui spin`, `tui pager` and `tui table` (`tests/cli/tui_tmux_test.go:379-470`), and the new subtests keep them covered.
- **Unix-only Go test in `tests/cli`.** It starts `tui spin` with no terminal under a new session, sends SIGINT to the group, and asserts exit 1 and stdout `started`. On the base it exits 255. Windows has no portable SIGINT, so its cancellation path is covered by the unit tests.

## Risks / Trade-offs

- [Grandchildren survive a cancel, because only the direct child is killed] → The same is true of a native cancellation. The fix belongs with N1/S2's signal forwarding. Tests use `exec sleep` so the direct child is the sleeper, and the spec scenario is worded the same way.
- [A Ctrl+C can take up to `WaitDelay` (10 s) when a grandchild holds the output pipe] → This is the bound the brief asks for. stderr says the command was cancelled once it ends.
- [Bubble Tea's SIGTERM `QuitMsg` now cancels the command while a spinner is shown] → This is a consequence of "every early spinner return is a cancellation" (D2 rule 2), which the race fix needs. Without a spinner, invowk still does not notify SIGTERM (N1, unchanged).
- [`stdoutIsTerminal` on Windows treats a mintty or MSYS pipe as not a terminal, so `pager` passes through there] → That is the correct reading of a pipe. A Windows console stdout is a console, and the pager draws as before.
- [Scripts that relied on `pager > file` showing a pager, or on a cancelled spin exiting 0] → Listed in the proposal's migration notes as BREAKING.
- [The corner case where stdout is a terminal, no controlling terminal opens, and stdin and stderr are not terminals] → The non-selectable `table` still exits 1 there, and the pager still writes to stdout. That combination is not part of N15. See Open questions.

## Migration Plan

Ship it in the next release with the proposal's migration notes. No data or config migration is needed. To roll back, revert the commits; nothing is persisted.

## Maintainer decisions

- **2026-11-24, group C brief** (`program-notes/briefs/c-brief.md`, binding):
  - On Ctrl+C, spin uses the native status found in the code and the exit-code spec (D1: 1).
  - Output and exit status are kept whenever the action ran.
  - The race is proven by a `-race` test that fails before the fix.
  - N14 stays standalone: no N1/S2 work.
- **2026-11-24, same brief:** one rule for display-only components whose stdout is not a terminal (pager passes through like `less`). The non-selectable table gets the smallest static path when none exists, with its format stated in the proposal. Value components are unchanged.
- **2026-11-24, maintainer session:** the scope is frozen to N14 and N15; other findings go to the result. `internal/uroot`, `internal/runtime` and the virtual-runtime specs are not touched. README edits stay in the tui section. The commit trailer is `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; this overrides the brief's stale trailer.

## Open questions

- Should a non-selectable `table` also fall back to plain text when stdout is a terminal but no terminal can be opened, to match `pager`'s `ErrNoTerminal` fallback? The answer changes only that corner, not this change's rule. It is listed for the maintainer.
- Killing a cancelled command's process group, so that grandchildren stop on Ctrl+C as they do under a terminal's SIGINT, belongs with N1/S2.
