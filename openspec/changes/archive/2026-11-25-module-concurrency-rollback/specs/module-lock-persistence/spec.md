# Spec Delta

## MODIFIED Requirements

### Requirement: A failed invowkmod.cue edit reports the rollback
When `invowk module add` or `invowk module remove` fails to edit `invowkmod.cue`, it restores the lock file it saved and SHALL say so, whether or not the save had written the lock. It SHALL NOT print a line saying that the lock file was updated or unchanged, it SHALL NOT print a success summary, and `invowk module remove` SHALL NOT print a `Removed <namespace>` line. A missing `invowkmod.cue` SHALL be recognised as missing even when the edit error wraps it. When restoring a file fails as well, the command SHALL NOT say that the changes were not persisted or were rolled back without showing the restore error. Whether every restore succeeded SHALL be decided from the rollback outcome the operation reports, not from the shape of its error. After any rollback, complete or not, the command SHALL exit with status 1.

#### Scenario: Add without invowkmod.cue
- **WHEN** `invowk module add` resolves a module in a directory that has no `invowkmod.cue`, and restoring the lock file succeeds
- **THEN** it SHALL print `invowkmod.cue not found — dependency was not persisted`, SHALL NOT print `Could not auto-edit invowkmod.cue`, the lock file SHALL be as it was before the command, and it SHALL exit with status 1

#### Scenario: Add whose invowkmod.cue edit fails otherwise
- **WHEN** `invowk module add` resolves a module but its `invowkmod.cue` edit fails for a reason other than a missing file, for example because `invowkmod.cue` already has a `requires` entry for that module
- **THEN** it SHALL print `Could not auto-edit invowkmod.cue; lock file changes were rolled back:` followed by the error, SHALL NOT print `Module lock file updated` or `Module lock file unchanged`, and SHALL exit with status 1

#### Scenario: Remove whose invowkmod.cue edit fails
- **WHEN** `invowk module remove` deletes lock entries but cannot edit `invowkmod.cue`, for example because it cannot be read, and restoring the files succeeds
- **THEN** it SHALL print `Could not auto-edit invowkmod.cue:` followed by the error and the summary `Lock file changes were rolled back; invowkmod.cue needs manual review`, SHALL NOT print `Removed`, `Lock file updated`, `Lock file unchanged` or `Lock file and invowkmod.cue updated`, the lock file SHALL be byte-identical to the lock before the command, and it SHALL exit with status 1

#### Scenario: A restore that fails as well
- **WHEN** the `invowkmod.cue` edit of `invowk module add` or `invowk module remove` fails and restoring the lock file or `invowkmod.cue` fails too
- **THEN** `invowk module add` SHALL print `Could not auto-edit invowkmod.cue; lock file rollback may be incomplete:` followed by the error, which includes the restore error, even when `invowkmod.cue` is missing, and SHALL NOT print `invowkmod.cue not found — dependency was not persisted`; `invowk module remove` SHALL print `Could not auto-edit invowkmod.cue:` followed by the error, which includes the restore error, and the summary `Lock file rollback may be incomplete; invowkmod.cue needs manual review`, and SHALL NOT print `Removed`; neither command SHALL print `Lock file changes were rolled back` or `Lock file updated`; both SHALL exit with status 1

#### Scenario: Rollback outcome is reported by the operation
- **WHEN** the add or remove operation rolls back after a failed `invowkmod.cue` edit
- **THEN** its result SHALL state whether every restore succeeded or at least one failed, and when no edit failed it SHALL state that no rollback happened

## ADDED Requirements

### Requirement: The rollback restores each file atomically and as it was
The rollback after a failed `invowkmod.cue` edit SHALL put `invowkmod.cue` and the lock file back as they were when the command took its snapshot, judging each path without following symbolic links. A regular file SHALL be restored by an atomic replace with its earlier content and permissions, a path that did not exist SHALL be removed, and a symbolic link SHALL be restored as the same link.

#### Scenario: Concurrent reader during a rollback
- **WHEN** a process reads the lock file or `invowkmod.cue` while `invowk module add` restores it
- **THEN** the reader SHALL see the whole earlier file or the whole file that was in place before the restore, never an empty or partial file

#### Scenario: Dangling invowkmod.cue link
- **WHEN** `invowkmod.cue` is a symbolic link whose target does not exist, and `invowk module add` rolls back after its edit fails
- **THEN** `invowkmod.cue` SHALL still be the same dangling link after the command

#### Scenario: Linked lock file
- **WHEN** the lock file is a symbolic link to a regular file, the command's save replaced the link with a regular file, and the command then rolls back
- **THEN** the lock path SHALL again be the same symbolic link, and the link's target SHALL be byte-identical to its content before the command

#### Scenario: Restore of a link fails
- **WHEN** a link cannot be recreated, for example because the platform refuses to create symbolic links
- **THEN** the rollback outcome SHALL be incomplete and the command SHALL report the restore error

### Requirement: Requires entries written on one line are matched
`invowk module add` and `invowk module remove` SHALL find a `requires` entry by its `git_url` and `path` whether the entry spans several lines or is written on one line, with or without other entries on that line. Braces, brackets and field names inside quoted strings or comments SHALL NOT be taken as structure. Whenever add or remove changes a `requires` block written on one line, the resulting `invowkmod.cue` SHALL be valid CUE that `invowk` parses.

#### Scenario: Remove a one-line entry
- **WHEN** `invowkmod.cue` has the entry `{git_url: "https://github.com/user/tools.git", version: "^1.0.0"},` on one line and `invowk module remove` removes that module
- **THEN** the entry SHALL be gone from `invowkmod.cue`, the other entries and the rest of the file SHALL be unchanged, and the command SHALL report `invowkmod.cue` as updated

#### Scenario: Remove one of two entries on one line
- **WHEN** one line holds two entries and `invowk module remove` removes one of them
- **THEN** the other entry SHALL remain, and `invowkmod.cue` SHALL stay valid CUE

#### Scenario: Remove the only entry of a one-line block
- **WHEN** `invowkmod.cue` has `requires: [{git_url: "…", version: "…"}]` on one line and `invowk module remove` removes that module
- **THEN** the `requires` field SHALL be removed and `invowkmod.cue` SHALL stay valid CUE

#### Scenario: Add a module already required on one line
- **WHEN** `invowkmod.cue` requires a module through a one-line entry and `invowk module add` adds the same `git_url` and `path`
- **THEN** the edit SHALL fail as a duplicate, `invowkmod.cue` SHALL be unchanged, and the command SHALL roll back the lock and exit with status 1

#### Scenario: Add to a one-line requires block
- **WHEN** `invowkmod.cue` has `requires: [{git_url: "https://github.com/user/tools.git", version: "^1.0.0"}]` on one line and `invowk module add` adds a different module
- **THEN** the new entry SHALL be inside the `requires` list after the existing entry, the existing entry and everything outside the `requires` field SHALL be unchanged, and `invowkmod.cue` SHALL be valid CUE

#### Scenario: Multi-line entries keep today's edit
- **WHEN** `invowk module add` or `invowk module remove` edits an `invowkmod.cue` whose entries use the multi-line layout `module add` writes
- **THEN** the resulting file SHALL be byte-identical to what the command wrote before this change
