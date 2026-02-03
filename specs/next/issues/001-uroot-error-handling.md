# Issue: Fix Error Handling in `internal/uroot/` Commands

**Category**: Quick Wins
**Priority**: High
**Effort**: Low (< 1 day)
**Labels**: `code-quality`, `good-first-issue`

## Summary

All 7 utility commands in `internal/uroot/` use naked `f.Close()` calls without error handling, violating the project's documented patterns in `.claude/rules/go-patterns.md`.

## Problem

The current code silently ignores file close errors:

```go
// Current pattern in all 7 files
f, err := os.Open(path)
if err != nil {
    return wrapError(c.name, err)
}
// ... process file ...
f.Close()  // Error silently ignored
```

Per project rules, close errors should either:
1. Be captured with named return values (preferred for write operations)
2. Use explicit `_ = f.Close()` with a comment explaining why (acceptable for read-only)

## Files to Modify

| File | Line(s) | Current Pattern |
|------|---------|-----------------|
| `internal/uroot/head.go` | 83, 86 | `f.Close()` |
| `internal/uroot/tail.go` | 87, 90 | `f.Close()` |
| `internal/uroot/grep.go` | 118 | `f.Close()` |
| `internal/uroot/cut.go` | 107 | `f.Close()` |
| `internal/uroot/sort.go` | 97 | `f.Close()` |
| `internal/uroot/wc.go` | 112 | `f.Close()` |
| `internal/uroot/uniq.go` | 77 | `f.Close()` |

## Solution

Since these are read-only file operations, use the explicit discard pattern with justification:

```go
// After opening file successfully
defer func() { _ = f.Close() }() // Read-only file; close error non-critical
```

### Reference Patterns

Existing correct patterns in the codebase:
- `cmd/invowk/tui_table.go:72`: `defer func() { _ = f.Close() }()` with comment
- `internal/runtime/provision.go:98,172`: Same pattern with justification

## Implementation Steps

1. [ ] Update `internal/uroot/head.go` - Replace naked close with deferred pattern
2. [ ] Update `internal/uroot/tail.go` - Replace naked close with deferred pattern
3. [ ] Update `internal/uroot/grep.go` - Replace naked close with deferred pattern
4. [ ] Update `internal/uroot/cut.go` - Replace naked close with deferred pattern
5. [ ] Update `internal/uroot/sort.go` - Replace naked close with deferred pattern
6. [ ] Update `internal/uroot/wc.go` - Replace naked close with deferred pattern
7. [ ] Update `internal/uroot/uniq.go` - Replace naked close with deferred pattern

## Acceptance Criteria

- [ ] All 7 files use the deferred close pattern
- [ ] Each close has explanatory comment: `// Read-only file; close error non-critical`
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] No functionality changes (verified by existing uroot tests)

## Testing

```bash
# Run uroot tests to verify no regression
go test -v ./internal/uroot/...

# Run linter to verify code quality
make lint
```

## Notes

This is a good first issue for contributors as it:
- Involves straightforward, mechanical changes
- Has clear acceptance criteria
- Teaches project code quality standards
- Low risk of breaking functionality
