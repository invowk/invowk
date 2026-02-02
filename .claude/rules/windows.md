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

### Path Assertions

**NEVER hardcode path separators in test assertions:**
```go
// WRONG: Hardcoded Unix separator - fails on Windows
expected := "/app/config/settings.json"

// CORRECT: Use filepath.Join for host path expectations
expected := filepath.Join("/app", "config", "settings.json")
```

### Platform-Specific Test Cases

**Skip tests that are semantically meaningless on a platform:**
```go
// Test struct with skip field
tests := []struct {
    name          string
    // ... other fields
    skipOnWindows bool
}{
    {
        name:          "Unix absolute path",
        path:          "/custom/Dockerfile",
        skipOnWindows: true, // Unix-style absolutes don't exist on Windows
    },
}

// In test loop
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        if tt.skipOnWindows && runtime.GOOS == "windows" {
            t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
        }
        // ... test logic
    })
}
```

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
import "invowk-cli/pkg/platform"

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

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Using `filepath.Join()` for container paths | Backslashes in container paths on Windows | Use `path.Join()` or string concatenation with `/` |
| Using `filepath.IsAbs()` for container paths | `/app` not detected as absolute on Windows | Use `strings.HasPrefix(path, "/")` |
| Hardcoded `/` in test expectations | Tests fail on Windows | Use `filepath.Join()` for host paths |
| Forgetting `filepath.ToSlash()` | Container receives Windows-style paths | Always convert before passing to container |
| Testing Unix absolute paths on Windows | Test fails or has wrong behavior | Add `skipOnWindows: true` |

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
