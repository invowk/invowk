# Feature Specification: u-root Utils Integration

**Feature Branch**: `005-uroot-utils`
**Created**: 2026-01-30
**Status**: Draft
**Input**: User description: "u-root Utils Integration - Enable enhanced file operations via u-root utilities in the virtual shell runtime"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Execute File Operations Without External Dependencies (Priority: P1)

A developer working in a minimal environment (embedded Linux, container with limited base image, CI runner without coreutils) wants to run invowk commands that use common file operations like `cp`, `mv`, `cat`, `ls` without relying on external binaries being present on the system.

**Why this priority**: This is the core value proposition of u-root integration. The virtual shell runtime already provides cross-platform bash-like script execution via mvdan/sh, but external commands still require host binaries. The u-root utilities would make the virtual runtime truly self-contained.

**Independent Test**: Can be fully tested by configuring `enable_uroot_utils: true` in config, writing an invowkfile that uses file operations (`cp`, `mv`, `cat`), and executing it on a minimal system without those binaries installed. Delivers value by enabling invowk to work in environments where coreutils are unavailable.

**Acceptance Scenarios**:

1. **Given** a user has `enable_uroot_utils: true` in their config and a script using `cp file1 file2`, **When** the script runs in virtual runtime on a system without `/bin/cp`, **Then** the file is successfully copied using the built-in u-root implementation.

2. **Given** a user has `enable_uroot_utils: false` (default), **When** a script tries to run `cp` without `/bin/cp` on the system, **Then** the command fails with "command not found" (current behavior preserved).

3. **Given** u-root utils are enabled, **When** a script runs `ls -la /some/directory`, **Then** the output matches the behavior of standard coreutils `ls` for common use cases.

---

### User Story 2 - Predictable Cross-Platform Behavior (Priority: P2)

A team maintaining invowkfiles that must work across Linux, macOS, and Windows wants consistent behavior for basic file operations regardless of the underlying operating system's utility implementations (GNU coreutils vs BSD vs Windows).

**Why this priority**: Cross-platform consistency is a secondary benefit that builds on the core u-root functionality. It matters for teams with heterogeneous development environments.

**Independent Test**: Can be tested by running the same invowkfile with file operations on Linux, macOS, and Windows (with virtual runtime + u-root enabled) and verifying identical output and behavior for supported operations.

**Acceptance Scenarios**:

1. **Given** an invowkfile with `cat file | grep pattern | wc -l`, **When** executed on Linux, macOS, and Windows with u-root enabled, **Then** the results are identical across all platforms.

2. **Given** a script using `cp -r source/ dest/`, **When** executed with u-root enabled, **Then** the recursive copy behavior is consistent across platforms (no BSD vs GNU flag differences).

---

### User Story 3 - Gradual Adoption with Fallback (Priority: P3)

An operator wants to try u-root utilities for specific commands while keeping system utilities as fallback for unsupported or edge-case scenarios.

**Why this priority**: Provides a migration path and safety net. Less critical than core functionality but important for production adoption.

**Independent Test**: Can be tested by enabling u-root and running scripts with both supported commands (e.g., `cp`) and unsupported commands (e.g., `git`), verifying u-root handles the former while the latter falls back to system binaries.

**Acceptance Scenarios**:

1. **Given** u-root utils are enabled and a script runs `cp file1 file2 && git status`, **When** `cp` is supported by u-root and `git` is not, **Then** `cp` uses u-root implementation while `git` falls back to system binary.

2. **Given** a u-root command implementation exists but fails, **When** the same command exists on the system, **Then** the system implementation is NOT automatically used as fallback (explicit behavior, no silent fallback that could mask bugs).

---

### Edge Cases

- **Unsupported flags**: When a command includes flags not supported by the u-root implementation (e.g., GNU extensions like `--color`), the unsupported flags are silently ignored and the command executes with supported flags only.
- **Symlink handling**: Follow symlinks by default (copy/move the target file content, not the link itself). This matches standard POSIX `cp` behavior and prevents symlink-based path traversal. Symlink preservation requires explicit `-P` flag when supported.
- **Large file handling**: All file operations MUST use streaming I/O regardless of file size. No buffering of entire file contents into memory. This prevents OOM conditions and ensures predictable memory usage.
- **Error reporting**: Errors from u-root utilities are prefixed with `[uroot]` to clearly identify the source (e.g., `[uroot] cp: cannot stat 'missing': No such file`). This aids debugging by distinguishing u-root errors from system utility errors.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide built-in implementations of 15 core file utilities when `enable_uroot_utils` is configured:
  - **pkg/core wrappers** (7): `cat`, `cp`, `ls`, `mkdir`, `mv`, `rm`, `touch`
  - **Custom implementations** (8): `cut`, `grep`, `head`, `sort`, `tail`, `tr`, `uniq`, `wc`

  Note: `echo` and `pwd` are shell builtins provided by mvdan/sh and do not require u-root implementations.

- **FR-002**: System MUST allow users to enable/disable u-root utilities via the existing `virtual_shell.enable_uroot_utils` configuration option.

- **FR-003**: System MUST delegate to system utilities for commands not implemented by u-root integration.

- **FR-004**: System MUST preserve the current behavior (system utilities only) when `enable_uroot_utils` is false or unset.

- **FR-005**: u-root utility implementations MUST support the most commonly used flags for each command as documented in POSIX specifications.

#### Supported Flags by Utility

| Utility | Supported Flags | Notes |
|---------|-----------------|-------|
| `cat` | `-n`, `-b`, `-s` | Number lines, number non-blank, squeeze blank |
| `cp` | `-r`, `-R`, `-f`, `-n`, `-P` | Recursive, force, no-clobber, no-follow-symlinks |
| `cut` | `-d`, `-f`, `-c`, `-b` | Delimiter, fields, characters, bytes |
| `grep` | `-i`, `-v`, `-c`, `-n`, `-l`, `-E`, `-F` | Case-insensitive, invert, count, line numbers, files-only, extended regex, fixed strings |
| `head` | `-n`, `-c` | Lines, bytes |
| `ls` | `-l`, `-a`, `-R`, `-h` | Long format, all files, recursive, human-readable |
| `mkdir` | `-p`, `-m` | Parents, mode |
| `mv` | `-f`, `-n` | Force, no-clobber |
| `rm` | `-r`, `-R`, `-f` | Recursive, force |
| `sort` | `-r`, `-n`, `-u`, `-k`, `-t` | Reverse, numeric, unique, key, delimiter |
| `tail` | `-n`, `-c`, `-f` | Lines, bytes, follow (limited support) |
| `touch` | `-c`, `-m`, `-a` | No-create, modify-time, access-time |
| `tr` | `-d`, `-s`, `-c` | Delete, squeeze, complement |
| `uniq` | `-c`, `-d`, `-u`, `-i` | Count, duplicates-only, unique-only, case-insensitive |
| `wc` | `-l`, `-w`, `-c`, `-m` | Lines, words, bytes, characters |

Unsupported flags (e.g., GNU extensions like `--color`) are silently ignored.

- **FR-006**: System MUST report errors from u-root utilities with a `[uroot]` prefix (e.g., `[uroot] cp: cannot stat 'missing': No such file`) to clearly identify the error source and aid debugging.

- **FR-007**: System MUST NOT silently fall back to system utilities when a u-root implementation fails (to prevent masking implementation bugs).

### Key Entities

- **u-root Handler**: The mechanism within VirtualRuntime that intercepts commands and routes them to u-root implementations when enabled.
- **Command Registry**: A mapping of command names to their u-root implementations.
- **Execution Context**: The runtime context (stdin, stdout, stderr, working directory, environment) passed to u-root command implementations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can execute invowkfiles containing basic file operations (`cp`, `mv`, `cat`, `ls`, `mkdir`, `rm`) in the virtual runtime on systems without those binaries installed when u-root is enabled.

- **SC-002**: 100% of the 15 u-root utilities (`cat`, `cp`, `cut`, `grep`, `head`, `ls`, `mkdir`, `mv`, `rm`, `sort`, `tail`, `touch`, `tr`, `uniq`, `wc`) pass their respective test suites when enabled.

- **SC-003**: Virtual runtime with u-root enabled produces identical output for supported commands across Linux, macOS, and Windows for common use cases.

- **SC-004**: Existing invowkfiles that work with `enable_uroot_utils: false` continue to work identically (no regression for users who don't enable the feature).

- **SC-005**: Documentation clearly lists which commands are implemented via u-root and which flags are supported for each.

## Assumptions

- The u-root project (https://github.com/u-root/u-root) provides importable Go packages for individual commands that can be integrated into mvdan/sh's exec handler.
- The u-root utilities are designed to be POSIX-compatible and will provide acceptable behavior for common use cases without implementing every GNU extension.
- Binary size increase from u-root dependencies is not a constraint. Functionality and self-containment take priority over binary size optimization.
- Users who need full GNU coreutils compatibility can keep `enable_uroot_utils: false` and rely on system binaries.

## Out of Scope

- Implementing every command available in u-root (115+ commands) - this feature focuses on the 16 most commonly used file utilities.
- Achieving 100% GNU coreutils flag compatibility - POSIX compliance is the target.
- Automatic fallback from failed u-root commands to system utilities (explicit design decision for debuggability).
- The u-root utilities in native or container runtimes (virtual runtime only).

## Clarifications

### Session 2026-01-30

- Q: When a user runs a command with flags not supported by u-root (e.g., GNU extensions like `ls --color=auto`), what should happen? → A: Silently ignore the unsupported flag and execute the command without it.
- Q: What threshold defines "large file" behavior for Go-based utilities, and should streaming be required? → A: Always stream regardless of file size (no threshold). This is the safest approach to prevent OOM conditions.
- Q: How should symlinks be handled when copying files? → A: Follow symlinks by default (copy the target file content, not the link itself). This is the safest default matching standard `cp` behavior.
- Q: How should errors from u-root utilities be reported? → A: Prefixed format with `[uroot]` prefix (e.g., `[uroot] cp: cannot stat 'missing': No such file`) to clearly identify the error source.
- Q: What's the maximum acceptable binary size increase from adding u-root utilities? → A: No limit. Binary size is not a constraint for this feature.
