---
name: server
description: Server state machine pattern for sshserver/, tuiserver/, core/serverbase/, and new long-running server components. Covers serverbase.Base lifecycle states (Created→Starting→Running→Stopping→Stopped/Failed), readiness, async errors, terminal states, idempotent stop, race-safe shutdown, and explicit SSH host keys.
---

# Server Pattern

Use this skill when changing `internal/sshserver/`, `internal/tuiserver/`,
`internal/core/serverbase/`, or adding a long-running server component.

## State Machine

Servers compose `internal/core/serverbase.Base` and use `serverbase.State`.

```
Created  -> Starting | Stopped (stop before start) | Failed
Starting -> Running  | Stopping | Failed
Running  -> Stopping | Failed
Stopping -> Stopped  | Failed
Stopped, Failed: terminal (no outgoing edges)
```

`TransitionToFailed` and `TransitionToStopped` CAS from any non-terminal state;
the first failure wins: `TransitionToFailed` on an already-terminal server
changes nothing and returns the recorded error (`LastError()`; nil if it
stopped cleanly).

`Stopped` and `Failed` are terminal. A server instance is single-use: once it
stops or fails, create a new instance. Terminal states are absorbing under
concurrent transitions: `TransitionToFailed` and `TransitionToStopped`
compare-and-swap from the observed state, and `TransitionToStarting` stores the
server context in the same critical section as its CAS, so a concurrent stop
always cancels it. `formal/tla/Serverbase.tla` checks both; keep them when
changing `internal/core/serverbase`. The `internal/sshserver` host-callback
token lifecycle is modelled separately by `formal/tla/HostCallbackToken.tla`
(`server_auth.go`, `server_conns.go`, and `server_lifecycle.go` are bound);
both directories follow `.agents/rules/formal-bindings.md`.

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

1. Call `s.base.TransitionToStarting(ctx)` first and return its error.
   It returns an error in any state but `Created` (matching
   `serverbase.ErrStartNotCreated`) and then changes nothing, whatever `ctx`
   says; only on `Created` does an already-cancelled `ctx` move the server to
   `Failed`. So a second `Start`, cancelled or not, never fails, binds or tears
   down a server another `Start` owns, and a `nil` return always means this
   call moved `Created -> Starting`.
2. Initialize listeners/resources.
3. Start background goroutines with `AddGoroutine()` and `defer DoneGoroutine()`.
4. Call `TransitionToRunning()` only when the server is ready.
   When a library's `Serve` registers the listener itself (charm.land/ssh,
   wish), readiness means the library has registered it: signal it from the
   first `Accept` of a listener wrapper passed to `Serve`, never before `Serve`
   is called. `Stop` must not call the library's `Shutdown` before that point
   or before the serve goroutine has returned: in charm.land/ssh v0.4.3 a
   `Shutdown` that runs first returns at once and `Serve` then serves on,
   racing its listener WaitGroup (charmbracelet/ssh#61; `readyListener` and
   `serveGate` in `internal/sshserver/server_lifecycle.go`).
5. Use `WaitForReady(ctx)` or `StartedChannel()` to block startup until ready or failed.
   Caveat: `startedCh` closes only on a successful Starting→Running CAS. If a
   `Stop` or a `Serve` failure wins first, `WaitForReady` blocks until `ctx` ends,
   and an interactive context has no deadline. When readiness is known
   synchronously (listeners already bound, as in `tuiserver.Server.serveListeners`),
   call `TransitionToRunning()` before launching the serve goroutines, check
   `IsRunning()`, and return instead of waiting.
6. On startup or serve errors, call `TransitionToFailed(err)` and return that error.
   A startup failure after resources are published (startup timeout, cancelled
   `ctx`, an error from the serve goroutine) tears them down through the same
   routine as `Stop` before `Start` returns, so a failed `Start` leaves no
   listener open and a later `Stop` or `Wait` returns (`failStart` and
   `teardown` in `internal/sshserver`).
   An `http.Server` behind an auth middleware must set
   `DisableGeneralOptionsHandler: true`; otherwise net/http answers `OPTIONS *`
   itself, before the handler, and the request bypasses the token check.
7. SSH servers must always configure a host key explicitly (see
   `(*Server).hostKeyOption()` in `internal/sshserver/server_lifecycle.go`): never let wish's default pick one — it writes
   `./id_ed25519` into the process working directory, which once leaked private
   keys into this repository.

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

The example above fits servers whose `Start` has launched everything before a
`Stop` can run (or whose library tolerates `Shutdown` before `Serve`, like
`net/http`). `TransitionToStopping()` also accepts `Starting`, so when `Start`
can still publish resources or call `AddGoroutine()` while a `Stop` runs:

- serialise `Stop` and the start-failure teardown with one mutex held for the
  whole call, so a `Stop` that loses the transition reaches
  `WaitForShutdown()` only after the winner's teardown, never while `Start`
  can still add a goroutine;
- publish the resources and call `AddGoroutine()` in one critical section that
  refuses once a teardown has begun; the refused `Start` closes what it bound,
  launches nothing and returns an error.

`internal/sshserver` (`stopMu`, `publish`, `teardown`, `failStart`) is the
reference for that variant.

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
2. Double `Start()` returns the `cannot start server in state <state>` error
   from `TransitionToStarting`, matching `serverbase.ErrStartNotCreated` (not
   `serverbase.InvalidStateError`, which is only for unrecognized `State`
   values), also with an already-cancelled context, and leaves the state,
   `LastError()` and the serving server unchanged.
3. Double `Stop()` is a no-op.
4. Stop before start transitions `Created -> Stopped`.
5. Cancelled startup transitions to `Failed`.
6. Runtime serve errors transition `Running -> Failed`, close `Err()`, and
   populate `LastError()`.
7. Terminal states are irreversible.
8. A `Stop` at each point of a concurrent `Start` (before publish, before the
   serve loop, during a startup failure) leaves no listener open and no
   goroutine running, and `Start` returns an error. Hold the window through an
   unexported hooks field on the server, never a package-level variable
   (`internal/sshserver/server_lifecycle_race_test.go`). A test that needs a
   timeout branch to win (for example the teardown skipping `Shutdown`) waits
   for a hook on that branch (`shutdownSkipped`), never for a multiple of the
   timeout: the timer starts later than the test's own clock on a loaded runner.

Use focused commands first:

```bash
go test -race -count=1 ./internal/core/serverbase
go test -race -count=1 ./internal/sshserver ./internal/tuiserver
```

Then follow `.agents/rules/checklist.md` for the final verification scope.
