# Testing

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
