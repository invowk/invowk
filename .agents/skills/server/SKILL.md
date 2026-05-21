---
name: server
description: Server state machine pattern for sshserver/, tuiserver/, and core/serverbase/. Covers lifecycle states (Created→Starting→Running→Stopping→Stopped), atomic state reads, idempotent stop.
---

# Server Pattern

This skill describes the standard pattern for implementing long-running server components in Invowk.

Use this skill when working on:
- `internal/sshserver/` - SSH server implementation
- `internal/tuiserver/` - TUI server implementation
- `internal/core/serverbase/` - Shared server base type
- Adding new server components

---

## State Machine

Servers use a formal state machine with the following states:

```
         ┌─────────────────────────────────────────────────────┐
         │                                                     │
         ▼                                                     │
    ┌─────────┐     ┌──────────┐     ┌─────────┐     ┌─────────┴─┐     ┌─────────┐
    │ Created │────▶│ Starting │────▶│ Running │────▶│ Stopping  │────▶│ Stopped │
    └─────────┘     └──────────┘     └─────────┘     └───────────┘     └─────────┘
                          │
                          │ (on error)
                          ▼
                     ┌────────┐
                     │ Failed │
                     └────────┘
```

### State Definitions

| State | Description |
|-------|-------------|
| `Created` | Server instance created but `Start()` not called |
| `Starting` | `Start()` called; server is initializing |
| `Running` | Server is accepting connections/requests |
| `Stopping` | `Stop()` called; server is shutting down |
| `Stopped` | Server has stopped (terminal state) |
| `Failed` | Server failed to start or fatal error (terminal state) |

---

## Required API

Every server must implement:

```go
// ServerState type and constants
type ServerState int32

const (
    StateCreated ServerState = iota
    StateStarting
    StateRunning
    StateStopping
    StateStopped
    StateFailed
)

func (s ServerState) String() string { ... }

// Server struct fields (minimum required)
type Server struct {
    base *serverbase.Base // Owns state, cancellation, async errors, and wait group
    // ... server-specific fields
}

// Constructor - may return an error when validation, credentials, or listeners fail
func New(cfg Config) (*Server, error)

// Lifecycle methods
func (s *Server) Start(ctx context.Context) error  // Blocks until ready or fails
func (s *Server) Stop() error                      // Graceful shutdown
func (s *Server) State() ServerState               // Current state (atomic read)
func (s *Server) IsRunning() bool                  // Convenience: state == Running
func (s *Server) Wait() error                      // Optional; SSH exposes this
func (s *Server) Err() <-chan error                // Async error notifications
```

---

## Implementation Rules

### 1. Atomic State Reads

Use `internal/core/serverbase.Base` for state ownership. Concrete servers delegate
state reads to the base, which uses atomic reads internally:

```go
func (s *Server) State() ServerState {
    return ServerState(s.base.State())
}

func (s *Server) IsRunning() bool {
    return s.base.IsRunning()
}
```

### 2. Base-Owned State Transitions

Concrete servers should use `serverbase.Base` transition helpers instead of
hand-rolling locks. `Base` uses compare-and-swap for the key transitions and an
idempotent stop loop:

```go
func (s *Server) Start(ctx context.Context) error {
    if err := s.base.TransitionToStarting(ctx); err != nil {
        return err
    }
    // ... initialization ...
}
```

### 3. Blocking Start with Ready Signal

`Start()` must block until the server is actually ready to serve:

```go
func (s *Server) Start(ctx context.Context) error {
    // ... setup listener/server ...

    // Signal ready when listener is accepting
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        close(s.startedCh)  // Signal ready
        // ... serve loop ...
    }()

    // Wait for ready or context cancellation
    select {
    case <-s.startedCh:
        s.state.Store(int32(StateRunning))
        return nil
    case <-ctx.Done():
        s.state.Store(int32(StateFailed))
        return ctx.Err()
    }
}
```

### 4. Idempotent Stop

`Stop()` must be safe to call multiple times and from any state:

```go
func (s *Server) Stop() error {
    s.stateMu.Lock()
    state := s.State()

    // Already stopped or stopping - no-op
    if state == StateStopped || state == StateStopping {
        s.stateMu.Unlock()
        return nil
    }

    // Never started - just mark as stopped
    if state == StateCreated || state == StateFailed {
        s.state.Store(int32(StateStopped))
        s.stateMu.Unlock()
        return nil
    }

    s.state.Store(int32(StateStopping))
    s.stateMu.Unlock()

    // Cancel context and wait for goroutines
    s.cancel()
    s.wg.Wait()

    s.stateMu.Lock()
    s.state.Store(int32(StateStopped))
    s.stateMu.Unlock()
    return nil
}
```

### 5. Single-Use Servers

Server instances are single-use. Once stopped or failed, create a new instance:

```go
// Document this in the type comment:
// Server represents the XYZ server.
// A Server instance is single-use: once stopped or failed, create a new instance.
```

### 6. WaitGroup for Goroutine Tracking

All background goroutines must be tracked with a WaitGroup:

```go
s.wg.Add(1)
go func() {
    defer s.wg.Done()
    // ... goroutine work ...
}()
```

### 7. Context-Based Cancellation

Pass context to `Start()` and derive internal context:

```go
func (s *Server) Start(ctx context.Context) error {
    s.ctx, s.cancel = context.WithCancel(ctx)
    // Use s.ctx for all internal operations
}
```

---

## Testing Requirements

Server tests must verify:

1. **State transitions**: Created → Running → Stopped
2. **Double Start**: Second `Start()` returns error
3. **Double Stop**: Second `Stop()` is no-op (no error)
4. **Stop without Start**: Safe, transitions to Stopped
5. **Cancelled context**: `Start()` fails, transitions to Failed
6. **Race conditions**: Run with `-race` flag

Example test structure:

```go
func TestServerStartStop(t *testing.T) { ... }
func TestServerDoubleStart(t *testing.T) { ... }
func TestServerDoubleStop(t *testing.T) { ... }
func TestServerStopWithoutStart(t *testing.T) { ... }
func TestServerStartWithCancelledContext(t *testing.T) { ... }
func TestServerStateString(t *testing.T) { ... }
```

---

## Reference Implementation

The server state machine is extracted into a reusable package:

- **Base type**: `internal/core/serverbase/` provides the `Base` struct that concrete servers compose
- **SSH server**: `internal/sshserver/server.go` owns `base *serverbase.Base`
- **TUI server**: `internal/tuiserver/server.go` owns `base *serverbase.Base`

### Using serverbase.Base

Concrete servers keep the base as a private field and expose only the lifecycle
methods that are part of their public API:

```go
type MyServer struct {
    base *serverbase.Base
    // server-specific fields
}

func New() (*MyServer, error) {
    return &MyServer{base: serverbase.NewBase()}, nil
}

func (s *MyServer) Start(ctx context.Context) error {
    if err := s.base.TransitionToStarting(ctx); err != nil {
        return err
    }
    // ... server-specific initialization ...
    s.base.TransitionToRunning()
    return nil
}

func (s *MyServer) Stop() error {
    if !s.base.TransitionToStopping() {
        return nil // Already stopped
    }
    // ... server-specific cleanup ...
    s.base.WaitForShutdown()
    s.Base.TransitionToStopped()
    return nil
}
```

The serverbase package provides:
- `TransitionToStarting(ctx)` - Checks context, transitions Created → Starting, derives lifecycle context from `ctx` (cancellation propagates from caller)
- `TransitionToRunning()` - Marks server ready, closes startedCh
- `TransitionToStopping()` - Cancels context, transitions to Stopping
- `TransitionToStopped()` - Final transition to terminal state
- `TransitionToFailed(err)` - Records error and transitions to Failed
- `WaitForReady(ctx)` - Blocks until running or context cancelled
- `WaitForShutdown()` - Waits for all goroutines to exit
- `AddGoroutine()` / `DoneGoroutine()` - WaitGroup management
- `Context()` - Returns server's internal context
- `SendError(err)` - Non-blocking error channel send
