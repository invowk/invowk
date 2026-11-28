# Spec Delta

## Purpose

Adds fail-closed discovery behavior for broken current-directory sources when `--ivk-no-cwd` / `INVOWK_NO_CWD` is off, while preserving cwd skip behavior when the switch is on.

## ADDED Requirements

### Requirement: Invalid current-directory sources fail closed when the switch is off
When `--ivk-no-cwd` / `INVOWK_NO_CWD` is off, if the current directory's `invowkfile.cue` or any sibling `*.invowkmod` fails parse, schema, or `invowk_version` validation, `invowk cmd` execution and resolution SHALL fail closed immediately with exit status `1` and surface the validation error rather than silently falling back to global or included sources. When the switch is on, broken current-directory sources SHALL remain ignored as specified above.

#### Scenario: Broken cwd module fails closed when switch is off
- **WHEN** `./cst.invowkmod/invowkfile.cue` has a syntax error, `~/.invowk/cmds/cst.invowkmod` is valid, and `invowk cmd cst doctor` runs without `--ivk-no-cwd`
- **THEN** invowk SHALL exit with status `1`, report the error in `./cst.invowkmod/invowkfile.cue`, and SHALL NOT execute the global module

#### Scenario: Broken cwd module is ignored when switch is on
- **WHEN** `./cst.invowkmod/invowkfile.cue` has a syntax error and `invowk --ivk-no-cwd cmd` runs
- **THEN** the broken cwd module is skipped because current-directory discovery is disabled
