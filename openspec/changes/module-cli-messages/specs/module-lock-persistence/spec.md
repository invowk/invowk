# Spec Delta

## MODIFIED Requirements

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

## ADDED Requirements

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
- **THEN** `invowk module add` SHALL print `Could not auto-edit invowkmod.cue; lock file changes were rolled back:` followed by the error, which includes the restore error, even when `invowkmod.cue` is missing, and SHALL NOT print `invowkmod.cue not found — dependency was not persisted`; `invowk module remove` SHALL print `Could not auto-edit invowkmod.cue:` followed by the error, which includes the restore error, and the summary `Lock file updated; invowkmod.cue needs manual review`, and SHALL NOT print `Lock file changes were rolled back`

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
