# Spec Delta

## Purpose

Defines opt-in raw argument passthrough (`passthrough: true`) on `#Command`, mutual exclusivity rules, pre-command `--ivk-*` flag handling, source-disambiguated passthrough routing, and runtime argument forwarding.

## ADDED Requirements

### Requirement: #Command supports opt-in passthrough mutually exclusive with flags, args, and subcommands
`#Command` SHALL admit an optional `passthrough?: bool` field (default `false`). A command with `passthrough: true` SHALL NOT declare `flags`, SHALL NOT declare `args`, and SHALL NOT have child subcommands (enforced in schema/model validation and `ValidateCommandTree`).

#### Scenario: Passthrough command with flags or args fails validation
- **WHEN** an `invowkfile.cue` defines a command with `passthrough: true` alongside `flags` or `args`
- **THEN** parsing/validation SHALL fail with an actionable error

#### Scenario: Passthrough command with subcommand fails command-tree validation
- **WHEN** commands `cli` (`passthrough: true`) and `cli sub` both exist
- **THEN** `ValidateCommandTree` SHALL fail with an error rejecting subcommands under a passthrough command

### Requirement: Passthrough forwards all tokens after the command path verbatim
When a command has `passthrough: true`, all `--ivk-*` flags before the command path SHALL configure Invowk, and every token after the command path—including `--help`, `-h`, unknown flags, and `--`—SHALL be forwarded verbatim to the script via runtime positional parameters (`$@`, `ARGn`/`ARGC`, `invowk.args` in Lua) without Cobra flag parsing or rejection. This SHALL hold across direct invocation, `@source`, and `--ivk-from`.

#### Scenario: Unknown flags and --help are forwarded verbatim
- **WHEN** command `wrap` has `passthrough: true` and the user runs `invowk cmd --ivk-env-var A=1 wrap --help --unknown=val -x`
- **THEN** `--ivk-env-var A=1` SHALL apply to the execution and the script SHALL receive `--help`, `--unknown=val`, and `-x` verbatim as positional parameters

#### Scenario: Disambiguated passthrough via @source forwards --help verbatim
- **WHEN** command `wrap` exists in multiple sources, `cst`'s `wrap` has `passthrough: true`, and the user runs `invowk cmd @cst wrap --help --json`
- **THEN** Invowk SHALL NOT print `invowk cmd` help or reject `--json` and SHALL execute `cst`'s `wrap` with `--help` and `--json`

### Requirement: Passthrough commands remain inspectable via help and listing
`invowk help cmd <name>` SHALL display Invowk's own command help without executing a `passthrough: true` command, and `invowk cmd` listings SHALL visually indicate passthrough commands.

#### Scenario: invowk help cmd inspects without executing
- **WHEN** `wrap` has `passthrough: true` and the user runs `invowk help cmd wrap`
- **THEN** Invowk SHALL print its command help to stdout and SHALL NOT execute `wrap`'s script
