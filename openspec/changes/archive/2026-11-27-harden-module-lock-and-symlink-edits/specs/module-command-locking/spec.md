# Spec Delta: module-command-locking

## MODIFIED Requirements

### Requirement: When the project lock excludes another command
Two commands that hold the project lock SHALL NOT overlap inside the spans that "Module edits hold the project lock" and "Agent module change writes under the project lock" state when they run on one machine against the same module cache root and their project directory resolves to the same project lock file.

#### Scenario: Same project, same cache root
- **WHEN** two such commands run at the same time on one machine, on one project directory, with the same module cache root
- **THEN** the second SHALL wait until the first has released the project lock

#### Scenario: Known limits of the project lock
- **WHEN** two commands on one project use different module cache roots (different `INVOWK_MODULES_PATH` values)
- **THEN** they MAY use different project lock files and MAY overlap, and the documentation SHALL state that limit

### Requirement: Lock files
Lock files SHALL be empty files in the `locks` directory of the module cache root (`$INVOWK_MODULES_PATH`, else `~/.invowk/modules`); that directory SHALL be created with mode 0700 when missing. A project's lock SHALL be named `project-<hex SHA-256 of the project directory's physical absolute path>.lock`, a source's `source-<hex SHA-256 of the source checkout key>.lock`. Lock files SHALL NOT be deleted by invowk.

#### Scenario: Locks stay out of module content
- **WHEN** a module command takes a project or source lock
- **THEN** the lock file SHALL be in `<cache root>/locks/`, which is never a module directory, an `invowk_modules/` directory, a cache checkout or a cache copy, and the lock file SHALL be part of no content hash, vendored copy, `validate` input or `audit` input, also when the cache root lies inside the project directory

#### Scenario: One project through two paths
- **WHEN** two commands run in one project, one through its real path and one through a symbolic link to it (on macOS also another letter case on a case-insensitive volume or a firmlink; on Windows also a junction, another letter case or the 8.3 short name)
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

### Requirement: Locking is documented
The README and the module commands reference (English and pt-BR) SHALL state which module commands wait for each other, the waiting lines, the 5-minute limit, Ctrl+C, where the lock files live, that `module remove` and an empty `sync`, `tidy` or `vendor` now create the module cache, and the known limits (different cache roots; advisory locks between processes on one machine).

#### Scenario: Reader looks up why a command waits
- **WHEN** a reader opens the module commands reference
- **THEN** it SHALL find the waiting lines, the commands that take the project lock, the 5-minute limit, the `locks` directory of the module cache, and the known limits without listing macOS letter-case differences as an unresolved limit
