---
paths:
  - "**/*_test.go"
  - "tests/cli/**"
  - "**/testdata/**"
---

# Testing

## Test File Size Limits

**Test files MUST NOT exceed 800 lines.** Large monolithic test files are difficult to navigate and maintain. When a test file approaches this limit, split it by logical concern.

**Naming convention for split files:** `<package>_<concern>_test.go` (e.g., `invkfile_parsing_test.go`, `invkfile_deps_test.go`). Each file should cover a single logical area.

## Test Organization

### Table-Driven Tests

Prefer table-driven tests for functions with multiple test cases:

```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    {
        name:  "valid input",
        input: "hello",
        want:  "HELLO",
    },
    {
        name:    "empty input",
        input:   "",
        wantErr: true,
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Transform(tt.input)
        if tt.wantErr {
            if err == nil {
                t.Error("expected error, got nil")
            }
            return
        }
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if got != tt.want {
            t.Errorf("got %q, want %q", got, tt.want)
        }
    })
}
```

## Test Parallelism

### Default Rule

All new test functions MUST call `t.Parallel()` unless they mutate global/process-wide state.

### Table-Driven Subtests

When a parent test calls `t.Parallel()`, **ALL** subtests inside `t.Run()` must also call `t.Parallel()`. This is enforced by the `tparallel` linter. If even one subtest cannot be parallelized, remove `t.Parallel()` from the parent too.

### Unsafe Patterns (do NOT parallelize)

Do not add `t.Parallel()` to tests that use any of these:
- `os.Chdir`, `os.Setenv`, or `t.Setenv` (process-wide side effects)
- Global state mutators: `config.Reset()`, `config.SetConfigDirOverride()`, `testutil.MustSetenv()`, `testutil.MustChdir()`
- `SetHomeDir` or similar process-wide overrides

### Critical Footgun: TempDir Lifetime with Parallel Subtests

Using `os.MkdirTemp` + `defer os.RemoveAll` in a parent test with `t.Parallel()` subtests causes data races — the parent's `defer` runs when the parent function returns, but parallel subtests are still executing.

**Fix:** Use `t.TempDir()` (lifecycle-managed by the testing framework), or do not parallelize the subtests.

### Correct Pattern

```go
func TestSomething(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name  string
        input string
        want  string
    }{
        {name: "case A", input: "a", want: "A"},
        {name: "case B", input: "b", want: "B"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // Required: parent has t.Parallel()

            got := Transform(tt.input)
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### Integration vs Unit Tests

- **Unit tests**: Fast, no external dependencies, run in short mode
- **Integration tests**: Require external resources (container engine, network, etc.)

Use `testing.Short()` to skip integration tests:

```go
func TestContainerRuntime_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // ... test code
}
```

### Testscript Environment for Container Tests

The `testscript` library intentionally sets `HOME=/no-home` to sandbox tests from user configuration. This breaks tools like Docker/Podman that require a valid `HOME` directory to store configuration in `~/.docker/` or `~/.config/containers/`.

**Symptom:** `mkdir /no-home: permission denied` errors during `docker build` or similar operations.

**Fix:** Set `HOME` to the testscript work directory in the `Setup` function:

```go
testscript.Run(t, testscript.Params{
    Dir: "testdata",
    Setup: func(env *testscript.Env) error {
        // Set HOME to $WORK directory for container build tests.
        // Docker/Podman CLI requires a valid HOME to store configuration.
        // By default, testscript sets HOME=/no-home which causes permission errors.
        env.Setenv("HOME", env.WorkDir)
        return nil
    },
})
```

**Why this works:**
- Each testscript test gets a unique `$WORK` directory that's automatically cleaned up
- Using `WorkDir` as `HOME` provides isolation while ensuring the directory exists and is writable
- Tests using pre-built images (e.g., `debian:stable-slim`) may not trigger this issue since they don't invoke `docker build`

### Container Test Timeout Strategy

Container tests use a multi-layer timeout strategy to prevent indefinite hangs:

**Layer 1: Per-Test Deadline (Primary)**
```go
deadline := time.Now().Add(3 * time.Minute)

testscript.Run(t, testscript.Params{
    Files:    []string{testFile},
    Setup:    containerSetup,
    Deadline: deadline,  // Enforces 3-minute max per test
})
```

**Layer 2: Container Cleanup on Timeout**
```go
func containerSetup(env *testscript.Env) error {
    // ... common setup ...

    testSuffix := generateTestSuffix(env.WorkDir)
    containerPrefix := "invowk-test-" + testSuffix

    // Cleanup runs regardless of test outcome (pass, fail, timeout, panic)
    env.Defer(func() {
        cleanupTestContainers(containerPrefix)
    })
    return nil
}
```

**Layer 3: CI Explicit Timeout (Safety Net)**
```yaml
run: go test -v -race -timeout 15m -coverprofile=coverage.out ./...
```

**Why three layers:**
1. **3-minute deadline** catches individual test hangs early
2. **Cleanup via `env.Defer()`** prevents orphaned containers from accumulating
3. **15-minute CI timeout** catches catastrophic failures faster than Go's default 10m

**When to adjust timeouts:**
- If tests consistently need more than 3 minutes (e.g., large image pulls), increase `containerTestTimeout`
- The 15m CI timeout should always exceed the sum of all sequential container test deadlines

## Cross-Platform Testing

### The skipOnWindows Pattern

When a test case is semantically meaningless on Windows (e.g., Unix-style absolute paths), use the `skipOnWindows` field pattern:

```go
import goruntime "runtime"

tests := []struct {
    name          string
    path          string
    want          string
    skipOnWindows bool
}{
    {
        name: "relative path",
        path: "relative/path",
        want: "relative/path",
    },
    {
        name:          "unix absolute path",
        path:          "/absolute/path",
        want:          "/absolute/path",
        skipOnWindows: true, // Unix-style absolute paths not meaningful on Windows
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        if tt.skipOnWindows && goruntime.GOOS == "windows" {
            t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
        }
        // ... test code
    })
}
```

**When to use `skipOnWindows`:**
- Test uses Unix-style absolute paths (`/foo/bar`)
- Test relies on Unix-specific behavior (symlinks, permissions)
- Test assumptions don't translate to Windows semantics

**When NOT to use `skipOnWindows`:**
- Test can be made cross-platform with `filepath.Join()`
- Platform differences are implementation bugs, not semantic differences

### Path Assertions

**Never hardcode path separators in assertions:**

```go
// WRONG: Fails on Windows
expected := "/app/config/file.json"

// CORRECT: Platform-aware path construction
expected := filepath.Join("/app", "config", "file.json")

// ALSO CORRECT: For container paths (always forward slash)
containerPath := "/workspace/scripts/run.sh"  // OK - container paths are always Unix-style
```

### Import Alias for runtime Package

When testing code that needs both the `runtime` package and `runtime.GOOS`, use an import alias:

```go
import (
    goruntime "runtime"  // For GOOS checks

    "invowk-cli/internal/runtime"  // For Runtime types
)
```

## Test Helpers

### Cleanup with t.TempDir()

Prefer `t.TempDir()` over manual temp directory creation:

```go
// PREFERRED: Automatic cleanup
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()
    // ... use tmpDir
}

// AVOID: Manual cleanup required
func TestSomething(t *testing.T) {
    tmpDir, err := os.MkdirTemp("", "test-*")
    if err != nil {
        t.Fatalf("failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)  // Easy to forget
    // ... use tmpDir
}
```

### Test Utility Functions

Use helpers from `internal/testutil`:

```go
// Environment variable management
restore := testutil.MustSetenv(t, "MY_VAR", "value")
defer restore()

// File cleanup
defer func() { testutil.MustRemoveAll(t, path) }()
```

## Common Pitfalls

| Pitfall | Fix |
|---------|-----|
| Hardcoded Unix paths in assertions | Use `filepath.Join()` or add `skipOnWindows` |
| Missing `testing.Short()` check | Add for any test requiring external resources |
| Time-based uniqueness tests failing | Add atomic counter or small delay |
| Import conflicts with `runtime` package | Use `goruntime` alias |
| Forgetting test cleanup | Use `t.TempDir()` and `defer` patterns |
| Testscript container tests fail with "mkdir /no-home" | Set `HOME` to `env.WorkDir` in Setup |
| Circular/trivial tests (constant == literal, zero-value == zero) | Test behavioral contracts: sentinel errors with `errors.Is`, default configs that affect user behavior, state machine transitions |
