# Code Style and Conventions

## Go Style
- Standard Go formatting (`gofmt`)
- Use clear, descriptive naming
- Follow Go idioms and conventions

## SPDX License Headers
**CRITICAL**: Every `.go` file MUST start with:
```go
// SPDX-License-Identifier: EPL-2.0
```
Followed by a blank line, then package documentation (if any), then `package` declaration.

## Comments
- Add comments above method/function/interface/struct declarations
- Add comments inside method bodies when behavior is non-obvious

## CUE Schemas
- All CUE structs must be closed: `close({ ... })`
- Always include validation constraints (regex, ranges, MaxRunes, etc.)
- Schema locations:
  - `pkg/invkfile/invkfile_schema.cue` - invkfile structure
  - `pkg/invkmod/invkmod_schema.cue` - invkmod structure
  - `internal/config/config_schema.cue` - config structure

## Testing
- Test files: `*_test.go` in same package
- Use `t.TempDir()` for temp directories
- Use table-driven tests for multiple cases
- Skip integration tests with `if testing.Short() { t.Skip(...) }`

## Container Runtime Limitations
- **Only Linux containers** (Debian-based, e.g., `debian:stable-slim`)
- **NO Alpine** (musl gotchas)
- **NO Windows containers**

## Error Handling
Use **named return values** to aggregate close errors with the primary operation's error:
```go
func processFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := f.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()
    // ... work with f ...
    return nil
}
```
- Apply this pattern for any `io.Closer` (files, connections, readers, writers)
- Never silently ignore close errors: `defer f.Close()` is **wrong**
- Exceptions: test code (use test helpers), terminal operations where errors can't be handled

## Git Commits
- **All commits MUST be signed** (`git commit -S`)
- Format: `type(scope): summary` (Conventional Commits)
- Body: 3-6 bullets describing changes
