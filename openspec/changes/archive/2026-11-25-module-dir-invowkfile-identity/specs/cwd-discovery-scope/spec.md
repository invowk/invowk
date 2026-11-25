# Spec Delta

## MODIFIED Requirements

### Requirement: Switch disables current-directory command sources

invowk SHALL provide a root persistent boolean flag `--ivk-no-cwd` and an environment variable `INVOWK_NO_CWD`. When the switch is on, command discovery SHALL NOT load the current directory's `invowkfile.cue` (source `invowkfile`), the sibling `*.invowkmod` directories in the current directory, or the vendored modules of those directories. When the current directory is itself a module directory, the switch SHALL also skip that module and its vendored modules.

#### Scenario: cwd invowkfile and modules are skipped
- **WHEN** the current directory contains `invowkfile.cue` with command `local` and `foo.invowkmod` with command `modcmd`, and `invowk --ivk-no-cwd cmd` runs
- **THEN** neither `local` nor `foo modcmd` is listed

#### Scenario: cwd-only command is not found
- **WHEN** `invowk --ivk-no-cwd cmd local` runs and `local` is defined only in the cwd `invowkfile.cue`
- **THEN** the exit code is non-zero and the command is reported as not found

#### Scenario: invowkfile source filter has nothing to match
- **WHEN** `invowk --ivk-no-cwd cmd @invowkfile local` runs
- **THEN** the exit code is non-zero and the source `invowkfile` is reported as not found

#### Scenario: a module directory as the current directory is skipped
- **WHEN** the current directory is `foo.invowkmod` with command `modcmd`, no other source provides `foo`, and `invowk --ivk-no-cwd cmd foo modcmd` runs
- **THEN** the exit code is non-zero and the command is reported as not found
