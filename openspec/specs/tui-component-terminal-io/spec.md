# tui-component-terminal-io Specification

## Purpose
Defines where the `invowk tui` components draw their UI and read keys when they render directly (without a TUI server), so that stdout carries only their result, and what each component does when no terminal is available.

## Requirements

### Requirement: Directly rendered components draw on the controlling terminal
When an `invowk tui` component other than `format` and `style` renders directly, it SHALL draw its UI on and read keys from the controlling terminal (`/dev/tty` on Unix; on Windows the console: keys from stdin when stdin is a console, otherwise from `CONIN$`, and output to `CONOUT$`), whatever stdin, stdout and stderr are connected to. It SHALL write no UI bytes to stdout, nor to stderr except under the fallback below.

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
`invowk tui confirm`, `spin` and `pager` SHALL draw on the controlling terminal under the same rule. `confirm` SHALL keep reporting its answer only through its exit status. `spin` SHALL write only the wrapped command's output to stdout. `format` and `style` SHALL keep writing their styled text to stdout.

#### Scenario: Captured spin output
- **WHEN** a script runs `out=$(invowk tui spin --title Wait -- echo done)` in a terminal
- **THEN** `$out` SHALL be exactly `done`, and the spinner SHALL draw only on the terminal

#### Scenario: Captured style output
- **WHEN** a script runs `s=$(invowk tui style --bold hi)`
- **THEN** `$s` SHALL hold the styled text, as before this change

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
When `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN` are set and the server answers, a component other than `spin` SHALL delegate to the server as before, SHALL open no terminal itself, and SHALL write only its result to stdout. `spin` SHALL keep notifying the server and drawing its own spinner, under the terminal rule above.

#### Scenario: Interactive host run with a server
- **WHEN** an interactive native command runs `name=$(invowk tui input)` and the user answers in the parent's TUI
- **THEN** `$name` SHALL be the answer, and the child SHALL not open the terminal
