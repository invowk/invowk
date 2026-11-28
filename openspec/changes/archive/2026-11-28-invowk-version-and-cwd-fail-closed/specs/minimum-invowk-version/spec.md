# Spec Delta

## Purpose

Defines the optional `invowk_version` SemVer constraint field on `invowkmod.cue` and `invowkfile.cue`, pre-pass evaluation before strict CUE schema decoding, `dev` build detection via `debug.ReadBuildInfo()`, and upgrade hints on unknown schema fields.

## ADDED Requirements

### Requirement: #Invowkmod and #Invowkfile support optional invowk_version SemVer constraint
`#Invowkmod` and `#Invowkfile` SHALL accept an optional `invowk_version` field containing a valid SemVer constraint (`>=`, `^`, `~`, exact, etc.).

#### Scenario: Valid invowk_version parses and validates cleanly
- **WHEN** `invowkmod.cue` or `invowkfile.cue` declares `invowk_version: ">=0.18.0"` and the current Invowk version is `0.18.0` or newer
- **THEN** parsing SHALL succeed without error

### Requirement: invowk_version is checked in a schema-less pre-pass before strict decoding
Before strict `close({...})` schema decoding, parsers SHALL evaluate `invowk_version` in a schema-less CUE pass against the normalized release version (`ldflags` or `debug.ReadBuildInfo().Main.Version`). Unsatisfied constraints SHALL fail with exit status `1` and an upgrade message naming the required constraint and running version, even when unknown newer schema fields are present.

#### Scenario: Older binary rejects newer file with clear version error despite unknown fields
- **WHEN** Invowk version `0.18.0` parses an `invowkfile.cue` declaring `invowk_version: ">=0.19.0"` alongside a field unknown to `0.18.0`
- **THEN** parsing SHALL fail with an `invowk_version` mismatch error instead of `field not allowed`, and Invowk SHALL exit with status `1`

### Requirement: True dev builds warn and continue on unsatisfied invowk_version
When neither ldflags nor `debug.ReadBuildInfo().Main.Version` supplies a release version (a true `(devel)` build), an unsatisfied `invowk_version` constraint SHALL emit a warning and allow schema decoding to proceed.

#### Scenario: True dev build warns and proceeds
- **WHEN** a `(devel)` build without ldflags or module release version parses a file with `invowk_version: ">=99.0.0"`
- **THEN** Invowk SHALL emit a warning and proceed to schema decoding

### Requirement: Unknown schema fields emit a version-upgrade hint
When strict CUE schema decoding fails with a `field not allowed` error, the formatted error SHALL include a hint noting that the file may require a newer version of Invowk.

#### Scenario: field not allowed appends upgrade hint
- **WHEN** an `invowkfile.cue` omits `invowk_version` and uses an unknown field rejected as `field not allowed`
- **THEN** stderr SHALL contain the CUE field error together with the newer-version hint
