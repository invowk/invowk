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
When `invowk module add` or `invowk module remove` edits `invowkmod.cue` without error, it SHALL report the lock file and `invowkmod.cue` separately: the lock as updated only when the save wrote it and as unchanged when the save skipped the write. `invowk module remove` SHALL report `invowkmod.cue` as updated only when at least one of its edits wrote the file, and as unchanged otherwise.

#### Scenario: Add of an already-locked module
- **WHEN** `invowk module add` resolves a module whose entry the lock already holds unchanged, and adds its missing `requires` entry to `invowkmod.cue`
- **THEN** it SHALL print `Module lock file unchanged` and `Updated invowkmod.cue with new requires entry`, and SHALL NOT print `Module lock file updated`

#### Scenario: Add that changes the lock
- **WHEN** `invowk module add` writes a new lock entry and adds the `requires` entry to `invowkmod.cue`
- **THEN** it SHALL print `Module lock file updated` and `Updated invowkmod.cue with new requires entry`

#### Scenario: Remove that changes the lock
- **WHEN** `invowk module remove` deletes lock entries, writes the lock, and removes the matching `requires` entry from `invowkmod.cue`
- **THEN** it SHALL print `Lock file and invowkmod.cue updated`

#### Scenario: Remove with a skipped lock write
- **WHEN** `invowk module remove` removes the matching `requires` entry from `invowkmod.cue` but the save skips the lock write, because the lock file on disk already lacks the removed entries when it is saved
- **THEN** it SHALL print `Lock file unchanged; invowkmod.cue updated` and SHALL NOT print `Lock file and invowkmod.cue updated`

#### Scenario: Remove with no matching requires entry
- **WHEN** `invowk module remove` deletes lock entries and writes the lock, but `invowkmod.cue` has no `requires` entry matching any removed module, or does not exist
- **THEN** it SHALL print `Lock file updated; invowkmod.cue unchanged`, SHALL NOT print `invowkmod.cue updated`, and SHALL leave `invowkmod.cue` byte-identical

#### Scenario: Remove that writes neither file
- **WHEN** `invowk module remove` skips the lock write because the lock file on disk already lacks the removed entries, and `invowkmod.cue` has no matching `requires` entry
- **THEN** it SHALL print `Lock file and invowkmod.cue unchanged`

#### Scenario: invowkmod.cue edit fails
- **WHEN** `invowk module add` or `invowk module remove` fails to edit `invowkmod.cue` and rolls the lock file back
- **THEN** it SHALL print none of the lines above, whether or not the save had written the lock, and SHALL report the rollback as the requirement "A failed invowkmod.cue edit reports the rollback" says

### Requirement: A failed invowkmod.cue edit reports the rollback
When `invowk module add` or `invowk module remove` fails to edit `invowkmod.cue`, it restores the lock file it saved and SHALL say so, whether or not the save had written the lock. It SHALL NOT print a line saying that the lock file was updated or unchanged, and it SHALL NOT print a success summary. A missing `invowkmod.cue` SHALL be recognised as missing even when the edit error wraps it. When restoring a file fails as well, the command SHALL NOT say that the changes were not persisted or were rolled back without showing the restore error.

#### Scenario: Add without invowkmod.cue
- **WHEN** `invowk module add` resolves a module in a directory that has no `invowkmod.cue`, and restoring the lock file succeeds
- **THEN** it SHALL print `invowkmod.cue not found — dependency was not persisted`, SHALL NOT print `Could not auto-edit invowkmod.cue`, and the lock file SHALL be as it was before the command

#### Scenario: Add whose invowkmod.cue edit fails otherwise
- **WHEN** `invowk module add` resolves a module but its `invowkmod.cue` edit fails for a reason other than a missing file, for example because `invowkmod.cue` already has a `requires` entry for that module
- **THEN** it SHALL print `Could not auto-edit invowkmod.cue; lock file changes were rolled back:` followed by the error, and SHALL NOT print `Module lock file updated` or `Module lock file unchanged`

#### Scenario: Remove whose invowkmod.cue edit fails
- **WHEN** `invowk module remove` deletes lock entries but cannot edit `invowkmod.cue`, for example because it cannot be read, and restoring the files succeeds
- **THEN** it SHALL print `Could not auto-edit invowkmod.cue:` followed by the error and the summary `Lock file changes were rolled back; invowkmod.cue needs manual review`, SHALL NOT print `Lock file updated`, `Lock file unchanged` or `Lock file and invowkmod.cue updated`, and the lock file SHALL be byte-identical to the lock before the command

#### Scenario: A restore that fails as well
- **WHEN** the `invowkmod.cue` edit of `invowk module add` or `invowk module remove` fails and restoring the lock file or `invowkmod.cue` fails too
- **THEN** `invowk module add` SHALL print `Could not auto-edit invowkmod.cue; lock file rollback may be incomplete:` followed by the error, which includes the restore error, even when `invowkmod.cue` is missing, and SHALL NOT print `invowkmod.cue not found — dependency was not persisted`; `invowk module remove` SHALL print `Could not auto-edit invowkmod.cue:` followed by the error, which includes the restore error, and the summary `Lock file rollback may be incomplete; invowkmod.cue needs manual review`; neither command SHALL print `Lock file changes were rolled back` or `Lock file updated`

### Requirement: Removing a requires entry reports whether it wrote
Removing a `requires` entry from `invowkmod.cue` SHALL tell its caller whether it wrote the file. It SHALL report a write only when it replaced the file with the entry removed. When the file does not exist, has no `requires` block or has no matching entry, it SHALL leave the file untouched and report no write. A failed removal SHALL return its error and SHALL NOT report a write. The existing way to remove a requirement SHALL keep working for callers that do not need the report.

#### Scenario: Matching entry removed
- **WHEN** a `requires` entry is removed from an `invowkmod.cue` that has an entry with the same Git URL and path
- **THEN** the removal SHALL report that it wrote the file, and the file SHALL no longer contain that entry

#### Scenario: Nothing to remove
- **WHEN** a `requires` entry is removed and `invowkmod.cue` does not exist, has no `requires` block, or has no entry with the same Git URL and path
- **THEN** the removal SHALL report no write, return no error, and leave the file (if any) byte-identical

#### Scenario: Failed removal is not a write
- **WHEN** removing a `requires` entry fails, for example because `invowkmod.cue` cannot be read
- **THEN** the removal SHALL return the error and SHALL NOT report that it wrote the file
