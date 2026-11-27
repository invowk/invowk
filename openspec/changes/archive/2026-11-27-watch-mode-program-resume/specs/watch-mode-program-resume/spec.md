# Spec Delta

## Purpose

Defines OSC 7577 Program Resume Protocol reporting, child token isolation, secret and protocol-limit guardrails, and termination-signal lifecycle behavior for `invowk cmd --ivk-watch`.

## ADDED Requirements

### Requirement: Watch mode reports a signed OSC 7577 resume command on supporting terminals
When `invowk cmd --ivk-watch` runs attached to a terminal and `OSC7577_TOKEN` is set, Invowk SHALL emit a valid signed OSC 7577 report carrying the Invowk executable, `--ivk-watch` flags and arguments, and the absolute working directory.

#### Scenario: Watch mode emits OSC 7577 report on TTY with OSC7577_TOKEN
- **GIVEN** `OSC7577_TOKEN` is set and watch mode runs attached to a terminal without `--ivk-env-var`
- **WHEN** `invowk cmd --ivk-watch test` starts watching
- **THEN** Invowk SHALL write a valid MAC-signed `OSC 7577` sequence containing the executable path, `--ivk-watch`, command arguments, and absolute working directory to the terminal

#### Scenario: Non-TTY or missing token emits no OSC 7577 sequence
- **WHEN** `invowk cmd --ivk-watch` runs with `OSC7577_TOKEN` unset OR without an attached terminal (e.g., redirected pipe or CI)
- **THEN** Invowk SHALL emit no `OSC 7577` bytes to stdout, stderr, or the terminal

### Requirement: Watch mode omits resume reporting when secret or protocol guardrails apply
Watch mode SHALL NOT report a resume command when `--ivk-env-var` is passed, or when any argument or working directory violates OSC 7577 encoding or size constraints.

#### Scenario: Watch mode with --ivk-env-var skips reporting
- **GIVEN** `OSC7577_TOKEN` is set and watch mode runs on a terminal
- **WHEN** the user invokes `invowk cmd --ivk-watch --ivk-env-var API_TOKEN=secret serve`
- **THEN** Invowk SHALL NOT emit a resume `argv` report, preventing secret leakage and broken resumed environments

#### Scenario: Oversized or control-character arguments skip reporting
- **GIVEN** `OSC7577_TOKEN` is set and watch mode receives an argument containing a control character (such as newline) or exceeding OSC 7577 limits
- **WHEN** watch mode prepares its resume report
- **THEN** Invowk SHALL skip emitting `argv` (and clear any prior record if already reported)

### Requirement: Watch mode strips OSC7577_TOKEN from watched child executions
Every command execution launched by `invowk cmd --ivk-watch` (initial execution and change-triggered re-executions across all runtimes, including non-interactive `native`) SHALL have `OSC7577_TOKEN` removed from its environment.

#### Scenario: Native child run under watch mode cannot overwrite resume record
- **GIVEN** `OSC7577_TOKEN` is set in Invowk's environment
- **WHEN** `invowk cmd --ivk-watch` runs a `native` command whose script prints `$OSC7577_TOKEN`
- **THEN** `$OSC7577_TOKEN` SHALL be empty inside the watched child process

### Requirement: Watch mode clears on voluntary stop and preserves on terminal termination
Watch mode SHALL emit an OSC 7577 `clear=1` report when the user interrupts it (`SIGINT`) or when watch mode exits on startup/watcher error, and SHALL NOT emit `clear=1` when terminated by terminal closure or restart (`SIGHUP` or `SIGTERM`).

#### Scenario: Ctrl+C clears the resume record
- **GIVEN** watch mode previously emitted an OSC 7577 `argv` report
- **WHEN** watch mode receives `SIGINT` (Ctrl+C) and exits
- **THEN** Invowk SHALL write a valid MAC-signed `OSC 7577` `clear=1` sequence before exiting

#### Scenario: SIGHUP or SIGTERM preserves the resume record
- **GIVEN** watch mode previously emitted an OSC 7577 `argv` report on Unix
- **WHEN** watch mode terminates due to `SIGHUP` or `SIGTERM` (terminal closing or restarting)
- **THEN** Invowk SHALL NOT emit `clear=1`, leaving the resume record intact for the terminal to restore

### Requirement: Interactive watch mode keeps watcher ownership of the resume record
When `invowk cmd --ivk-watch -i` runs, the watcher SHALL remain the sole owner of the OSC 7577 resume record, and re-report its own record after an interactive child viewer completes if needed.

#### Scenario: Interactive watch run preserves watcher record across child completion
- **GIVEN** `OSC7577_TOKEN` is set and the user runs `invowk cmd --ivk-watch -i dev`
- **WHEN** an interactive execution completes inside the viewer and returns to watching
- **THEN** the child command SHALL NOT receive `OSC7577_TOKEN`, and the watcher's OSC 7577 record SHALL remain active until `SIGINT`
