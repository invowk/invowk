# tui-component-terminal-io Specification

## Purpose
Defines where the `invowk tui` components draw their UI and read keys when they render directly (without a TUI server), so that stdout carries only their result, and what each component does when no terminal is available.

## Requirements

### Requirement: Directly rendered components draw on the controlling terminal
When an `invowk tui` component other than `format` and `style` renders directly, it SHALL draw its UI on and read keys from the controlling terminal (`/dev/tty` on Unix; on Windows the console: keys from stdin when stdin is a console, otherwise from `CONIN$`, and output to `CONOUT$`), whatever stdin, stdout and stderr are connected to. It SHALL write no UI bytes to stdout, nor to stderr except under the fallback below. Exception: `pager` and a non-selectable `table` whose stdout is not a terminal follow "Display-only components pass content through when stdout is not a terminal".

#### Scenario: Captured value is byte-exact
- **WHEN** a script in a terminal runs `name=$(invowk tui input)`, the user types `Hello World` and presses Enter
- **THEN** the prompt SHALL be visible on the terminal, and `$name` SHALL be exactly `Hello World`, with no control sequences

#### Scenario: Every value component
- **WHEN** stdout of `invowk tui choose`, `filter`, `file`, `write` or `table --selectable` is captured and the user selects or enters a value
- **THEN** the captured stdout SHALL hold only that value and its trailing newline

#### Scenario: stderr redirected
- **WHEN** a script runs `name=$(invowk tui input 2>err.log)`
- **THEN** the prompt SHALL be visible on the terminal, `$name` SHALL be the typed text, and `err.log` SHALL hold no UI bytes

#### Scenario: stdin is a pipe
- **WHEN** a script runs `printf 'a\nb\n' | invowk tui filter` or `invowk tui input </dev/null`
- **THEN** the component SHALL read keys from the terminal, and `filter` SHALL take its options from the pipe

### Requirement: Display-only and status components draw on the terminal
`invowk tui confirm`, `spin` and `pager` SHALL draw on the controlling terminal under the same rule; `pager` and a non-selectable `table` do so only while their stdout is a terminal. `confirm` SHALL keep reporting its answer only through its exit status. `spin` SHALL write only the wrapped command's output to stdout. `format` and `style` SHALL keep writing their styled text to stdout.

#### Scenario: Captured spin output
- **WHEN** a script runs `out=$(invowk tui spin --title Wait -- echo done)` in a terminal
- **THEN** `$out` SHALL be exactly `done`, and the spinner SHALL draw only on the terminal

#### Scenario: Captured style output
- **WHEN** a script runs `s=$(invowk tui style --bold hi)`
- **THEN** `$s` SHALL hold the styled text, as before this change

#### Scenario: pager on a terminal stdout
- **WHEN** a user in a terminal runs `invowk tui pager notes.txt` with stdout on the terminal
- **THEN** the pager SHALL draw on the terminal and wait for `q`, `Esc` or `Enter`

### Requirement: Fallback when no controlling terminal opens
On Windows, console handles that open but are not a usable console SHALL count as a controlling terminal that cannot be opened. If the controlling terminal cannot be opened but stdin and stderr are both terminals, a component SHALL read keys from stdin and draw on stderr. If neither works, a drawing component SHALL fail with exit status 1 and an error that names the component and says it needs a terminal. It SHALL write nothing to stdout.

#### Scenario: No terminal at all
- **WHEN** `invowk tui input` runs with no controlling terminal and with stdin and stderr not terminals, as in a CI job
- **THEN** it SHALL exit with status 1, stdout SHALL be empty, and stderr SHALL say that `input` needs a terminal

#### Scenario: Windows process without a usable console
- **WHEN** `invowk tui input` runs on Windows where `CONIN$` or `CONOUT$` is not a usable console, and stdin and stderr are not consoles
- **THEN** it SHALL exit with status 1, stdout SHALL be empty, and stderr SHALL say that `input` needs a terminal

#### Scenario: No controlling terminal, but terminal stdio
- **WHEN** the process has no controlling terminal and its stdin and stderr are terminals
- **THEN** the component SHALL draw on stderr, read keys from stdin, and write only its result to stdout

### Requirement: spin and pager degrade without a terminal
Without any usable terminal, `invowk tui spin` SHALL run the wrapped command exactly once, without a spinner, and keep its stdout and exit-code behavior, and `invowk tui pager` SHALL write the content unchanged to stdout and exit 0.

#### Scenario: spin in CI
- **WHEN** `invowk tui spin -- sh -c 'echo ok; exit 3'` runs with no terminal
- **THEN** stdout SHALL be `ok`, and the exit status SHALL be 3

#### Scenario: spin runs its command once
- **WHEN** `invowk tui spin -- sh -c 'echo run >>count.log'` runs with no terminal
- **THEN** `count.log` SHALL hold exactly one `run` line

#### Scenario: pager in a pipe without a terminal
- **WHEN** `printf 'x\n' | invowk tui pager` runs with no terminal
- **THEN** stdout SHALL be `x` and the exit status SHALL be 0

### Requirement: Server-delegated rendering is unchanged
When `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN` are set and the server answers, a component other than `spin` SHALL delegate to the server as before, SHALL open no terminal itself, and SHALL write only its result to stdout. `spin` SHALL keep notifying the server and drawing its own spinner, under the terminal rule above. Exception: `pager` and a non-selectable `table` whose stdout is not a terminal SHALL write their content to stdout and send no request to the server.

#### Scenario: Interactive host run with a server
- **WHEN** an interactive native command runs `name=$(invowk tui input)` and the user answers in the parent's TUI
- **THEN** `$name` SHALL be the answer, and the child SHALL not open the terminal

### Requirement: Ctrl+C in spin cancels the wrapped command
When the user presses Ctrl+C while `invowk tui spin` draws its spinner, or the spinner program is interrupted, spin SHALL cancel the wrapped command and wait for it to end, up to 15 seconds. When the command has ended, spin SHALL write the output it produced to stdout and exit with status 1, the status a native `invowk cmd` gives when it is interrupted. stderr SHALL say that the command was cancelled.

#### Scenario: Ctrl+C stops the command
- **WHEN** a user in a terminal runs `invowk tui spin --title Wait -- sh -c 'echo started; exec sleep 30'` and presses Ctrl+C while the spinner shows
- **THEN** spin SHALL exit with status 1 well before the 30 seconds pass
- **AND** stdout SHALL be `started`
- **AND** stderr SHALL say that the command was cancelled
- **AND** the `sleep` process SHALL no longer be running

#### Scenario: Cancelled spin is not a success
- **WHEN** a script runs `invowk tui spin -- make build || echo failed` and the user presses Ctrl+C during the build
- **THEN** the script SHALL print `failed`

### Requirement: A command that does not stop is reported, not read
If the wrapped command has not ended 15 seconds after spin cancelled it, `invowk tui spin` SHALL exit with status 1 without waiting longer and without reading the command's result. It SHALL write nothing from the command to stdout, and stderr SHALL say that the command did not stop and may still be running.

#### Scenario: Command ignores cancellation
- **WHEN** the user presses Ctrl+C in spin and the wrapped command is still running 15 seconds after the cancellation
- **THEN** spin SHALL exit with status 1
- **AND** stdout SHALL hold nothing from the command
- **AND** stderr SHALL say that the command may still be running

### Requirement: spin reports the command's result whenever it ran
Whenever the wrapped command ran and ended, `invowk tui spin` SHALL write its output to stdout and exit with its status, also when the spinner program failed after the command started. A spinner failure SHALL then be reported on stderr without changing the exit status.

#### Scenario: Spinner fails while the command runs
- **WHEN** the spinner program fails after the wrapped command `sh -c 'echo ok; exit 3'` started
- **THEN** after the command ends, stdout SHALL be `ok`
- **AND** spin SHALL exit with status 3
- **AND** stderr SHALL report the spinner error

#### Scenario: Spinner fails and the command succeeds
- **WHEN** the spinner program fails after the wrapped command `echo ok` started
- **THEN** stdout SHALL be `ok`, spin SHALL exit with status 0, and stderr SHALL report the spinner error

### Requirement: A command ended by a signal exits spin with status 1
When the wrapped command ends without an exit status because a signal killed it, `invowk tui spin` SHALL write its output to stdout and exit with status 1, as a native `invowk cmd` does, with an error on stderr. It SHALL NOT exit with 255 or any other value derived from a negative status.

#### Scenario: SIGINT while spin runs without a terminal
- **WHEN** `invowk tui spin -- sh -c 'echo started; exec sleep 30'` runs with no terminal and its process receives SIGINT while the command runs
- **THEN** spin SHALL exit with status 1
- **AND** stdout SHALL be `started`

### Requirement: Display-only components pass content through when stdout is not a terminal
When the stdout of `invowk tui pager`, or of `invowk tui table` without `--selectable`, is not a terminal, the component SHALL write its content to stdout, exit with status 0, and SHALL NOT open the terminal or delegate to a TUI server. `pager` SHALL write the content unchanged, without its title or line numbers. Value components SHALL keep the terminal rule.

#### Scenario: pager in a pipe with a terminal present
- **WHEN** a user in a terminal runs `invowk tui pager notes.txt | cat`
- **THEN** the pager SHALL NOT appear on the terminal
- **AND** the piped output SHALL equal the bytes of `notes.txt`
- **AND** the exit status SHALL be 0 without any key press

#### Scenario: pager with flags into a file
- **WHEN** a user in a terminal runs `invowk tui pager --title Notes --line-numbers notes.txt > out.txt`
- **THEN** `out.txt` SHALL equal `notes.txt`, with no title and no line numbers

#### Scenario: Non-selectable table in a pipe
- **WHEN** a user in a terminal runs `invowk tui table --file rows.csv | cat`
- **THEN** the table SHALL NOT appear on the terminal
- **AND** the piped output SHALL be the rows as plain text, and the exit status SHALL be 0 without any key press

#### Scenario: Non-selectable table without any terminal
- **WHEN** `invowk tui table --file rows.csv` runs with no terminal, as in a CI job
- **THEN** stdout SHALL be the rows as plain text, and the exit status SHALL be 0

#### Scenario: Redirected stdout under a TUI server
- **WHEN** `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN` are set and a script runs `invowk tui pager notes.txt > out.txt`
- **THEN** `out.txt` SHALL equal `notes.txt`, and no pager request SHALL reach the server

#### Scenario: Selectable table keeps the terminal rule
- **WHEN** a script in a terminal runs `row=$(invowk tui table --file rows.csv --selectable)`
- **THEN** the table SHALL draw on the terminal, and `$row` SHALL hold only the selected row

### Requirement: Plain-text table format
The plain-text `table` output SHALL start with a line of column titles (the header row, or `--columns`), followed by one line per data row in input order. Each cell SHALL be left-aligned and padded with spaces to the widest cell of its column, measured in terminal cells, with two spaces between columns, trailing spaces trimmed, and `\n` line endings. Each control character in a cell (a newline, a tab, an escape) SHALL be written as a space, so every row stays on one line. It SHALL hold no control sequences, borders or header rule. Every row SHALL be printed, whatever `--height` and `--widths` say.

#### Scenario: Aligned rows
- **WHEN** `invowk tui table --separator '|'` runs with stdout not a terminal and stdin `name|age|city`, `Alice|30|NYC`, `Bob|25|LA`
- **THEN** stdout SHALL be exactly `name   age  city`, `Alice  30   NYC`, `Bob    25   LA`, one per line

#### Scenario: Short rows and overridden titles
- **WHEN** `invowk tui table --columns a,b,c` runs with stdout not a terminal and stdin `x,y,z` and `1`
- **THEN** stdout SHALL be exactly `a  b  c`, `x  y  z`, `1`, one per line

#### Scenario: Control characters in a cell
- **WHEN** `invowk tui table --file rows.csv` runs with stdout not a terminal and `rows.csv` holds `name,notes` and `Alice,"line1` / `line2"` (a quoted field with a newline)
- **THEN** stdout SHALL be exactly `name   notes`, `Alice  line1 line2`, one per line

#### Scenario: Height does not cut rows
- **WHEN** `invowk tui table --height 2 --file rows.csv` runs with stdout not a terminal and `rows.csv` holds 30 data rows
- **THEN** stdout SHALL hold the title line and all 30 rows
