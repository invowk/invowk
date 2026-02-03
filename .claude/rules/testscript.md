# Testscript CLI Tests

## Overview

CLI integration tests use `github.com/rogpeppe/go-internal/testscript` with `.txtar` files in `tests/cli/testdata/`.

## Key Rules

### 1. Working Directory Management

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

1. **Tests with embedded `invkfile.cue`** - Use `cd $WORK`:
   ```txtar
   # Set working directory to where embedded files are
   cd $WORK
   exec invowk cmd my-embedded-command
   ```

2. **Tests against project's `invkfile.cue`** - Use `cd $PROJECT_ROOT`:
   ```txtar
   # Run against project's invkfile.cue
   cd $PROJECT_ROOT
   exec invowk cmd some-project-command
   ```

### 2. Environment Variables

- Only set environment variables that are actually used by production code.
- Do NOT set placeholder env vars "for future use" - they cause confusion.
- If a test needs a specific env var cleared, do it in the test file: `env MY_VAR=`

### 3. Container Runtime Tests

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

### 4. Shell Script Behavior in Containers

Scripts executed via `/bin/sh -c` do NOT have `set -e` by default. Always add `set -e` when you want scripts to fail on any command failure:

```cue
script: """
    set -e  # Required for fail-on-error behavior
    echo "Starting..."
    some_command_that_might_fail
    echo "Done"
    """
```

### 5. Test File Structure

Each `.txtar` test file should:
1. Have a descriptive comment at the top explaining what it tests.
2. Include skip conditions for optional features (e.g., `[!container-available] skip`).
3. Use `cd $WORK` to set working directory if it uses embedded files.
4. Include the embedded `invkfile.cue` and any other required files.

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

-- invkfile.cue --
cmds: [...]

-- other-file.txt --
content
```

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Setting `env.Cd` in Setup | Tests find wrong `invkfile.cue` | Remove `env.Cd`, let tests use `cd $WORK` |
| CLI-only container check | Windows tests run but fail | Add smoke test that runs actual container |
| Missing `set -e` in scripts | Failed commands don't cause script failure | Add `set -e` at script start |
| Unused env vars in Setup | Confusion, false assumptions | Only set vars used by production code |
