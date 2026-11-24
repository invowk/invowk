# Proposal

## Why

Two `invowk tui` paths leave the terminal or the caller in a bad state (tracker N14 and N15, found in the S5a review, PR #175).

- **N14.** Ctrl+C in `invowk tui spin` quits the spinner but abandons the wrapped command. The adapter then reads the command's result while the command goroutine may still write it, a data race. The run exits 0 with empty output, and the command keeps running. A spinner that fails after the command started also drops the command's output and exit status.
- **N15.** With stdout redirected and a terminal present, `invowk tui pager` opens the pager and writes nothing to stdout, unlike `less`. A non-selectable `invowk tui table` is treated as a value component, so it opens the terminal too, and with no terminal at all it exits 1.

## What Changes

N14, `invowk tui spin`:

- **BREAKING (exit status).** Ctrl+C in the spinner cancels the wrapped command, waits up to 15 seconds for it to end, prints what it wrote, and exits with status 1. Today it exits 0. Status 1 is what a native `invowk cmd` gives when it is interrupted (design D1). If the command has not ended after 15 seconds, spin exits 1, prints nothing from it and says that the command may still be running.
- Whenever the wrapped command ran, spin prints its output and maps its exit status, even if the spinner itself failed. The spinner error goes to stderr.
- A wrapped command ended by a signal (no exit status) makes spin exit 1, as a native `invowk cmd` does. Today it exits 255.
- No data race: the command's result reaches the adapter only once the command has ended.

N15, display-only components with stdout redirected:

- **BREAKING (output).** One rule: when its stdout is not a terminal, a display-only component writes its content to stdout and never opens the terminal. This holds with or without a TUI server.
  - `pager` then works like `less` in a pipe: the content goes through unchanged, without the title or line numbers, and the exit status is 0.
  - A non-selectable `table` prints its rows as plain text with aligned columns and no control sequences. There is no static render path today (`internal/tui/table.go:151-176` always runs a Bubble Tea program), so this change adds the smallest one.
- **Table text format** (for the maintainer to check):
  - the first line holds the column titles (the header row or `--columns`), followed by one line per data row in input order;
  - cells are left-aligned and padded with spaces to the widest cell of their column, measured in terminal cells, so wide characters line up;
  - columns are separated by two spaces, trailing spaces are trimmed, and every line ends with `\n`;
  - there is no border, no header rule and no ANSI;
  - every row is printed: `--height` and `--widths` do not apply, so nothing is cut off;
  - a row with fewer cells than the widest row is padded with empty cells.

  ```text
  name   age  city
  Alice  30   NYC
  Bob    25   LA
  ```
- Value components (`input`, `choose`, `filter`, `file`, `write`, `confirm` and `table --selectable`) keep today's behavior.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `tui-component-terminal-io`: adds the spin Ctrl+C and result-reporting rules (N14) and the stdout-not-a-terminal rule for `pager` and the non-selectable `table` (N15). It also amends the terminal-drawing, display-component and server-delegation requirements so that they carve out that rule.

`cli-exit-codes` is unchanged. Its "Non-usage failures exit with status 1" requirement already covers cancellation, and its `tui spin` exception ("exits with the wrapped command's code") still holds whenever the command ends by itself.

## Impact

- `internal/tui/spin.go`: `SpinWithAction` takes a context and gives the action its own cancellable context. Ctrl+C makes the program report an interrupt. A spinner that returns before the action ended cancels the action and waits for it, up to the bound.
- `cmd/invowk/tui_spin.go`: the result is passed through a channel; exit statuses are mapped for cancellation and for signals; output is printed after a spinner failure.
- `cmd/invowk/tui_pager.go`, `cmd/invowk/tui_table.go`: a stdout-terminal check before the server delegation and before direct rendering.
- `internal/tui/table.go` (or a sibling file): the plain-text table writer.
- Tests: unit tests in `internal/tui` and `cmd/invowk`, including one under `-race` that fails before the fix; `tests/cli/testdata/tui_no_terminal.txtar` (notty, all three OSes); `tests/cli/tui_tmux_test.go` (a terminal is present and stdout is redirected; Ctrl+C in spin); and a Unix-only Go test in `tests/cli` for SIGINT during a spin that has no terminal.
- Docs: README (tui section only), `website/docs/tui/{overview,pager,table-and-spin}.mdx` and their pt-BR copies, plus the CLI reference's exit-code note for `tui spin` if it needs one.
- Not touched: `internal/tui/interactive*.go`, `internal/app/commandadapters/interactive_session.go`, `internal/uroot`, `internal/runtime`, termination-signal handling (N1/S2). No formal-bound symbols are involved.

## Migration notes

- **Changed:** pressing Ctrl+C during `invowk tui spin` now stops the wrapped command and exits with status 1. Before, it exited 0 while the command kept running in the background. Scripts that treated a cancelled spin as success must handle status 1.
- **Changed:** a spin whose wrapped command is killed by a signal now exits 1. Before, it exited 255.
- **Changed:** `invowk tui pager` with stdout redirected (`invowk tui pager file | grep x`, `> out.txt`) now writes the content to stdout instead of opening the pager on the terminal.
- **Changed:** `invowk tui table` without `--selectable` and with stdout redirected now prints the rows as aligned plain text. Before, it opened the interactive table on the terminal, or exited 1 when there was no terminal.
- **Unchanged:** every value component, `table --selectable`, and `pager` and the non-selectable `table` when stdout is a terminal.
