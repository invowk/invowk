# Spec Delta

## MODIFIED Requirements

### Requirement: Command authoring modes remain safe
Invowk SHALL support explicit non-writing and verification modes for command create and change operations. Verification SHALL resolve the written command from the command source discovery publishes it under, so that a command written in a module directory is verified under that module's namespace.

#### Scenario: Command dry-run prints patch only
- **WHEN** a user runs `invowk agent cmd create lint --dry-run 'run lint'`
- **THEN** Invowk SHALL validate the generated command and print the planned patch
- **AND** Invowk SHALL NOT write the target invowkfile

#### Scenario: Command print emits generated command only
- **WHEN** a user runs `invowk agent cmd create lint --print 'run lint'`
- **THEN** Invowk SHALL validate the generated command and print the generated command CUE object
- **AND** Invowk SHALL NOT write the target invowkfile

#### Scenario: Command verify checks written command
- **WHEN** a user runs `invowk agent cmd create lint --verify 'run lint'`
- **THEN** Invowk SHALL write the command after validation succeeds
- **AND** Invowk SHALL verify the written command with a dry-run execution plan

#### Scenario: Command verify in a module directory
- **WHEN** a user runs `invowk agent cmd create lint --verify 'run lint'` (or `invowk agent cmd change lint --verify …`) with `tools.invowkmod` as the current directory
- **THEN** Invowk SHALL write `lint` into the module's `invowkfile.cue`
- **AND** the dry-run verification SHALL resolve `lint` from module `tools` and succeed

#### Scenario: Command mode flags remain mutually exclusive
- **WHEN** a user combines `--dry-run`, `--print`, or `--verify` in an invalid mode combination
- **THEN** Invowk SHALL fail before calling an LLM provider
- **AND** Invowk SHALL explain the invalid mode combination
