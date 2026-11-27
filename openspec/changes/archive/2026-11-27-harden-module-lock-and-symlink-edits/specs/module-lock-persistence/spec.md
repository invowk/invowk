# Spec Delta: module-lock-persistence

## ADDED Requirements

### Requirement: Manifest edits preserve symlinks to regular files
Adding or removing a `requires` entry in `invowkmod.cue` SHALL preserve a symbolic link that resolves to a regular file by atomically replacing the resolved target file while preserving its permissions, leaving the symbolic link entry intact. If `invowkmod.cue` is a dangling symbolic link, a circular link, or resolves to anything other than a regular file, the edit SHALL fail closed without creating or overwriting any target.

#### Scenario: Add requirement through symlinked invowkmod.cue
- **WHEN** `invowkmod.cue` is a symbolic link to an existing regular file with mode `0600` and a `requires` entry is added
- **THEN** `invowkmod.cue` SHALL remain a symbolic link to the same path, the resolved target file SHALL contain the new requirement and preserve mode `0600`, and the symlink entry SHALL not be replaced by a regular file

#### Scenario: Remove requirement through symlinked invowkmod.cue
- **WHEN** `invowkmod.cue` is a symbolic link to an existing regular file containing a matching `requires` entry and that entry is removed
- **THEN** the removal SHALL report that it wrote, `invowkmod.cue` SHALL remain a symbolic link to the same path, and the resolved target file SHALL no longer contain that entry

#### Scenario: Symlink to directory or special file fails closed
- **WHEN** `invowkmod.cue` is a symbolic link resolving to a directory or non-regular file and an add or remove requirement edit runs
- **THEN** the edit SHALL fail with an error and report no write, leaving the symbolic link and target untouched

## MODIFIED Requirements

### Requirement: Removing a requires entry reports whether it wrote
Removing a `requires` entry from `invowkmod.cue` SHALL tell its caller whether it wrote the file. It SHALL report a write only when it replaced the file with the entry removed. When the path is absent, has no `requires` block or has no matching entry, it SHALL leave the file untouched and report no write. A dangling or non-regular symlink SHALL fail with an error and report no write. The existing way to remove a requirement SHALL keep working for callers that do not need the report.

#### Scenario: Matching entry removed
- **WHEN** a `requires` entry is removed from an `invowkmod.cue` that has an entry with the same Git URL and path
- **THEN** the removal SHALL report that it wrote the file, and the file SHALL no longer contain that entry

#### Scenario: Nothing to remove
- **WHEN** a `requires` entry is removed and `invowkmod.cue` is absent, has no `requires` block, or has no entry with the same Git URL and path
- **THEN** the removal SHALL report no write, return no error, and leave the file (if any) byte-identical

#### Scenario: Failed removal is not a write
- **WHEN** removing a `requires` entry fails, for example because `invowkmod.cue` cannot be read
- **THEN** the removal SHALL return the error and SHALL NOT report that it wrote the file

#### Scenario: Dangling invowkmod.cue symlink fails closed
- **WHEN** `invowkmod.cue` is a dangling symbolic link and removing a `requires` entry runs
- **THEN** the removal SHALL return an error (wrapping not-exist), SHALL NOT report a write, and SHALL leave the dangling symbolic link untouched

### Requirement: The rollback restores each file atomically and as it was
Rollback after a failed `invowkmod.cue` edit SHALL restore each path to its snapshot state without dereferencing the link path itself. Regular files SHALL be restored atomically with earlier content and permissions, absent paths SHALL be removed, and symbolic links SHALL be restored as the same link. When a symbolic link resolved to a regular file at snapshot time, rollback SHALL also atomically restore that resolved target's earlier content and permissions.

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

#### Scenario: Symlinked invowkmod.cue target modified before rollback
- **WHEN** `invowkmod.cue` is a symbolic link to a regular file, its resolved target is modified during an edit before a subsequent failure triggers rollback
- **THEN** `invowkmod.cue` SHALL remain the same symbolic link and the resolved target regular file SHALL be atomically restored to its earlier bytes and permissions
