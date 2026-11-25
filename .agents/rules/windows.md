---
paths:
  - "internal/container/**"
  - "internal/runtime/**"
  - "internal/provision/**"
  - "internal/config/**"
  - "internal/app/**"
  - "internal/modulelock/**"
  - "internal/audit/**"
  - "internal/uroot/**"
  - "internal/watch/**"
  - "internal/testutil/**"
  - "pkg/platform/**"
  - "pkg/fspath/**"
  - "pkg/types/**"
  - "pkg/invowkmod/**"
  - "pkg/invowkfile/**"
  - "tests/cli/*.go"
  - "tests/cli/testdata/native_*.txtar"
---

# Windows Compatibility

## Path Handling

### The Two Path Domains

| Domain | Separator | Example | When Used |
|--------|-----------|---------|-----------|
| **Host paths** | Platform-native (`\` on Windows, `/` on Unix) | `C:\app\config.json` | File I/O, CLI arguments for local files |
| **Container paths** | Always forward slash (`/`) | `/workspace/script.sh` | Docker/Podman container interiors |

### Critical Functions

| Function | Behavior | Use For |
|----------|----------|---------|
| `filepath.Join()` | Platform-native separators | Host filesystem paths |
| `filepath.IsAbs()` | Platform-specific (`/app` is NOT absolute on Windows) | Checking host paths |
| `filepath.ToSlash()` | Converts `\` to `/` | Converting host path → container path |
| `filepath.FromSlash()` | Converts `/` to native | Converting container path → host path |
| `path.Join()` | Always uses `/` | Joining container path segments |

### Repo-Relative Path Value Types (Cross-Platform Contract)

Typed values that represent **repository-relative paths** (for example `SubdirectoryPath` in module metadata) validate the same way on every OS:
1. Normalize separators first: `filepath.ToSlash(input)`.
2. Canonicalize with slash semantics: `path.Clean(...)`.
3. Reject absolute forms across platforms: slash-rooted (`/foo`), Windows drive prefix (`C:/foo`), UNC/rooted forms (`//server/share`, `\\server\share`, `\foo`).
4. Reject traversal after normalization (`..`, `../...`).

Do **not** rely on `filepath.IsAbs()` alone for repo-relative typed validation. It is host-dependent and hides Windows failures when tests run on Unix hosts. Test such validators with the matrix in `testing.md` "Cross-Platform Path Validator Matrix".

### Host Absolute Paths in Config Includes

`includes[*].path` and `container.auto_provision.includes[*].path` are **host paths** validated in Go with `filepath.IsAbs()`. On Linux/macOS `/path/to/module.invowkmod` is absolute; on Windows it is **not** (use `C:\path\to\module.invowkmod` or UNC paths). Keep this OS-native behavior unless there is an explicit product decision to change it. Test fixtures: `testing.md` "Cross-Platform Testing".

### Container Path Patterns

- Host → container: `containerWorkspacePrefix + filepath.ToSlash(relPath)`, never `filepath.Join("/workspace", relPath)` (backslashes on Windows). Reference implementation: `buildInterpreterCommand` and `getContainerWorkDir` in `internal/runtime/container_provision.go`.
- Container-absolute check: a leading `/` (see `isContainerAbsolutePath`), never `filepath.IsAbs` (`/app` is not absolute on Windows).
- Join container segments with `path.Join`, never `filepath.Join`.
- Always `filepath.ToSlash()` a host path before passing it to a container.

## Tests

- Path assertions, `skipOnWindows`, absolute fixtures, the `goruntime` alias and PowerShell rules for `native_*.txtar` live in `testing.md`.
- Unix-style absolute inputs (`/app`) → skip on Windows; platform behavior → `runtime.GOOS` conditionals; path joins → `filepath.Join()` in expectations; container-path tests → forward-slash inputs.
- Windows and macOS CI run `-short` unit tests (so `testing.Short()` integration tests skip) plus the CLI tests, with no container engine (container tests skip). All path handling code must still be cross-platform correct.

## Reserved Names and Limits

- Reserved filenames (`CON`, `PRN`, `AUX`, `NUL`, `COM1`-`COM9`, `LPT1`-`LPT9`): check with `platform.IsWindowsReservedName` (`pkg/platform/windows.go`).
- Default `MAX_PATH` is 260 characters: avoid deeply nested paths in tests and generated files.
- Windows paths cannot contain `< > : " | ? *`.

## Process Pitfall

On Windows, `exec.CommandContext` kills via `TerminateProcess(pid, 1)`, and exit code 1 looks valid, so a context timeout can be silently dropped. `promoteContextError()` in `internal/runtime/native_helpers.go` promotes `ctx.Err()` when `result.Error == nil`; keep that pattern in new process-running code.

Related skills: `windows-testing` (process lifecycle, NTFS, timers, PowerShell), `go-testing`.
