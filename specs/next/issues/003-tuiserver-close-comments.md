# Issue: Add Missing Close Error Comments in `internal/tuiserver/`

**Category**: Quick Wins
**Priority**: Low
**Effort**: Low (< 1 hour)
**Labels**: `documentation`, `code-quality`, `good-first-issue`

## Summary

Close errors in `internal/tuiserver/server.go` are discarded without explanatory comments, inconsistent with the pattern established in `internal/sshserver/`.

## Problem

Current code discards errors without explanation:

```go
// internal/tuiserver/server.go
_ = s.httpServer.Close()  // No comment - why is error discarded?
_ = s.listener.Close()    // No comment - why is error discarded?
```

The sshserver package provides a good reference pattern:

```go
// internal/sshserver/server_lifecycle.go:73
_ = listener.Close() // Best-effort cleanup on error
```

## Files to Modify

| File | Line | Current | Proposed |
|------|------|---------|----------|
| `internal/tuiserver/server.go` | 127 | `_ = s.httpServer.Close()` | `_ = s.httpServer.Close() // Best-effort cleanup during state transition` |
| `internal/tuiserver/server.go` | 141 | `_ = s.listener.Close()` | `_ = s.listener.Close() // Best-effort cleanup; server already stopping` |

## Solution

Add comments explaining why the close errors are being discarded:

```go
// Line 127 - During state transition (e.g., error during startup)
_ = s.httpServer.Close() // Best-effort cleanup during state transition

// Line 141 - During shutdown sequence
_ = s.listener.Close() // Best-effort cleanup; server already stopping
```

## Implementation Steps

1. [ ] Open `internal/tuiserver/server.go`
2. [ ] Add comment to line 127 explaining httpServer close
3. [ ] Add comment to line 141 explaining listener close
4. [ ] Verify consistency with sshserver patterns

## Acceptance Criteria

- [ ] All discarded close errors have explanatory comments
- [ ] Comments follow established pattern from sshserver
- [ ] `make lint` passes
- [ ] `make test` passes

## Testing

```bash
# Verify code compiles and tests pass
make test

# Visual inspection of changes
git diff internal/tuiserver/server.go
```

## Notes

This is an excellent first issue for new contributors:
- Minimal code changes required
- Teaches the importance of documenting error handling decisions
- Low risk of breaking functionality
- Quick to review and merge
