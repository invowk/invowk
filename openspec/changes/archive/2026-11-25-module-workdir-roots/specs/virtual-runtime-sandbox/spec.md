# Spec Delta

## MODIFIED Requirements

### Requirement: Path sanitization for virtual runtimes
All `virtual-*` runtimes SHALL enforce shared path sanitization for VM-controlled filesystem operations. In restricted mode, access SHALL be allowed only for implicit safe roots, standard allowed anchors, or selected-platform `virtual.filesystem.paths` mappings. In full mode, VM-controlled filesystem operations SHALL be allowed to access normalized host filesystem paths after resolver checks.

#### Scenario: Relative workdir access is allowed
- **WHEN** a `virtual-sh` or `virtual-lua` script reads `./manifest.json` inside the effective work directory, and that work directory is an allowed root ("The work directory is an allowed root unless a module declares it outside the module")
- **THEN** Invowk SHALL allow the operation after resolving and validating the final host path

#### Scenario: Module file access is allowed
- **WHEN** a virtual script loaded from an invowkmod reads a file inside the source module directory
- **THEN** Invowk SHALL allow the operation after resolving and validating the final host path

#### Scenario: Unauthorized absolute path is blocked
- **WHEN** a `virtual-sh` script attempts to read `/etc/passwd` in restricted mode without a matching anchor or selected-platform `virtual.filesystem.paths` mapping
- **THEN** Invowk SHALL fail the operation with a permission-denied diagnostic from the shared path validator

#### Scenario: Traversal out of an allowed root is blocked
- **WHEN** a virtual script uses `../`, symlinks, Windows drive syntax, UNC paths, or mixed separators to escape an allowed root
- **THEN** Invowk SHALL reject the resolved path before performing the filesystem operation

#### Scenario: Shell redirection is path checked
- **WHEN** a `virtual-sh` script redirects output to `/tmp/../etc/hosts`
- **THEN** Invowk SHALL validate the resolved target path and block the write if it escapes the allowed roots

#### Scenario: Lua file I/O is path checked
- **WHEN** a `virtual-lua` script calls `io.open` for a host path
- **THEN** Invowk SHALL validate the resolved path with the same path validator used by `virtual-sh`

#### Scenario: u-root built-ins are path checked
- **WHEN** a virtual script invokes a u-root built-in such as `cp`, `rm`, `cat`, or `tee`
- **THEN** Invowk SHALL ensure the built-in uses path-validated filesystem operations

## ADDED Requirements

### Requirement: The work directory is an allowed root unless a module declares it outside the module
Under `restricted` access the effective work directory SHALL be an allowed root, except when the invowkfile belongs to a module, the work directory comes from that invowkfile's root, command or implementation `workdir`, and it resolves physically outside the module directory. That work directory SHALL NOT be an allowed root by being the work directory, and SHALL still be the process's working directory.

#### Scenario: Other roots may still enclose an outside workdir
- **WHEN** a module-declared work directory outside the module lies below a `virtual.filesystem.paths` entry or another implicit root (the script base, a standard anchor, the temporary roots)
- **THEN** that root SHALL still admit reads and writes below it; only the root that the work directory itself would add is removed

#### Scenario: Module workdir outside the module by a relative path
- **WHEN** a module command run with a virtual runtime under `restricted` access declares `workdir: "../shared"`, its script reads `./data.txt` or writes `./out.txt` in that directory, and no other allowed root encloses that directory
- **THEN** Invowk SHALL refuse both operations with `virtual path denied`, the script SHALL start with that directory as its working directory, and no file SHALL be created there

#### Scenario: Module workdir outside the module by an absolute path
- **WHEN** a module implementation under `restricted` access declares an absolute `workdir` outside the module directory, such as the filesystem root
- **THEN** reads and writes below that directory SHALL be refused unless another allowed root encloses them

#### Scenario: Module workdir that reaches outside through a link
- **WHEN** a module declares a `workdir` inside the module directory whose path crosses a symbolic link, or on Windows a junction, to a directory outside the module
- **THEN** Invowk SHALL treat the work directory as outside the module, and SHALL refuse reads and writes below it under `restricted` access unless another allowed root encloses them

#### Scenario: Module workdir inside the module
- **WHEN** a module declares a `workdir` that resolves physically inside the module directory, through links or not
- **THEN** reads and writes below it SHALL be allowed under `restricted` access, as before

#### Scenario: Module default workdir
- **WHEN** a module command declares no `workdir` at any level and no `--ivk-workdir` is given
- **THEN** the work directory SHALL be the module directory, and it SHALL be an allowed root

#### Scenario: CLI override stays a root
- **WHEN** the user runs a module command with `--ivk-workdir` naming a directory outside the module, under `restricted` access
- **THEN** that directory SHALL be an allowed root, and reads and writes below it SHALL be allowed

#### Scenario: Plain invowkfile workdir stays a root
- **WHEN** an invowkfile that does not belong to a module declares a `workdir` outside its own directory, such as `../packages/x`, and a virtual script reads there under `restricted` access
- **THEN** that directory SHALL be an allowed root, and the read SHALL be allowed

#### Scenario: Declared path keeps an outside workdir readable
- **WHEN** a module declares an outside `workdir` and its selected platform declares `virtual.filesystem.paths: {WORK: "@work"}`, or another entry that resolves to that directory
- **THEN** that directory SHALL be an allowed root through the declared path, and reads and writes below it SHALL be allowed under `restricted` access

#### Scenario: Full access is unchanged
- **WHEN** a module declares an outside `workdir` and its selected platform uses `virtual.filesystem.access: "full"`
- **THEN** reads and writes below that directory SHALL be allowed, as before

#### Scenario: Interactive mode makes the same decision
- **WHEN** a module command with an outside `workdir` runs a virtual runtime in interactive mode
- **THEN** the interactive subprocess SHALL refuse the same reads and writes as the non-interactive run, and SHALL allow the same ones

#### Scenario: Native and container runtimes are unaffected
- **WHEN** a module command with an outside `workdir` runs with the `native` or `container` runtime
- **THEN** its working directory and file access SHALL be unchanged

### Requirement: Outside the module is decided physically and fails closed
Whether a module-declared work directory lies inside the module SHALL be decided on the physically resolved work directory and module directory: every symbolic link and Windows junction on both is followed, as for any virtual path. When either path cannot be resolved, or the two are on different volumes, the work directory SHALL count as outside the module.

#### Scenario: Unresolvable workdir counts as outside
- **WHEN** a module-declared `workdir` crosses a link cycle, or another error stops its resolution
- **THEN** the work directory SHALL NOT be an allowed root by being the work directory under `restricted` access

#### Scenario: Missing workdir below the module
- **WHEN** a module-declared `workdir` names a directory below the module directory that does not exist yet
- **THEN** it SHALL be resolved through its deepest existing ancestor, and SHALL count as inside the module

#### Scenario: Another volume on Windows
- **WHEN** on Windows a module-declared `workdir` is on another drive or UNC share than the module directory
- **THEN** the work directory SHALL count as outside the module

#### Scenario: macOS temporary-directory aliases
- **WHEN** on macOS the module directory and the work directory are written once through `/var` or `/tmp` and once through `/private/var` or `/private/tmp`
- **THEN** the decision SHALL be the same as for the resolved paths
