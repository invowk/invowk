# Testing

## Test File Organization

### Size Limits

**Test files MUST NOT exceed 800 lines.** Large monolithic test files are difficult to navigate and maintain for both humans and AI agents. When a test file approaches this limit, split it by logical concern.

**Naming convention for split files:**
- `<package>_<concern>_test.go` (e.g., `invkfile_parsing_test.go`, `invkfile_deps_test.go`)
- Each file should cover a single logical area (parsing, dependencies, flags, schema validation, etc.)

### Test Helper Consolidation

**Avoid duplicating test helpers across packages.** Common patterns belong in the `testutil` package:

```go
// WRONG: Duplicated in multiple test files
func testCommand(name, script string) Command { ... }

// CORRECT: Centralized in testutil
import "invowk-cli/internal/testutil/invkfiletest"
cmd := invkfiletest.NewTestCommand("hello", invkfiletest.WithScript("echo hello"))
```

When you need a test helper that might be useful elsewhere, add it to `testutil` with clear documentation.

**Acceptable exceptions (local helpers are OK when):**

1. **Same-package testing**: Test files in `pkg/invkfile/` cannot import `internal/testutil/invkfiletest` because it would create an import cycle (invkfiletest imports invkfile). Local helpers like `testCommand()` are acceptable in this case.

2. **Specialized signatures**: Helpers with package-specific signatures that don't generalize well (e.g., `testCommandWithInterpreter()` in runtime tests for interpreter-specific testing).

3. **Single-use helpers**: Helpers used only within one test file that aren't worth extracting.

**Current intentional local helpers:**
- `pkg/invkfile/invkfile_deps_test.go`: `testCommand()`, `testCommandWithDeps()` (import cycle)
- `internal/runtime/runtime_env_test.go`: `testCommandWithScript()`, `testCommandWithInterpreter()` (specialized signatures)

## Testing Patterns

- Test files are named `*_test.go` in the same package.
- Use `t.TempDir()` for temporary directories (auto-cleaned).
- Use table-driven tests for multiple cases.
- Skip integration tests with `if testing.Short() { t.Skip(...) }`.
- Reset global state in tests using cleanup functions.

```go
func TestExample(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    originalEnv := os.Getenv("VAR")
    defer os.Setenv("VAR", originalEnv)

    // Test
    result, err := DoSomething()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Assert
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

## Avoiding Flaky Tests

### Time-Dependent Tests

**NEVER use `time.Sleep()` to verify time-dependent behavior.** This creates flaky tests that fail intermittently based on system load.

```go
// WRONG: Flaky - may pass or fail based on system speed
func TestTokenExpiration(t *testing.T) {
    token := createToken(ttl: 1*time.Millisecond)
    time.Sleep(10 * time.Millisecond)  // FRAGILE!
    if token.IsValid() {
        t.Error("token should be expired")
    }
}

// CORRECT: Deterministic - use clock injection
func TestTokenExpiration(t *testing.T) {
    clock := testutil.NewFakeClock(time.Time{})
    token := createTokenWithClock(ttl: 1*time.Minute, clock: clock)
    clock.Advance(2 * time.Minute)  // Deterministic advance
    if token.IsValid() {
        t.Error("token should be expired")
    }
}
```

### Filesystem Paths

**Always use `t.TempDir()` instead of hardcoded paths like `/tmp`.**

```go
// WRONG: May fail on some systems, leaves files behind
f, _ := os.Create("/tmp/test-file.txt")

// CORRECT: Auto-cleaned, isolated per test
tmpDir := t.TempDir()
f, _ := os.Create(filepath.Join(tmpDir, "test-file.txt"))
```

## TUI Component Testing

TUI components (Bubble Tea models) should have unit tests even though terminal I/O is difficult to mock. Focus on:

1. **Model state transitions**: Test `Init()`, `Update()` with various messages
2. **Text processing**: Test formatting, truncation, wrapping logic
3. **Edge cases**: Empty inputs, very long inputs, unicode, special characters

```go
// Testing a Bubble Tea model without terminal I/O
func TestChooseModel_Navigation(t *testing.T) {
    model := NewChooseModel([]string{"a", "b", "c"})

    // Simulate key press
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})

    if model.selected != 1 {
        t.Errorf("expected selected=1, got %d", model.selected)
    }
}
```

## Container Runtime Testing

Container runtime code (Docker/Podman) should have both unit tests and integration tests:

1. **Unit tests**: Mock `exec.Command` to verify argument construction without running containers
2. **Integration tests**: Gate with `testing.Short()` and require actual container engine

```go
// Unit test with mocked exec
func TestDockerBuild_Arguments(t *testing.T) {
    execCmd = mockExecCommand  // Inject mock
    defer func() { execCmd = exec.Command }()

    engine := &DockerEngine{}
    engine.Build(ctx, opts)

    // Verify expected arguments were passed
    if !contains(capturedArgs, "--no-cache") {
        t.Error("expected --no-cache flag")
    }
}

// Integration test with real container engine
func TestDockerBuild_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // ... test with real Docker ...
}
```

## CLI Integration Tests (testscript)

CLI integration tests use [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) for deterministic output verification. Tests live in `tests/cli/testdata/` as `.txtar` files.

### Running CLI Tests

```bash
make test-cli    # Run CLI integration tests
make test        # Runs all tests including CLI tests
```

### Writing testscript Tests

Test files use the txtar format with inline assertions:

```txtar
# Test: Basic command execution
exec invowk cmd hello
stdout 'Hello from invowk!'
! stderr .

# Test: Command with flags (use -- to separate invowk flags from command flags)
exec invowk cmd 'flags validation' -- --env=staging
stdout '=== Flag Validation Demo ==='
! stderr .
```

### testscript Syntax Reference

| Command | Description |
|---------|-------------|
| `exec cmd args...` | Run a command |
| `stdout 'pattern'` | Assert stdout matches regex pattern |
| `stderr 'pattern'` | Assert stderr matches regex pattern |
| `! stdout .` | Assert stdout is empty |
| `! stderr .` | Assert stderr is empty |
| `env VAR=value` | Set environment variable |
| `cd path` | Change working directory |

### Environment Isolation

testscript runs tests in an isolated environment:
- `HOME` is set to `/no-home` by default
- `USER` and other env vars are not passed through
- Use `env VAR=value` to explicitly set required variables

Example for tests that need environment variables:

```txtar
env HOME=/test-home
env USER=testuser

exec invowk cmd 'deps env single'
stdout 'HOME = '
```

### Flag Separator (`--`)

When passing flags to invowk commands (not to invowk itself), use `--` to separate:

```txtar
# WRONG: --env is interpreted as invowk global flag
exec invowk cmd 'flags validation' --env=staging

# CORRECT: -- separates invowk flags from command flags
exec invowk cmd 'flags validation' -- --env=staging
```

### Current Test Files

| File | Description |
|------|-------------|
| `simple.txtar` | Basic hello + env hierarchy |
| `virtual.txtar` | Virtual shell runtime |
| `deps_tools.txtar` | Tool dependency checks |
| `deps_files.txtar` | File dependency checks |
| `deps_caps.txtar` | Capability checks |
| `deps_custom.txtar` | Custom validation |
| `deps_env.txtar` | Environment dependencies |
| `flags.txtar` | Command flags |
| `args.txtar` | Positional arguments |
| `env.txtar` | Environment configuration |
| `isolation.txtar` | Variable isolation |

### When to Add CLI Tests

Add CLI tests when:
- Adding new CLI commands or subcommands
- Changing command output format
- Modifying flag/argument handling
- Testing environment variable behavior

## VHS Demo Recordings

VHS is used **only for generating demo GIFs** for documentation and website, not for CI testing.

### Generating Demos

```bash
make vhs-demos     # Generate all demo GIFs (requires VHS, ffmpeg, ttyd)
make vhs-validate  # Validate VHS tape syntax
```

Demo tapes live in `vhs/demos/`. See `vhs/README.md` for details.

## testutil Package Reference

The `internal/testutil` package provides reusable test helpers. All helpers accept `testing.TB` to work with both `*testing.T` and `*testing.B`.

### Current Public API

| Function | Description |
|----------|-------------|
| `MustChdir(t, dir)` | Changes working directory; returns cleanup function |
| `MustSetenv(t, key, value)` | Sets environment variable; returns cleanup function |
| `MustUnsetenv(t, key)` | Unsets environment variable; returns cleanup function |
| `MustMkdirAll(t, path, perm)` | Creates directory tree; fails test on error |
| `MustRemoveAll(t, path)` | Removes path; logs warning on error |
| `MustClose(t, closer)` | Closes io.Closer; fails test on error |
| `MustStop(t, stopper)` | Stops server; logs warning on error |
| `DeferClose(t, closer)` | Returns cleanup function for io.Closer |
| `DeferStop(t, stopper)` | Returns cleanup function for Stopper |

### New Helpers (003-test-suite-audit)

**internal/testutil** (clock and home directory):

| Function | Description |
|----------|-------------|
| `SetHomeDir(t, dir)` | Sets HOME/USERPROFILE; returns cleanup function |
| `NewFakeClock(initial)` | Creates fake clock for time mocking |
| `Clock` interface | `Now()`, `After(d)`, `Since(t)` for time abstraction |
| `RealClock` | Production clock using actual time |
| `FakeClock` | Test clock with `Advance(d)` and `Set(t)` |

**internal/testutil/invkfiletest** (command builder - separate package to avoid import cycles):

| Function | Description |
|----------|-------------|
| `NewTestCommand(name, opts...)` | Creates test command with options pattern |
| `WithScript(s)`, `WithRuntime(r)` | Command options for script, runtime |
| `WithFlag(name, opts...)` | Add flag with `FlagRequired()`, `FlagDefault(v)` |
| `WithArg(name, opts...)` | Add arg with `ArgRequired()`, `ArgVariadic()` |

## Common Pitfalls

- **Large test files** - Split test files exceeding 800 lines by logical concern.
- **Duplicated helpers** - Consolidate common test patterns in `testutil` package.
- **`time.Sleep()` in tests** - Use clock injection for deterministic time-dependent tests.
- **Hardcoded `/tmp` paths** - Use `t.TempDir()` for isolation and auto-cleanup.
- **Testing struct fields** - Test behavior, not that Go can store values in structs.
- **Missing TUI tests** - Test model state transitions even if terminal I/O can't be mocked.
