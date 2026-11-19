# Proposal

## Why

Rendered directly, the `invowk tui` components draw to stdout. So the documented capture form `name=$(invowk tui input)` gives the script Bubble Tea's terminal control sequences in front of the answer, draws nothing and exits 0. This happens on `main` in a plain terminal, not only under `-i`. The v0.17.0 tag waits for this fix (maintainer decision 27).

S3 (`restrict-tui-server-bind`, PR #173) refuses `-i` in engine modes without a narrow TUI route. Whether those modes can run without a TUI server once this defect is fixed is a separate question, and it is **not** part of this change (maintainer decision 39, see "Moved out" below).

## What Changes

**Components draw on the terminal, not stdout.**

- `input`, `choose`, `filter`, `file`, `write` and `table` draw their UI on the controlling terminal: `/dev/tty` on Unix, `CONIN$`/`CONOUT$` on Windows. stdout carries only the result. `confirm`, `spin` and `pager` draw there too. `format` and `style` are unchanged, because their stdout is their product.
- When no controlling terminal can be opened (on Windows: when the opened console handles are not a usable console), invowk falls back to stdin and stderr, if both are terminals. Otherwise the drawing components fail with exit status 1 and an error saying the component needs a terminal, with empty stdout. `spin` then runs its command once, without the spinner. `pager` then writes the content to stdout.
- stdout captured, stderr redirected and stdin piped no longer corrupt a captured value or hide the UI.
- The tmux capture tests run on the Linux CI legs, and fail instead of skipping there when tmux is missing.

**Moved out to a later change (S5b, not yet proposed).** The no-server `-i` path (S3 stage 3), the hardened gate test of a real no-server `-i` session (maintainer decision 33), the `tui-server-exposure` deltas, the config-default change and the 2 s connect-failure fallback all move to S5b. Their earlier drafts are recoverable from commit 9d8483ca (before the rebase) and 97d1e6a5 (rebased on 562f6842). S3's refusal stays in force until S5b ships, and if S5b's gate fails, it stays (decision 34).

## Capabilities

### New Capabilities
- `tui-component-terminal-io`: where the `invowk tui` components draw and read keys when they render directly, and what they do without a terminal.

### Modified Capabilities
- None. `tui-server-exposure` is unchanged by this change.

## Impact

- `internal/tui`: every directly rendered `tea.NewProgram` call site, a new terminal resolver (with a Windows console check), `SpinWithAction`, `Pager`, and the unused `Config.Output` (and its tests `tui_test.go:140`, `tui_write_test.go:164`).
- `cmd/invowk/tui_*.go`: the no-terminal error, and spin and pager without a terminal.
- `tests/cli/tui_tmux_test.go` (capture variants, fail-instead-of-skip on Linux CI), a new `tests/cli/testdata/tui_no_terminal.txtar`, and `.github/workflows/ci.yml` (the variable that makes tmux required on the Linux full legs).
- The `invowk tui` docs: README, website TUI pages and their pt-BR copies.

## Migration notes

- **Changed (user-facing):** directly rendered `invowk tui` components now draw on the terminal, not on stdout. `v=$(invowk tui input)` and the other value components capture exactly the answer, and the UI stays visible even when stdout or stderr is redirected. Scripts that stripped escape sequences from captured values can stop. A component run with no terminal at all now fails with a clear "needs a terminal" error and empty stdout; before, it failed with Bubble Tea's "could not open TTY" error. `invowk tui spin` and `invowk tui pager` instead degrade to plain output.
- **Unchanged:** S3's refusal of `invowk cmd … -i` in modes without a narrow TUI route, and its migration note, stand. This change supersedes nothing in S3.
