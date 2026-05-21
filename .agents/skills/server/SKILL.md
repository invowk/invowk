---
name: server
description: Server state machine pattern for sshserver/, tuiserver/, core/serverbase/, and new long-running server components. Covers serverbase.Base lifecycle states (Created→Starting→Running→Stopping→Stopped/Failed), readiness, async errors, terminal states, idempotent stop, and race-safe shutdown.
---

# Server Pattern

Use this skill when changing `internal/sshserver/`, `internal/tuiserver/`,
`internal/core/serverbase/`, or adding a long-running server component.

## State Machine

Servers compose `internal/core/serverbase.Base` and use `serverbase.State`.

```
Created -> Starting -> Running -> Stopping -> Stopped
              |              |
              +-----------> Failed
```

`Stopped` and `Failed` are terminal. A server instance is single-use: once it
stops or fails, create a new instance.

| State | Meaning |
|---|---|
| `serverbase.StateCreated` | Constructed, not started |
| `serverbase.StateStarting` | `Start(ctx)` is initializing |
| `serverbase.StateRunning` | Ready and accepting work |
| `serverbase.StateStopping` | Graceful shutdown in progress |
| `serverbase.StateStopped` | Terminal clean stop |
| `serverbase.StateFailed` | Terminal startup or serve failure |

## Required Shape

Concrete servers keep the base private and expose only their API:

```go
type Server struct {
    base *serverbase.Base
    // server-specific fields
}

func (s *Server) Start(ctx context.Context) error
func (s *Server) Stop() error
func (s *Server) State() serverbase.State { return s.base.State() }
func (s *Server) IsRunning() bool { return s.base.IsRunning() }
func (s *Server) Err() <-chan error { return s.base.Err() }
```

Use `LastError()` when callers need the recorded failure after the server reaches
`Failed`.

## Lifecycle Rules

### Start

1. Call `s.base.TransitionToStarting(ctx)` first.
2. Initialize listeners/resources.
3. Start background goroutines with `AddGoroutine()` and `defer DoneGoroutine()`.
4. Call `TransitionToRunning()` only when the server is ready.
5. Use `WaitForReady(ctx)` or `StartedChannel()` to block startup until ready or failed.
6. On startup or serve errors, call `TransitionToFailed(err)` and return that error.

```go
func (s *Server) Start(ctx context.Context) error {
    if err := s.base.TransitionToStarting(ctx); err != nil {
        return err
    }

    // Initialize listener/server resources here.

    s.base.AddGoroutine()
    go func() {
        defer s.base.DoneGoroutine()
        s.base.TransitionToRunning()
        if err := s.serve(s.base.Context()); err != nil {
            _ = s.base.TransitionToFailed(err)
        }
    }()

    if err := s.base.WaitForReady(ctx); err != nil {
        return s.base.TransitionToFailed(err)
    }
    return nil
}
```

### Stop

`Stop()` must be idempotent and safe from any state. Use the base transition
loop instead of hand-rolled locks.

```go
func (s *Server) Stop() error {
    if !s.base.TransitionToStopping() {
        s.base.WaitForShutdown()
        return nil
    }

    // Close listeners, request shutdown, and clean server-specific resources.
    s.base.WaitForShutdown()
    s.base.TransitionToStopped()
    return nil
}
```

For stop-before-start, `TransitionToStopping()` moves `Created -> Stopped`,
closes the error channel, and returns false.

## Goroutines And Errors

- Call `AddGoroutine()` before starting each background goroutine.
- `defer DoneGoroutine()` at the top of the goroutine.
- Use `s.base.Context()` inside goroutines after `TransitionToStarting()`.
- Send non-fatal async errors with `SendError(err)`; transition fatal serve
  errors with `TransitionToFailed(err)`.
- Ensure listeners unblock promptly on shutdown before waiting for goroutines.

## Testing Requirements

Server tests should cover:

1. `Created -> Running -> Stopped`.
2. Double `Start()` returns an invalid-state error.
3. Double `Stop()` is a no-op.
4. Stop before start transitions `Created -> Stopped`.
5. Cancelled startup transitions to `Failed`.
6. Runtime serve errors transition `Running -> Failed`, close `Err()`, and
   populate `LastError()`.
7. Terminal states are irreversible.

Use focused commands first:

```bash
go test -race -count=1 ./internal/core/serverbase
go test -race -count=1 ./internal/sshserver ./internal/tuiserver
```

Then follow `.agents/rules/checklist.md` for the final verification scope.
