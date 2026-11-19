# module-lock-persistence Specification

## Purpose
Defines when Invowk writes the module lock file `invowkmod.lock.cue`, so that module commands leave an unchanged lock byte-identical and record a new generation time only when the locked content changes.

## Requirements

### Requirement: Unchanged lock is not rewritten
When a module command would save a lock whose format version and module entries equal those of the valid lock file already on disk, Invowk SHALL NOT write the file. The file SHALL stay byte-identical, its `generated:` value and formatting included. This SHALL hold for `module sync`, `module add`, `module remove` and `module update`.

#### Scenario: Repeated sync with unchanged modules
- **WHEN** `invowk module sync` runs twice and every requirement resolves to the same locked version, commit and content hash
- **THEN** the lock file after the second run SHALL be byte-identical to the lock file after the first run, even if the runs fall in different seconds

#### Scenario: Equivalent lock with an older timestamp
- **WHEN** the lock file on disk has a `generated:` value in the past and a module command saves a lock with the same format version and module entries
- **THEN** the lock file SHALL keep that past `generated:` value and its exact bytes

#### Scenario: Equivalent lock written in another layout
- **WHEN** the lock file on disk is valid and equal in meaning to the lock being saved, but its layout differs from the writer's canonical output (spacing, field order, or an equivalent time-zone notation)
- **THEN** Invowk SHALL NOT rewrite it

#### Scenario: Update with nothing to change
- **WHEN** `invowk module update` runs against a valid lock file and no module's locked entry changes
- **THEN** the lock file SHALL be byte-identical afterwards

### Requirement: Changed lock gets a fresh generation time
When the lock being saved differs from the valid lock on disk in its format version or in any module entry (a module added, removed, or with any changed field), Invowk SHALL write the lock and SHALL set `generated:` to the time of that write. This SHALL apply to every command that saves the lock.

#### Scenario: Module entry changes
- **WHEN** a module command saves a lock whose module entries differ from the lock on disk, whose `generated:` value is in the past
- **THEN** the written lock SHALL contain the new entries and a `generated:` value later than the old one

#### Scenario: Add, remove or update changes content
- **WHEN** `invowk module add`, `invowk module remove` or `invowk module update` changes the lock's module entries
- **THEN** the written lock's `generated:` value SHALL be the time of that write, not the value loaded from the previous lock

### Requirement: Missing or invalid lock is written
When no lock file exists, or the existing file cannot be read or parsed as a valid lock, saving SHALL write the lock with `generated:` set to the time of the write, as an atomic replace.

#### Scenario: First save
- **WHEN** a module command saves a lock and no lock file exists
- **THEN** Invowk SHALL create the lock file with the current `generated:` time

#### Scenario: Unparseable existing file
- **WHEN** a lock is saved over a file that does not parse as a valid lock
- **THEN** Invowk SHALL replace the file with the new lock
