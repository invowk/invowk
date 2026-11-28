# Proposal: Program Status Protocol (OSC 7501)

## Why

Long-running commands in `invowk cmd -i`, `invowk cmd --ivk-watch`, and interactive `invowk tui` prompts often run in background tabs or panes while the user works elsewhere. Terminals supporting the Program Status Protocol (OSC 7501, already supported in `charm.land/bubbletea/v2` and `github.com/charmbracelet/x/ansi`) cannot tell whether an Invowk session is actively working, blocked waiting for user confirmation/input/credentials, finished successfully, or failed without ad-hoc screen scraping. Adopting OSC 7501 lets Invowk report structured execution and prompt states cleanly without altering piped output or interfering with non-interactive commands.

## What Changes

- Report OSC 7501 status via Bubble Tea `View.ProgramStatus` (plus explicit `ansi.ClearProgramStatus` fallback on killed/interrupted exits) in `invowk cmd -i` (`internal/tui/interactive_model.go`, `internal/tui/interactive_session.go`):
  - `working` while `stateExecuting` (and `spin` overlays).
  - `blocked` while `stateTUI` displays interactive overlays, mapped by kind: `permission` (`confirm`), `auth` (`input` in password mode), and `question` (`input`, `textarea`, `choose`, `filter`, `file`, selectable `table`). Non-selectable `table` and `pager` overlays report `idle` (passive reading). (`ComponentTypeWrite` is a non-blocking styled print inside `stateExecuting`.)
  - `done` (exit code 0 and nil error) or `error` (non-zero exit code or error) upon transitioning to `stateCompleted`.
  - `clear` when the user dismisses `stateCompleted`, force-quits (`Ctrl+\`), or when `RunInteractiveSession` ends via context cancellation (`tea.ErrProgramKilled`).
- Report OSC 7501 status in standalone `invowk tui *` components (`confirm`, `input`, `write`, `choose`, `filter`, `file`, `table`, `spin`, `pager`), only when rendering directly (`!forModal`) on a terminal (`componentTerminal`), emitting `clear` on completion, cancellation (`Ctrl+C`), or external `SIGTERM`/`QuitMsg` so scripts receive byte-exact stdout with no stale status left behind.
- Report OSC 7501 status in `invowk cmd --ivk-watch` (`cmd/invowk/cmd_watch.go`) via `github.com/charmbracelet/x/ansi` (`SetProgramStatus` / `ClearProgramStatus`) on TTYs only: `working` during initial and re-run executions, `idle` while awaiting changes after a passing run, `error` while awaiting changes after a failing run, and `clear` on exit.
- Sanitize and bound all `Title`/`Message` payloads via `x/ansi` rules (non-clear reports carry `app="invowk"`, control chars normalized, invalid UTF-8 replaced, UTF-8 byte limits `title <= 192`, `msg <= 2048`).
- Keep non-interactive `invowk cmd` (without `-i` or `--ivk-watch`) silent so child tools emitting OSC 7501 (`cargo`, `terraform`, `claude`) and pipes remain untouched.
- Non-breaking change: terminals without OSC 7501 ignore the escape sequences, and `stdout` capture contracts in `tui-component-terminal-io` remain unchanged.

## Capabilities

### New Capabilities

- `program-status-protocol`: Defines OSC 7501 status reporting across `invowk cmd -i`, standalone `invowk tui *` components, and `invowk cmd --ivk-watch`, including state/kind mapping, lifetime clearing, payload sanitization, and non-interactive silence.

### Modified Capabilities

- `interactive-mode-terminal`: Clarifies that while child terminal queries and control sequences remain isolated inside `x/vt`, the `-i` viewer itself emits OSC 7501 status reports for the viewer session's lifecycle.

## Impact

- Code: `internal/tui/` (`program_status.go`, `interactive_model.go`, `interactive_session.go`, `terminal.go`, `tui.go`, `confirm.go`, `input.go`, `write.go`, `choose.go`, `choose_builders.go`, `filter.go`, `file.go`, `table.go`, `spin.go`, `pager.go`, `embeddable.go`), `cmd/invowk/` (`app.go`, `cmd_watch.go`, `watch_status.go`).
- Tests & Docs: `internal/tui/` and `cmd/invowk/` unit tests, tmux interactive/TUI verification, and EN + pt-BR documentation under `website/docs/`.
