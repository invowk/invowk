# Spec Delta

## Purpose

Updates `cli-exit-codes` to reflect `128 + signal` exit statuses when `invowk` terminates from a trapped OS signal on Unix, distinguishing signal termination from non-signal cancellation/timeout (`1`).

## MODIFIED Requirements

### Requirement: Non-usage failures exit with status 1
Invowk SHALL exit with status 1 for failures that do not come from the shape of the command line, for example config parse errors that abort the command, discovery errors, invalid invowkfiles or command trees, unsatisfied `depends_on` checks, an unsupported host platform, an unavailable container engine or runtime, command timeout (deadline exceeded) or non-signal cancellation, and any error without a more specific mapping. When a Unix `invowk` process terminates because it received a trapped termination signal (`SIGINT`, `SIGTERM`, `SIGHUP`), it SHALL instead exit with `128 + signal` (`130`, `143`, `129`), except where a built-in explicitly defines another code (such as `invowk tui spin`).

#### Scenario: Unsatisfied dependency
- **WHEN** a command depends on a tool that is not installed and the user runs it
- **THEN** invowk SHALL exit with status 1

#### Scenario: Invalid command tree
- **WHEN** an invowkfile declares `args` on a command that also has subcommands, and the user runs `invowk cmd`
- **THEN** invowk SHALL exit with status 1

#### Scenario: Command timeout exits with status 1
- **WHEN** a command exceeds its configured `timeout` deadline without an external OS signal
- **THEN** invowk SHALL exit with status 1

### Requirement: Per-command exit-code exceptions are documented and preserved
These built-in codes SHALL keep their meanings and SHALL be listed in the exit-code documentation: `invowk audit` exits 0 when clean, 1 for findings and 2 for a scan or configuration error, and its command-line rejections also exit 2; `invowk tui confirm` exits 1 when the user declines; `invowk tui spin` exits with the wrapped command's code, or 1 in the cases its scenarios list; `invowk validate` exits 1 when validation finds errors.

#### Scenario: Audit findings keep status 1
- **WHEN** `invowk audit` detects confirmed findings at or above the severity threshold
- **THEN** invowk SHALL exit with status 1

#### Scenario: Audit argument error now exits 2
- **WHEN** the user runs `invowk audit a b c`
- **THEN** invowk SHALL exit with status 2

#### Scenario: tui confirm declined
- **WHEN** the user answers no to `invowk tui confirm 'Proceed?'`
- **THEN** invowk SHALL exit with status 1

#### Scenario: tui spin forwards the wrapped command's code
- **WHEN** the command wrapped by `invowk tui spin` exits with status 42
- **THEN** invowk SHALL exit with status 42

#### Scenario: tui spin cancelled or ended by a signal exits 1
- **WHEN** `invowk tui spin` is cancelled with Ctrl+C, or its wrapped command is ended by a signal
- **THEN** invowk SHALL exit with status 1

#### Scenario: tui spin whose command outlives the cancellation exits 1
- **WHEN** the command wrapped by `invowk tui spin` is still running when the bounded wait that follows a cancellation ends
- **THEN** invowk SHALL exit with status 1 and report that the command did not stop and may still be running

#### Scenario: validate finds errors
- **WHEN** the user runs `invowk validate` and the invowkfile does not pass validation
- **THEN** invowk SHALL exit with status 1

#### Scenario: Audit LLM configuration error keeps status 2
- **WHEN** the user runs `invowk audit --llm-provider claude --llm-concurrency -1`, which fails in the LLM helpers that `audit` shares with `agent`
- **THEN** invowk SHALL exit with status 2

## ADDED Requirements

### Requirement: Unix termination signals exit with 128 plus signal number
On Unix platforms, when `invowk` is terminated by a trapped signal (`SIGHUP` = 1, `SIGINT` = 2, `SIGTERM` = 15), the process SHALL exit with status `128 + signal` (`129`, `130`, `143`) after bounded cleanup completes, except for built-ins whose per-command contract specifies status `1` (such as `invowk tui spin`).

#### Scenario: SIGTERM produces exit code 143
- **WHEN** `invowk cmd long-running` receives `SIGTERM` on Unix
- **THEN** after cleanup finishes, `invowk` SHALL exit with status `143`
