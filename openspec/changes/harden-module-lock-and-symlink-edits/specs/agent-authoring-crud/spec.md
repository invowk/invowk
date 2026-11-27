# Spec Delta: agent-authoring-crud

## MODIFIED Requirements

### Requirement: Module change updates existing local module
Invowk SHALL provide `invowk agent mod change <module-id-or-path> [description...]` for LLM-assisted updates to an existing local module's CUE files.

#### Scenario: Module change updates module CUE files
- **WHEN** a local module `com.example.tools.invowkmod` exists
- **AND** a user runs `invowk agent mod change com.example.tools 'add a lint command'`
- **THEN** Invowk SHALL send the current `invowkmod.cue` and `invowkfile.cue` content to the LLM provider
- **AND** Invowk SHALL write validated updates only to those module CUE files
- **AND** the module ID SHALL remain `com.example.tools`

#### Scenario: Module change rejects missing module
- **WHEN** no local module can be resolved from `com.example.tools`
- **AND** a user runs `invowk agent mod change com.example.tools 'add a lint command'`
- **THEN** Invowk SHALL fail without writing files
- **AND** the error SHALL direct the user to `invowk agent mod create com.example.tools`

#### Scenario: Module change rejects arbitrary file writes
- **WHEN** the LLM response attempts to create or modify files other than `invowkmod.cue` and `invowkfile.cue`
- **THEN** Invowk SHALL reject those file writes for this change
- **AND** Invowk SHALL NOT create or modify arbitrary script files

#### Scenario: Module change preserves regular-file symlinks
- **WHEN** `invowkmod.cue` or `invowkfile.cue` is a symbolic link to an existing regular file during module change read and write operations
- **THEN** Invowk SHALL read from and atomically replace the resolved regular target file, preserve the target file's permissions, and keep the symbolic link intact

#### Scenario: Module change fails closed on dangling or non-regular symlinks
- **WHEN** `invowkmod.cue` or `invowkfile.cue` is a dangling symbolic link or resolves to a non-regular file during module change read or write operations
- **THEN** Invowk SHALL fail with an error without falling back to default file contents and without modifying or replacing the symbolic link or any target
