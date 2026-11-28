# Spec Delta

## Purpose

Defines Invowk's CLI output color control (`--ivk-color`, `NO_COLOR`, `CLICOLOR_FORCE`, `TERM=dumb`, TTY detection), orthogonal `ui.color_scheme` theme wiring for Glamour rendering, child stdio preservation, and quiet exit-status forwarding without `fang` error framing.

## ADDED Requirements

### Requirement: Non-TTY and NO_COLOR Invowk output omits ANSI escapes by default
Invowk's own CLI output (help text, command listings, diagnostics, dry-run plans, issue catalog cards, and Invowk-generated error messages) SHALL omit ANSI escape sequences when writing to a non-TTY stream, when `TERM=dumb`, or when `NO_COLOR` is non-empty, unless overridden by `--ivk-color=always` or `CLICOLOR_FORCE`.

#### Scenario: Redirected or piped Invowk output is ANSI-free
- **WHEN** `invowk cmd` or `invowk validate` runs with stdout and stderr redirected to non-TTY pipes and neither `--ivk-color=always` nor `CLICOLOR_FORCE` is set
- **THEN** stdout and stderr SHALL contain no ANSI escape sequences (`\x1b[`)

#### Scenario: NO_COLOR suppresses ANSI on TTY
- **WHEN** `NO_COLOR=1` is set and `--ivk-color` is not set to `always`
- **THEN** Invowk's own CLI output and Glamour-rendered issue catalog entries SHALL omit ANSI escape sequences and use Glamour's `"notty"` style

### Requirement: Persistent --ivk-color flag overrides environment and TTY detection
Invowk SHALL provide a persistent root flag `--ivk-color` accepting `auto` (default), `always`, and `never`, with precedence `--ivk-color` > `NO_COLOR` / `CLICOLOR_FORCE` / `TERM=dumb` > TTY detection. Invalid `--ivk-color` values SHALL fail as a usage error (exit status `2`).

#### Scenario: --ivk-color=always forces color in non-TTY output
- **WHEN** `invowk --ivk-color=always cmd` runs with stdout redirected to a pipe
- **THEN** Invowk's own CLI output MAY emit ANSI styling according to the configured color profile and `ui.color_scheme`

#### Scenario: --ivk-color=never disables color even with CLICOLOR_FORCE
- **WHEN** `CLICOLOR_FORCE=1` is set and `invowk --ivk-color=never cmd` runs
- **THEN** Invowk's own CLI output SHALL contain no ANSI escape sequences

#### Scenario: Invalid --ivk-color exits with usage error
- **WHEN** the user runs `invowk --ivk-color=bogus cmd`
- **THEN** invowk SHALL exit with status `2` and print an error naming `--ivk-color`

### Requirement: ui.color_scheme selects the theme orthogonally to color enablement
When color is enabled, Glamour rendering SHALL select the `"auto"`, `"dark"`, or `"light"` style matching `ui.color_scheme`; when color is disabled, Glamour rendering SHALL use `"notty"` regardless of `ui.color_scheme`.

#### Scenario: Light color scheme applied when color is enabled
- **WHEN** `ui.color_scheme` is `"light"` and color is enabled (`--ivk-color=always` or TTY)
- **THEN** Glamour-rendered issue catalog entries SHALL use the `"light"` style instead of hardcoding `"dark"`

### Requirement: Child command stdio FDs and opt-in TUI styling primitives remain intact
Invowk SHALL NOT wrap raw child-process `ExecutionContext.IO` writers with color-stripping wrappers, so user commands observe true terminal/pipe file descriptors. `invowk tui style` and `invowk tui format` SHALL remain explicitly opt-in to ANSI styling unless `--ivk-color=never` is passed.

#### Scenario: Child command observes real TTY or raw FD behavior
- **WHEN** a user command inspects its own stdout/stderr file descriptors or emits styled output under `--ivk-color=auto`
- **THEN** Invowk SHALL forward the child command's stdout/stderr bytes without re-stripping or downsampling them

#### Scenario: tui style preserves styling in pipelines by default
- **WHEN** `invowk tui style --foreground "#FF0000" "hello"` runs in a pipeline without `--ivk-color=never`
- **THEN** stdout SHALL preserve the requested ANSI styling

### Requirement: Forwarded exit codes suppress fang error framing unconditionally
When a command or built-in returns an `*ExitError` without an underlying Invowk error message (`Err == nil`), Invowk SHALL forward the exit code without printing a `fang` `ERROR / Exit status N.` banner on stderr in either TTY or non-TTY mode. Under `--ivk-verbose`, Invowk MAY print a single plain status line on stderr. Errors with non-nil diagnostic/usage text (`*UsageError`, `*ServiceError`, Cobra parse errors) SHALL still render cleanly on stderr.

#### Scenario: Failing user command emits no fang Exit status banner
- **WHEN** a user command exits with status `1` or `42` (on TTY or non-TTY) without `--ivk-verbose`
- **THEN** invowk SHALL exit with that status and stderr SHALL NOT contain `Exit status`

#### Scenario: Declined tui confirm or audit findings omit fang Exit status banner
- **WHEN** `invowk audit` exits `1` due to findings or `invowk tui confirm` exits `1` on decline
- **THEN** invowk SHALL exit with status `1` and stderr SHALL NOT contain `Exit status 1`
