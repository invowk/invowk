---
paths:
  - "internal/container/**"
  - "internal/runtime/**"
  - "internal/provision/**"
  - "pkg/platform/**"
---

# Windows Compatibility

This Go codebase builds and runs on Windows. Follow these rules to ensure cross-platform compatibility.

## Path Handling

### The Two Path Domains

There are two distinct path domains in container-related code:

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

### Host Absolute Paths in Config Includes

`includes[*].path` and `container.auto_provision.includes[*].path` are **host paths** validated in Go with `filepath.IsAbs()`.

- On Linux/macOS, `/path/to/module.invowkmod` is absolute.
- On Windows, `/path/to/module.invowkmod` is **not** absolute; use `C:\path\to\module.invowkmod` (or UNC paths).
- Keep this OS-native behavior unless there is an explicit product decision to change it.

When writing tests for this validation:
- Build valid absolute fixtures with `t.TempDir()` + `filepath.Join(...)`.
- Keep relative-path negative cases (`relative/...`, `./...`) explicit.
- If a case is intentionally Unix-only, use `skipOnWindows` with a clear reason.

### Common Patterns

**Converting host path to container path:**
```go
// CORRECT: Convert Windows backslashes to forward slashes
containerPath := "/workspace/" + filepath.ToSlash(relPath)

// WRONG: filepath.Join uses backslashes on Windows
containerPath := filepath.Join("/workspace", relPath)  // Produces \workspace\relPath on Windows!
```

**Checking if a path is "container-absolute":**
```go
// CORRECT: Check for Unix-style absolute (starts with /)
isContainerAbsolute := strings.HasPrefix(path, "/")

// WRONG: filepath.IsAbs uses platform semantics
filepath.IsAbs("/app")  // Returns FALSE on Windows!
```

**Joining container path segments:**
```go
// CORRECT: Use path.Join for container paths (always uses /)
import "path"
containerPath := path.Join("/workspace", "scripts", "run.sh")

// WRONG: filepath.Join uses platform separators
filepath.Join("/workspace", "scripts", "run.sh")  // Backslashes on Windows
```

### Existing Correct Patterns

The codebase already has correct patterns in `internal/runtime/container_provision.go`:
```go
// Line 206: Convert to container path
containerPath := "/workspace/" + filepath.ToSlash(relPath)

// Line 263: Same pattern
return "/workspace/" + filepath.ToSlash(relPath)
```

## Test Assertions

For path assertions and the `skipOnWindows` pattern, see `testing.md` § "Cross-Platform Testing". Key rules:
- Never hardcode path separators — use `filepath.Join()` for host paths.
- Use the `skipOnWindows` table-driven pattern when test cases are semantically meaningless on Windows.
- When the file also imports `internal/runtime`, use `goruntime "runtime"` alias for `GOOS` checks.

### When to Skip vs When to Adapt

| Scenario | Action |
|----------|--------|
| Test uses Unix-style absolute path (`/app`) | Skip on Windows |
| Test checks platform-specific behavior | Use `runtime.GOOS` conditionals |
| Test verifies path joining | Use `filepath.Join()` in expectations |
| Test checks container paths | Ensure test inputs use forward slashes |

## Reserved Names and Limitations

### Windows Reserved Filenames

Windows has reserved filenames that cannot be used. The `pkg/platform/windows.go` package provides detection:

```go
import "github.com/invowk/invowk/pkg/platform"

if platform.IsReservedWindowsName(filename) {
    return fmt.Errorf("cannot use reserved Windows filename: %s", filename)
}
```

**Reserved names:** `CON`, `PRN`, `AUX`, `NUL`, `COM1`-`COM9`, `LPT1`-`LPT9`

### Path Length Limits

Windows has a default path length limit of 260 characters (`MAX_PATH`). While modern Windows can exceed this with long path support, avoid creating deeply nested paths in tests and generated files.

### Illegal Characters

Windows paths cannot contain: `< > : " | ? *`

These are valid in Unix filenames but will cause failures on Windows.

## CI/CD Considerations

### Windows CI Configuration

The CI workflow (`.github/workflows/ci.yml`) runs Windows tests in "short" mode:
```yaml
- os: windows
  runner: windows-latest
  engine: ""  # No container engine on Windows CI
  test-mode: short
```

**Key points:**
- Windows CI runs unit tests only (no container integration tests)
- Container runtime tests are skipped on Windows
- All path handling code must still be cross-platform correct

### Testing Locally on Windows

```bash
# Run tests with race detector
go test -race ./...

# Run specific package
go test -v ./internal/container/...

# Skip integration tests
go test -short ./...
```

### PowerShell Script Testing in Testscript

Native runtime tests (`native_*.txtar`) include PowerShell implementations for Windows. These tests exercise the native shell path through Invowk, where Invowk internally selects PowerShell as the native shell on Windows.

**How native tests work in CI:**
- The testscript runner itself uses the Go test runner (bash-based on all CI platforms)
- The `exec invowk cmd ...` calls within testscript invoke Invowk, which internally selects the native shell per platform
- On Windows CI: Invowk selects PowerShell to execute the command's script
- The testscript `stdout`/`stderr` assertions verify the output regardless of which shell produced it

**PowerShell version guidance:**
- Scripts MUST be compatible with both PowerShell 5.1 (Windows built-in) and PowerShell 7+ (cross-platform)
- Avoid PowerShell 7-only features (e.g., ternary operator `? :`, null-coalescing `??`, pipeline chain operators `&&`/`||`)
- Use `Write-Output` instead of `echo` for explicit intent

**Common PowerShell CI pitfalls:**

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Using `$VAR` instead of `$env:VAR` | Empty value; PowerShell treats `$VAR` as PS variable | Always use `$env:VAR` for environment variables |
| `\r\n` line endings in output | Extra blank lines or assertion mismatches | Testscript normalizes line endings; no action needed |
| Using bash comparison syntax (`=`, `!=`) | PowerShell parse error | Use `-eq`, `-ne`, `-lt`, `-gt`, `-like`, `-match` |
| Using `echo` for reliable output | Inconsistent behavior across PS versions | Use `Write-Output` for string output |
| Relying on `set -e` behavior | PowerShell does not support `set -e` | Use `$ErrorActionPreference = 'Stop'` at script start |

**Local testing on Windows:**
```powershell
# Run all CLI tests (includes native tests)
make test-cli

# Run a specific native test
go test -v -run TestCLI/native_simple ./tests/cli/...
```

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Using `filepath.Join()` for container paths | Backslashes in container paths on Windows | Use `path.Join()` or string concatenation with `/` |
| Using `filepath.IsAbs()` for container paths | `/app` not detected as absolute on Windows | Use `strings.HasPrefix(path, "/")` |
| Hardcoded `/` in test expectations | Tests fail on Windows | Use `filepath.Join()` for host paths |
| Forgetting `filepath.ToSlash()` | Container receives Windows-style paths | Always convert before passing to container |
| Testing Unix absolute paths on Windows | Test fails or has wrong behavior | Add `skipOnWindows: true` (see `testing.md`) |

## Quick Reference

**Before writing path-related code, ask:**
1. Is this a **host path** or **container path**?
2. Will this code run on Windows CI?
3. Are test assertions platform-independent?

**Container path checklist:**
- [ ] Using `filepath.ToSlash()` when converting from host path
- [ ] Using `path.Join()` or string concat with `/` for joining
- [ ] NOT using `filepath.IsAbs()` to check container paths
- [ ] Tests skip or adapt for Windows where needed
