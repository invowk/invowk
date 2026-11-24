# virtual-runtime-sandbox Specification

## Purpose
Despite the directory name, this spec defines the Go-native safety harness shared by the virtual runtimes (virtual-sh and virtual-lua), which is explicitly not a sandbox. It covers the shared utility-command config namespace, path sanitization, host binaries denied unless listed in allowed_binaries, binary lookup modes, and environment control. Allowed host binaries run as native host processes, and the harness gates path access without being a kernel jail; execution isolation is the container runtime's responsibility.

## Requirements

### Requirement: Virtual runtimes define a Go-native safety harness
Invowk SHALL define every `virtual-*` runtime as a Go-native safety harness, not as a kernel-level jail. Invowk SHALL enforce path validation and execution gating for VM-controlled operations and SHALL document that launched host binaries run as normal host processes.

#### Scenario: Host binary boundary is explicit
- **WHEN** a `virtual-sh` or `virtual-lua` runtime launches an explicitly allowed host binary
- **THEN** Invowk SHALL NOT claim that the launched process is filesystem-sandboxed by the virtual runtime
- **THEN** Invowk SHALL document that `container` is required for process-level isolation

#### Scenario: Go-native operations remain inside the harness
- **WHEN** a virtual script uses VM-controlled file I/O, shell redirection, Lua file I/O, or a u-root built-in
- **THEN** Invowk SHALL route the operation through the virtual runtime safety harness before touching the host filesystem

### Requirement: Virtual config namespace controls shared utility commands
Invowk SHALL expose shared virtual-family configuration under `virtual`. The `virtual` config namespace SHALL NOT make `virtual` a valid runtime selector.

#### Scenario: Family-level utility config defaults to enabled
- **WHEN** generated config or effective config includes the virtual-family settings
- **THEN** Invowk SHALL use `virtual.utilities.enabled: true` by default

#### Scenario: Virtual config namespace is distinct from runtime selection
- **WHEN** config declares `virtual.utilities.enabled`
- **THEN** runtime selection SHALL continue to use the explicit `virtual-sh` or `virtual-lua` runtime names

#### Scenario: Disabling utilities affects both virtual interpreters
- **WHEN** config sets `virtual.utilities.enabled: false`
- **THEN** `virtual-sh` SHALL NOT resolve Invowk-provided external-style utility commands from the shared virtual utility set
- **THEN** `virtual-lua` SHALL NOT expose those utility commands through `invowk.cmd` or `invowk.capture`
- **THEN** host binary execution SHALL remain governed by `allowed_binaries` and `binary_lookup_mode`

### Requirement: Path sanitization for virtual runtimes
All `virtual-*` runtimes SHALL enforce shared path sanitization for VM-controlled filesystem operations. In restricted mode, access SHALL be allowed only for implicit safe roots, standard allowed anchors, or selected-platform `virtual.filesystem.paths` mappings. In full mode, VM-controlled filesystem operations SHALL be allowed to access normalized host filesystem paths after resolver checks.

#### Scenario: Relative workdir access is allowed
- **WHEN** a `virtual-sh` or `virtual-lua` script reads `./manifest.json` inside the effective work directory
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

### Requirement: Host binary execution is denied by default
All `virtual-*` runtimes SHALL deny host binary execution unless the selected runtime configuration explicitly allows it through `allowed_binaries`.

#### Scenario: Empty allowed binaries denies host binary
- **WHEN** a virtual runtime omits `allowed_binaries` or sets `allowed_binaries: []`
- **THEN** Invowk SHALL deny attempts to execute host binaries such as `git`, `curl`, `python`, or `node`

#### Scenario: Named allowed binary may execute
- **WHEN** a virtual runtime declares `allowed_binaries: ["git"]`
- **THEN** Invowk SHALL allow `git` to be resolved according to `binary_lookup_mode`
- **THEN** Invowk SHALL continue to deny non-listed host binaries

#### Scenario: Wildcard explicitly allows all host binaries
- **WHEN** a virtual runtime declares `allowed_binaries: ["*"]`
- **THEN** Invowk SHALL allow all host binaries to resolve according to `binary_lookup_mode`
- **THEN** dry-run and documentation SHALL identify that this opts out of host-binary gating

#### Scenario: u-root built-ins do not require allowlisting
- **WHEN** a virtual script invokes a u-root built-in
- **THEN** Invowk SHALL allow the built-in even when `allowed_binaries` is empty
- **THEN** the built-in SHALL remain subject to path validation

#### Scenario: Absolute executable path is exact
- **WHEN** `allowed_binaries` contains an absolute executable path
- **THEN** Invowk SHALL allow only that exact executable path
- **THEN** `binary_lookup_mode` SHALL NOT search alternate directories for that entry

### Requirement: Binary lookup mode controls allowed binary resolution
Invowk SHALL support `binary_lookup_mode` for virtual runtimes with allowed values `"host"` and `"strict"`. The default SHALL be `"host"`.

#### Scenario: Host lookup mode uses full effective PATH
- **WHEN** a virtual runtime allows `git` and omits `binary_lookup_mode`
- **THEN** Invowk SHALL use `"host"` lookup mode
- **THEN** Invowk SHALL resolve `git` from the effective host `PATH` visible to the runtime

#### Scenario: Strict lookup mode uses hardcoded system paths
- **WHEN** a virtual runtime declares `binary_lookup_mode: "strict"`
- **THEN** Invowk SHALL resolve bare allowed binary names only from hardcoded platform system paths
- **THEN** Invowk SHALL NOT use user-writable PATH entries such as home-bin directories or Windows AppData shims

#### Scenario: Invalid lookup mode is rejected by schema or validation
- **WHEN** a virtual runtime declares `binary_lookup_mode: "custom"`
- **THEN** Invowk SHALL reject the configuration before execution

#### Scenario: Resolved host binary path is exposed
- **WHEN** a virtual runtime resolves and launches an allowed host binary
- **THEN** `virtual-sh` SHALL expose the resolved executable path as `INVOWK_STATE_BIN_PATH`
- **THEN** `virtual-lua` SHALL expose the resolved executable path as `invowk.state.bin_path`

### Requirement: Virtual runtime environment is controlled
Virtual runtimes SHALL pass host binaries the same effective environment constructed for the virtual runtime after Invowk environment inheritance, filtering, and internal variable injection. User-provided environment values SHALL NOT override Invowk-reserved `INVOWK_` state variables.

#### Scenario: Inherited environment obeys existing env policy
- **WHEN** a virtual runtime uses `env_inherit_mode: "allow"`
- **THEN** host binaries launched from that runtime SHALL only receive the allowed host variables plus Invowk-injected variables and configured command environment

#### Scenario: Reserved state variables cannot be spoofed
- **WHEN** user configuration attempts to set `INVOWK_STATE_BIN_PATH`
- **THEN** Invowk SHALL keep its own generated state value authoritative

### Requirement: Virtual paths are resolved physically before the allowed-root check
The shared path validator SHALL resolve a requested path physically before it decides: it SHALL follow every symbolic link and every Windows junction on the path, find the deepest ancestor that exists after following them, and join the missing components to that ancestor. For an operation that follows its last component, the resolved path SHALL hold no link and no `..`. Allowed roots SHALL be resolved by the same procedure. For an operation that does not follow its last component ("Operations on the last component do not follow it"), the same procedure SHALL resolve the parent directory, and the final name SHALL be joined to it as written, without being read. No part of the decision SHALL use an unresolved path, except that final name.

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
- **WHEN** the validator resolves a path for an operation that follows its last component
- **THEN** the existing part of the result SHALL hold no link, and the missing components joined to it SHALL be plain names, with no `.` and no `..`

#### Scenario: Shape of a no-follow path
- **WHEN** the validator resolves a path for an operation that does not follow its last component
- **THEN** the result SHALL be the parent directory resolved as in "Shape of a resolved path", joined with the request's final name, and the final name SHALL NOT be inspected or replaced, whether or not it is a link

#### Scenario: Every consumer uses the same resolution
- **WHEN** a path through an escaping link or junction is used in a `virtual-sh` redirection, by a u-root built-in, by a Lua `io` function, or by Lua `require`
- **THEN** each SHALL be refused by the same resolution and allowed-root check

### Requirement: Unresolvable virtual paths are refused under restricted access
Wherever a containment decision is made, a path that cannot be resolved SHALL be refused: in the validator under `restricted` access, for the allowed roots, and in Lua `require` under either access mode. An error other than "this component does not exist" SHALL never be treated as a missing component, and SHALL never fall back to the unresolved path. For an operation that does not follow its last component, only the parent directory is resolved: a parent that cannot be resolved SHALL be refused, and the final name SHALL NOT be read, so a final name that is a link in a cycle or a link or reparse point whose target cannot be read SHALL NOT cause a refusal.

#### Scenario: Link cycle
- **WHEN** a restricted virtual script uses a path that crosses a link cycle, in a component that the operation follows
- **THEN** Invowk SHALL refuse the operation with an error that names the path

#### Scenario: Removing a link in a cycle
- **WHEN** a restricted virtual script runs `rm cyclelink`, where `cyclelink` is a symbolic link inside an allowed root that is part of a link cycle
- **THEN** the link SHALL be removed, and no other entry SHALL change

#### Scenario: Link that cannot be read
- **WHEN** a path component that the operation follows is reported by the operating system as a link or reparse point, its target cannot be read, and filesystem access is `restricted`
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
The path validator SHALL accept a path through a symbolic link or junction when the resolved path is inside an allowed root. The link's target MAY be relative or absolute. A root that is itself reached through a link or junction SHALL stay an allowed root, and requests written through that link SHALL be accepted when they resolve inside it. For an operation that does not follow its last component ("Operations on the last component do not follow it"), this acceptance SHALL apply to the links on the parent directory: a final name that is itself a link SHALL be checked as the no-follow path, so a request whose final name is the link that reaches a root (for example `rm -r /tmp` on macOS) SHALL be judged by where that link lies, not by its target.

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

#### Scenario: No-follow operation below a root reached through a link
- **WHEN** a restricted virtual script on macOS runs `rm /tmp/f` or `rm -r /tmp/d`, where `f` and `d` exist in the allowed root `/private/tmp`
- **THEN** Invowk SHALL allow the operation, because the parent `/tmp` resolves inside the root, and `f` or `d` SHALL be removed

### Requirement: Checked and reported virtual paths are the resolved path
The path validator SHALL perform a filesystem operation on the path it checked. For an operation that follows its last component, that path SHALL be the resolved path. For an operation that does not follow it, that path SHALL be the no-follow path of "Operations on the last component do not follow it". Path strings that Invowk reports (denial and not-a-directory messages, `invowk.path` results and `INVOWK_PATH_*` values) SHALL be the path the decision used: the resolved path, with links and junctions replaced by their targets, or, in a denial of an operation that does not follow its last component, the no-follow path, whose final name is kept as written. `invowk.path` and `INVOWK_PATH_*` values SHALL always be resolved paths. The only other form is the `full` access fallback path of "Full access hands unresolvable paths to the operating system".

#### Scenario: Operation on a junction acts on its target
- **WHEN** a virtual script on Windows runs an operation that follows its last component, such as `ls` or `cat`, on a path that ends at or crosses a junction whose target is inside an allowed root
- **THEN** the operation SHALL act on the junction's target path, as it does for a symbolic link on every platform

#### Scenario: Removing a junction removes the junction
- **WHEN** a virtual script on Windows runs `rm -rf` on a path that ends at a junction, under `restricted` or `full` access
- **THEN** the built-in SHALL remove the junction entry, and the junction's target directory and its contents SHALL remain

#### Scenario: Nested new path under a linked temporary directory
- **WHEN** a virtual script on macOS resolves `/tmp/new1/new2`, where neither component exists
- **THEN** the resolved path SHALL be `/private/tmp/new1/new2`

#### Scenario: Path with no link keeps its text
- **WHEN** a virtual script resolves an absolute path that crosses no link or junction and whose existing part is already in the platform's canonical form
- **THEN** the resolved path SHALL equal the cleaned input path

### Requirement: Operations on the last component do not follow it
Under both access modes, these operations SHALL act on the last component itself: `rm` operands (also with `-r`), `mv` sources, `ln`'s link name (its `-f` removal and its creation), and the `virtual-sh` `-L` and `-h` tests. They SHALL run on the no-follow path: the parent resolved physically, joined with the final name. Under `restricted` access that path SHALL be inside an allowed root. See "No-follow operands with a trailing separator".

#### Scenario: Removing a live link keeps its target
- **WHEN** a virtual script runs `rm live`, where `live` is a symbolic link to an existing file inside the allowed root
- **THEN** the link SHALL be removed, and its target file SHALL remain with its content

#### Scenario: Removing a dangling link
- **WHEN** a restricted virtual script runs `rm -f stale`, where `stale` is a symbolic link whose target does not exist
- **THEN** the link SHALL be removed

#### Scenario: Recursive removal of a link to a directory
- **WHEN** a virtual script runs `rm -r` or `rm -rf` on a symbolic link to a directory
- **THEN** the link SHALL be removed, and the target directory and its contents SHALL remain

#### Scenario: Replacing a link with ln
- **WHEN** a virtual script runs `ln -sf target.txt stale`, where `stale` is a dangling symbolic link
- **THEN** `stale` SHALL be replaced by a symbolic link to `target.txt`, and nothing SHALL be created at the old target's path

#### Scenario: Moving a link
- **WHEN** a virtual script runs `mv live dir`, where `live` is a symbolic link and `dir` an existing directory inside the allowed root
- **THEN** `dir/live` SHALL be the moved link, and the link's target SHALL stay where it was

#### Scenario: Link file test
- **WHEN** a `virtual-sh` script evaluates `[ -L live ]`, `[ -h live ]` or `test -L live` for an existing symbolic link `live`, under either access mode
- **THEN** the test SHALL be true, and for a regular file it SHALL be false

#### Scenario: Link inside the root with a target outside
- **WHEN** a restricted virtual script runs `rm outlink`, where `outlink` is a symbolic link inside an allowed root whose target is a file outside every allowed root
- **THEN** the link SHALL be removed, and the outside file SHALL remain
- **WHEN** the same script runs `cat outlink`
- **THEN** Invowk SHALL refuse the operation with `virtual path denied`

#### Scenario: Parent reached through an escaping link
- **WHEN** a restricted virtual script runs `rm` on `esc/name`, where `esc` is a symbolic link or junction inside an allowed root to a directory outside every allowed root
- **THEN** Invowk SHALL refuse the operation, and nothing outside the allowed roots SHALL change

#### Scenario: Removing an allowed root reached through a link
- **WHEN** a restricted virtual script on macOS runs `rm -r /tmp`, where `/tmp` is a symbolic link to the allowed root `/private/tmp`
- **THEN** Invowk SHALL refuse the operation with `virtual path denied`, because the link `/tmp` itself lies outside every allowed root, and `/tmp` and `/private/tmp` SHALL remain unchanged

#### Scenario: Built-ins called from virtual-lua
- **WHEN** a `virtual-lua` script calls `invowk.cmd.rm`, `invowk.cmd.mv` or `invowk.cmd.ln` (or the `invowk.capture` forms) on a link
- **THEN** the built-in SHALL act on the link itself, as in `virtual-sh`

#### Scenario: Operations that follow the link are unchanged
- **WHEN** a virtual script opens, reads, writes, copies, lists or changes into a path whose last component is a link, or uses it as the `mv` destination
- **THEN** Invowk SHALL resolve the link and check the resolved path against the allowed roots as before

### Requirement: No-follow operands with a trailing separator
An `rm` operand, an `mv` source or `ln`'s link name that ends in a separator, `.` or `..` SHALL use full resolution. After the allowed-root check, Invowk SHALL refuse it with a "not a directory" error naming the resolved path when that path, read without a trailing separator, exists and is not a directory, on every platform.

#### Scenario: Trailing separator follows the link
- **WHEN** a virtual script names an `rm` operand, an `mv` source or `ln`'s link name with a trailing separator, or with a final `.` or `..`
- **THEN** the operand SHALL be resolved through its last component like any other path

#### Scenario: Trailing separator on a link to a file
- **WHEN** a virtual script runs `rm live/`, `rm -r live/`, `rm -rf live/`, `rm -f live/`, `rm -r live/.`, `mv live/ x` or `ln -sf t live/`, where `live` is a symbolic link to the file `target.txt`, under either access mode
- **THEN** the command SHALL fail with a "not a directory" error that names the resolved path of `target.txt`, and `live`, `target.txt` and `x` SHALL remain as they were

#### Scenario: Trailing separator on a link to a directory
- **WHEN** a virtual script runs `rm -r dirlink/`, where `dirlink` is a symbolic link to a directory inside the allowed roots
- **THEN** the target directory and its contents SHALL be removed, and `dirlink` SHALL remain

#### Scenario: The mv destination and a hard-link target
- **WHEN** a virtual script names the `mv` destination or a hard link's target with a trailing separator
- **THEN** it SHALL keep full resolution with no not-a-directory check

#### Scenario: Link test on a cleaned operand
- **WHEN** a `virtual-sh` script evaluates `[ -L live/ ]`, where `live` is a symbolic link
- **THEN** the stat handler SHALL use the no-follow path of the operand as the shell passes it, after the shell has made it absolute and cleaned it to `live`, so the test SHALL be true (a host shell, which follows `live/`, says false)

### Requirement: ln never replaces a directory
`ln` SHALL refuse with a "cannot overwrite directory" error, before it removes anything, whenever its resolved link name is an existing directory, whatever form names it (`dir`, `dir/`, `dir/.`, `.`, `..`, or a link to a directory with a trailing separator). A link to a directory named without a trailing separator is a link, not a directory, and SHALL be replaced like any other link with `-f`.

#### Scenario: ln on a directory in any form
- **WHEN** a virtual script runs `ln -sf t dir`, `ln -sf t dir/`, `ln -sf t dir/.`, `ln -sf t emptylink/`, `ln -sf t emptylink/.`, or `ln -sf ../t .` inside `dir`, where `dir` is an empty directory and `emptylink` a symbolic link to it, under either access mode
- **THEN** `ln` SHALL fail with a "cannot overwrite directory" error before it removes anything, and `dir` and `emptylink` SHALL remain

#### Scenario: ln replaces a link to a directory
- **WHEN** a virtual script runs `ln -sf target.txt emptylink`, where `emptylink` is a symbolic link to the empty directory `dir`, under either access mode
- **THEN** `emptylink` SHALL become a symbolic link to `target.txt`, and `dir` SHALL remain

### Requirement: Built-in operands are found as the built-in parses them
Under a validating context, the operands of every validated built-in (`base64`, `cat`, `cp`, `find`, `gzip`, `ls`, `mkdir`, `mv`, `rm`, `shasum`, `tar`, `touch`) SHALL be found as its parser finds them, subject to "A double dash ends a built-in's options", and each SHALL be validated, whatever it starts with, except `cat`'s lone `-` and the operands of `tar` with only `-t`, which stay as written. Option values SHALL pass as written, except `tar`'s archive path, which SHALL be validated.

#### Scenario: A dash-prefixed operand after the first operand is validated
- **WHEN** a restricted virtual script runs `cat a <p>`, `touch a <p>`, `rm keep <p>` or `mv a b <p>`, where `<p>` is a relative operand that starts with `-` and names a path outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, nothing of the outside file SHALL be printed, and nothing inside or outside the allowed roots SHALL change

#### Scenario: A lone dash
- **WHEN** a virtual script runs `cat a -` with data on standard input, or `cp a -`
- **THEN** `cat` SHALL print `a` and then standard input, and `cp` SHALL write the file `-` in the working directory after validating it like any other operand

#### Scenario: A leading lone dash
- **WHEN** a virtual script runs `cat - a` or `rm - a`
- **THEN** the lone `-` SHALL be dropped as the built-in's preprocessing drops it, so `cat` SHALL print only `a` and `rm` SHALL remove only `a`

#### Scenario: Option values are not rewritten
- **WHEN** a virtual script runs `mkdir -m 755 d`, `touch -d 2024-01-02T03:04:05Z f`, `shasum -a 256 f` or `gzip -k -S .z f`
- **THEN** each value SHALL reach the built-in as written, and `d` SHALL be created with mode 755, `f` SHALL get that time, the SHA-256 digest of `f` SHALL be printed, and `f.z` SHALL be written

#### Scenario: find starting path after its options
- **WHEN** a restricted virtual script runs `find -name x <dir>`, where `<dir>` is outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`

#### Scenario: tar extraction directory
- **WHEN** a restricted virtual script runs `tar -x -f a.tar <dir>`, where `a.tar` is inside an allowed root and `<dir>` is outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing SHALL be written below `<dir>`

#### Scenario: tar archive path
- **WHEN** a restricted virtual script runs `tar -c -f <archive> src` or `tar -t --file=<archive>`, where `<archive>` is outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`

#### Scenario: Rejected option
- **WHEN** a virtual script runs a validated built-in with an option the built-in does not define, followed by operands
- **THEN** the command SHALL fail with the built-in's own error for that option, and no operand SHALL be read, written or validated

### Requirement: A double dash ends a built-in's options
For the validated built-ins that rewrite their arguments with u-root's `ArgsToGoArgs` (all of them but `find` and `gzip`), a `--` that comes before the first argument that does not start with `-` SHALL end the options, as on a host shell, and SHALL NOT be handed on, although `ArgsToGoArgs` would read it as the operand `-`. The arguments after it SHALL be operands, validated like any other.

#### Scenario: Operands after a double dash
- **WHEN** a virtual script runs `rm -- -x` or `cat -- -x`, where `-x` is a file in the working directory
- **THEN** `rm` SHALL remove the file `-x` and no other entry, and `cat` SHALL print the file `-x` and read nothing from standard input

### Requirement: gzip's output path is validated
Under a validating context, unless `-c` or `-t` is given, the output path that `gzip` derives from each operand and its suffix SHALL be validated like a path the script opens for writing. A `gzip -S` suffix that contains a path separator SHALL be refused before `gzip` runs, under either access mode.

#### Scenario: gzip output path outside the roots
- **WHEN** a restricted virtual script runs `gzip` without `-c` or `-t` on an operand inside an allowed root, and the output path `gzip` derives from it (the operand plus the suffix, or with `-d` the operand without it) lies outside every allowed root, for example `gzip -k .` in a working directory that is an allowed root's own path
- **THEN** Invowk SHALL refuse the command with `virtual path denied` before `gzip` runs, and nothing SHALL be created or changed

#### Scenario: gzip suffix with a path separator
- **WHEN** a virtual script runs `gzip` with a `-S` value that contains a path separator, in any of the forms `-S v`, `-S=v` and `--S=v`, with or without `-d`, under either access mode
- **THEN** the command SHALL fail before `gzip` runs, with an error that names the suffix, and nothing SHALL be created or changed

### Requirement: Built-in option tables follow u-root
Each validated wrapper's declared options SHALL equal the options its u-root command defines, including which of them take a value, and the operand collector SHALL read them from that declaration.

#### Scenario: Declared options follow u-root
- **WHEN** the u-root dependency changes the options of a validated command, or which of them take a value
- **THEN** a unit test SHALL fail until the wrapper's declared options match again

### Requirement: Restricted cd outside the allowed roots names the denial
Under `restricted` access, a `virtual-sh` `cd` to a directory whose resolved path is outside every allowed root SHALL fail with exit status 1 and SHALL write a line to standard error that contains `virtual path denied` and the path. A `cd` to an empty directory path SHALL get no line beyond the shell's own message. `cd` and the file tests SHALL NOT crash the runtime.

#### Scenario: cd outside the roots
- **WHEN** a restricted `virtual-sh` script runs `cd` to an existing directory outside every allowed root
- **THEN** the command SHALL fail with exit status 1, standard error SHALL contain `cd: virtual path denied`, and the working directory SHALL NOT change

#### Scenario: cd and file tests inside the roots
- **WHEN** a `virtual-sh` script under `restricted` or `full` access runs `cd` into a directory inside the allowed roots, and evaluates `test -f`, `[ -e ]`, `[ -d ]` and `[[ -f ]]` on paths there
- **THEN** each SHALL give the same result as on a host shell, and the runtime SHALL NOT panic

#### Scenario: cd to an empty path
- **WHEN** a restricted `virtual-sh` script runs `cd ""`
- **THEN** the command SHALL fail with the shell's `cd: empty directory path` message only

#### Scenario: cd outside the roots under full access
- **WHEN** a `virtual-sh` script with `virtual.filesystem.access: "full"` runs `cd` to an existing directory outside the allowed roots
- **THEN** the command SHALL succeed

### Requirement: Shell access checks run on the validated path
On every platform and under both access modes, the access check of `virtual-sh` `cd` and of the `-r`, `-w` and `-x` tests SHALL resolve the path in full, following its last component, and under `restricted` access SHALL check it against the allowed roots. Only a path that passes SHALL reach the shell's own access check, as the resolved path; a refused path SHALL make the test false and `cd` fail. `-L` and `-h` SHALL keep the no-follow path.

#### Scenario: cd into a directory link or junction
- **WHEN** a `virtual-sh` script under `restricted` or `full` access runs `cd` into a symbolic link to a directory inside the allowed roots, or on Windows into a junction to a directory inside the allowed roots
- **THEN** the command SHALL succeed, and files of the target directory SHALL then be reachable by relative paths

#### Scenario: Access tests on links inside the roots
- **WHEN** a `virtual-sh` script under either access mode evaluates `[ -r live ]` for a symbolic link to a readable file, or `[ -x dirlink ]` for a symbolic link to a directory, both inside the allowed roots
- **THEN** each test SHALL be true, and `[ -L live ]` and `[ -L dirlink ]` SHALL stay true

#### Scenario: Access tests outside the roots
- **WHEN** a restricted `virtual-sh` script evaluates `[ -r <f> ]`, `[ -w <f> ]` or `[ -x <d> ]`, where `<f>` is a file and `<d>` a directory outside every allowed root
- **THEN** each test SHALL be false, and the shell's own access check SHALL NOT run on that path
