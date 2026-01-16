# Server Pattern

This document describes the standard pattern for implementing long-running server components in Invowk. All servers (SSH server, TUI server, etc.) should follow this pattern to ensure consistent, race-free lifecycle management.

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
    state     atomic.Int32      // Lock-free state reads
    stateMu   sync.Mutex        // Protects state transitions
    ctx       context.Context   // Server context for cancellation
    cancel    context.CancelFunc
    wg        sync.WaitGroup    // Tracks background goroutines
    startedCh chan struct{}     // Closed when server is ready
    errCh     chan error        // Async error channel
    lastErr   error             // Stores error for Failed state
    // ... server-specific fields
}

// Constructor - returns single value, no error from New()
func New(cfg Config) *Server

// Lifecycle methods
func (s *Server) Start(ctx context.Context) error  // Blocks until ready or fails
func (s *Server) Stop() error                      // Graceful shutdown
func (s *Server) State() ServerState               // Current state (atomic read)
func (s *Server) IsRunning() bool                  // Convenience: state == Running
func (s *Server) Wait() error                      // Block until stopped, return error
func (s *Server) Err() <-chan error                // Async error notifications
```

## Implementation Rules

### 1. Atomic State Reads

Use `atomic.Int32` for the state field to allow lock-free reads:

```go
func (s *Server) State() ServerState {
    return ServerState(s.state.Load())
}

func (s *Server) IsRunning() bool {
    return s.State() == StateRunning
}
```

### 2. Mutex-Protected State Transitions

All state transitions must be protected by a mutex to prevent concurrent transitions:

```go
func (s *Server) Start(ctx context.Context) error {
    s.stateMu.Lock()
    defer s.stateMu.Unlock()

    if s.State() != StateCreated {
        return fmt.Errorf("server already started or stopped")
    }
    s.state.Store(int32(StateStarting))
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

## Reference Implementation

See `internal/sshserver/server.go` for the canonical implementation of this pattern.
