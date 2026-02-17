---
paths:
  - "**/*_test.go"
  - "tests/cli/**"
  - "**/testdata/**"
---

# Testing

## Test File Size Limits

**Test files MUST NOT exceed 800 lines.** Large monolithic test files are difficult to navigate and maintain. When a test file approaches this limit, split it by logical concern.

**Naming convention for split files:** `<package>_<concern>_test.go` (e.g., `invowkfile_parsing_test.go`, `invowkfile_deps_test.go`). Each file should cover a single logical area.

**Splitting protocol:** When moving test functions to a new file, follow the File Splitting Protocol from `go-patterns.md`. The most common mistake is copying tests to the new file but forgetting to delete the originals — Go test files in the same package share a namespace, so duplicate `Test*` function names cause compiler errors. After moving:
1. Delete the moved test functions from the source file.
2. Clean up unused imports in the source file (e.g., `"errors"`, `"fmt"` that were only used by moved tests).
3. Run `go build ./path/to/package/...` immediately to catch duplicates before running `make test`.

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
- Global state mutators: `testutil.MustSetenv()`, `testutil.MustChdir()`
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

**Layer 3: CI Test Runner with Retry and Timeout (Safety Net)**
```yaml
run: |
  gotestsum --format testdox --junitfile test-results.xml --rerun-fails \
    --rerun-fails-max-failures 5 --packages ./... -- -race -timeout 15m
```

**Why three layers:**
1. **3-minute deadline** catches individual test hangs early
2. **Cleanup via `env.Defer()`** prevents orphaned containers from accumulating
3. **15-minute CI timeout** catches catastrophic failures faster than Go's default 10m

**Layer 4: Test-Level Concurrency Limiting**

All container integration tests must acquire a slot from the process-wide container semaphore before running container operations. This prevents Podman resource exhaustion on constrained CI runners.

```go
sem := testutil.ContainerSemaphore()
sem <- struct{}{}
defer func() { <-sem }()
```

Place this **after** `t.Parallel()` and any `testing.Short()` skip, but **before** any container operations. The semaphore is a `sync.OnceValue` singleton — each test binary gets its own instance. Default capacity is `min(GOMAXPROCS, 2)`, overridable via `INVOWK_TEST_CONTAINER_PARALLEL` env var (set to `"2"` in CI).

**When to use the semaphore:**
- Integration tests that run real container operations (Execute, ExecuteCapture, Build)
- CLI testscript tests that invoke container commands

**When NOT to use the semaphore:**
- Unit tests with mocked container engines
- Validation-only tests that don't start containers (e.g., `Validate()`, type assertions)
- Error-path tests that fail before container operations (e.g., missing SSH server)

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

Never hardcode path separators in assertions — use `filepath.Join()` for host paths. Container paths (always forward slash) are fine as literals. For the full path handling guide (host vs container paths, `filepath.ToSlash()`, common patterns), see `windows.md` § "Path Handling".

### Absolute Fixture Pattern (Host Paths)

When validation logic relies on OS-native absoluteness (`filepath.IsAbs`), avoid hardcoded Unix-style fixtures as "valid absolute" inputs.

```go
// CORRECT: OS-native absolute fixture
absPath := filepath.Join(t.TempDir(), "modules", "tools.invowkmod")

// RELATIVE NEGATIVE CASES: keep explicit
relPath := "modules/tools.invowkmod"
dotRelPath := "./modules/tools.invowkmod"
```

Why: `/foo/bar` is absolute on Unix but not on Windows. Using `t.TempDir()` + `filepath.Join` keeps tests valid on all CI platforms.

### Import Alias for runtime Package

When testing code that needs both the `runtime` package and `runtime.GOOS`, use an import alias:

```go
import (
    goruntime "runtime"  // For GOOS checks

    "github.com/invowk/invowk/internal/runtime"  // For Runtime types
)
```

## Cross-Platform Test Coverage Requirements

### Mandatory Platform Coverage

**CRITICAL**: All user-facing features MUST be tested on Linux, macOS, AND Windows.

The CI pipeline tests on three platform tiers:
- **Linux (Ubuntu)**: Full test suite (unit + integration + CLI + container)
- **macOS**: Unit tests (short) + CLI integration tests
- **Windows**: Unit tests (short) + CLI integration tests

### Runtime Coverage Requirements

Each test file exercises a specific runtime path through the system:

| Runtime | Linux Shell | macOS Shell | Windows Shell | When Used |
|---------|-------------|-------------|---------------|-----------|
| **virtual** | mvdan/sh | mvdan/sh | mvdan/sh | Cross-platform POSIX scripts (default for feature tests) |
| **native** | bash | zsh | PowerShell | Platform-specific shell integration |
| **container** | /bin/sh (in container) | N/A | N/A | Linux-only container execution |

**What native runtime tests catch that virtual tests miss:**
- Shell-specific quoting/escaping differences (bash vs zsh vs PowerShell)
- Environment variable expansion syntax (`$VAR` vs `$env:VAR`)
- Platform-specific command availability and behavior
- Real shell startup behavior (profile loading, PATH resolution)
- PowerShell-specific gotchas (cmdlet aliases, object pipeline, encoding)

### Full Feature Mirror Requirement

**CRITICAL**: Every feature-level testscript file that uses the virtual runtime MUST have a corresponding native runtime mirror that tests the same behaviors using native shell implementations.

The pattern is:
- `virtual_<feature>.txtar` — Tests with `runtimes: [{name: "virtual"}]`, all platforms
- `native_<feature>.txtar` — Tests with `runtimes: [{name: "native"}]`, platform-split CUE (bash for Linux/macOS, PowerShell for Windows)

This ensures that features work correctly through both the virtual shell (mvdan/sh) and each platform's native shell.

### Exemptions from the Feature Mirror

| Test Category | Files | Reason |
|---------------|-------|--------|
| **u-root** | `virtual_uroot_*.txtar` (all u-root tests) | u-root commands are virtual shell built-ins; native shell has its own implementations |
| **virtual shell** | `virtual_shell.txtar` | Tests virtual-shell-specific features (u-root integration, cross-platform POSIX semantics) |
| **container** | `container_*.txtar` | Linux-only by design; container runtime is not a native shell |
| **CUE validation** | `virtual_edge_cases.txtar`, `virtual_args_subcommand_conflict.txtar` | Tests schema parsing and validation, not runtime behavior |
| **discovery/ambiguity** | `virtual_ambiguity.txtar`, `virtual_disambiguation.txtar`, `virtual_multi_source.txtar`, `virtual_diagnostics_footer.txtar` | Tests command resolution logic and CLI presentation, not shell execution |
| **dogfooding** | `dogfooding_invowkfile.txtar` | Already exercises native runtime through the project's own invowkfile.cue |
| **built-in commands** | `config_*.txtar`, `module_*.txtar`, `completion.txtar`, `tui_format.txtar`, `tui_style.txtar`, `init_*.txtar`, `validate.txtar` | Built-in Cobra commands exercise CLI handlers directly, not user-defined command runtimes |

### Testscript (.txtar) Test Strategy

**Preferred: Inline CUE** — All new testscript tests MUST use inline CUE:

````txtar
cd $WORK

exec invowk cmd hello
stdout 'Hello!'

-- invowkfile.cue --
cmds: [{
    name: "hello"
    description: "Test command"
    implementations: [{
        script: "echo 'Hello!'"
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}]
````

**Why inline CUE over referencing project invowkfile.cue:**
- Self-contained: test is readable without cross-referencing
- Cross-platform: can declare all platforms using virtual runtime
- Isolated: changes to project invowkfile.cue don't break feature tests
- Portable: runs on all CI platforms without platform skips

**Exception: Dogfooding tests** in `tests/cli/testdata/dogfooding_*.txtar` MAY reference `$PROJECT_ROOT` to validate the actual project configuration.

### Native Runtime Tests

Native runtime tests use platform-split CUE with separate implementations for Unix shells (bash/zsh) and Windows PowerShell:

````cue
implementations: [
    {
        script: """
            echo "Hello from native shell"
            echo "VAR=$MY_VAR"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: """
            Write-Output "Hello from native shell"
            Write-Output "VAR=$($env:MY_VAR)"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
````

**Key design rules for native tests:**

1. **Always platform-split**: Every native command needs separate Linux/macOS and Windows implementations. Bash syntax fails in PowerShell and vice versa.
2. **Same assertions**: The `stdout` assertions in the txtar file must be identical for all platforms — only the script syntax differs, not the output.
3. **Use `Write-Output` on Windows**: Prefer `Write-Output` over `echo` in PowerShell implementations for consistent behavior.
4. **Use `$env:VAR` on Windows**: PowerShell accesses environment variables via `$env:VAR`, not `$VAR`.
5. **No virtual fallback**: Native tests declare `runtimes: [{name: "native"}]` only — adding a virtual fallback would defeat the purpose of testing native shell behavior.

### Platform Skip Policy

**Allowed platform skips** (document the design constraint):
- `[!container-available] skip` — Container tests (Linux-only by design)
- `[!net] skip` — Tests requiring network connectivity
- `[in-sandbox] skip` — Tests incompatible with Flatpak/Snap sandboxes
- `[GOOS:windows] skip` — Genuine Windows platform limitations (e.g., Unix permission bits, hardcoded `/tmp` in upstream code)

**NOT allowed** (fix the implementation instead):
- `[windows] skip 'command only has linux/macos implementations'`
- `[macos] skip 'not implemented'`

### When Adding New Commands

1. All implementations MUST declare all applicable platforms
2. Use virtual runtime for cross-platform portability
3. Create both `virtual_<feature>.txtar` (all platforms) and `native_<feature>.txtar` (platform-split CUE for bash on Linux/macOS and PowerShell on Windows) mirrors
4. Add testscript test with inline CUE covering all platforms
5. Container runtime commands are exempt (Linux-only by design)

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
| Windows testscript tests share config dirs | `commonSetup()` must set `APPDATA=WorkDir/appdata` and `USERPROFILE=WorkDir` on Windows. Without this, all tests share the real `%APPDATA%\invowk`, causing cross-test contamination (e.g., one test's `config init` affects another) |
| CI pre-sets `XDG_CONFIG_HOME` breaking shell tests | Ubuntu CI runners set `XDG_CONFIG_HOME=/home/runner/.config`. Tests that rely on XDG fallback (e.g., `${XDG_CONFIG_HOME:-${HOME}/.config}`) must `unset XDG_CONFIG_HOME` before testing the fallback path |
| Circular/trivial tests (constant == literal, zero-value == zero) | Test behavioral contracts: sentinel errors with `errors.Is`, default configs that affect user behavior, state machine transitions |
| Pattern guardrail tests fail after adding comments | `TestNoGlobalConfigAccess` scans all non-test `.go` files for prohibited call signatures using raw `strings.Contains`. Comments mentioning deprecated APIs (e.g., the old global config accessor) must use indirect phrasing. See go-patterns.md "Guardrail-safe references" |
| Testscript `[windows]` skip doesn't work | `commonCondition` returns `(false, nil)` for unknown conditions, so `[!windows]` is always true. Use `[GOOS:windows]` — it's a built-in testscript condition checked BEFORE the custom callback |
| SHA hash mismatches in txtar on Windows | Git `core.autocrlf=true` converts `\n` → `\r\n` in txtar fixtures, changing checksums. The `.gitattributes` file forces `*.txtar text eol=lf` project-wide |
| Issue templates contain stale guidance | `TestIssueTemplates_NoStaleGuidance` (`internal/issue/issue_test.go`) scans all embedded `.md` templates for stale tokens like `"invowk fix"` and `"apk add --no-cache"`. When updating issue templates, avoid Alpine-specific commands and deprecated CLI subcommands |
| Duplicate `Test*` declarations after file split | When splitting test files to stay under 800 lines, delete moved functions from the source file and clean up orphaned imports. Run `go build` before `make test` to catch duplicates early. See go-patterns.md "File Splitting Protocol" |
| New built-in command missing txtar test | `TestBuiltinCommandTxtarCoverage` (`cmd/invowk/coverage_test.go`) fails when a non-hidden, runnable, leaf built-in command has no txtar test. Add a `.txtar` file in `tests/cli/testdata/` with `exec invowk <command>` |
| Removing shared test helper breaks other test files | In Go, all `_test.go` files in the same package share a namespace. Removing a helper (e.g., `containsString` from `dotenv_test.go`) that is also called by `container_test.go` or `env_test.go` causes compile errors. Always grep the ENTIRE package for the function name before deleting it |
