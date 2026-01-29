# Invowk Coding Conventions

## Go Code Style

### SPDX License Headers
Every `.go` file must start with:
```go
// SPDX-License-Identifier: MPL-2.0
```

### Error Handling with Named Returns
Use named returns to capture close errors:
```go
func processFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil { return err }
    defer func() {
        if closeErr := f.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()
    // ...
}
```

### Declaration Order (enforced by linters)
1. const
2. var
3. type
4. Exported functions
5. Unexported functions

## CUE Schemas
- All structs must be closed: `close({ ... })`
- Include validation constraints, not just types

## Testing
- Use `t.TempDir()` for temp directories
- Table-driven tests for multiple cases
- `testscript` for CLI integration tests in `tests/cli/testdata/`

## Git Commits
- **Always sign commits** (`git commit -S`)
- Conventional Commit format: `type(scope): summary`
- 3-6 bullet body describing changes

## mvdan/sh Gotcha
Always prepend `"--"` when passing args to `interp.Params()`:
```go
params := append([]string{"--"}, args...)
opts = append(opts, interp.Params(params...))
```

## Bash Scripts
Use `VAR=$((VAR + 1))` instead of `((VAR++))` with `set -e`
