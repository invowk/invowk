## MODIFIED Requirements

### Requirement: Explicit config failures outside the exit-1 flows

`invowk audit` SHALL report a failed explicit config source through its documented scan-error exit code, 2, not 1 (`audit` uses 1 for findings). `invowk agent` SHALL follow the general exit-code contract and exit 1. Shell completion SHALL offer no candidates. `invowk cmd --help` MAY render help without discovered commands. None of them SHALL fall back to the user config file or to defaults.

#### Scenario: audit reports the config failure as an audit error
- **WHEN** `INVOWK_CONFIG` names an absolute path that does not exist and `invowk audit` runs on a target that needs config
- **THEN** the exit code is audit's error code, not its findings code, and no scan result is printed

#### Scenario: agent reports the config failure as an agent error
- **WHEN** `--ivk-config` names a missing file and an `invowk agent cmd create` command runs
- **THEN** the exit code is 1, no LLM request is made and no file is written

#### Scenario: completion offers nothing
- **WHEN** `--ivk-config` names a missing file and `invowk __complete cmd ''` runs in a directory with an `invowkfile.cue`
- **THEN** no command candidates are offered
