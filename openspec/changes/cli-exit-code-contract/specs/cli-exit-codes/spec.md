## Purpose

Define the process exit-code contract of the `invowk` binary, so that shell wrappers, CI jobs and service managers can tell success, a user command's own failure, a rejected command line and an invowk failure apart. It also covers where usage-error output goes for command groups, the per-command exceptions, and the command-registration rules that keep existing commands reachable under that contract.

## ADDED Requirements

### Requirement: Four exit-code classes
The `invowk` binary SHALL exit with one of four code classes: `0` when the invocation succeeded; the user command's own exit code `N` when a user-defined command ran and exited non-zero; `2` when invowk rejected the command line before running any user command code; and `1` when invowk itself failed for any other reason. Built-in commands MAY define additional codes only when the per-command exceptions requirement lists them.

#### Scenario: Successful command
- **WHEN** the user runs `invowk cmd hello` and the command's script exits 0
- **THEN** invowk SHALL exit with status 0

#### Scenario: Internal failure is not a usage error
- **WHEN** a command's declared tool dependency is missing from `PATH`
- **THEN** invowk SHALL exit with status 1
- **AND** stderr SHALL contain the dependency error

### Requirement: The exit-code contract is documented
The exit-code contract SHALL be documented in the CLI reference and in the README. The documentation SHALL list the four classes and the per-command exceptions. It SHALL state that a user command can also exit with `1` or `2`, so a wrapper that must tell them apart reads stderr. It SHALL state that container-engine codes 125 and 126 are forwarded as the command's code after retries.

#### Scenario: CLI reference lists the contract
- **WHEN** a reader opens the "Exit Codes" section of the CLI reference
- **THEN** it SHALL list codes 0, N, 2 and 1 with their meanings
- **AND** it SHALL list the `audit`, `tui confirm`, `tui spin` and `validate` exceptions
- **AND** it SHALL note the collision with a script's own `exit 1` or `exit 2`

### Requirement: Cobra and pflag rejections exit with status 2
Invowk SHALL exit with status 2 when Cobra or pflag rejects the command line: an unknown flag at any level, a flag value that does not parse for the flag's type, a flag that requires a value but has none, an unknown root-level command, or a positional-argument arity or valid-args error on a built-in command.

#### Scenario: Unknown flag on a user command
- **WHEN** the user runs `invowk cmd test unit --bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `unknown flag: --bogus`, matched case-insensitively

#### Scenario: Unknown root flag
- **WHEN** the user runs `invowk --bogus`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Invalid flag value for a typed invowkfile flag
- **WHEN** a command declares an `int` flag `n` and the user runs `invowk cmd f --n=abc`
- **THEN** invowk SHALL exit with status 2
- **AND** the command's script SHALL NOT run

#### Scenario: Unknown root command
- **WHEN** the user runs `invowk bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `unknown command "bogus"`, matched case-insensitively

#### Scenario: Built-in argument arity error
- **WHEN** the user runs `invowk init a b c`
- **THEN** invowk SHALL exit with status 2

### Requirement: Usage error messages are unchanged
Classifying an error as a usage error SHALL NOT change its message text, so that existing stderr assertions and fang's "Try --help for usage" hint still apply. The styled rendering of command-resolution errors (not found, ambiguous, unknown source) SHALL also stay the same.

#### Scenario: Unknown flag keeps its message
- **WHEN** the user runs `invowk --bogus`
- **THEN** stderr SHALL contain `unknown flag: --bogus`, matched case-insensitively
- **AND** stderr SHALL contain the `--help` hint

#### Scenario: Unknown command keeps its styled message
- **WHEN** the user runs `invowk cmd nope` and no command `nope` exists
- **THEN** stderr SHALL contain `command 'nope' not found`

### Requirement: Command groups reject unmatched tokens
A command group is a command node that exists only to hold subcommands, such as the synthesized parent `test` of `test unit` or a built-in parent like `config`, `module`, `tui`, `agent` or `internal`. When a group gets one or more positional tokens that do not resolve to a subcommand, invowk SHALL exit with status 2 and SHALL write nothing to stdout.

#### Scenario: Typo under a discovered group
- **WHEN** commands `test unit` and `test other` exist and the user runs `invowk cmd test nonexistent`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Typo under a built-in group
- **WHEN** the user runs `invowk config bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Typo under a non-runnable built-in group
- **WHEN** the user runs `invowk module bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

### Requirement: Group rejections explain themselves on stderr
When a command group rejects an unmatched token, invowk SHALL write to stderr an error naming the unmatched token and the group's command path, any Cobra `SuggestionsFor` matches as `Did you mean` suggestions, and the group's visible subcommands.

#### Scenario: Group typo lists the subcommands
- **WHEN** commands `test unit` and `test other` exist and the user runs `invowk cmd test nonexistent`
- **THEN** stderr SHALL contain `unknown command "nonexistent"`, matched case-insensitively
- **AND** stderr SHALL list `unit` and `other`

#### Scenario: Near-miss under a discovered group gets a suggestion
- **WHEN** command `test unit` exists and the user runs `invowk cmd test uint`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL suggest `unit`

### Requirement: Bare command groups still show help
When a command group is invoked with no positional tokens, invowk SHALL print the group's help to stdout and SHALL exit with status 0. This holds for discovered groups, built-in groups and the root command.

#### Scenario: Bare discovered group
- **WHEN** the user runs `invowk cmd test`
- **THEN** invowk SHALL print the group's help to stdout
- **AND** invowk SHALL exit with status 0

#### Scenario: Bare built-in group
- **WHEN** the user runs `invowk module`
- **THEN** invowk SHALL print the group's help to stdout
- **AND** invowk SHALL exit with status 0

#### Scenario: Bare root command
- **WHEN** the user runs `invowk` with no arguments
- **THEN** invowk SHALL print help to stdout
- **AND** invowk SHALL exit with status 0

### Requirement: Discovered groups resolve the full path before rejecting
A discovered group node SHALL handle `--ivk-from` first, then check whether the full command path (the group prefix plus the tokens) is ambiguous across sources, and only then reject an unmatched token. An ambiguous full path SHALL render the ambiguous-command message and exit with status 2, whether or not the group prefix itself is ambiguous. A node whose own name is an ambiguous command SHALL do so even with no tokens.

#### Scenario: Source-filtered invocation through a group node
- **WHEN** command `deploy prod` exists in both the invowkfile and module `m`, a unique `deploy dev` exists in the invowkfile (so group node `deploy` is registered and the ambiguous leaf is not), and the user runs `invowk cmd deploy prod --ivk-from invowkfile`
- **THEN** invowk SHALL run the invowkfile's `deploy prod` and SHALL NOT report an unmatched token

#### Scenario: Ambiguous name reached through a group node
- **WHEN** command `deploy prod` exists in both the invowkfile and module `m`, a unique `deploy dev` exists in the invowkfile, and the user runs `invowk cmd deploy prod`
- **THEN** invowk SHALL render the ambiguous-command message
- **AND** invowk SHALL exit with status 2

#### Scenario: Bare node whose name is an ambiguous command
- **WHEN** command `deploy` exists in both the invowkfile and module `m`, a unique `deploy dev` exists in the invowkfile, and the user runs `invowk cmd deploy`
- **THEN** invowk SHALL render the ambiguous-command message
- **AND** invowk SHALL exit with status 2

### Requirement: Command resolution errors exit with status 2
Invowk SHALL exit with status 2 when the requested command name is not found under `invowk cmd` (including in `--ivk-watch` mode), when the command name is ambiguous across sources, when `@source` or `--ivk-from` names a source that does not exist, and when a source filter is given without a command name.

#### Scenario: Unknown command
- **WHEN** the user runs `invowk cmd nope` and no command `nope` exists
- **THEN** invowk SHALL exit with status 2

#### Scenario: Ambiguous command
- **WHEN** command `e2` exists in both the invowkfile and module `m`, and the user runs `invowk cmd e2`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain the ambiguous-command message listing both sources

#### Scenario: Unknown source
- **WHEN** the user runs `invowk cmd @bogus test unit`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Source filter without a command name
- **WHEN** the user runs `invowk cmd @invowkfile`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Unknown command in watch mode
- **WHEN** the user runs `invowk cmd --ivk-watch nope`
- **THEN** invowk SHALL exit with status 2

### Requirement: Declared argument and flag validation errors exit with status 2
Invowk SHALL exit with status 2, and SHALL NOT run the command's script, when declared positional arguments fail validation (missing required, too many, or a value that does not match the declared type or pattern) or when invowkfile-declared flag values fail validation.

#### Scenario: Missing required argument
- **WHEN** command `a` declares required argument `x` and the user runs `invowk cmd a`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `Missing required arguments`

#### Scenario: Too many arguments
- **WHEN** command `a` declares one argument and the user runs `invowk cmd a 1 2`
- **THEN** invowk SHALL exit with status 2

### Requirement: Invalid reserved flag values exit with status 2
Invowk SHALL exit with status 2, and SHALL NOT run the command's script, when a reserved `--ivk-*` flag of `invowk cmd` has a value that fails syntactic validation. This covers `--ivk-runtime` (unknown mode, or a runtime the command does not allow), `--ivk-env-inherit-mode`, `--ivk-env-var`, `--ivk-env-inherit-allow`, `--ivk-env-inherit-deny`, `--ivk-workdir`, `--ivk-env-file` and `--ivk-container-name`.

#### Scenario: Unknown runtime mode
- **WHEN** the user runs `invowk cmd --ivk-runtime bogus test unit`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Runtime not allowed for the command
- **WHEN** command `test unit` allows only `virtual-sh` and the user runs `invowk cmd --ivk-runtime native test unit`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Unknown env inherit mode
- **WHEN** the user runs `invowk cmd test unit --ivk-env-inherit-mode bogus`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Invalid env inherit allowlist name
- **WHEN** the user runs `invowk cmd test unit --ivk-env-inherit-allow 1BAD`
- **THEN** invowk SHALL exit with status 2

### Requirement: A malformed --ivk-env-var value is rejected
An `--ivk-env-var` value SHALL have the form `KEY=VALUE` with a non-empty `KEY`. A value with no `=`, or with `=` as its first character, SHALL be rejected with an error naming the value and exit status 2, instead of being skipped with a warning. An empty `VALUE` (`KEY=`) SHALL remain valid and set the variable to the empty string.

#### Scenario: Value without an equals sign
- **WHEN** the user runs `invowk cmd test unit --ivk-env-var NOEQ`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `NOEQ`
- **AND** the command's script SHALL NOT run

#### Scenario: Value with an empty name
- **WHEN** the user runs `invowk cmd test unit --ivk-env-var =v`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Empty value is still accepted
- **WHEN** the user runs `invowk cmd test unit --ivk-env-var FOO=`
- **THEN** the command SHALL run with `FOO` set to the empty string

### Requirement: Non-usage failures exit with status 1
Invowk SHALL exit with status 1 for failures that do not come from the shape of the command line, for example config parse errors that abort the command, discovery errors, invalid invowkfiles or command trees, unsatisfied `depends_on` checks, an unsupported host platform, an unavailable container engine or runtime, command timeout or cancellation, and any error without a more specific mapping.

#### Scenario: Unsatisfied dependency
- **WHEN** a command depends on a tool that is not installed and the user runs it
- **THEN** invowk SHALL exit with status 1

#### Scenario: Invalid command tree
- **WHEN** an invowkfile declares `args` on a command that also has subcommands, and the user runs `invowk cmd`
- **THEN** invowk SHALL exit with status 1

### Requirement: An unusable explicit config file exits with status 1
When `--ivk-config` or `INVOWK_CONFIG` names a config file that is missing or invalid, and the config load is fatal for that invocation, invowk SHALL exit with status 1 and not 2. The path is accepted as argv; what fails is the file. The fatal behaviour itself is specified by the `harden-config-trust` change.

#### Scenario: Missing explicit config file
- **WHEN** `harden-config-trust` is in effect and the user runs `invowk --ivk-config /nonexistent/config.cue cmd test unit`
- **THEN** invowk SHALL exit with status 1
- **AND** the command's script SHALL NOT run

### Requirement: User command exit codes are forwarded unchanged
When a user-defined command runs and exits with a non-zero status `N` in the range 1–255, invowk SHALL exit with `N`. This SHALL hold for every runtime, including when `N` is 1 or 2.

#### Scenario: Script exits with 2
- **WHEN** a command's script runs `exit 2`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Script exits with a custom code
- **WHEN** a command's script runs `exit 42`
- **THEN** invowk SHALL exit with status 42

### Requirement: Per-command exit-code exceptions are documented and preserved
These built-in codes SHALL keep their meanings and SHALL be listed in the exit-code documentation: `invowk audit` exits 0 when clean, 1 for findings and 2 for a scan or configuration error, and its command-line rejections also exit 2; `invowk tui confirm` exits 1 when the user declines; `invowk tui spin` exits with the wrapped command's code; `invowk validate` exits 1 when validation finds errors.

#### Scenario: Audit findings keep status 1
- **WHEN** `invowk audit` detects confirmed findings at or above the severity threshold
- **THEN** invowk SHALL exit with status 1

#### Scenario: Audit argument error now exits 2
- **WHEN** the user runs `invowk audit a b c`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Audit LLM configuration error keeps status 2
- **WHEN** the user runs `invowk audit --llm-provider claude --llm-concurrency -1`, which fails in the LLM helpers that `audit` shares with `agent`
- **THEN** invowk SHALL exit with status 2

### Requirement: Agent commands follow the general contract
`invowk agent` subcommands SHALL NOT define their own codes. An error detected from argv alone, before any file is read or any LLM is called, SHALL exit with status 2: an invalid command name or module ID, mutually exclusive flags, a missing description, an invalid `--llm-provider`, `--llm-url`, `--llm-model` or `--llm-concurrency` value, or an unknown prompt format or operation. Every other agent failure, including errors that need the config file or an LLM, SHALL exit with status 1.

#### Scenario: Unknown prompt format
- **WHEN** the user runs `invowk agent cmd prompt --format yaml`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Missing description
- **WHEN** the user runs `invowk agent mod create io.example.tools` with no description and no `--from-file`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Command already exists
- **WHEN** command `agenthello` already exists and the user runs `invowk agent cmd create agenthello --llm-provider claude 'duplicate'`
- **THEN** invowk SHALL exit with status 1

#### Scenario: Module removal without --force
- **WHEN** module `io.example.tools` exists and the user runs `invowk agent mod remove io.example.tools`
- **THEN** invowk SHALL exit with status 1

### Requirement: Undeclared positionals after a leaf command are still forwarded
A leaf command that declares no `args` SHALL keep accepting undeclared positional tokens and passing them to the script as positional parameters. This includes a leaf that also has subcommands, where a token that is not a subcommand name is passed to the leaf. Rejecting such tokens is deferred to the `passthrough` schema change.

#### Scenario: Unmatched token after a leaf with children
- **WHEN** leaf `w` and its child `w sub` both exist and the user runs `invowk cmd w typo`
- **THEN** invowk SHALL run `w` with `typo` as its first positional parameter
- **AND** invowk SHALL exit with the script's exit code

### Requirement: A command that is also a group prefix stays runnable
When command `X` and a command `X Y` both exist, invowk SHALL register `X` as a runnable leaf with `Y` as a subcommand, and SHALL NOT register `X` as a help-only group. This SHALL hold whether the two come from the same source or from different sources, and regardless of discovery order.

#### Scenario: Leaf in the invowkfile, child in a module
- **WHEN** the invowkfile defines `w2` and module `m` defines `w2 sub`, and the user runs `invowk cmd w2`
- **THEN** invowk SHALL run the invowkfile's `w2` command
- **AND** `invowk cmd w2 sub` SHALL run the module's `w2 sub` command

### Requirement: Command listing order is unaffected
The registration-order fix SHALL NOT change the output or the order of the `invowk cmd` command listing, for any mix of sources.

#### Scenario: Listing with a cross-source prefix leaf
- **WHEN** the invowkfile defines `w2` and `a`, module `m` defines `w2 sub`, and the user runs `invowk cmd`
- **THEN** the listing SHALL show the same commands in the same order as before this change
- **AND** invowk SHALL exit with status 0

### Requirement: Shell completion is unaffected
Usage-error classification and the registration-order fix SHALL NOT change the candidates or the exit status of Cobra's hidden `__complete` and `__completeNoDesc` commands for any path, including group nodes and the root.

#### Scenario: Completing under a discovered group
- **WHEN** the shell runs `invowk __complete cmd test ""`
- **THEN** the output SHALL list the group's subcommands as before this change
- **AND** invowk SHALL exit with status 0

#### Scenario: Completing under a built-in group
- **WHEN** the shell runs `invowk __complete config ""`
- **THEN** the output SHALL list `config`'s subcommands as before this change
- **AND** invowk SHALL exit with status 0
