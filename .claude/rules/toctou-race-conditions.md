# TOCTOU Race Conditions

## Overview

TOCTOU (Time-Of-Check-Time-Of-Use) race conditions occur when there's a gap between checking a condition and acting on it, during which the condition can change. These are particularly common in concurrent Go code with goroutines.

## Context Cancellation Race Pattern

### The Problem

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

**Why it fails:**
- `net.ListenConfig.Listen()` only checks context during blocking operations
- With an already-cancelled context, non-blocking setup can still succeed
- The goroutine can transition to `StateRunning` before the `select` checks `ctx.Done()`
- On faster machines/runners, the goroutine is more likely to win the race

### The Solution

Check context cancellation **before** any setup work:

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

### Key Principles

1. **Check early**: Validate preconditions (including context) before any work
2. **Check at boundaries**: Re-check context after long-running or async operations
3. **Atomic state transitions**: Use `CompareAndSwap` for state changes to prevent concurrent transitions
4. **Don't trust non-blocking success**: Even if an operation succeeds, the context may have been cancelled

## Testing Race Conditions

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

## Real-World Example

**GitHub Action failure** (ubuntu-latest, faster runners):
```
=== RUN   TestServerStartWithCancelledContext
INFO ssh-server: SSH server started address=127.0.0.1:45163
    server_test.go:307: Start with cancelled context should return error
    server_test.go:313: State should be Failed, got stopped
--- FAIL: TestServerStartWithCancelledContext
```

The test passed on slower runners (ubuntu-24.04) but failed on faster ones where the goroutine consistently won the race.

## Common Pitfall

- **Flaky tests across environments** - If a test passes locally but fails in CI (or vice versa), suspect a race condition. Different CPU speeds, scheduling, and runner configurations affect goroutine timing.
