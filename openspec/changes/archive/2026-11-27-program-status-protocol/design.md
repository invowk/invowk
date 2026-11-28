# Design: Program Status Protocol (OSC 7501)

## Context

See `proposal.md` for motivation. Current codebase observations:

- `go.mod:7` pins `charm.land/bubbletea/v2` (`programstatus.go` in `charm.land/bubbletea/v2@v2.1.1`) exposing `View.ProgramStatus *tea.ProgramStatus` (`= uv.ProgramStatus = ansi.ProgramStatus`).
- In Bubble Tea's `cursedRenderer` (`cursed_renderer.go:520-522,877-884`), changing `View.ProgramStatus` emits `ansi.SetProgramStatus(*cur)`, and changing it back to `nil` emits `ansi.ClearProgramStatusID(prev.ID)`. However, `cursedRenderer.close()` (`cursed_renderer.go:251-253`) intentionally leaves `ProgramStatus` in place, AND `Program.Run()` (`tea.go:1182-1203`) sets `killed := p.externalCtx.Err() != nil || p.ctx.Err() != nil || err != nil`: whenever `killed` is true (including `tea.Interrupt` at `tea.go:796-797` and external `ctx` cancellation), Bubble Tea **skips** the final `p.render(model)` (`tea.go:1196-1200`) and skips `p.renderer.flush(true)` (`tea.go:1308,1481-1484`). Additionally, external `SIGTERM` injects `QuitMsg` directly (`tea.go:676-699,793-794`) without running `model.Update`, so `m.done` stays `false` during the final render.
- `github.com/charmbracelet/x/ansi` (`programstatus.go:93,111-113,127-204`) provides `ansi.ClearProgramStatus` (`"\x1b]7501;state=clear\x07"`, which omits `app=`), `ansi.ClearProgramStatusID`, `ansi.SetProgramStatus`, and `programStatusText` (replaces invalid UTF-8 with `U+FFFD`, collapses control characters `< 0x20` and `0x7f..0x9f` into spaces, trims whitespace, and truncates UTF-8 to 192 bytes for `Title` and 2048 bytes for `Message`).
- `internal/tui/interactive.go:17-21` defines `stateExecuting`, `stateCompleted`, and `stateTUI`. `internal/tui/interactive_model.go:127-164` implements `interactiveModel.View()` (including early return on `!m.ready` at `interactive_model.go:128-132`); `handleKeyMsg` (`interactive_model.go:212-255`) handles `Ctrl+\` force-quit (`interactive_model.go:216-218`) and `stateCompleted` quit keys (`interactive_model.go:226-227`).
- `internal/tui/interactive_model.go:258-284` (`handleTUIComponentRequest`) branches `ComponentTypeWrite` (`StyledTextOptions`, non-blocking styled output via `handleStyledTextRequest` at `interactive_model.go:259-261`) and creates all modal overlays via `CreateEmbeddableComponent` (`internal/tui/embeddable.go:220-320`).
- Standalone `invowk tui *` components run via `runComponentProgramWith` (`internal/tui/terminal.go:176-187`) using `componentTerminal` (`internal/tui/terminal.go:31-35,101-130`, `/dev/tty`/`CONOUT$` or `stdin`+`stderr` fallback at `111-114`, never `stdout`) or `runSpinnerProgram` (`internal/tui/spin.go:570-583`).
  - `renderComponentView` (`internal/tui/tui.go:317-349`) serves `inputModel` (`input.go:97`), `writeModel` (`write.go:98`), and `fileModel` (`file.go:101`).
  - `confirmModel` (`confirm.go:44,107`), `pagerModel` (`pager.go:44,94`), `inputModel` (`input.go:46`), `writeModel` (`write.go:47`), and `fileModel` (`file.go:51`) store `forModal bool`.
  - `chooseModel` (`choose.go:37-51`), `filterModel` (`filter.go:56-67`), `tableModel` (`table.go:48-61`), and `spinModel` (`spin.go:88-97`) do NOT currently store `forModal` (`NewSpinModelForModal` does not exist yet at `embeddable.go:313`); `tableModel` also does not store `title` or `selectable`; `choose_builders.go:211-216,275-280` and `filter.go:466-470` do not call `l.DisableQuitKeybindings()`, so `bubbles/v2/list`'s `v` quit binding bypasses `m.done = true`.
  - `spinActionModel` (`spin.go:115-119,534-559`) handles `spinnerDoneMsg` (`tea.Quit`) and `Ctrl+C` (`tea.Interrupt`) without a `done` field.
- `cmd/invowk/cmd_watch.go:56-179` (`runWatchMode`) executes `session.InitialExecution(ctx)` (`cmd_watch.go:135-138`) and `session.HandleChange(cbCtx, changed)` (`cmd_watch.go:156-164`) from `internal/watch/watcher.go:378-392,459` (timer goroutine). `cmd/invowk/root.go:107` notifies only `os.Interrupt`; `cmd/invowk/cmd_watch_test.go:54-67` constructs `App` with `Stdout: io.Discard, Stderr: io.Discard`.

## Goals / Non-Goals

**Goals:**
- Emit accurate, sanitized OSC 7501 `ProgramStatus` reports across `invowk cmd -i`, standalone `invowk tui *`, and `invowk cmd --ivk-watch`.
- Ensure `clear` is emitted reliably even on `tea.Interrupt`, external `SIGTERM`/`QuitMsg`, and `tea.ErrProgramKilled` (where Bubble Tea skips final view rendering).
- Keep `stdout` of `invowk tui *` and redirected pipes 100% clean, and keep embedded modal overlays (`forModal: true`) from fighting `interactiveModel` over root status.

**Non-Goals:**
- Performing blocking OSC 7501 feature-detection queries (`tea.RequestProgramStatusSupport()`) at startup.
- Emitting OSC 7501 in non-interactive `invowk cmd` runs.
- Supporting sub-record hierarchies (`id=parent/child`) in v1; all reports address the root record (`ID: ""`) with `App: "invowk"`.

## Decisions

### D1. Shared OSC 7501 helper builders and standardized fields (`internal/tui/program_status.go`)
Create `internal/tui/program_status.go` defining:
- `programStatusApp = "invowk"` (matches `[A-Za-z0-9_.+-]{1,32}`).
- Explicit `sanitizeProgramStatusText(s string, maxBytes int) string` matching `x/ansi` (`ToValidUTF8`, control-char normalization `<0x20` and `0x7f..0x9f` to single space, `TrimSpace`, UTF-8-safe byte truncation `<= 192` / `<= 2048`), so `tea.ProgramStatus` values are already clean and deterministically testable before encoding.
- Standardized field semantics: `App: "invowk"`, `Title` holds the concise subject (command name in `-i` / `--ivk-watch`, or component title standalone), and `Message` holds the contextual prompt/activity line (when applicable).
- Shared `programStatusMeta` struct (`title string`, `password bool`, `selectable bool`) and `statusForTUIComponent(component ComponentType, meta programStatusMeta) *tea.ProgramStatus`:
  - `ComponentTypeConfirm` -> `blocked` (`ProgramStatusKindPermission`)
  - `ComponentTypeInput` (`password == true`) -> `blocked` (`ProgramStatusKindAuth`)
  - `ComponentTypeInput` (`!password`), `ComponentTypeWrite`, `ComponentTypeTextArea`, `ComponentTypeChoose`, `ComponentTypeFilter`, `ComponentTypeFile`, `ComponentTypeTable` (`selectable == true`) -> `blocked` (`ProgramStatusKindQuestion`)
  - `ComponentTypePager`, `ComponentTypeTable` (`selectable == false`) -> `idle`
  - `ComponentTypeSpin` -> `working`

### D2. `interactiveModel` (`-i`) and `RunInteractiveSession` cancellation cleanup
- Add `quitting bool` and `activeComponentMeta programStatusMeta` to `interactiveModel`.
- In `handleTUIComponentRequest` (`interactive_model.go:258`), after `CreateEmbeddableComponent` succeeds, populate `m.activeComponentMeta` (`Title` from the decoded options struct, `password` for `InputOptions`, `selectable` for `TableOptions`). Clear `m.activeComponentMeta` in `handleTUIComponentDone` and `doneMsg`.
- Set `m.quitting = true` before `tea.Quit` on `Ctrl+\` (`interactive_model.go:216-218`) and `stateCompleted` quit keys (`interactive_model.go:226-227`).
- In `interactiveModel.View()` (`interactive_model.go:127-164`), compute status for BOTH `!m.ready` and `m.ready` (checking `m.quitting` first!):
  - If `m.quitting` -> `ProgramStatus = nil` (`state=clear`).
  - Else if `m.state == stateTUI` -> `statusForTUIComponent(m.activeComponentType, m.activeComponentMeta)` (`Title: m.cmdName`, `Message: meta.title` if non-empty).
  - Else if `m.state == stateCompleted` -> `done` (if `ExitCode == 0 && Error == nil`) or `error`, with `Title: m.cmdName`.
  - Else (`stateExecuting` or `!m.ready`) -> `working` (`Title: m.cmdName`).
- **Killed/cancelled session cleanup**: In `RunInteractiveSession` (`internal/tui/interactive_session.go:65-75`), when `p.Run()` returns `errors.Is(err, tea.ErrProgramKilled)` (e.g., `ctx` cancelled via `SIGINT`/`SIGHUP`), Bubble Tea skips rendering `clear`; if `opts.ProgramOptions` did not override output to a test writer (or through an explicit TTY/output seam), write `ansi.ClearProgramStatus` to the outer controlling terminal so neither `working` nor `blocked` lingers.

### D3. Standalone `invowk tui *` components and forced-exit `clear`
- Add `forModal bool` to `chooseModel`, `filterModel`, `tableModel`, and `spinModel` (plus `NewSpinModelForModal` used by `embeddable.go:313`, `title string` on `chooseModel`/`filterModel`/`tableModel`, and `selectable bool` on `tableModel`).
- Call `l.DisableQuitKeybindings()` in `choose_builders.go` and `filter.go` so `v` cannot exit `bubbles/v2/list` without setting `m.done = true`.
- Set `view.ProgramStatus` ONLY when `!done && !forModal` across `renderComponentView` (`tui.go:317`), `confirmModel`, `chooseModel`, `filterModel`, `tableModel`, `pagerModel`, and `spinModel`.
- Update `spinActionModel` (`spin.go:115-119,534-559`) with `done bool`, set `done = true` on `spinnerDoneMsg` and `Ctrl+C`, and return `tea.NewView("")` (`ProgramStatus: nil`) when `done`, otherwise `working` (`App: "invowk"`, `Title: m.title.String()`).
- **Killed / interrupted / `SIGTERM` cleanup (`B1` fix)**:
  - In `runComponentProgramWith` (`internal/tui/terminal.go:176-187`), if `p.Run()` returns an error (`tea.ErrInterrupted`, `tea.ErrProgramKilled`) OR the returned `EmbeddableComponent` reports `!IsDone()` (e.g., external `SIGTERM` injected `QuitMsg` without running `Update`), write `ansi.ClearProgramStatus` directly to `terminal.out` before `terminal.Close()`.
  - In `runSpinnerProgram` (`internal/tui/spin.go:570-583`), if `program.Run()` returns an error (such as `tea.ErrInterrupted` on `Ctrl+C` or `tea.ErrProgramKilled`), write `ansi.ClearProgramStatus` directly to `terminal.out`.

### D4. `invowk cmd --ivk-watch` status reporting (`cmd/invowk/cmd_watch.go`)
- Add `watchStatusReporter` (mutex-guarded `sync.Mutex` for concurrent `OnChange` safety) writing `ansi.SetProgramStatus` / `ansi.ClearProgramStatus` when a terminal is attached:
  - Resolve target via an injectable `WatchStatusTarget` / `App` dependency seam (production default: open `/dev/tty` on Unix / `CONOUT$` on Windows, falling back to `app.stdout` if `term.IsTerminal`, else disabled/`nil`), so `newWatchTestApp` (`cmd_watch_test.go:54`) with `Stdout: io.Discard` never writes to the test runner's real `/dev/tty`.
  - Emit `working` (`Title: args[0]`, `Message: "Running..."`) right before `InitialExecution` and `HandleChange`.
  - After `InitialExecution` / `HandleChange` returns **without fatal watcher error** (`err == nil` / `handleErr == nil`): emit `error` if `outcome.Err != nil || outcome.ExitCode != 0`, otherwise emit `idle`. When returning a fatal error to exit `runWatchMode`, skip `idle`/`error` and proceed straight to `defer Clear()`.
  - Defer `Clear()` (`ansi.ClearProgramStatus`) on `runWatchMode` return. Note on `SIGHUP`/`SIGTERM`: if `watch-mode-program-resume` catches `SIGHUP`/`SIGTERM`, `defer Clear()` runs cleanly on terminal close; if OS default signal disposition kills the process immediately, the terminal drops `working` automatically on process exit and clears on restart. (When `--ivk-watch -i` runs, `interactiveModel` emits viewer status while running and `watchStatusReporter` restores `idle`/`error` when returning to watch.)

## Risks / Trade-offs

- **Risk**: In `-i` or `invowk tui *`, existing unit tests that call `model.View()` and compare whole `tea.View` structs could fail if `ProgramStatus` is now populated.
  -> **Mitigation**: Audit and update `internal/tui/*_test.go` assertions where whole `tea.View` equality is checked; `Content` and `AltScreen` stay identical.
- **Risk**: Unit tests in `cmd/invowk/` writing to real `/dev/tty` when run in an interactive developer shell.
  -> **Mitigation**: Inject a no-op or buffer `WatchStatusWriter` in `newWatchTestApp` and unit tests via `Dependencies` (`App`).

## Maintainer decisions

- **2026-11-27**: Open `program-status-protocol` as Change 4 (independently reviewed) covering `invowk cmd -i`, standalone `invowk tui *`, and `invowk cmd --ivk-watch`, while keeping non-interactive `invowk cmd` silent.
- **2026-11-27**: Use `idle` (rather than `done`) as the steady-state OSC 7501 status after a passing `--ivk-watch` run, matching `ProgramStateIdle` ("at rest, waiting for the user's next instruction") while `error` flags failed watch builds.

## Open questions

(none — all blocking and high design ambiguities resolved during independent spec review)
