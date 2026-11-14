## ADDED Requirements

### Requirement: Switch to disable current-directory command sources

invowk SHALL provide a root persistent boolean flag `--ivk-no-cwd` and an environment variable `INVOWK_NO_CWD`. When the switch is on, command discovery SHALL NOT load the current directory's `invowkfile.cue` (source `invowkfile`). It SHALL NOT load sibling `*.invowkmod` directories in the current directory, or the vendored modules of those directories. Configured `includes`, the user commands directory (`~/.invowk/cmds`), provisioned module entries and their vendored modules SHALL still be discovered.

#### Scenario: cwd invowkfile and modules are skipped
- **WHEN** the current directory contains `invowkfile.cue` with command `local` and `foo.invowkmod` with command `modcmd`, and `invowk --ivk-no-cwd cmd` runs
- **THEN** neither `local` nor `foo modcmd` is listed

#### Scenario: configured includes still apply
- **WHEN** the selected config declares `includes` with an absolute path to `bar.invowkmod` containing command `inc`, and `invowk --ivk-no-cwd cmd bar inc` runs in a directory that also has an `invowkfile.cue`
- **THEN** `bar inc` runs successfully

#### Scenario: user commands directory still applies
- **WHEN** `~/.invowk/cmds/g.invowkmod` provides command `glob` and `INVOWK_NO_CWD=1 invowk cmd g glob` runs
- **THEN** `g glob` runs successfully

#### Scenario: cwd-only command is not found
- **WHEN** `invowk --ivk-no-cwd cmd local` runs and `local` is defined only in the cwd `invowkfile.cue`
- **THEN** the exit code is non-zero and the command is reported as not found

#### Scenario: invowkfile source filter has nothing to match
- **WHEN** `invowk --ivk-no-cwd cmd @invowkfile local` runs
- **THEN** the exit code is non-zero and the source `invowkfile` is reported as not found

### Requirement: Switch value parsing and precedence

`INVOWK_NO_CWD` SHALL be parsed with Go `strconv.ParseBool` semantics: `1`, `t`, `T`, `TRUE`, `true`, `True` turn it on, and `0`, `f`, `F`, `FALSE`, `false`, `False` turn it off. An unset or empty value SHALL mean off. Any other value SHALL make a flow that resolves the switch fail with a non-zero exit and an error that names `INVOWK_NO_CWD` and lists the accepted values. Only the flows listed under "Commands affected by the switch" resolve it, so `--version`, `config path` and the other unaffected commands SHALL NOT fail because of it. An explicitly given `--ivk-no-cwd[=bool]` SHALL override the environment variable, and in that case the environment value SHALL NOT be parsed or validated.

#### Scenario: environment variable turns the switch on
- **WHEN** `INVOWK_NO_CWD=1` and `invowk cmd` runs in a directory with an `invowkfile.cue`
- **THEN** cwd commands are not listed

#### Scenario: explicit flag false overrides the environment variable
- **WHEN** `INVOWK_NO_CWD=1` and `invowk --ivk-no-cwd=false cmd local` runs
- **THEN** the cwd `invowkfile.cue` is discovered and `local` runs

#### Scenario: invalid environment value rejected
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk cmd` runs
- **THEN** the exit code is non-zero and stderr names `INVOWK_NO_CWD` and the accepted values

#### Scenario: invalid environment value ignored by unaffected commands
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk config path` runs
- **THEN** the command succeeds

#### Scenario: explicit flag skips environment validation
- **WHEN** `INVOWK_NO_CWD=yes` and `invowk --ivk-no-cwd cmd` runs
- **THEN** the listing runs with the switch on and no error about `INVOWK_NO_CWD` is reported

### Requirement: Commands affected by the switch

The switch SHALL apply to every flow that discovers commands as runnable sources. These are `invowk cmd` (listing, execution, `--ivk-from` and `@source` filtering, `--ivk-dry-run`, `--ivk-watch`), shell completion of `invowk cmd`, the dynamic registration of discovered commands that runs before flag parsing, workspace `invowk validate` (no path argument), `invowk module list` (it lists discovered command-source modules) and hidden `invowk internal` commands that use the discovery service. The flag SHALL take effect in the pre-parse registration step whether it appears before or after the `cmd` token. A `--` terminator SHALL end the scan.

The switch SHALL NOT change commands whose directory or file is an explicit target or the object being worked on, rather than a command source. These are `invowk audit [path]`, `invowk validate <path>`, `invowk module` subcommands other than `list` (they operate on `./invowkmod.cue` or an explicit target), `invowk init` and `invowk config *`. `invowk agent cmd create --verify` and `invowk agent cmd change --verify` dry-run the cwd `invowkfile.cue`, so they SHALL be rejected with a clear error when the switch is on.

#### Scenario: flag after the cmd token reaches dynamic registration
- **WHEN** `invowk cmd --ivk-no-cwd local` runs with `local` defined only in the cwd invowkfile
- **THEN** `local` is not registered as a subcommand and the exit code is non-zero

#### Scenario: completion omits cwd commands
- **WHEN** shell completion is requested with `invowk __complete --ivk-no-cwd cmd ""` in a directory with an `invowkfile.cue`
- **THEN** cwd commands are not offered

#### Scenario: workspace validation skips cwd sources
- **WHEN** `invowk --ivk-no-cwd validate` runs in a directory whose `invowkfile.cue` is invalid
- **THEN** the invalid cwd file is not reported, because it is not discovered

#### Scenario: explicit validate target is unaffected
- **WHEN** `invowk --ivk-no-cwd validate ./invowkfile.cue` runs
- **THEN** the named file is validated as it is without the switch

#### Scenario: module list omits cwd modules
- **WHEN** `invowk --ivk-no-cwd module list` runs in a directory that contains `foo.invowkmod`
- **THEN** `foo` is not listed, but modules from `includes` and `~/.invowk/cmds` are

#### Scenario: audit target is unaffected
- **WHEN** `invowk --ivk-no-cwd audit .` runs
- **THEN** the audit scans the current directory as its explicit target, as it does without the switch

#### Scenario: agent verify rejected under the switch
- **WHEN** `invowk --ivk-no-cwd agent cmd create --verify …` runs
- **THEN** the exit code is non-zero and the error says that `--verify` needs current-directory discovery, which `--ivk-no-cwd` disables

### Requirement: Nested invocations inherit the switch and the config variable

`INVOWK_NO_CWD` and `INVOWK_CONFIG` SHALL NOT be added to the nested-invocation environment filter. Native and virtual runtime scripts that inherit the host environment SHALL pass them on unchanged, so a nested `invowk` call keeps the caller's trust settings. The `--ivk-no-cwd` flag on its own SHALL NOT be translated into an environment variable for children. The container runtime does not inherit the host environment, so these values SHALL NOT reach containers.

#### Scenario: nested native invocation keeps the switch
- **WHEN** `INVOWK_NO_CWD=1` and a native command with the default env-inherit mode runs `invowk cmd` in a directory with an `invowkfile.cue`
- **THEN** the nested listing omits cwd commands
