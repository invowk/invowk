# Spec Delta

## Purpose

Module commands that write a project's `invowkmod.cue` or lock file, or that fetch a Git source into the shared module cache, serialise through exclusive operating-system locks, so concurrent invowk processes never lose or mix each other's results.

## ADDED Requirements

### Requirement: Module edits hold the project lock
`invowk module add`, `remove`, `sync`, `update`, `tidy` and `vendor` SHALL each hold an exclusive lock on the project directory from before their first read of `invowkmod.cue` or the lock file until after their last write or rollback of either file; `invowk module vendor` SHALL hold it until after its last copy into `invowk_modules/` as well.

#### Scenario: Sync overlaps an add
- **WHEN** `invowk module sync` and `invowk module add` of a module not yet required run at the same time on one project
- **THEN** after both finish successfully, the lock file SHALL have an entry for every `requires` entry of `invowkmod.cue`, including the added module

#### Scenario: Update overlaps an add or a remove
- **WHEN** `invowk module update` runs at the same time as a successful `invowk module add` or `invowk module remove` on one project
- **THEN** after both finish, the added module SHALL be in both files, or the removed module in neither

#### Scenario: Two adds of the same module
- **WHEN** two `invowk module add` commands for the same module run at the same time on one project
- **THEN** after both finish, the module SHALL be in `invowkmod.cue` and in the lock file, and the command that found it already required SHALL report a rollback that leaves both files as the other command wrote them

#### Scenario: Vendor overlaps an update
- **WHEN** `invowk module vendor` and `invowk module update` run at the same time on one project
- **THEN** the vendored copies SHALL match the lock file as it was when the vendor finished copying

#### Scenario: Nothing is read before the project lock
- **WHEN** `invowk module add`, `remove`, `sync`, `update`, `tidy` or `vendor` runs and `invowkmod.cue` or the lock file changes between the command's start and the moment it acquires the project lock
- **THEN** the command SHALL act on the files as they are once it holds the lock

### Requirement: When the project lock excludes another command
Two commands that hold the project lock SHALL NOT overlap inside the span "Module edits hold the project lock" states when they run on one machine against the same module cache root and their project directory resolves to the same project lock file.

#### Scenario: Same project, same cache root
- **WHEN** two such commands run at the same time on one machine, on one project directory, with the same module cache root
- **THEN** the second SHALL wait until the first has released the project lock

#### Scenario: Known limits of the project lock
- **WHEN** two commands on one project use different module cache roots (different `INVOWK_MODULES_PATH` values), or name the project directory by two letter cases on a case-insensitive macOS volume
- **THEN** they MAY use different project lock files and MAY overlap, and the documentation SHALL state both limits

### Requirement: Fetching a Git source holds the source lock
A module command that checks out a Git source into the module cache SHALL hold an exclusive lock on that source from before the checkout until the module copy taken from it has been verified and read. The lock SHALL be keyed on the cache's checkout of the source, so every URL that maps to that checkout shares it.

#### Scenario: Two projects fetch two versions of one source
- **WHEN** two projects that share a module cache add two different versions of the same Git repository at the same time
- **THEN** each project's lock entry SHALL record a content hash equal to the content of the commit that entry records

### Requirement: Lock order
A command SHALL take the project lock before any source lock, SHALL hold at most one source lock at a time, and SHALL NOT request a project lock while it holds a source lock.

#### Scenario: Sync that fetches
- **WHEN** `invowk module sync` fetches a Git source
- **THEN** it SHALL already hold the project lock when it requests the source lock, and SHALL release the source lock before it requests another one

#### Scenario: Reverse order is refused
- **WHEN** code holding a source lock requests a project lock
- **THEN** the request SHALL fail with an error and SHALL NOT wait

### Requirement: Lock files
Lock files SHALL be empty files in the `locks` directory of the module cache root (`$INVOWK_MODULES_PATH`, else `~/.invowk/modules`); that directory SHALL be created with mode 0700 when missing. A project's lock SHALL be named `project-<hex SHA-256 of the project directory's physical absolute path>.lock`, a source's `source-<hex SHA-256 of the source checkout key>.lock`. Lock files SHALL NOT be deleted by invowk.

#### Scenario: Locks stay out of module content
- **WHEN** a module command takes a project or source lock
- **THEN** the lock file SHALL be in `<cache root>/locks/`, which is never a module directory, an `invowk_modules/` directory, a cache checkout or a cache copy, and the lock file SHALL be part of no content hash, vendored copy, `validate` input or `audit` input, also when the cache root lies inside the project directory

#### Scenario: One project through two paths
- **WHEN** two commands run in one project, one through its real path and one through a symbolic link to it (on Windows also a junction, another letter case or the 8.3 short name)
- **THEN** both SHALL use the same project lock file

#### Scenario: Lock directory created on demand
- **WHEN** `invowk module remove`, or `invowk module sync`, `tidy` or `vendor` on a project with no `requires`, runs while the module cache root does not exist
- **THEN** the command SHALL create the cache root, its `locks` directory and the project lock file, and SHALL create nothing else in the cache

#### Scenario: Lock directory cannot be created
- **WHEN** a module command that takes the project lock cannot determine the module cache root, or cannot create the cache root, its `locks` directory or the lock file
- **THEN** the command SHALL fail with exit status 1 before it reads `invowkmod.cue` or the lock file, with an error that names the path (or says the cache root could not be determined) and says to set `INVOWK_MODULES_PATH` to a writable directory

#### Scenario: Lock released when the holder dies
- **WHEN** the process holding a lock exits or is killed
- **THEN** the operating system SHALL release the lock, and the next command SHALL acquire it without manual cleanup

### Requirement: A waiting command reports and bounds its wait
A command whose lock is held by another process SHALL print once on stderr `Waiting for another invowk module command on this project (lock: <path>)`, or for a source `Waiting for another invowk module command on module source <url> (lock: <path>)`, and retry about every 100 ms. It SHALL stop waiting when it gets the lock, when its context is cancelled, or after 5 minutes.

#### Scenario: Lock becomes free
- **WHEN** a command finds the project lock busy and the holder finishes within 5 minutes
- **THEN** the command SHALL print the waiting line exactly once and then run normally

#### Scenario: Lock is free
- **WHEN** a command finds its lock free
- **THEN** it SHALL print no waiting line

#### Scenario: Wait for the project lock cancelled
- **WHEN** the user presses Ctrl+C while a command waits for the project lock
- **THEN** the command SHALL stop waiting, SHALL NOT read or write `invowkmod.cue` or the lock file, and SHALL exit non-zero

#### Scenario: Wait for the project lock times out
- **WHEN** another invowk process holds the project lock for more than 5 minutes
- **THEN** the waiting command SHALL fail with exit status 1 and an error that names the lock file and says another invowk process holds it, and SHALL NOT have read or written `invowkmod.cue` or the lock file

#### Scenario: Wait for a source lock cancelled or timed out
- **WHEN** the user presses Ctrl+C while a command waits for a source lock, or another invowk process holds that source lock for more than 5 minutes
- **THEN** the command SHALL fail (exit non-zero after Ctrl+C, exit status 1 with an error that names the lock file and says another invowk process holds it after the timeout), and `invowkmod.cue` and the project's lock file SHALL be as the command found them: it SHALL NOT have written either file, and `invowk module add` SHALL report no rollback

### Requirement: Read-only consumers take no lock
Discovery, command runs, `invowk validate`, `invowk audit` and `invowk module deps` SHALL NOT take a project or source lock. They SHALL rely on every write of `invowkmod.cue` and of the lock file by `invowk module add`, `remove`, `sync`, `update`, `tidy` and `vendor`, the rollback included, being an atomic replace. Writers outside these module commands are not covered by this requirement.

#### Scenario: Command run during a module edit
- **WHEN** `invowk cmd <name>` runs in a project while `invowk module add` holds that project's lock
- **THEN** the command run SHALL NOT wait for the lock and SHALL read either the old or the new `invowkmod.cue` and lock file, never a partial one

### Requirement: Locking is documented
The README and the module commands reference (English and pt-BR) SHALL state which module commands wait for each other, the waiting lines, the 5-minute limit, Ctrl+C, where the lock files live, that `module remove` and an empty `sync`, `tidy` or `vendor` now create the module cache, and the known limits (different cache roots; letter case on case-insensitive macOS volumes; advisory locks between processes on one machine).

#### Scenario: Reader looks up why a command waits
- **WHEN** a reader opens the module commands reference
- **THEN** it SHALL find the waiting lines, the commands that take the project lock, the 5-minute limit, the `locks` directory of the module cache and the known limits
