# Spec Delta

## ADDED Requirements

### Requirement: Virtual paths are resolved physically before the allowed-root check
The shared path validator SHALL resolve a requested path physically before it decides: it SHALL follow every symbolic link and every Windows junction on the path, find the deepest ancestor that exists after following them, and join the missing components to that ancestor. The resolved path SHALL hold no link and no `..`. Allowed roots SHALL be resolved by the same procedure. No part of the decision SHALL use an unresolved path.

#### Scenario: Deep new path below a link that leaves the root
- **WHEN** an allowed root contains a symbolic link to a directory outside every allowed root, and a restricted virtual script creates a path with two or more missing components below that link
- **THEN** Invowk SHALL refuse the operation, and nothing SHALL be created outside the allowed roots

#### Scenario: Dangling link to a file outside the root
- **WHEN** an allowed root contains a symbolic link whose target does not exist and lies in an existing directory outside every allowed root, and a restricted virtual script writes to that link
- **THEN** Invowk SHALL refuse the operation, and the target SHALL NOT be created

#### Scenario: Windows junction that leaves the root
- **WHEN** an allowed root on Windows contains a junction to a directory outside every allowed root, and a restricted virtual script reads an existing file below it, creates a new file directly below it, or creates a new file two missing levels below it
- **THEN** Invowk SHALL refuse each operation, and nothing SHALL be created outside the allowed roots

#### Scenario: Shape of a resolved path
- **WHEN** the validator resolves a path
- **THEN** the existing part of the result SHALL hold no link, and the missing components joined to it SHALL be plain names, with no `.` and no `..`

#### Scenario: Every consumer uses the same resolution
- **WHEN** a path through an escaping link or junction is used in a `virtual-sh` redirection, by a u-root built-in, by a Lua `io` function, or by Lua `require`
- **THEN** each SHALL be refused by the same resolution and allowed-root check

### Requirement: Unresolvable virtual paths are refused under restricted access
Wherever a containment decision is made, a path that cannot be resolved SHALL be refused: in the validator under `restricted` access, for the allowed roots, and in Lua `require` under either access mode. An error other than "this component does not exist" SHALL never be treated as a missing component, and SHALL never fall back to the unresolved path.

#### Scenario: Link cycle
- **WHEN** a restricted virtual script uses a path that crosses a link cycle
- **THEN** Invowk SHALL refuse the operation with an error that names the path

#### Scenario: Link that cannot be read
- **WHEN** a path component is reported by the operating system as a link or reparse point, its target cannot be read, and filesystem access is `restricted`
- **THEN** Invowk SHALL refuse the operation

#### Scenario: Link target that climbs out of a missing directory
- **WHEN** an allowed root contains a link `in` to a directory outside every allowed root and a link `d` whose target is `nope/../in/new`, where `nope` does not exist, and a restricted virtual script writes to `d`
- **THEN** Invowk SHALL refuse the operation: the `..` after the missing `nope` SHALL NOT be applied to the path's text, and nothing SHALL be created outside the allowed roots

#### Scenario: Missing volume
- **WHEN** a restricted virtual script on Windows uses a path on a drive or UNC share that does not exist
- **THEN** the path has no existing ancestor, and Invowk SHALL refuse the operation with an error that says so

#### Scenario: Root that cannot be resolved
- **WHEN** an implicit root or anchor cannot be resolved
- **THEN** it SHALL NOT be an allowed root, and requests below it SHALL be refused by the same resolution error

#### Scenario: Declared path that cannot be resolved
- **WHEN** a selected-platform `virtual.filesystem.paths` entry cannot be resolved, and filesystem access is `restricted`
- **THEN** the command SHALL fail before the script runs, with an error that names the entry

#### Scenario: Standard-stream device link on a pipe
- **WHEN** a restricted virtual script on Linux opens `/dev/stdout`, `/dev/stderr` or `/dev/stdin` while that stream is a pipe or a socket, and `/dev` is a declared path
- **THEN** Invowk SHALL judge the link by its target like any other link, and SHALL NOT accept the path because the link itself lies in `/dev`

#### Scenario: Descriptor duplication is not a path
- **WHEN** a restricted `virtual-sh` script writes with `>&2` or `>&1`
- **THEN** the write SHALL NOT be subject to the path check

### Requirement: Full access hands unresolvable paths to the operating system
Under `full` filesystem access the path validator SHALL NOT refuse a path because it cannot be resolved: it SHALL pass the path to the operating system in a fallback form. An empty path SHALL still be an error. No containment decision SHALL use the fallback form.

#### Scenario: Fallback form
- **WHEN** a path under `full` access names or crosses a link whose target does not resolve
- **THEN** the validator SHALL return the link's own path: the resolved form of the link's directory, the link's name, and the rest of the request
- **WHEN** a path under `full` access cannot be resolved for any other reason
- **THEN** the validator SHALL return the cleaned absolute path

#### Scenario: Standard-stream device path on a pipe
- **WHEN** a `virtual-sh` script with `virtual.filesystem.access: "full"` runs `echo msg > /dev/stderr` on Linux while stderr is a pipe
- **THEN** the write SHALL succeed and `msg` SHALL arrive on stderr

#### Scenario: Dangling link under full access
- **WHEN** a virtual script with `full` access writes to a symbolic link whose target does not exist
- **THEN** Invowk SHALL pass the link's own path to the operating system, and the link's target SHALL be created

#### Scenario: Declared path that cannot be resolved under full access
- **WHEN** a selected-platform `virtual.filesystem.paths` entry cannot be resolved, and filesystem access is `full`
- **THEN** the command SHALL run, and the entry's `INVOWK_PATH_*` value SHALL be its fallback path

### Requirement: Links inside allowed roots keep working
The path validator SHALL accept a path through a symbolic link or junction when the resolved path is inside an allowed root. The link's target MAY be relative or absolute. A root that is itself reached through a link or junction SHALL stay an allowed root, and requests written through that link SHALL be accepted when they resolve inside it.

#### Scenario: In-root link with a relative or an absolute target
- **WHEN** a restricted virtual script uses a path through a symbolic link whose target is inside the same allowed root, at zero, one, or two or more missing components below the link
- **THEN** Invowk SHALL allow the operation, for a relative and for an absolute link target

#### Scenario: In-root junction on Windows
- **WHEN** a restricted virtual script on Windows uses a path through a junction whose target is inside an allowed root
- **THEN** Invowk SHALL allow the operation

#### Scenario: In-root dangling link
- **WHEN** a restricted virtual script writes to a symbolic link whose missing target lies inside an allowed root
- **THEN** Invowk SHALL allow the operation, and the target SHALL be created inside that root

#### Scenario: Root reached through a link or junction
- **WHEN** an allowed root is given by a path that crosses a symbolic link (for example `/tmp` on macOS) or a Windows junction, and a restricted virtual script creates a path two or more missing components below that root, written through the link
- **THEN** Invowk SHALL allow the operation

### Requirement: Checked and reported virtual paths are the resolved path
The path validator SHALL perform a filesystem operation on the resolved path it checked. Path strings that Invowk reports for a resolved path (denial messages, `invowk.path` results and `INVOWK_PATH_*` values) SHALL be that resolved path, with links and junctions replaced by their targets. The only other form is the `full` access fallback path of "Full access hands unresolvable paths to the operating system".

#### Scenario: Operation on a junction acts on its target
- **WHEN** a virtual script on Windows runs a u-root built-in such as `rm -rf` on a path that ends at a junction whose target is inside an allowed root
- **THEN** the built-in SHALL act on the junction's target path, as it does for a symbolic link on every platform

#### Scenario: Nested new path under a linked temporary directory
- **WHEN** a virtual script on macOS resolves `/tmp/new1/new2`, where neither component exists
- **THEN** the resolved path SHALL be `/private/tmp/new1/new2`

#### Scenario: Path with no link keeps its text
- **WHEN** a virtual script resolves an absolute path that crosses no link or junction and whose existing part is already in the platform's canonical form
- **THEN** the resolved path SHALL equal the cleaned input path
