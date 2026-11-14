## ADDED Requirements

### Requirement: Documented process exit-code contract
The `invowk` binary SHALL exit with one of four code classes:
- `0` when the invocation succeeded;
- the user command's own exit code `N` when a user-defined command ran and exited non-zero;
- `2` when invowk rejected the command line before running any user command code;
- `1` when invowk itself failed for any other reason.

Built-in commands MAY define additional codes only when the per-command exceptions requirement lists them. The contract SHALL be documented in the CLI reference and the README, and the documentation SHALL state that a user command can also exit with `1` or `2`.

#### Scenario: Successful command
- **WHEN** the user runs `invowk cmd hello` and the command's script exits 0
- **THEN** invowk SHALL exit with status 0

#### Scenario: Internal failure is not a usage error
- **WHEN** a command's declared tool dependency is missing from `PATH`
- **THEN** invowk SHALL exit with status 1
- **AND** stderr SHALL contain the dependency error

### Requirement: Command-line rejections exit with status 2
Invowk SHALL exit with status 2 when Cobra or pflag rejects the command line. This covers:
- an unknown flag at any level;
- a flag value that does not parse for the flag's type;
- a flag that requires a value but has none;
- an unknown root-level command;
- a positional-argument arity or valid-args error on a built-in command.

The error message text SHALL be the same as before this change, so that existing stderr assertions and fang's "Try --help for usage" hint still apply.

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
- **AND** stderr SHALL contain `unknown command "bogus"`

#### Scenario: Built-in argument arity error
- **WHEN** the user runs `invowk init a b c`
- **THEN** invowk SHALL exit with status 2

### Requirement: Command groups reject unmatched tokens
A command group is a command node that exists only to hold subcommands. This covers synthesized parents of space-separated discovered command names (for example `test` for `test unit`) and built-in parents such as `config`, `module`, `tui`, `agent` and `internal`.

When a group is invoked with one or more positional tokens that do not resolve to a subcommand, invowk SHALL:
- write an error naming the unmatched token and the group's command path to stderr;
- write any Cobra `SuggestionsFor` matches to stderr as `Did you mean` suggestions;
- write the group's available subcommands to stderr;
- write nothing to stdout;
- exit with status 2.

When a group is invoked with no positional tokens, invowk SHALL keep printing the group's help to stdout and SHALL exit with status 0.

The existing `--ivk-from` and ambiguous-prefix handling of discovered group nodes SHALL run before the unmatched-token rejection.

#### Scenario: Typo under a discovered group
- **WHEN** commands `test unit` and `test other` exist and the user runs `invowk cmd test nonexistent`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `unknown command "nonexistent"`
- **AND** stderr SHALL list `unit` and `other`
- **AND** stdout SHALL be empty

#### Scenario: Near-miss under a discovered group gets a suggestion
- **WHEN** command `test unit` exists and the user runs `invowk cmd test uint`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL suggest `unit`

#### Scenario: Bare discovered group still shows help
- **WHEN** the user runs `invowk cmd test`
- **THEN** invowk SHALL print the group's help to stdout
- **AND** invowk SHALL exit with status 0

#### Scenario: Typo under a built-in group
- **WHEN** the user runs `invowk config bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Bare built-in group still shows help
- **WHEN** the user runs `invowk module`
- **THEN** invowk SHALL print the group's help to stdout
- **AND** invowk SHALL exit with status 0

#### Scenario: Source-filtered invocation through a group node
- **WHEN** command `deploy prod` exists in both the invowkfile and module `m`, a unique `deploy dev` exists in the invowkfile (so group node `deploy` is registered and the ambiguous leaf is not), and the user runs `invowk cmd deploy prod --ivk-from invowkfile`
- **THEN** invowk SHALL run the invowkfile's `deploy prod` and SHALL NOT report an unmatched token

#### Scenario: Ambiguous name reached through a group node
- **WHEN** command `deploy prod` exists in both the invowkfile and module `m`, a unique `deploy dev` exists in the invowkfile, and the user runs `invowk cmd deploy prod`
- **THEN** invowk SHALL render the ambiguous-command message
- **AND** invowk SHALL exit with status 2

### Requirement: Command resolution errors exit with status 2
Invowk SHALL exit with status 2 in each of these cases:
- the requested command name is not found under `invowk cmd`, including in `--ivk-watch` mode;
- the command name is ambiguous across sources;
- `@source` or `--ivk-from` names a source that does not exist;
- a source filter is given without a command name.

The styled error rendering for these cases SHALL be unchanged.

#### Scenario: Unknown command
- **WHEN** the user runs `invowk cmd nope` and no command `nope` exists
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `command 'nope' not found`

#### Scenario: Ambiguous command
- **WHEN** command `e2` exists in both the invowkfile and module `m`, and the user runs `invowk cmd e2`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain the ambiguous-command message listing both sources

#### Scenario: Unknown source
- **WHEN** the user runs `invowk cmd @bogus test unit`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Unknown command in watch mode
- **WHEN** the user runs `invowk cmd --ivk-watch nope`
- **THEN** invowk SHALL exit with status 2

### Requirement: Argument, flag and runtime-selection validation errors exit with status 2
Invowk SHALL exit with status 2, and SHALL NOT run the command's script, in each of these cases:
- declared positional arguments fail validation: missing required, too many, or a value that does not match the declared type or pattern (`deps.ArgumentValidationError`);
- invowkfile-declared flag values fail validation (`deps.FlagValidationError`);
- `--ivk-runtime` names an unknown runtime mode;
- `--ivk-runtime` names a runtime the command does not allow.

#### Scenario: Missing required argument
- **WHEN** command `a` declares required argument `x` and the user runs `invowk cmd a`
- **THEN** invowk SHALL exit with status 2
- **AND** stderr SHALL contain `Missing required arguments`

#### Scenario: Too many arguments
- **WHEN** command `a` declares one argument and the user runs `invowk cmd a 1 2`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Unknown runtime mode
- **WHEN** the user runs `invowk cmd --ivk-runtime bogus test unit`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Runtime not allowed for the command
- **WHEN** command `test unit` allows only `virtual-sh` and the user runs `invowk cmd --ivk-runtime native test unit`
- **THEN** invowk SHALL exit with status 2

### Requirement: Non-usage failures exit with status 1
Invowk SHALL exit with status 1 for failures that do not come from the shape of the command line. These include:
- config parse errors that abort the command;
- discovery errors and invalid invowkfiles or command trees;
- unsatisfied `depends_on` checks;
- an unsupported host platform;
- a container engine or runtime that is not available;
- command timeout or cancellation;
- any error without a more specific mapping.

#### Scenario: Unsatisfied dependency
- **WHEN** a command depends on a tool that is not installed and the user runs it
- **THEN** invowk SHALL exit with status 1

#### Scenario: Invalid command tree
- **WHEN** an invowkfile declares `args` on a command that also has subcommands, and the user runs `invowk cmd`
- **THEN** invowk SHALL exit with status 1

### Requirement: User command exit codes are forwarded unchanged
When a user-defined command runs and exits with a non-zero status `N` in the range 1–255, invowk SHALL exit with `N`. This holds for every runtime, including when `N` is 1 or 2.

#### Scenario: Script exits with 2
- **WHEN** a command's script runs `exit 2`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Script exits with a custom code
- **WHEN** a command's script runs `exit 42`
- **THEN** invowk SHALL exit with status 42

### Requirement: Per-command exit-code exceptions are documented and preserved
The following built-in exit codes SHALL keep their current meanings and SHALL be listed in the exit-code documentation:
- `invowk audit`: 0 clean, 1 findings, 2 scan or configuration error. Command-line rejections for `audit` SHALL also exit 2.
- `invowk agent` subcommands: 2 on any failure.
- `invowk tui confirm`: 1 when the user declines.
- `invowk tui spin`: the wrapped command's exit code.
- `invowk validate`: 1 when validation finds errors.

#### Scenario: Audit findings keep status 1
- **WHEN** `invowk audit` detects confirmed findings at or above the severity threshold
- **THEN** invowk SHALL exit with status 1

#### Scenario: Audit argument error now exits 2
- **WHEN** the user runs `invowk audit a b c`
- **THEN** invowk SHALL exit with status 2

### Requirement: Undeclared positionals after a leaf command are still forwarded
A leaf command that declares no `args` SHALL keep accepting undeclared positional tokens and passing them to the script as positional parameters. This includes a leaf that also has subcommands, where a token that is not a subcommand name is passed to the leaf. Rejecting such tokens is deferred to the `passthrough` schema change.

#### Scenario: Unmatched token after a leaf with children
- **WHEN** leaf `w` and its child `w sub` both exist and the user runs `invowk cmd w typo`
- **THEN** invowk SHALL run `w` with `typo` as its first positional parameter
- **AND** invowk SHALL exit with the script's exit code

### Requirement: A command that is also a group prefix stays runnable
When command `X` and a command `X Y` both exist, invowk SHALL register `X` as a runnable leaf with `Y` as a subcommand. This SHALL hold whether the two come from the same source or from different sources, and regardless of discovery order. It SHALL NOT register `X` as a help-only group.

#### Scenario: Leaf in the invowkfile, child in a module
- **WHEN** the invowkfile defines `w2` and module `m` defines `w2 sub`, and the user runs `invowk cmd w2`
- **THEN** invowk SHALL run the invowkfile's `w2` command
- **AND** `invowk cmd w2 sub` SHALL run the module's `w2 sub` command

### Requirement: Shell completion is unaffected
Usage-error classification SHALL NOT change the candidates or exit status of Cobra's hidden `__complete` and `__completeNoDesc` commands for any path, including group nodes and the root.

#### Scenario: Completing under a group
- **WHEN** the shell runs `invowk __complete cmd test ""`
- **THEN** the output SHALL list the group's subcommands as before this change
- **AND** invowk SHALL exit with status 0
