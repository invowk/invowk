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

## VHS Integration Tests

VHS-based integration tests live in `vhs/` and test CLI input/output behavior.

### Running VHS Tests

```bash
make test-vhs          # Run all VHS tests
make test-vhs-update   # Update golden files
make test-vhs-validate # Validate tape syntax
```

### Writing VHS Tapes

Tape files use a declarative format:

```tape
# NN-category.tape - Description
Output vhs/output/NN-category.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test: description
Type "./bin/invowk cmd 'command name'"
Enter
Sleep 500ms
```

### Required VHS Settings

**CRITICAL: All VHS tapes MUST use these settings for deterministic, working tests:**

| Setting | Value | Reason |
|---------|-------|--------|
| `Width` | `1280` | VHS requires minimum 120x120 dimensions |
| `Height` | `720` | Standard HD resolution, well above minimum |
| `TypingSpeed` | `50ms` | **NOT 0ms** - Zero causes non-deterministic frame capture |
| `FontSize` | `14` | Consistent rendering across environments |

### Why TypingSpeed Must Be Non-Zero

With `TypingSpeed 0ms`, VHS batches characters non-deterministically, causing frame captures to vary between runs. For example:
- Run 1 might capture: `> ./bin/invowk cmd hello`
- Run 2 might capture: `> ./bin/invowk cmd hell` (partial)

Using `50ms` ensures each character is captured in sequence, producing deterministic output.

### Key VHS Patterns

- **Deterministic timing**: Use `Set TypingSpeed 50ms` (not 0ms) and fixed `Sleep` values.
- **Text output**: Use `Output *.txt` for text capture (not video).
- **Normalization**: Variable content (paths, timestamps) is normalized via `normalize.sh`.
- **Deduplication**: The `normalize.sh` script uses `| uniq` to collapse repeated frames.
- **Golden files**: Committed to `vhs/golden/`, updated via `make test-vhs-update`.
- **Native/Virtual only**: Skip container runtime tests to avoid Docker/Podman dependencies.

### When to Add VHS Tests

Add VHS tests when:
- Adding new CLI commands or subcommands
- Changing command output format
- Modifying flag/argument handling
- Testing environment variable behavior

For test commands, see `docs/agents/commands.md`.
For VHS details, see `vhs/README.md`.
