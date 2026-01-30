// SPDX-License-Identifier: MPL-2.0

// Package serverbase provides the base implementation for long-running server components.
//
// All servers (SSH server, TUI server, etc.) embed Base to get consistent lifecycle
// management with race-free state transitions.
//
// # State Machine
//
// Servers use a formal state machine with the following states:
//
//	Created → Starting → Running → Stopping → Stopped
//	              ↓                     ↑
//	           Failed ─────────────────┘
//
// # Usage
//
// Concrete server implementations embed Base and use its lifecycle helpers:
//
//	type MyServer struct {
//	    *serverbase.Base
//	    // server-specific fields
//	}
//
//	func New() *MyServer {
//	    return &MyServer{
//	        Base: serverbase.NewBase(),
//	    }
//	}
//
//	func (s *MyServer) Start(ctx context.Context) error {
//	    if err := s.Base.TransitionToStarting(ctx); err != nil {
//	        return err
//	    }
//	    // ... server-specific initialization ...
//	    s.Base.TransitionToRunning()
//	    return nil
//	}
package serverbase

const (
	// StateCreated indicates the server instance was created but Start() not called.
	StateCreated State = iota
	// StateStarting indicates Start() was called and the server is initializing.
	StateStarting
	// StateRunning indicates the server is accepting connections/requests.
	StateRunning
	// StateStopping indicates Stop() was called and graceful shutdown is in progress.
	StateStopping
	// StateStopped is terminal: the server has stopped.
	StateStopped
	// StateFailed is terminal: the server failed to start or encountered a fatal error.
	StateFailed
)

// State represents the lifecycle state of a server.
type State int32

// String returns a human-readable representation of the server state.
func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if the state is a terminal state (Stopped or Failed).
func (s State) IsTerminal() bool {
	return s == StateStopped || s == StateFailed
}
