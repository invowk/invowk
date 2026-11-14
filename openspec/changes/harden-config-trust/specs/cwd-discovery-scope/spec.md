## Purpose

This capability defines the `--ivk-no-cwd` flag and the `INVOWK_NO_CWD` environment variable, which stop command discovery from loading the current directory's `invowkfile.cue` and sibling `*.invowkmod` modules. It defines how the switch is parsed, which command sources stay available, which commands it applies to and which it leaves alone, and how it reaches nested invocations. It lets wrappers, CI jobs and root-run tooling run only commands from trusted sources (configured `includes`, the user commands directory and provisioned modules).

## ADDED Requirements

### Requirement: Switch disables current-directory command sources

invowk SHALL provide a root persistent boolean flag `--ivk-no-cwd` and an environment variable `INVOWK_NO_CWD`. When the switch is on, command discovery SHALL NOT load the current directory's `invowkfile.cue` (source `invowkfile`), the sibling `*.invowkmod` directories in the current directory, or the vendored modules of those directories.

#### Scenario: cwd invowkfile and modules are skipped
- **WHEN** the current directory contains `invowkfile.cue` with command `local` and `foo.invowkmod` with command `modcmd`, and `invowk --ivk-no-cwd cmd` runs
- **THEN** neither `local` nor `foo modcmd` is listed

#### Scenario: cwd-only command is not found
- **WHEN** `invowk --ivk-no-cwd cmd local` runs and `local` is defined only in the cwd `invowkfile.cue`
- **THEN** the exit code is non-zero and the command is reported as not found

#### Scenario: invowkfile source filter has nothing to match
- **WHEN** `invowk --ivk-no-cwd cmd @invowkfile local` runs
- **THEN** the exit code is non-zero and the source `invowkfile` is reported as not found

### Requirement: Non-cwd command sources stay available under the switch

When the switch is on, configured `includes`, the user commands directory (`~/.invowk/cmds`), provisioned module entries, provisioned global module entries and the vendored modules of all of these SHALL still be discovered, in their usual precedence order.

#### Scenario: configured includes still apply
- **WHEN** the selected config declares `includes` with an absolute path to `bar.invowkmod` containing command `inc`, and `invowk --ivk-no-cwd cmd bar inc` runs in a directory that also has an `invowkfile.cue`
- **THEN** `bar inc` runs successfully

#### Scenario: user commands directory still applies
- **WHEN** `~/.invowk/cmds/g.invowkmod` provides command `glob` and `INVOWK_NO_CWD=1 invowk cmd g glob` runs
- **THEN** `g glob` runs successfully

#### Scenario: provisioned modules still apply
- **WHEN** the discovery service is given provisioned module entries and the switch is on
- **THEN** the provisioned modules' commands are discovered

### Requirement: INVOWK_NO_CWD value parsing

`INVOWK_NO_CWD` SHALL be parsed with Go `strconv.ParseBool`: `1`, `t`, `T`, `TRUE`, `true`, `True` turn the switch on, and `0`, `f`, `F`, `FALSE`, `false`, `False` turn it off. An unset or empty value SHALL mean off. Any other value SHALL make a flow that resolves the switch fail with a non-zero exit and an error that names `INVOWK_NO_CWD` and lists the accepted values.

#### Scenario: environment variable turns the switch on
- **WHEN** `INVOWK_NO_CWD=1` and `invowk cmd` runs in a directory with an `invowkfile.cue`
- **THEN** cwd commands are not listed

#### Scenario: empty environment variable means off
- **WHEN** `INVOWK_NO_CWD` is the empty string and `invowk cmd local` runs with `local` defined in the cwd `invowkfile.cue`
- **THEN** `local` runs

#### Scenario: invalid environment value rejected
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk cmd` runs
- **THEN** the exit code is non-zero and stderr names `INVOWK_NO_CWD` and the accepted values

### Requirement: Explicit flag overrides the environment variable

An explicitly given `--ivk-no-cwd` or `--ivk-no-cwd=<bool>` SHALL decide the switch on its own. In that case `INVOWK_NO_CWD` SHALL be neither parsed nor validated.

#### Scenario: explicit flag false overrides the environment variable
- **WHEN** `INVOWK_NO_CWD=1` and `invowk --ivk-no-cwd=false cmd local` runs
- **THEN** the cwd `invowkfile.cue` is discovered and `local` runs

#### Scenario: explicit flag skips environment validation
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk --ivk-no-cwd cmd` runs
- **THEN** the listing runs with the switch on and no error about `INVOWK_NO_CWD` is reported

### Requirement: Only affected flows resolve the switch

The switch SHALL be resolved only by the flows that discover command sources (see "Commands affected by the switch"). Other commands, including `invowk --version`, `invowk config *`, `invowk init` and `invowk audit`, SHALL NOT fail because `INVOWK_NO_CWD` holds an invalid value.

#### Scenario: invalid environment value ignored by config path
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk config path` runs
- **THEN** the command exits 0

#### Scenario: invalid environment value ignored by version
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk --version` runs
- **THEN** the command exits 0 and prints the version

### Requirement: Commands affected by the switch

The switch SHALL apply to `invowk cmd` (listing, execution, `--ivk-from` and `@source` filtering, `--ivk-dry-run`, `--ivk-watch`), shell completion of `invowk cmd`, the dynamic registration of discovered commands that runs before flag parsing, workspace `invowk validate` (no path argument), `invowk module list`, and hidden `invowk internal` commands that use the discovery service.

#### Scenario: completion omits cwd commands
- **WHEN** shell completion is requested with `invowk __complete --ivk-no-cwd cmd ""` in a directory with an `invowkfile.cue`
- **THEN** cwd commands are not offered

#### Scenario: workspace validation skips cwd sources
- **WHEN** `invowk --ivk-no-cwd validate` runs in a directory whose `invowkfile.cue` is invalid
- **THEN** the invalid cwd file is not reported, because it is not discovered

#### Scenario: module list omits cwd modules
- **WHEN** `invowk --ivk-no-cwd module list` runs in a directory that contains `foo.invowkmod`
- **THEN** `foo` is not listed, but modules from `includes` and `~/.invowk/cmds` are

#### Scenario: dry-run cannot resolve a cwd command
- **WHEN** `invowk --ivk-no-cwd cmd --ivk-dry-run local` runs with `local` defined only in the cwd `invowkfile.cue`
- **THEN** the exit code is non-zero and the command is reported as not found

### Requirement: Pre-parse registration sees the switch

The dynamic registration step that runs before Cobra parses flags SHALL apply the switch whether `--ivk-no-cwd` appears before or after the `cmd` token, and SHALL fall back to `INVOWK_NO_CWD` when the flag is absent. A `--` terminator SHALL end the scan, so a `--ivk-no-cwd` after `--` is an argument to the user command.

#### Scenario: flag after the cmd token reaches dynamic registration
- **WHEN** `invowk cmd --ivk-no-cwd local` runs with `local` defined only in the cwd invowkfile
- **THEN** `local` is not registered as a subcommand and the exit code is non-zero

#### Scenario: flag after the terminator is not the switch
- **WHEN** `invowk cmd local -- --ivk-no-cwd` runs with `local` defined in the cwd invowkfile
- **THEN** `local` runs and receives `--ivk-no-cwd` as an argument

### Requirement: Explicit-target commands are unaffected

The switch SHALL NOT change commands whose directory or file is an explicit target or the object being worked on, rather than a command source: `invowk audit [path]`, `invowk validate <path>`, `invowk module` subcommands other than `list`, `invowk init`, `invowk config *` and `invowk tui *`.

#### Scenario: explicit validate target is unaffected
- **WHEN** `invowk --ivk-no-cwd validate ./invowkfile.cue` runs
- **THEN** the named file is validated as it is without the switch

#### Scenario: audit target is unaffected
- **WHEN** `invowk --ivk-no-cwd audit .` runs
- **THEN** the audit scans the current directory as its explicit target, as it does without the switch

### Requirement: agent verify is rejected under the switch

`invowk agent cmd create --verify` and `invowk agent cmd change --verify` dry-run the cwd `invowkfile.cue`. When the switch is on, they SHALL be rejected with a non-zero exit before any generation starts, with an error saying that `--verify` needs current-directory discovery, which the switch disables.

#### Scenario: agent verify rejected under the switch
- **WHEN** `invowk --ivk-no-cwd agent cmd create --verify …` runs
- **THEN** the exit code is non-zero, no LLM request is made and no file is written, and the error names `--verify` and `--ivk-no-cwd`

### Requirement: The switch is visible in verbose listing and workspace validation

When the switch is on, `invowk --ivk-verbose cmd` (listing) SHALL print the line `current-directory sources disabled (<origin>)` on stdout in its "Discovery Sources" section, and workspace `invowk validate` SHALL print the same line in its header. `<origin>` SHALL be `--ivk-no-cwd` when the flag decided the switch and `INVOWK_NO_CWD` otherwise.

#### Scenario: verbose listing names the switch
- **WHEN** `invowk --ivk-verbose --ivk-no-cwd cmd` runs
- **THEN** stdout contains `current-directory sources disabled (--ivk-no-cwd)`

#### Scenario: workspace validation names the environment origin
- **WHEN** `INVOWK_NO_CWD=1` and `invowk validate` runs with no path argument
- **THEN** stdout contains `current-directory sources disabled (INVOWK_NO_CWD)`

#### Scenario: non-verbose listing stays unchanged
- **WHEN** `invowk --ivk-no-cwd cmd` runs without `--ivk-verbose`
- **THEN** stdout does not contain `current-directory sources disabled`

### Requirement: Nested invocations see only the environment forms

`INVOWK_NO_CWD` and `INVOWK_CONFIG` SHALL NOT be added to the nested-invocation environment filter, and invowk SHALL NOT export either variable to child processes. A child sees them only when the caller set them, subject to the runtime's normal `env_inherit_mode` rules. The `--ivk-no-cwd` and `--ivk-config` flags SHALL NOT be translated into environment variables for children.

#### Scenario: inherited environment keeps the switch
- **WHEN** `INVOWK_NO_CWD=1` and `INVOWK_CONFIG` are set in the caller's environment and a native command with `env_inherit_mode: "all"` runs
- **THEN** the command's environment contains both variables with the caller's values

#### Scenario: flag forms are not exported
- **WHEN** `invowk --ivk-no-cwd --ivk-config /abs/c.cue cmd x` runs a native command and neither variable is set in the caller's environment
- **THEN** the command's environment contains neither `INVOWK_NO_CWD` nor `INVOWK_CONFIG`

#### Scenario: env_inherit_mode none still applies
- **WHEN** `INVOWK_NO_CWD=1` is set in the caller's environment and a command with `env_inherit_mode: "none"` runs
- **THEN** the command's environment does not contain `INVOWK_NO_CWD`
