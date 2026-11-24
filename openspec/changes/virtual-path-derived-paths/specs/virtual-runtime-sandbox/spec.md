## ADDED Requirements

### Requirement: cp checks the destinations it derives before it runs
Under `restricted` access, before `cp` runs, every path it will create or write SHALL be resolved in full, following links, and checked against the allowed roots: the destination directory joined with each source's base name and, with `-r`, every destination path below it, from a walk of the source tree that does not follow links. One denial SHALL refuse the whole command with `virtual path denied` before anything is written.

#### Scenario: Destination directory entry that is a link
- **WHEN** a restricted virtual script runs `cp f d` or `cp f g d`, where `d` is a directory inside an allowed root and `d/f` is a symbolic link, live or dangling, whose target lies outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing inside or outside the allowed roots SHALL change

#### Scenario: Recursive destination below a link
- **WHEN** a restricted virtual script runs `cp -r src dst` or `cp -P -r src dst`, where a path `cp` would write below `dst` (`dst/src` itself, a file below it or a directory below it) is a symbolic link whose target lies outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing inside or outside the allowed roots SHALL change

#### Scenario: Windows junction on a recursive destination
- **WHEN** a restricted virtual script on Windows runs `cp -r src dst`, where `dst/src` or a directory below it is a junction to a directory outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing SHALL be created outside the allowed roots

#### Scenario: Links inside the roots keep working
- **WHEN** a restricted virtual script runs `cp f d` or `cp -r src dst`, where the destination paths cross symbolic links or Windows junctions whose targets lie inside the allowed roots
- **THEN** the copy SHALL succeed and write through those links, as on a host shell

#### Scenario: Full access is unchanged
- **WHEN** a virtual script with `virtual.filesystem.access: "full"` runs any `cp` command
- **THEN** Invowk SHALL NOT compute or check derived paths, and `cp` SHALL behave as the u-root command does

### Requirement: cp checks the source links it follows before it runs
Under `restricted` access, without `-P`, before `cp -r` runs, every entry of the source tree that is a link (a symbolic link or, on Windows, a junction or another reparse point) SHALL be resolved in full and checked against the allowed roots, because `cp` follows it. One denial SHALL refuse the whole command with `virtual path denied` before anything is written. With `-P`, a source link SHALL NOT be followed and its target SHALL NOT be checked.

#### Scenario: Source link to a file outside the roots
- **WHEN** a restricted virtual script runs `cp -r src dst`, where `src` holds a symbolic link to a file outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, nothing of the file SHALL be copied, and nothing SHALL be created below `dst`

#### Scenario: Source link to a directory outside the roots
- **WHEN** a restricted virtual script runs `cp -r src dst`, where `src` holds a symbolic link to a directory outside every allowed root
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing SHALL be created below `dst`

#### Scenario: Links copied as links under full access
- **WHEN** a virtual script with `virtual.filesystem.access: "full"` runs `cp -P -r src dst`, where `dst` is an existing directory and `src` holds a symbolic link
- **THEN** the copy SHALL succeed, and `dst/src` SHALL hold a symbolic link with the same target

#### Scenario: Source link inside the roots
- **WHEN** a restricted virtual script runs `cp -r src dst`, where `src` holds a symbolic link to a file inside an allowed root
- **THEN** the copy SHALL succeed, and the destination SHALL hold a copy of the file's content

### Requirement: Derived paths are checked in resolved form and derived again by u-root
Under `restricted` access, each path `cp` or `tar -x` derives SHALL be checked in resolved form and not handed on: the command SHALL receive the resolved operands ("Checked and reported virtual paths are the resolved path") and derive the same paths again, following the precedent of "gzip's output path is validated". The prediction rules below SHALL keep those paths equal to what was checked. Every refusal SHALL carry `virtual path denied` and name the path it refuses.

#### Scenario: Refusal names the resolved derived path
- **WHEN** a restricted virtual script runs `cp f d`, where `d/f` is a symbolic link whose target lies outside every allowed root
- **THEN** the error SHALL contain `virtual path denied` and the resolved path of `d/f`, which is the link's target

### Requirement: cp refuses a later source that does not exist when it starts
Under `restricted` access, with or without `-r`, `cp` SHALL refuse a command with a source operand after the first whose resolved path does not exist when the command starts, before anything is written, because an earlier source of the same command could create it. A missing first or only source operand SHALL keep the u-root command's own error, because no earlier source can create it. Under `full` access such a command SHALL run as the u-root command does.

#### Scenario: Later source that does not exist when cp starts
- **WHEN** a restricted virtual script runs `cp -r a/s d/s/x d` or `cp -P -r a/s d/s/l/f d`, where `d` is a directory inside an allowed root and `d/s` does not exist
- **THEN** the command SHALL fail with `virtual path denied` and an error that names the missing source, and nothing SHALL be created

#### Scenario: Missing first source keeps cp's own error
- **WHEN** a restricted virtual script runs `cp missing d` or `cp missing f d`, where `missing` does not exist, `f` is a file and `d` is a directory inside an allowed root
- **THEN** no `virtual path denied` refusal SHALL occur and the result SHALL be the u-root command's: `cp missing d` SHALL fail with its own "no such file or directory" error naming `missing`, and `cp missing f d` SHALL copy `f` to `d/f` (u-root reports only the last source's error)

### Requirement: cp -r does not write into a source tree of the same command
Under `restricted` access, `cp -r` SHALL refuse, before anything is written, a destination whose resolved path, or an existing ancestor of that path, is the same file as any source operand of the command, whatever its spelling, unless the destination, read without following its last component, is the same file as the source entry it copies. Under `full` access such a command SHALL run as the u-root command does.

#### Scenario: Directory copied into itself
- **WHEN** a restricted virtual script runs `cp -r d d` or `cp -r d d/sub`, where `d` is a directory inside an allowed root
- **THEN** the command SHALL fail with `virtual path denied` and an error that names the destination, and nothing SHALL be created

#### Scenario: Earlier source copied into a later source
- **WHEN** a restricted virtual script runs `cp -r a/s d/s d` or `cp -r a/s d/s/x d`, where `a/s` holds an entry `x`, and `d/s` and `d/s/x` exist
- **THEN** the command SHALL fail with `virtual path denied` and an error that names the destination at or inside the later source, and nothing SHALL be created

#### Scenario: Destination that names a source with another letter case
- **WHEN** a restricted virtual script on a case-insensitive volume runs `cp -r b B`, where `b` is a directory and `B` names the same directory
- **THEN** the command SHALL fail with `virtual path denied`, and nothing SHALL be created

#### Scenario: Copy onto the same entries
- **WHEN** a restricted virtual script runs `cp -r d .` in the directory that holds `d`
- **THEN** the command SHALL succeed, and `d` SHALL keep its entries and their content

#### Scenario: Same-named trees are merged
- **WHEN** a restricted virtual script runs `cp -r a/s b/s d`, where `a/s` and `b/s` are directories without links and `d` is a directory
- **THEN** the command SHALL succeed, and `d/s` SHALL hold the entries of both

### Requirement: cp -P creates a link only as its only destination
Under `restricted` access, `cp -P` SHALL refuse the whole command before anything is written, with `virtual path denied` and an error that names the link, when it would create a link and also has any other destination (a link, a file or a directory, from any source operand), comparing no names. A command whose only destination is the one link it creates SHALL NOT be refused by this rule. Under `full` access, `cp -P` SHALL run as the u-root command does.

#### Scenario: Several sources, one of which holds a link
- **WHEN** a restricted virtual script runs `cp -P -r a/s b/s d`, where `a/s` holds a symbolic link `l`, `b/s` holds a file, and `d` is a directory inside an allowed root
- **THEN** the command SHALL fail with `virtual path denied` and an error that names the link `d/s/l`, and nothing in `d` SHALL change

#### Scenario: One source tree that holds a link
- **WHEN** a restricted virtual script runs `cp -P -r src dst`, where `dst` is an existing directory and `src` holds a symbolic link, alone or next to a file
- **THEN** the command SHALL fail with `virtual path denied` and an error that names the link below `dst/src`, because the directory `dst/src` is another destination, and nothing in `dst` SHALL change

#### Scenario: Link operand without -r
- **WHEN** a restricted virtual script runs `cp -P lnk d`, where `lnk` is a symbolic link to a file `real` inside an allowed root and `d` is a directory
- **THEN** this rule SHALL NOT refuse the command: `cp` receives the resolved operand, as for every operand, creates no link, and copies `real` to `d/real`, as it does under `full` access

### Requirement: tar -x checks every entry destination before it runs
Under `restricted` access, before `tar -x` runs, the archive SHALL be read once and every path `tar -x` will create, write or change the mode of SHALL be computed as u-root computes it (the extraction directory joined with each entry name, with the same entries skipped), resolved in full, following links, and checked against the allowed roots. One denial SHALL refuse the whole command with `virtual path denied` before anything is extracted.

#### Scenario: File entry onto a link
- **WHEN** a restricted virtual script runs `tar -x -f a.tar ex`, where `ex/t` is a symbolic link, live or dangling, whose target lies outside every allowed root, and the archive holds a file entry `t`
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and the link's target SHALL NOT be created, written or have its mode changed

#### Scenario: Entries below a directory link
- **WHEN** a restricted virtual script runs `tar -x -f a.tar ex`, where `ex/l` is a symbolic link to a directory outside every allowed root, and the archive holds a directory entry `l`, a file entry `l/f`, or directory and file entries `l/a/b` and `l/a/b/f`
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing in that directory SHALL be created or have its mode changed

#### Scenario: Windows junction in the extraction directory
- **WHEN** a restricted virtual script on Windows runs `tar -x -f a.tar ex`, where `ex/l` is a junction to a directory outside every allowed root, and the archive holds the entries of "Entries below a directory link"
- **THEN** Invowk SHALL refuse the command with `virtual path denied`, and nothing in that directory SHALL be created or change

#### Scenario: Entries u-root skips stay skipped
- **WHEN** a virtual script runs `tar -x` on an archive whose entry names are absolute or climb out of the extraction directory with `..`
- **THEN** those entries SHALL be skipped as u-root skips them, they SHALL NOT cause a refusal, and the other entries SHALL be extracted

#### Scenario: Links inside the roots keep working
- **WHEN** a restricted virtual script runs `tar -x -f a.tar ex`, where `ex/l` is a symbolic link or Windows junction to a directory inside an allowed root, and the archive holds `l/f`
- **THEN** the extraction SHALL succeed, and `f` SHALL be created in the link's target directory

#### Scenario: tar -x from virtual-lua
- **WHEN** a restricted virtual-lua script runs `invowk.capture("tar", "-x", "-f", "a.tar", "ex")` in the shape of "File entry onto a link"
- **THEN** the call SHALL fail with `virtual path denied`, and nothing outside the allowed roots SHALL change

#### Scenario: Full access is unchanged
- **WHEN** a virtual script with `virtual.filesystem.access: "full"` runs `tar -x`
- **THEN** Invowk SHALL NOT read the archive ahead or check derived paths, and `tar` SHALL behave as the u-root command does

### Requirement: tar -x reads its archive ahead only from a regular file
Under `restricted` access, `tar -x` SHALL refuse an archive whose resolved path exists and is not a regular file (for example a named pipe or a device), with `virtual path denied` and an error that names the archive, before anything is extracted, because its first reading would not be repeated by the second. The archive SHALL always be named by `-f`; `-f -` SHALL name the file `-` in the working directory, not standard input.

#### Scenario: Archive that is a named pipe
- **WHEN** a restricted virtual script runs `tar -x -f p ex`, where `p` is a named pipe inside an allowed root
- **THEN** the command SHALL fail with `virtual path denied` and an error that names `p`, and nothing SHALL be extracted

#### Scenario: Dash as the archive name
- **WHEN** a virtual script runs `tar -x -f - ex` with a file named `-` in the working directory
- **THEN** `tar` SHALL read that file, and SHALL NOT read standard input

### Requirement: cp from virtual-lua gets the same checks
virtual-lua's `invowk.cmd` and `invowk.capture` SHALL run `cp` through the same registry as virtual-sh, so every derived-path check above SHALL apply to them under the same access mode.

#### Scenario: cp from virtual-lua
- **WHEN** a restricted virtual-lua script runs `invowk.capture("cp", "f", "d")`, where `d/f` is a symbolic link whose target lies outside every allowed root
- **THEN** the call SHALL fail with `virtual path denied`, and the link's target SHALL NOT change
