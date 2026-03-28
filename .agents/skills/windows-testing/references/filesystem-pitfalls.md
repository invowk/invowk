# Windows Filesystem Pitfalls for Go Tests

Deep dive on Windows filesystem behavior that causes test failures. This
reference supports the parent skill (`SKILL.md`) with detailed explanations
and mitigation strategies.

## NTFS Case-Insensitivity

NTFS is **case-preserving** but **case-insensitive** by default. This means:

- Creating `Foo.txt` stores the name as `Foo.txt`.
- Attempting to create `foo.txt` in the same directory references the
  existing `Foo.txt` -- it does NOT create a second file.
- `os.Open("FOO.TXT")` successfully opens `Foo.txt`.
- `os.Stat("foo.txt")` returns information about `Foo.txt`.

### Impact on Go tests

- Tests that create two files with case-variant names (e.g., `Config.cue`
  and `config.cue`) will silently overwrite on Windows.
- Path comparison tests that rely on exact case will behave differently:
  `filepath.Clean("Foo/Bar")` preserves case, but file operations treat
  `Foo/Bar` and `foo/bar` as equivalent.
- Discovery tests that scan directories may return different-cased names
  than expected if the files were created with a different case.

### Mitigation

- When comparing file paths in tests, normalize to lowercase for comparison
  or use `strings.EqualFold`.
- Avoid creating files with case-variant names in the same test directory.
- If testing case-sensitive behavior, use `skipOnWindows` with a comment
  explaining that NTFS is case-insensitive.

## MAX_PATH (260 Characters)

The traditional Windows path length limit is 260 characters (`MAX_PATH`),
which includes the drive letter, colon, backslash, path components, and
null terminator. The effective usable length is 259 characters.

### When it applies

- Most Win32 API calls enforce MAX_PATH by default.
- The `cmd.exe` shell and PowerShell enforce it for command-line arguments.
- File Explorer enforces it in many operations.

### Long path support

Windows 10 (version 1607+) supports long paths when:
1. The `LongPathsEnabled` registry key is set to 1
   (`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`).
2. The application manifest declares `longPathAware = true`.

Go 1.21+ handles long paths internally by using extended-length path
syntax (`\\?\` prefix) for most file operations. However:
- Not all Go standard library functions support extended-length paths.
- External tools (compilers, container engines) may not support them.
- `t.TempDir()` names include the full test name, which can be long for
  table-driven subtests with descriptive names.

### Impact on Go tests

- `t.TempDir()` generates paths like:
  `C:\Users\runneradmin\AppData\Local\Temp\TestLongNamedFunction_very_descriptive_subtest_name123456789`
- With deeply nested test fixture directories, the total path can exceed
  260 characters.
- CI runners (GitHub Actions `windows-latest`) may have long base paths.

### Mitigation

- Keep test names reasonably short for table-driven subtests.
- Avoid deeply nesting directories in test fixtures (3-4 levels max).
- If long paths are unavoidable, test on Windows 10+ with long path support
  enabled (GitHub Actions runners have this enabled by default).
- Use `os.MkdirTemp("", "short")` with short prefixes when `t.TempDir()`
  paths are too long, but remember to handle cleanup manually.

## Reserved Names

Windows reserves the following filenames. They cannot be used as file or
directory names, regardless of extension:

| Category | Names |
|----------|-------|
| Device names | `CON`, `PRN`, `AUX`, `NUL` |
| Serial ports | `COM1` through `COM9` |
| Parallel ports | `LPT1` through `LPT9` |

These names are reserved regardless of:
- Extension: `NUL.txt`, `CON.log` are also reserved.
- Case: `con`, `Con`, `CON` are all reserved.
- Directory depth: `test/subdir/NUL` is reserved.

### Detection

Use `pkg/platform`'s detection function:

```go
import "github.com/invowk/invowk/pkg/platform"

if platform.IsReservedWindowsName(filename) {
    // Skip or adapt the test case
}
```

### Impact on Go tests

- `t.TempDir()` subdirectory creation fails if a reserved name is used.
- Module or command names that happen to match reserved names will fail
  on Windows.
- Test fixtures with reserved names in their paths will not work.

### Mitigation

- Never use reserved names in test fixtures or generated file names.
- Use `platform.IsReservedWindowsName` to validate user-provided names.
- If testing reserved name handling, use `[!windows]` guard or
  `skipOnWindows`.

## ERROR_SHARING_VIOLATION (0x80070020)

This error occurs when a file is opened by one process and another process
attempts to open it in an incompatible sharing mode. On Windows, file
locking is **mandatory** (unlike Unix where locks are **advisory**).

### Common causes in tests

1. **Windows Defender real-time scanning**: Defender scans newly created
   files in temp directories. While scanning, it holds the file open with
   a sharing mode that may block other opens. This is the most common
   cause of intermittent test failures on Windows.

2. **Antivirus/indexer services**: Windows Search Indexer, other antivirus
   software, and backup agents may scan files in temp directories.

3. **Test subprocess holding handles**: A child process started by the test
   may still have file handles open when the test tries to clean up.

4. **Concurrent test access**: Multiple parallel tests accessing the same
   file without proper synchronization.

### Symptoms

- `os.Remove` or `os.RemoveAll` fails with "The process cannot access the
  file because it is being used by another process."
- `os.Open` or `os.Create` fails intermittently with ACCESS_DENIED or
  SHARING_VIOLATION.
- `t.TempDir()` cleanup reports errors at test end.
- Failures are intermittent and timing-dependent.

### Retry pattern

```go
func removeWithRetry(path string) error {
    var err error
    for attempt := range 5 {
        err = os.RemoveAll(path)
        if err == nil {
            return nil
        }
        // Wait for Defender/indexer to release the file.
        time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
    }
    return fmt.Errorf("failed to remove %s after 5 attempts: %w", path, err)
}
```

### Mitigation

- Use `t.TempDir()` instead of manual temp directory management. Go's
  testing framework handles cleanup retries on some platforms.
- Close all file handles before attempting deletion.
- Ensure subprocesses are fully terminated (check `cmd.Wait()` returned)
  before cleaning up their files.
- Set `cmd.WaitDelay` to ensure I/O goroutines finish and release handles.
- If retries are needed, use exponential backoff (100ms, 200ms, 300ms...).

## Mandatory vs Advisory File Locking

### Windows: Mandatory locking

When a Windows process opens a file, the OS enforces the sharing mode:

- `FILE_SHARE_READ`: Other processes can open for reading.
- `FILE_SHARE_WRITE`: Other processes can open for writing.
- `FILE_SHARE_DELETE`: Other processes can delete/rename.
- No sharing flags: Exclusive access; all other opens fail.

Go's `os.Open` uses `FILE_SHARE_READ | FILE_SHARE_WRITE` by default,
which allows concurrent access. But Defender and other system services may
use more restrictive sharing modes.

### Unix: Advisory locking

Unix file locks (`flock`, `fcntl`) are advisory -- they are only enforced
if the accessing process explicitly checks for locks. An `os.Open` call
always succeeds regardless of any locks held by other processes.

### Impact on tests

- On Unix, tests can freely open, read, and delete files even when other
  processes have them open.
- On Windows, tests may get `ERROR_SHARING_VIOLATION` when accessing files
  that system services (Defender, indexer) are scanning.
- Concurrent test functions in the same package can conflict if they access
  the same files without synchronization.

## Junction Points vs Symlinks

Windows has two types of filesystem links:

### Junction points (directory only)

- Created without special privileges.
- Only work for directories, not files.
- Only work for local paths (not network paths).
- Resolved at the filesystem level (transparent to applications).
- Created via `mklink /J target link_name`.

### Symbolic links

- Require **Developer Mode** or **SeCreateSymbolicLinkPrivilege** (admin).
- Work for both files and directories.
- Work for network paths.
- `os.Symlink` on Windows creates symlinks but may fail without privileges.

### Impact on Go tests

- `os.Symlink` may fail on Windows CI runners that don't have Developer
  Mode enabled. GitHub Actions `windows-latest` runners DO have the
  necessary privileges.
- Tests that create symlinks should handle `os.IsPermission` errors and
  skip on Windows if needed.
- `filepath.EvalSymlinks` works for both junction points and symlinks.

### Mitigation

- Prefer `os.Symlink` (it works on GitHub Actions runners).
- If symlink creation might fail, use `t.Skip` with an explanatory message.
- For cross-platform tests, consider using file copies instead of symlinks.

## Alternate Data Streams

NTFS supports Alternate Data Streams (ADS), which are additional named data
streams attached to a file. The syntax is `filename:streamname`.

- `os.Stat("file.txt")` reports only the size of the default stream.
- ADS are invisible to most Go file operations.
- Rarely encountered in practice, but can affect file size calculations
  and backup/restore operations.

**For invowk**: ADS are not used and do not affect the codebase. Mentioned
here for completeness when debugging unusual file size discrepancies.

## .gitattributes and core.autocrlf

Git's line ending handling can corrupt binary fixtures and change checksums:

### The problem

- `core.autocrlf=true` (common on Windows Git installations) converts `\n`
  to `\r\n` on checkout for text files.
- Txtar fixtures contain embedded file content. If Git converts line endings,
  the content changes, which can:
  - Break SHA hash comparisons.
  - Change file sizes.
  - Cause regex assertions to fail.

### The invowk solution

The project's `.gitattributes` file forces LF line endings for txtar files:

```
*.txtar text eol=lf
```

This ensures txtar fixtures have consistent `\n` line endings regardless of
the developer's `core.autocrlf` setting.

### Verification

If a txtar test fails on Windows with unexpected content, check:
1. Is `.gitattributes` correctly applied? Run `git check-attr eol *.txtar`.
2. Was the file re-checked out after `.gitattributes` was added?
   Run `git rm --cached *.txtar && git checkout *.txtar`.

## File Deletion Semantics

### Windows file deletion

Windows file deletion is more complex than Unix `unlink`:

- `os.Remove` calls `DeleteFile` which marks the file for deletion. The
  file is not actually removed until all handles to it are closed.
- If any handle is open, `DeleteFile` fails with `ERROR_SHARING_VIOLATION`
  (unless `FILE_SHARE_DELETE` was used and Windows 10 1809+ POSIX delete
  semantics apply).
- `MoveFileEx` with `MOVEFILE_DELAY_UNTIL_REBOOT` can schedule deletion
  for files that cannot be deleted immediately (not useful in tests).

### Unix file deletion

- `unlink` removes the directory entry immediately. The file data remains
  accessible to any process that has it open (via its file descriptor) until
  the last handle is closed.
- This is why Unix tests rarely have file deletion issues -- even if a
  subprocess has a file open, the test can delete it.

### Impact on tests

- `os.RemoveAll` in test cleanup may fail on Windows if subprocesses still
  have files open.
- `t.TempDir()` cleanup may report errors for the same reason.
- The order of operations matters: always terminate subprocesses and close
  file handles BEFORE attempting cleanup.

### Mitigation

- Use `cmd.WaitDelay` to ensure subprocess I/O is fully drained.
- Call `cmd.Wait()` (or verify context cancellation completed) before
  cleanup.
- Use `t.TempDir()` which handles cleanup errors more gracefully than
  manual `defer os.RemoveAll`.
- If manual cleanup is needed, implement a retry loop (see the
  ERROR_SHARING_VIOLATION section above).

## Illegal Filename Characters

Windows paths cannot contain these characters: `< > : " | ? *`

These characters are valid in Unix filenames. Tests that create files with
these characters will fail on Windows.

Additionally:
- Filenames cannot end with a space or period.
- Filenames cannot consist entirely of spaces.

### Mitigation

- Sanitize filenames in cross-platform code before file creation.
- Use `skipOnWindows` for test cases that intentionally use illegal
  characters.
- The colon (`:`) is particularly tricky because it is used for both drive
  letters (`C:`) and Alternate Data Streams (`file:stream`).
