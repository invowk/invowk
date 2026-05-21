## MODIFIED Requirements

### Requirement: Schema coherence changes are covered by tests and docs
Invowk SHALL update tests and documentation for every schema or validation contract changed by this capability, and SHALL remove compatibility-only schema/model/docs leftovers when a clean-break field move is specified.

#### Scenario: Schema sync tests cover changed fields
- **WHEN** CUE schemas or Go JSON-tagged structs are changed
- **THEN** schema sync tests SHALL verify that CUE fields and Go fields remain aligned

#### Scenario: Behavioral tests cover parser parity
- **WHEN** CUE regexes or Go validators are tightened
- **THEN** behavioral tests SHALL cover accepted examples and rejected counterexamples for the affected fields

#### Scenario: Docs describe current validation behavior
- **WHEN** README, website docs, snippets, samples, or generated references mention changed fields
- **THEN** they SHALL describe the current validation behavior and no longer repeat stale schema comments

#### Scenario: Script interpreter field is schema and Go coherent
- **WHEN** `script.interpreter` is added to implementation scripts and custom-check scripts
- **THEN** CUE schema sync and Go behavioral tests SHALL verify the field exists on both script structs with the same JSON spelling, validation length, and interpreter safety semantics

#### Scenario: Virtual filesystem config is schema and Go coherent
- **WHEN** virtual filesystem config moves to `platforms[].virtual.filesystem`
- **THEN** CUE schema sync and Go behavioral tests SHALL verify platform config structs, JSON spelling, default access mode, access enum values, logical path name validation, path value validation, and generated CUE remain aligned

#### Scenario: Clean-break changes do not leave tombstones
- **WHEN** a field is removed as part of a clean-break schema change
- **THEN** user-facing CUE structs SHALL rely on closed-schema unknown-field rejection and SHALL NOT retain explicit legacy tombstones, aliases, ignored fields, or compatibility-only decode paths
