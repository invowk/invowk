# Spec Delta

## Purpose

Keeps every script file, custom-check file and env file that a module makes Invowk read inside that module's physical directory, so that a link cannot turn a module into a reader of files outside it (SC-01).

## ADDED Requirements

### Requirement: Module script files are physically contained
When an invowkfile is loaded from a module, Invowk SHALL read an implementation `script.file` or a custom-check script `file` only if the path, with every link on it followed, resolves inside the module directory. Otherwise it SHALL deny the read with the script path traversal error and read nothing.

#### Scenario: Link under invowk_modules to an outside file
- **WHEN** a module's `script.file` names `invowk_modules/x`, and `x` is a symlink to a file outside the module
- **THEN** resolving the script SHALL fail with the script path traversal error, and the outside file's content SHALL NOT be returned

#### Scenario: Custom-check file through the same link
- **WHEN** a module's custom check names a script `file` that resolves through a link to outside the module
- **THEN** resolving the custom check SHALL fail with the script path traversal error, and the check SHALL be reported as failed

#### Scenario: Real vendored file
- **WHEN** a module's `script.file` names a regular file inside a real (not linked) vendored module directory under `invowk_modules/`
- **THEN** the script SHALL be read as before

#### Scenario: Relative link inside the module
- **WHEN** a module's `script.file` crosses a symlink under `invowk_modules/` whose relative target lies inside the module
- **THEN** the script SHALL be read

#### Scenario: Missing script file
- **WHEN** a module's `script.file` names a path at which nothing exists
- **THEN** resolving the script SHALL fail with the read error it reports today, not with the traversal error

### Requirement: Links with absolute targets are denied in module reads
A contained module read SHALL deny a path that crosses a link whose target is absolute, even when the target lies inside the module.

#### Scenario: Absolute link to an in-module file
- **WHEN** a module's `script.file` or `env.files` entry crosses a symlink whose absolute target is a file inside the same module
- **THEN** the read SHALL be denied with a containment error

### Requirement: Containment holds for every runtime and caller
The containment of module script files SHALL hold for every production path that resolves them, whatever file reader that path supplies: `invowk cmd` on the native, virtual-sh, virtual-lua and container runtimes, custom-check evaluation, and `invowk validate`.

#### Scenario: invowk cmd on native and virtual-sh
- **WHEN** `invowk cmd` runs a module command whose `script.file` resolves through an `invowk_modules/` link to an outside file, on the native runtime and on the virtual-sh runtime
- **THEN** the command SHALL fail with the containment error, it SHALL NOT execute the outside file, and the outside file's content SHALL NOT appear in its output

#### Scenario: invowk validate does not read the outside file
- **WHEN** `invowk validate` analyses interpreters for such a module
- **THEN** it SHALL NOT read the outside file

### Requirement: Module env files are physically contained
When an invowkfile is loaded from a module, Invowk SHALL load an `env.files` entry (root, command or implementation level) only if the file, with every link followed, resolves inside the module directory. Otherwise the command SHALL fail with a containment error before it runs.

#### Scenario: Env file through an invowk_modules link
- **WHEN** a module's `env.files` entry names `invowk_modules/x`, and `x` is a symlink to a dotenv file outside the module
- **THEN** building the command environment SHALL fail with a containment error, and no variable from the outside file SHALL be set

#### Scenario: Absolute env file path in module context
- **WHEN** a module-context env file entry with an absolute path reaches the env loader, for example one built directly in Go and bypassing parse-time validation
- **THEN** loading it SHALL fail with a containment error

#### Scenario: Real env file in the module
- **WHEN** a module's `env.files` entry names a regular file inside the module
- **THEN** its variables SHALL be loaded as before

### Requirement: Optional module env files are skipped only when absent
An optional (`?`-suffixed) env file in module context SHALL be skipped only when nothing exists at its path inside the module. A dangling link or an escaping link at that path SHALL fail like a required file.

#### Scenario: Optional file missing
- **WHEN** an optional module env file names a path at which no file or link exists
- **THEN** it SHALL be skipped without error

#### Scenario: Optional file is a dangling link
- **WHEN** an optional module env file names a symlink whose target does not exist
- **THEN** loading SHALL fail with an error, and it SHALL NOT be silently skipped

#### Scenario: Optional file is an escaping link
- **WHEN** an optional module env file names a symlink to a file outside the module
- **THEN** loading SHALL fail with a containment error

### Requirement: Plain invowkfile env files keep following links
An `env.files` entry of an invowkfile that is not loaded from a module SHALL keep its current behaviour: it is resolved against the invowkfile directory, and links on its path are followed wherever they lead.

#### Scenario: Symlinked .env next to a plain invowkfile
- **WHEN** a plain invowkfile's `env.files` names `.env`, and `.env` is a symlink to a file outside the invowkfile directory
- **THEN** the file's variables SHALL be loaded

### Requirement: Containment is independent of path aliases and name case
Module read containment SHALL judge the path physically, not by comparing path strings. A module directory reached through an aliased parent, or named in a different case on a case-insensitive filesystem, SHALL NOT cause a false denial.

#### Scenario: Module under an aliased temporary directory
- **WHEN** the module directory is given through an alias whose physical path differs, for example `/var/...` resolving to `/private/var/...` on macOS
- **THEN** a contained script file and a contained env file SHALL be read

#### Scenario: Different name case on a case-insensitive filesystem
- **WHEN** the filesystem is case-insensitive and the module path or the script path uses a different case than the names on disk
- **THEN** a contained script file SHALL be read
