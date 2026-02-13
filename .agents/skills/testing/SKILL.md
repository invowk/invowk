---
name: testing
description: Testing patterns for *_test.go files, testscript CLI tests (.txtar), race conditions, TUI component testing, container test timeouts. Use when writing tests, debugging flaky tests, or setting up testscript.
disable-model-invocation: false
---

# Testing Patterns

Use this skill when:
- Writing or modifying test files (`*_test.go`)
- Working with testscript CLI integration tests (`.txtar` files)
- Debugging flaky tests or race conditions
- Testing TUI components (Bubble Tea models)
- Testing container runtimes (Docker/Podman)

**Cross-references**: For universal test patterns (table-driven tests, `testing.Short()`, `skipOnWindows`, `t.TempDir()`, testscript HOME fix, container test timeouts, cross-platform path assertions, test file size limits), see `.claude/rules/testing.md`. This skill covers domain-specific testing guidance that extends those rules.

---

# Testing

## Test File Organization

### Test Helper Consolidation

**Avoid duplicating test helpers across packages.** Common patterns belong in the `testutil` package:

```go
// WRONG: Duplicated in multiple test files
func testCommand(name, script string) Command { ... }

// CORRECT: Centralized in testutil
import "invowk-cli/internal/testutil/invowkfiletest"
cmd := invowkfiletest.NewTestCommand("hello", invowkfiletest.WithScript("echo hello"))
```

When you need a test helper that might be useful elsewhere, add it to `testutil` with clear documentation.

**Acceptable exceptions (local helpers are OK when):**

1. **Same-package testing**: Test files in `pkg/invowkfile/` cannot import `internal/testutil/invowkfiletest` because it would create an import cycle (invowkfiletest imports invowkfile). Local helpers like `testCommand()` are acceptable in this case.

2. **Specialized signatures**: Helpers with package-specific signatures that don't generalize well (e.g., `testCommandWithInterpreter()` in runtime tests for interpreter-specific testing).

3. **Single-use helpers**: Helpers used only within one test file that aren't worth extracting.

**Current intentional local helpers:**
- `pkg/invowkfile/invowkfile_deps_test.go`: `testCommand()`, `testCommandWithDeps()` (import cycle)
- `internal/runtime/runtime_env_test.go`: `testCommandWithScript()`, `testCommandWithInterpreter()` (specialized signatures)

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

1. **Unit tests**: Use per-test `MockCommandRecorder` instances injected via `WithExecCommand()` — never use package-level global mutation
2. **Integration tests**: Gate with `testing.Short()` and require actual container engine
3. **All container tests run in parallel** with `t.Parallel()` — transient Podman errors are handled by production `runWithRetry()` retry logic

```go
// Unit test with per-test mock recorder (parallel-safe)
func TestDockerBuild_Arguments(t *testing.T) {
    t.Parallel()

    t.Run("basic build", func(t *testing.T) {
        t.Parallel()
        recorder := NewMockCommandRecorder()
        engine := newTestDockerEngine(t, recorder) // Injects mock via WithExecCommand()

        engine.Build(ctx, opts)

        // Verify expected arguments were passed
        if !contains(recorder.LastArgs, "--no-cache") {
            t.Error("expected --no-cache flag")
        }
    })
}

// Integration test with real container engine
func TestDockerBuild_Integration(t *testing.T) {
    t.Parallel()
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // ... test with real Docker ...
}
```

**Important**: Never share a `MockCommandRecorder` across parallel subtests with `Reset()`. Each parallel subtest must create its own recorder and engine instance.

### Container Runtime Test Conditions

**Custom conditions** for container tests must verify actual functionality, not just CLI availability:

```go
containerAvailable = func() bool {
    engine, err := container.AutoDetectEngine()
    if err != nil || !engine.Available() {
        return false
    }
    // CRITICAL: Run a smoke test to verify Linux containers work.
    // This catches Windows Docker in Windows-container mode.
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    result, err := engine.Run(ctx, container.RunOptions{
        Image:   "debian:stable-slim",
        Command: []string{"echo", "ok"},
        Remove:  true,
    })
    return err == nil && result.ExitCode == 0
}()
```

### Shell Script Behavior in Containers

Scripts executed via `/bin/sh -c` do NOT have `set -e` by default. Always add `set -e` when you want scripts to fail on any command failure. **Note:** This applies to CUE command scripts executed in container runtimes, not to project-level bash scripts (for those, see the shell skill).

```cue
script: """
    set -e  # Required for fail-on-error behavior
    echo "Starting..."
    some_command_that_might_fail
    echo "Done"
    """
```

## CLI Integration Tests (testscript)

CLI integration tests use [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) for deterministic output verification. Tests live in `tests/cli/testdata/` as `.txtar` files.

### Running CLI Tests

```bash
make test-cli    # Run CLI integration tests
make test        # Runs all tests including CLI tests
```

### Working Directory Management

**CRITICAL**: Do NOT set `env.Cd` in test setup. Each test must control its own working directory.

```go
// BAD: Sets initial CWD that conflicts with tests' cd commands
Setup: func(env *testscript.Env) error {
    env.Cd = projectRoot  // NEVER do this!
    return nil
},

// GOOD: Let each test control its own working directory via environment variable
Setup: func(env *testscript.Env) error {
    binDir := filepath.Dir(binaryPath)
    env.Setenv("PATH", binDir+string(os.PathListSeparator)+env.Getenv("PATH"))
    env.Setenv("PROJECT_ROOT", projectRoot)  // Tests can use 'cd $PROJECT_ROOT'
    return nil
},
```

**Two types of tests require different working directories:**

1. **Tests with embedded `invowkfile.cue`** - Use `cd $WORK`:
   ```txtar
   # Set working directory to where embedded files are
   cd $WORK
   exec invowk cmd my-embedded-command
   ```

2. **Tests against project's `invowkfile.cue`** - Use `cd $PROJECT_ROOT`:
   ```txtar
   # Run against project's invowkfile.cue
   cd $PROJECT_ROOT
   exec invowk cmd some-project-command
   ```

### Environment Variables in Setup

- Only set environment variables that are actually used by production code.
- Do NOT set placeholder env vars "for future use" - they cause confusion.
- If a test needs a specific env var cleared, do it in the test file: `env MY_VAR=`

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

### Test File Structure

Each `.txtar` test file should:
1. Have a descriptive comment at the top explaining what it tests.
2. Include skip conditions for optional features (e.g., `[!container-available] skip`).
3. Use `cd $WORK` to set working directory if it uses embedded files.
4. Include the embedded `invowkfile.cue` and any other required files.

Example structure:
```txtar
# Test: Description of what this tests
# Tests specific behavior X and verifies Y

# Skip if required feature is unavailable
[!container-available] skip 'no functional container runtime available'

# Set working directory to where test files are
cd $WORK

# Run tests
exec invowk cmd my-command
stdout 'expected output'

-- invowkfile.cue --
cmds: [...]

-- other-file.txt --
content
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

### Cross-Platform Testscript Patterns

**All testscript tests must support Linux, macOS, and Windows** unless platform-specific by design.

**Standard cross-platform command pattern:**
```cue
cmds: [{
    name: "test-cmd"
    description: "Description"
    implementations: [{
        script: "echo 'output'"
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}]
```

**Linux-only container command pattern:**
```cue
cmds: [{
    name: "test-cmd"
    description: "Description"
    implementations: [{
        script: "echo 'output'"
        runtimes: [{name: "container", image: "debian:stable-slim"}]
        platforms: [{name: "linux"}]
    }]
}]
```

**Native-only platform-split pattern** (for `native_*.txtar` mirror tests):

Basic hello (native-only, platform-split):
```cue
implementations: [
    {
        script:    "echo 'Hello from invowk!'"
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script:    "Write-Output 'Hello from invowk!'"
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

Env vars access (native with `$env:VAR` for Windows):
```cue
implementations: [
    {
        script: """
            echo "APP_ENV: $APP_ENV"
            echo "LOG_LEVEL: $LOG_LEVEL"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
        env: { vars: { LOG_LEVEL: "debug" } }
    },
    {
        script: """
            Write-Output "APP_ENV: $($env:APP_ENV)"
            Write-Output "LOG_LEVEL: $($env:LOG_LEVEL)"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
        env: { vars: { LOG_LEVEL: "debug" } }
    },
]
```

Flags/args (native with `$env:INVOWK_ARG_NAME` for Windows):
```cue
implementations: [
    {
        script: """
            echo "Name: $INVOWK_ARG_NAME"
            echo "Env: $INVOWK_FLAG_ENV"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: """
            Write-Output "Name: $($env:INVOWK_ARG_NAME)"
            Write-Output "Env: $($env:INVOWK_FLAG_ENV)"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

Conditionals (native with `if`/`else` for each shell):
```cue
implementations: [
    {
        script: """
            if [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
                echo "Verbose mode ON"
            else
                echo "Verbose mode OFF"
            fi
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: """
            if ($env:INVOWK_FLAG_VERBOSE -eq 'true') {
                Write-Output 'Verbose mode ON'
            } else {
                Write-Output 'Verbose mode OFF'
            }
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

### PowerShell Equivalents Reference

When writing native test mirrors for Windows, use these translations:

**Output and Variables:**

| Bash/Zsh | PowerShell | Notes |
|----------|------------|-------|
| `echo "text"` | `Write-Output "text"` | `echo` is an alias but `Write-Output` is explicit |
| `echo -n "text"` | `Write-Host -NoNewline "text"` | No trailing newline |
| `$VAR` | `$env:VAR` | Environment variable access |
| `$($VAR)` | `$($env:VAR)` | Embedded in double-quoted strings |
| `export VAR=val` | `$env:VAR = 'val'` | Set environment variable |

**Conditionals:**

| Bash/Zsh | PowerShell | Notes |
|----------|------------|-------|
| `if [ "$V" = "x" ]; then ... fi` | `if ($env:V -eq 'x') { ... }` | String equality |
| `if [ -n "$V" ]; then ...` | `if ($env:V) { ... }` | Non-empty check |
| `if [ -z "$V" ]; then ...` | `if (-not $env:V) { ... }` | Empty check |
| `if [ "$A" != "$B" ]; then ...` | `if ($env:A -ne $env:B) { ... }` | Inequality |

**Loops and Flow Control:**

| Bash/Zsh | PowerShell | Notes |
|----------|------------|-------|
| `for x in a b c; do ... done` | `foreach ($x in @('a','b','c')) { ... }` | Iteration |
| `exit 1` | `exit 1` | Same syntax |
| `set -e` | `$ErrorActionPreference = 'Stop'` | Fail on error |

**String Operations:**

| Bash/Zsh | PowerShell | Notes |
|----------|------------|-------|
| `${VAR:-default}` | `if ($env:VAR) { $env:VAR } else { 'default' }` | Default value |
| `${VAR^^}` | `$env:VAR.ToUpper()` | Uppercase |
| `${VAR,,}` | `$env:VAR.ToLower()` | Lowercase |

**Invowk-Specific Patterns:**

| Pattern | Bash/Zsh | PowerShell |
|---------|----------|------------|
| Flag check | `echo "flag: $INVOWK_FLAG_NAME"` | `Write-Output "flag: $($env:INVOWK_FLAG_NAME)"` |
| Arg access | `echo "arg: $INVOWK_ARG_NAME"` | `Write-Output "arg: $($env:INVOWK_ARG_NAME)"` |
| Variadic args | `echo "args: $INVOWK_ARGS"` | `Write-Output "args: $($env:INVOWK_ARGS)"` |

**Key PowerShell gotchas:**

1. **`$VAR` vs `$env:VAR`**: In PowerShell, `$VAR` is a PowerShell variable, `$env:VAR` is an environment variable. Invowk sets environment variables, so always use `$env:`.
2. **Interpolation in strings**: Use `$()` subexpression syntax inside double-quoted strings: `"Value: $($env:MY_VAR)"`.
3. **Line endings**: PowerShell on Windows may produce `\r\n`. Testscript normalizes line endings, so `stdout` assertions work cross-platform.
4. **Boolean comparisons**: PowerShell uses `-eq`, `-ne`, `-lt`, `-gt` operators, not `=`, `!=`, `<`, `>`.
5. **Semicolons as statement separators**: PowerShell uses newlines or `;` as statement separators, not `&&` or `||` (use `-and`/`-or` in conditionals).

### Current Test Files

| File | Runtime | Description | Strategy |
|------|---------|-------------|----------|
| `virtual_simple.txtar` | virtual | Basic hello + env hierarchy | Inline CUE, all platforms |
| `native_simple.txtar` | native | Native mirror of virtual_simple.txtar | Inline CUE, platform-split |
| `virtual_shell.txtar` | virtual | Virtual shell runtime tests | Inline CUE, all platforms |
| `virtual_flags.txtar` | virtual | Command flags | Inline CUE, all platforms |
| `native_flags.txtar` | native | Native mirror of virtual_flags.txtar | Inline CUE, platform-split |
| `virtual_args.txtar` | virtual | Positional arguments | Inline CUE, all platforms |
| `native_args.txtar` | native | Native mirror of virtual_args.txtar | Inline CUE, platform-split |
| `virtual_env.txtar` | virtual | Environment configuration | Inline CUE, all platforms |
| `native_env.txtar` | native | Native mirror of virtual_env.txtar | Inline CUE, platform-split |
| `virtual_isolation.txtar` | virtual | Variable isolation | Inline CUE, all platforms |
| `native_isolation.txtar` | native | Native mirror of virtual_isolation.txtar | Inline CUE, platform-split |
| `virtual_deps_tools.txtar` | virtual | Tool dependency checks | Inline CUE, all platforms |
| `native_deps_tools.txtar` | native | Native mirror of virtual_deps_tools.txtar | Inline CUE, platform-split |
| `virtual_deps_files.txtar` | virtual | File dependency checks | Inline CUE, all platforms |
| `native_deps_files.txtar` | native | Native mirror of virtual_deps_files.txtar | Inline CUE, platform-split |
| `virtual_deps_env.txtar` | virtual | Environment dependencies | Inline CUE, all platforms |
| `native_deps_env.txtar` | native | Native mirror of virtual_deps_env.txtar | Inline CUE, platform-split |
| `virtual_deps_caps.txtar` | virtual | Capability checks | Inline CUE, all platforms |
| `native_deps_caps.txtar` | native | Native mirror of virtual_deps_caps.txtar | Inline CUE, platform-split |
| `virtual_deps_custom.txtar` | virtual | Custom validation | Inline CUE, all platforms |
| `native_deps_custom.txtar` | native | Native mirror of virtual_deps_custom.txtar | Inline CUE, platform-split |
| `virtual_uroot_basic.txtar` | virtual | U-root basic utilities (exempt) | Inline CUE, all platforms |
| `virtual_uroot_file_ops.txtar` | virtual | U-root file operations (exempt) | Inline CUE, all platforms |
| `virtual_uroot_text_ops.txtar` | virtual | U-root text processing (exempt) | Inline CUE, all platforms |
| `virtual_multi_source.txtar` | virtual | Multi-source discovery (exempt) | Inline CUE, all platforms |
| `virtual_ambiguity.txtar` | virtual | Ambiguous command detection (exempt) | Inline CUE, all platforms |
| `virtual_disambiguation.txtar` | virtual | Disambiguation prompt (exempt) | Inline CUE, all platforms |
| `virtual_edge_cases.txtar` | virtual | Edge case handling (exempt) | Inline CUE, all platforms |
| `virtual_args_subcommand_conflict.txtar` | virtual | Args+subcommand conflict (exempt) | Inline CUE, all platforms |
| `dogfooding_invowkfile.txtar` | native | Project invowkfile smoke test (exempt) | `$PROJECT_ROOT` (dogfooding) |
| `container_*.txtar` | container | Container runtime tests (exempt) | Inline CUE, Linux only |

### When to Add CLI Tests

Add CLI tests when:
- Adding new CLI commands or subcommands
- Changing command output format
- Modifying flag/argument handling
- Testing environment variable behavior

**Native mirror creation checklist:**
1. Create `virtual_<feature>.txtar` with virtual runtime (all platforms)
2. Create `native_<feature>.txtar` with native runtime (platform-split)
3. Verify both files produce identical `stdout` assertions
4. Ensure Windows PowerShell implementations use `$env:VAR` and `Write-Output`
5. Run `make test-cli` to validate both virtual and native tests pass

**Exempt from native mirrors** (do NOT create `native_` versions for):
- `virtual_uroot_*.txtar` — u-root commands are virtual shell built-ins
- `virtual_shell.txtar` — tests virtual shell-specific behavior
- `container_*.txtar` — Linux-only container runtime
- `virtual_ambiguity.txtar`, `virtual_disambiguation.txtar`, `virtual_multi_source.txtar` — command resolution logic
- `virtual_edge_cases.txtar`, `virtual_args_subcommand_conflict.txtar` — CUE schema validation
- `dogfooding_invowkfile.txtar` — already exercises native runtime

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
| `SetHomeDir(t, dir)` | Sets HOME/USERPROFILE; returns cleanup function |
| `NewFakeClock(initial)` | Creates fake clock for time mocking |
| `Clock` interface | `Now()`, `After(d)`, `Since(t)` for time abstraction |
| `RealClock` | Production clock using actual time |
| `FakeClock` | Test clock with `Advance(d)` and `Set(t)` |

### invowkfiletest Package

**`internal/testutil/invowkfiletest`** (separate package to avoid import cycles):

| Function | Description |
|----------|-------------|
| `NewTestCommand(name, opts...)` | Creates test command with options pattern |
| `WithScript(s)`, `WithRuntime(r)` | Command options for script, runtime |
| `WithFlag(name, opts...)` | Add flag with `FlagRequired()`, `FlagDefault(v)` |
| `WithArg(name, opts...)` | Add arg with `ArgRequired()`, `ArgVariadic()` |

## Race Condition Testing

### TOCTOU Race Conditions

TOCTOU (Time-Of-Check-Time-Of-Use) race conditions occur when there's a gap between checking a condition and acting on it, during which the condition can change. These are particularly common in concurrent Go code with goroutines.

#### Context Cancellation Race Pattern

When a function accepts a `context.Context` and spawns goroutines, there's a race between:
1. The goroutine completing its work
2. The caller detecting context cancellation

**Vulnerable Pattern:**

```go
func (s *Server) Start(ctx context.Context) error {
    // Setup work that may succeed even with cancelled context
    listener, err := lc.Listen(ctx, "tcp", addr)  // May succeed!
    if err != nil {
        return err
    }

    // Start goroutine that transitions state
    go func() {
        s.state.Store(StateRunning)  // Wins the race!
        close(s.startedCh)
        s.serve()
    }()

    // Race: goroutine may complete before this select runs
    select {
    case <-s.startedCh:
        return nil  // Returns success even though ctx was cancelled
    case <-ctx.Done():
        return ctx.Err()  // Never reached if goroutine wins
    }
}
```

**The Solution - Check context cancellation before any setup work:**

```go
func (s *Server) Start(ctx context.Context) error {
    // Early exit if context is already cancelled
    select {
    case <-ctx.Done():
        s.transitionToFailed(fmt.Errorf("context cancelled before start: %w", ctx.Err()))
        return s.lastErr
    default:
    }

    // Now safe to proceed with setup...
    listener, err := lc.Listen(ctx, "tcp", addr)
    // ...
}
```

**Key Principles:**
1. **Check early**: Validate preconditions (including context) before any work
2. **Check at boundaries**: Re-check context after long-running or async operations
3. **Atomic state transitions**: Use `CompareAndSwap` for state changes to prevent concurrent transitions
4. **Don't trust non-blocking success**: Even if an operation succeeds, the context may have been cancelled

### Testing Race Conditions

When fixing race conditions:

```bash
# Run multiple times with race detector, bypassing cache
for i in {1..10}; do
    go test -count=1 -race ./path/to/package/... -run TestName
done
```

- `-count=1`: Bypasses test cache, forces fresh execution
- `-race`: Enables Go's race detector
- Run 10+ times: A single pass doesn't prove the race is fixed

### Common Symptom: Flaky CI Tests

If a test passes locally but fails in CI (or vice versa), suspect a race condition. Different CPU speeds, scheduling, and runner configurations affect goroutine timing.

**Real-World Example** (GitHub Action failure on ubuntu-latest):
```
=== RUN   TestServerStartWithCancelledContext
INFO ssh-server: SSH server started address=127.0.0.1:45163
    server_test.go:307: Start with cancelled context should return error
    server_test.go:313: State should be Failed, got stopped
--- FAIL: TestServerStartWithCancelledContext
```

The test passed on slower runners (ubuntu-24.04) but failed on faster ones where the goroutine consistently won the race.

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Large test files | Hard to navigate, maintain | Split files exceeding 800 lines by logical concern (see rules/testing.md) |
| Duplicated helpers | Same code in multiple test files | Consolidate in `testutil` package |
| `time.Sleep()` in tests | Flaky, timing-dependent failures | Use clock injection for deterministic tests |
| Testing struct fields | Testing Go's ability to store values | Test behavior, not struct storage |
| Missing TUI tests | State bugs not caught | Test model state transitions |
| Flaky tests across environments | Passes locally, fails in CI | Suspect race conditions; run with `-race` |
| Setting `env.Cd` in testscript Setup | Tests find wrong `invowkfile.cue` | Remove `env.Cd`, let tests use `cd $WORK` |
| CLI-only container check | Windows tests run but fail | Add smoke test that runs actual container |
| Missing `set -e` in scripts | Failed commands don't cause script failure | Add `set -e` at script start |
| Unused env vars in testscript Setup | Confusion, false assumptions | Only set vars used by production code |

For cross-platform testing pitfalls (path separators, `skipOnWindows`, `filepath.Join()`), see `.claude/rules/testing.md` and `.claude/rules/windows.md`.
