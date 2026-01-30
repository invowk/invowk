// SPDX-License-Identifier: MPL-2.0

//go:build ignore

// Package serverbase provides the base implementation for long-running server components.
//
// This is a CONTRACT FILE for planning purposes. It defines the API that will be
// implemented in internal/core/serverbase/.
//
// All servers (SSH server, TUI server, etc.) embed Base to get consistent lifecycle
// management with race-free state transitions.
package serverbase

import (
	"context"
	"sync"
	"sync/atomic"
)

// State represents the lifecycle state of a server.
type State int32

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

// String returns the string representation of the state.
func (s State) String() string

// Option configures a Base instance.
type Option func(*Base)

// WithErrorChannel sets a custom error channel buffer size.
func WithErrorChannel(size int) Option

// Base provides common fields and lifecycle infrastructure for servers.
// Concrete server implementations embed this struct.
//
// A server instance is single-use: once stopped or failed, create a new instance.
type Base struct {
	state     atomic.Int32
	stateMu   sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startedCh chan struct{}
	errCh     chan error
	lastErr   error
}

// NewBase creates a new Base with the given options.
func NewBase(opts ...Option) *Base

// State returns the current server state (atomic, lock-free read).
func (b *Base) State() State

// IsRunning returns true if the server is in the Running state.
func (b *Base) IsRunning() bool

// Err returns a channel for receiving async errors.
func (b *Base) Err() <-chan error

// LastError returns the error that caused the Failed state, or nil.
func (b *Base) LastError() error

// --- Lifecycle helpers for concrete implementations ---

// TransitionToStarting attempts to transition from Created to Starting.
// Returns an error if the current state is not Created.
// Must be called at the beginning of Start().
func (b *Base) TransitionToStarting(ctx context.Context) error

// TransitionToRunning marks the server as running.
// Must be called when the server is ready to accept connections.
// Closes the startedCh channel to signal readiness.
func (b *Base) TransitionToRunning()

// TransitionToFailed marks the server as failed with the given error.
// Can be called from Starting state on initialization failure.
func (b *Base) TransitionToFailed(err error)

// TransitionToStopping attempts to transition to Stopping state.
// Returns true if transition occurred, false if already stopped/stopping.
// Cancels the context and signals shutdown.
func (b *Base) TransitionToStopping() bool

// TransitionToStopped marks the server as fully stopped.
// Must be called after all goroutines have exited.
func (b *Base) TransitionToStopped()

// WaitForReady blocks until the server is ready or context is cancelled.
// Returns nil if server is running, ctx.Err() if cancelled.
func (b *Base) WaitForReady(ctx context.Context) error

// WaitForShutdown blocks until all goroutines tracked by WG have completed.
func (b *Base) WaitForShutdown()

// Context returns the server's context for use in goroutines.
func (b *Base) Context() context.Context

// AddGoroutine increments the WaitGroup counter.
// Must be called before starting a goroutine.
func (b *Base) AddGoroutine()

// DoneGoroutine decrements the WaitGroup counter.
// Must be deferred at the start of each goroutine.
func (b *Base) DoneGoroutine()

// SendError sends an error to the error channel (non-blocking).
// If the channel is full, the error is dropped.
func (b *Base) SendError(err error)

// CheckContextCancelled returns an error if the context was cancelled
// before starting. This prevents TOCTOU race conditions.
func (b *Base) CheckContextCancelled(ctx context.Context) error
