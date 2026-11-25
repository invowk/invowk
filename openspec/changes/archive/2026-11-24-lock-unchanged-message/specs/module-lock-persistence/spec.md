# Spec Delta

## ADDED Requirements

### Requirement: Saving the lock reports whether it wrote
Saving the lock SHALL tell its caller whether it wrote `invowkmod.lock.cue` or skipped the write under "Unchanged lock is not rewritten". The report SHALL say "skipped" only when the file was left untouched, and "written" only when the new lock replaced or created the file. A failed save SHALL return its error and SHALL NOT report a write. The existing way to save the lock SHALL keep working for callers that do not need the report.

#### Scenario: Skipped save is reported
- **WHEN** a lock is saved over a valid lock file whose format version and module entries are equal to it
- **THEN** the save SHALL report that it skipped the write, and the file SHALL be byte-identical afterwards

#### Scenario: Write is reported
- **WHEN** a lock is saved and no lock file exists, the existing file does not parse as a valid lock, or its content differs
- **THEN** the save SHALL report that it wrote the file

#### Scenario: Failed save is not a write
- **WHEN** saving the lock fails, for example because the lock is invalid or the file cannot be written
- **THEN** the save SHALL return the error and SHALL NOT report that it wrote the file

### Requirement: Sync and update say whether the lock changed
After a successful save, `invowk module sync` and `invowk module update` SHALL print `Lock file updated: invowkmod.lock.cue` when the save wrote the file and `Lock file unchanged: invowkmod.lock.cue` when it skipped the write. `invowk module update` SHALL print that line also when it has no module to update. A sync that finds no requirements saves nothing and SHALL print no lock line.

#### Scenario: Repeated sync
- **WHEN** `invowk module sync` runs twice with the same requirements, and the first run creates or changes the lock
- **THEN** the first run SHALL print `Lock file updated: invowkmod.lock.cue`, the second SHALL print `Lock file unchanged: invowkmod.lock.cue` and not the updated line, and the lock SHALL be byte-identical after the second run

#### Scenario: Update with nothing to change
- **WHEN** `invowk module update` runs against a valid lock file and no module's locked entry changes
- **THEN** it SHALL print `Lock file unchanged: invowkmod.lock.cue` and SHALL NOT print `Lock file updated`

#### Scenario: Update creates a missing lock
- **WHEN** `invowk module update` runs with no lock file and no module to update, and so creates an empty lock
- **THEN** it SHALL print `No modules to update` and `Lock file updated: invowkmod.lock.cue`; a second run SHALL print `Lock file unchanged: invowkmod.lock.cue` and leave the lock byte-identical

### Requirement: Add and remove report each file truthfully
When `invowk module add` or `invowk module remove` edits `invowkmod.cue` without error, it SHALL report the lock file and `invowkmod.cue` separately: the lock as updated only when the save wrote it, and as unchanged when the save skipped the write, whatever happened to `invowkmod.cue`. When an `invowkmod.cue` edit fails, the command SHALL keep the messages it prints today for that case.

#### Scenario: Add of an already-locked module
- **WHEN** `invowk module add` resolves a module whose entry the lock already holds unchanged, and adds its missing `requires` entry to `invowkmod.cue`
- **THEN** it SHALL print `Module lock file unchanged` and `Updated invowkmod.cue with new requires entry`, and SHALL NOT print `Module lock file updated`

#### Scenario: Add that changes the lock
- **WHEN** `invowk module add` writes a new lock entry and adds the `requires` entry to `invowkmod.cue`
- **THEN** it SHALL print `Module lock file updated` and `Updated invowkmod.cue with new requires entry`

#### Scenario: Remove that changes the lock
- **WHEN** `invowk module remove` deletes lock entries, writes the lock, and edits `invowkmod.cue` without error
- **THEN** it SHALL print `Lock file and invowkmod.cue updated`

#### Scenario: Remove with a skipped lock write
- **WHEN** `invowk module remove` edits `invowkmod.cue` without error but the save skips the write, because the lock file on disk already lacks the removed entries when it is saved
- **THEN** it SHALL print `Lock file unchanged; invowkmod.cue updated` and SHALL NOT print `Lock file and invowkmod.cue updated`

#### Scenario: invowkmod.cue edit fails
- **WHEN** `invowk module add` or `invowk module remove` fails to edit `invowkmod.cue` and rolls the lock file back
- **THEN** it SHALL print the same lines as before this change for that case, whether or not the save had written the lock
