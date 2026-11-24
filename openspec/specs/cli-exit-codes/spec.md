# cli-exit-codes Specification

## Purpose
Define the process exit-code contract of the `invowk` binary, so that shell wrappers, CI jobs and service managers can tell success, a user command's own failure, a rejected command line and an invowk failure apart. It also covers where usage-error output goes for command groups, the per-command exceptions, and the command-registration rules that keep existing commands reachable under that contract.

## Requirements

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
Invowk SHALL exit with status 2 when Cobra or pflag rejects the command line: an unknown flag at any level, a flag value that does not parse for the flag's type, a flag that requires a value but has none, an unknown root-level command (also when `--help`, `-h` or `--version` is given), or a positional-argument arity or valid-args error on a built-in command.

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

#### Scenario: Unknown root command with a help or version flag
- **WHEN** the user runs `invowk bogus --help`, `invowk bogus -h` or `invowk bogus --version`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Flag missing its value
- **WHEN** the user runs `invowk cmd test unit --ivk-workdir` with no value
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `flag needs an argument`, matched case-insensitively

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
A command group is a command node that exists only to hold subcommands, such as the synthesized parent `test` of `test unit` or a built-in parent such as `config` or `module`. When a group gets one or more positional tokens that do not resolve to a subcommand, invowk SHALL exit with status 2 and SHALL write nothing to stdout. Exception: see "A command missing because a source failed to load exits with status 1".

#### Scenario: Typo under a discovered group
- **WHEN** commands `test unit` and `test other` exist and the user runs `invowk cmd test nonexistent`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Typo under a discovered group while a module failed to load
- **WHEN** commands `test unit` and `test other` exist in the invowkfile, module directory `broken.invowkmod` fails to load, and the user runs `invowk cmd test typo`
- **THEN** invowk SHALL exit with status 1
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
Invowk SHALL exit with status 2 when the requested command name is not found under `invowk cmd` (including in `--ivk-watch` mode), when the command name is ambiguous across sources, when `@source` or `--ivk-from` names a source that does not exist, and when a source filter is given without a command name. A not-found error after a source failed to load is the exception below (status 1).

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

### Requirement: A malformed source name exits with status 2
A `@source` or `--ivk-from` value SHALL be normalized (a leading `@` and a `.invowkmod` suffix removed; `invowkfile.cue` names the invowkfile) and SHALL then be a well-formed source name. A value that is not, such as the path `./m.invowkmod` or `1bad`, SHALL be rejected with an error naming the value and exit status 2 on every invocation path, including watch mode, without running any script.

#### Scenario: A path given as the source
- **WHEN** the user runs `invowk cmd @./m.invowkmod test unit`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `invalid source`, matched case-insensitively

#### Scenario: A malformed --ivk-from after a leaf
- **WHEN** the user runs `invowk cmd test unit --ivk-from 1bad`
- **THEN** invowk SHALL exit with status 2

#### Scenario: A malformed source in watch mode
- **WHEN** the user runs `invowk cmd --ivk-watch --ivk-from 1bad test unit`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Normalized source forms still resolve
- **WHEN** command `test unit` exists in the invowkfile and the user runs `invowk cmd test unit --ivk-from @invowkfile`
- **THEN** invowk SHALL run the command and exit with status 0

### Requirement: A command missing because a source failed to load exits with status 1
When the command or source named on the command line is not found, and discovery reported in the same invocation an invowkfile or module that it skipped because it failed to parse or load, invowk SHALL exit with status 1 instead of 2. This covers unknown commands, group typos under `invowk cmd`, unknown sources and watch mode. Policy skips, such as reserved module names, do not count. Ambiguity, argument and flag errors keep status 2.

#### Scenario: Command defined in an invowkfile that does not parse
- **WHEN** the only invowkfile defines `build` but has a field the schema does not allow, and the user runs `invowk cmd build`
- **THEN** stderr SHALL report that the invowkfile was skipped
- **AND** invowk SHALL exit with status 1

#### Scenario: Source filter on a skipped invowkfile
- **WHEN** the only invowkfile does not parse and the user runs `invowk cmd @invowkfile build`
- **THEN** invowk SHALL exit with status 1

#### Scenario: Source filter on the skipped module
- **WHEN** module directory `broken.invowkmod` fails to load and the user runs `invowk cmd @broken x`
- **THEN** invowk SHALL exit with status 1

### Requirement: A scoped miss in a source that loaded exits with status 2
For a request that names a source (`@source` or `--ivk-from`), only a skipped invowkfile or module that may be that source SHALL count for the status-1 exception: a root invowkfile may be `invowkfile`, a scanned module directory the source of its name (residuals: design.md, Decision 14). Included and vendored modules, an invowkfile inside a module, and a diagnostic without a path SHALL always count. Otherwise the miss is certain and SHALL exit with status 2.

#### Scenario: Scoped miss in a source that loaded
- **WHEN** the invowkfile is valid, module directory `broken.invowkmod` fails to load, and the user runs `invowk cmd @invowkfile nope`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Unknown source that cannot be the skipped one
- **WHEN** module directory `broken.invowkmod` fails to load and the user runs `invowk cmd @bogus x`
- **THEN** invowk SHALL exit with status 2

### Requirement: A skipped-source not-found error names the skipped sources
When a not-found error exits with status 1 because a source failed to load, its message SHALL name each skipped source that counted by a short label (its file or module directory name, the module directory plus the file for an invowkfile inside a module, or `an unnamed source` when the diagnostic has no path), with the discovery diagnostic code (for example `broken.invowkmod (module_load_skipped)`), so a wrapper can tell why the status is 1.

#### Scenario: The error names the skipped source
- **WHEN** the invowkfile is valid, module directory `broken.invowkmod` fails to load, and the user runs `invowk cmd nope`
- **THEN** invowk SHALL exit with status 1
- **AND** stderr SHALL contain `broken.invowkmod` and `module_load_skipped`

### Requirement: Declared argument and flag validation errors exit with status 2
Invowk SHALL exit with status 2, and SHALL NOT run the command's script, when declared positional arguments fail validation (missing required, too many, or a value that does not match the declared type or pattern) or when invowkfile-declared flag values fail validation.

#### Scenario: Missing required argument
- **WHEN** command `a` declares required argument `x` and the user runs `invowk cmd a`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `Missing required arguments`

#### Scenario: Too many arguments
- **WHEN** command `a` declares one argument and the user runs `invowk cmd a 1 2`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Missing required flag
- **WHEN** command `rf` declares a required flag `name` and the user runs `invowk cmd rf`
- **THEN** invowk SHALL exit with status 2
- **AND** the command's script SHALL NOT run

### Requirement: Invalid reserved flag values exit with status 2
Invowk SHALL exit with status 2, and SHALL NOT run the command's script, when a reserved `--ivk-*` flag of `invowk cmd` has a value that fails syntactic validation. This covers `--ivk-runtime` (unknown mode, or a runtime the command does not allow), `--ivk-env-inherit-mode`, `--ivk-env-var`, `--ivk-env-inherit-allow`, `--ivk-env-inherit-deny`, `--ivk-workdir`, `--ivk-env-file` and `--ivk-container-name`. A malformed `--ivk-from` is covered below.

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
When `--ivk-config` or `INVOWK_CONFIG` names a config file that is missing or invalid, or `INVOWK_CONFIG` is relative, and the config load is fatal for that invocation, invowk SHALL exit with status 1 and not 2. The fatal behaviour itself is specified by the `config-source-resolution` capability; `invowk audit` keeps its scan-error code 2.

#### Scenario: Missing explicit config file
- **WHEN** the user runs `invowk --ivk-config /nonexistent/config.cue cmd test unit`
- **THEN** invowk SHALL exit with status 1
- **AND** the command's script SHALL NOT run

#### Scenario: Missing file named by INVOWK_CONFIG
- **WHEN** `INVOWK_CONFIG=/nonexistent` is set and the user runs `invowk cmd test unit`
- **THEN** invowk SHALL exit with status 1
- **AND** stderr SHALL name `INVOWK_CONFIG`

### Requirement: Flag and environment conflicts exit with status 2
Invowk SHALL exit with status 2 when it refuses an invocation from argv and the environment alone, without reading any file: an invalid `INVOWK_NO_CWD` value in a flow that resolves the switch, `--ivk-watch` with `--ivk-dry-run`, `invowk config set` while `--ivk-config` or `INVOWK_CONFIG` is active, and `invowk agent cmd create` or `change` with `--verify` while the no-cwd switch is on.

#### Scenario: Invalid INVOWK_NO_CWD
- **WHEN** `INVOWK_NO_CWD=yes` is set and the user runs `invowk cmd`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL name `INVOWK_NO_CWD`

#### Scenario: Watch together with dry-run
- **WHEN** the user runs `invowk cmd hello --ivk-watch --ivk-dry-run`
- **THEN** invowk SHALL exit with status 2
- **AND** the command's script SHALL NOT run

#### Scenario: config set while an explicit source is active
- **WHEN** `INVOWK_CONFIG` names an absolute path and the user runs `invowk config set default_runtime virtual-sh`
- **THEN** invowk SHALL exit with status 2
- **AND** no config file SHALL be created or changed

#### Scenario: agent verify under the no-cwd switch
- **WHEN** the user runs `invowk --ivk-no-cwd agent cmd create --verify hello 'say hello'`
- **THEN** invowk SHALL exit with status 2
- **AND** no LLM request SHALL be made

### Requirement: User command exit codes are forwarded unchanged
When a user-defined command runs and exits with a non-zero status `N` in the range 1–255, invowk SHALL exit with `N`. This SHALL hold for every runtime, including when `N` is 1 or 2.

#### Scenario: Script exits with 2
- **WHEN** a command's script runs `exit 2`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Script exits with a custom code
- **WHEN** a command's script runs `exit 42`
- **THEN** invowk SHALL exit with status 42

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
- **THEN** invowk SHALL exit with status 1, as a native `invowk cmd` does

#### Scenario: tui spin whose command outlives the cancellation exits 1
- **WHEN** the command wrapped by `invowk tui spin` is still running when the bounded wait that follows a cancellation ends
- **THEN** invowk SHALL exit with status 1 and report that the command did not stop and may still be running

#### Scenario: validate finds errors
- **WHEN** the user runs `invowk validate` and the invowkfile does not pass validation
- **THEN** invowk SHALL exit with status 1

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

#### Scenario: Malformed module ID for remove or change
- **WHEN** the user runs `invowk agent mod remove 'bad id'` or `invowk agent mod change 'bad id' 'describe it' --llm-provider claude`
- **THEN** invowk SHALL exit with status 2
- **AND** no LLM provider SHALL be resolved

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

### Requirement: Command listing order is alphabetical within each source
The `invowk cmd` listing SHALL show the root invowkfile first and then modules alphabetically by source, and within each source SHALL list commands alphabetically by full command name, within each category group, whatever the declaration order. Registration SHALL NOT reorder the discovery result.

#### Scenario: Commands declared out of order
- **WHEN** the invowkfile declares `zeta`, `w2` and `alpha` in that order, module `m` declares `w2 sub` and then `beta`, and the user runs `invowk cmd`
- **THEN** the listing SHALL show `alpha`, `w2`, `zeta` under the invowkfile, then `beta`, `w2 sub` under `m`
- **AND** invowk SHALL exit with status 0

#### Scenario: Commands within a category
- **WHEN** the invowkfile declares `link` and then `compile`, both in category `Build`, and the user runs `invowk cmd`
- **THEN** the `[Build]` group SHALL list `compile` before `link`

### Requirement: Shell completion is unaffected
Usage-error classification and the registration-order fix SHALL NOT change the candidates or the exit status of Cobra's hidden `__complete` and `__completeNoDesc` commands for any path, including group nodes and the root. A prefix leaf that was shadowed by a group node SHALL now show its own description instead of the group's.

#### Scenario: Completing under a discovered group
- **WHEN** the shell runs `invowk __complete cmd test ""`
- **THEN** the output SHALL list the group's subcommands as before this change
- **AND** invowk SHALL exit with status 0

#### Scenario: Completing under a built-in group
- **WHEN** the shell runs `invowk __complete config ""`
- **THEN** the output SHALL list `config`'s subcommands as before this change
- **AND** invowk SHALL exit with status 0

#### Scenario: Completing without descriptions
- **WHEN** the shell runs `invowk __completeNoDesc cmd ""`
- **THEN** the output SHALL list the same names as `__complete`, without descriptions
- **AND** invowk SHALL exit with status 0

#### Scenario: Completing a prefix leaf
- **WHEN** the invowkfile defines `w2`, module `m` defines `w2 sub`, and the shell runs `invowk __complete cmd ""`
- **THEN** the `w2` candidate SHALL carry the leaf's own description
- **AND** `invowk __complete cmd w2 ""` SHALL list `sub`
