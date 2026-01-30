// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// Base provides common fields and lifecycle infrastructure for servers.
// Concrete server implementations embed this struct.
//
// A server instance is single-use: once stopped or failed, create a new instance.
type Base struct {
	// State management (atomic for lock-free reads)
	state atomic.Int32

	// State transition protection
	stateMu sync.Mutex

	// Lifecycle management
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startedCh chan struct{}
	errCh     chan error
	lastErr   error
}

// NewBase creates a new Base with the given options.
// Default error channel buffer size is 1.
func NewBase(opts ...Option) *Base {
	b := &Base{
		startedCh: make(chan struct{}),
		errCh:     make(chan error, 1), // Default buffer size
	}
	b.state.Store(int32(StateCreated))

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// State returns the current server state (atomic, lock-free read).
func (b *Base) State() State {
	return State(b.state.Load())
}

// IsRunning returns true if the server is in the Running state.
func (b *Base) IsRunning() bool {
	return b.State() == StateRunning
}

// Err returns a channel for receiving async errors.
func (b *Base) Err() <-chan error {
	return b.errCh
}

// LastError returns the error that caused the Failed state, or nil.
func (b *Base) LastError() error {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.lastErr
}

// --- Lifecycle helpers for concrete implementations ---

// TransitionToStarting attempts to transition from Created to Starting.
// Returns an error if the current state is not Created or if the context
// is already cancelled.
// Must be called at the beginning of Start().
func (b *Base) TransitionToStarting(ctx context.Context) error {
	// Check for already-cancelled context BEFORE any setup.
	// This prevents a TOCTOU race where the serve goroutine could transition
	// to StateRunning before the cancelled context is detected.
	select {
	case <-ctx.Done():
		b.TransitionToFailed(fmt.Errorf("context cancelled before start: %w", ctx.Err()))
		return b.lastErr
	default:
	}

	// Atomic state transition: Created -> Starting
	if !b.state.CompareAndSwap(int32(StateCreated), int32(StateStarting)) {
		currentState := State(b.state.Load())
		return fmt.Errorf("cannot start server in state %s", currentState)
	}

	// Create internal context for lifecycle management
	b.ctx, b.cancel = context.WithCancel(context.Background())

	return nil
}

// TransitionToRunning marks the server as running.
// Must be called when the server is ready to accept connections.
// Closes the startedCh channel to signal readiness.
func (b *Base) TransitionToRunning() {
	if b.state.CompareAndSwap(int32(StateStarting), int32(StateRunning)) {
		close(b.startedCh)
	}
}

// TransitionToFailed marks the server as failed with the given error.
// Can be called from Starting state on initialization failure.
func (b *Base) TransitionToFailed(err error) {
	b.stateMu.Lock()
	b.lastErr = err
	b.stateMu.Unlock()

	b.state.Store(int32(StateFailed))

	if b.cancel != nil {
		b.cancel()
	}

	// Send error to channel for Err() consumers (non-blocking)
	select {
	case b.errCh <- err:
	default:
	}
}

// TransitionToStopping attempts to transition to Stopping state.
// Returns true if transition occurred, false if already stopped/stopping.
// Cancels the context and signals shutdown.
func (b *Base) TransitionToStopping() bool {
	for {
		currentState := State(b.state.Load())
		switch currentState {
		case StateStopped, StateFailed:
			return false // Already stopped
		case StateCreated:
			if b.state.CompareAndSwap(int32(StateCreated), int32(StateStopped)) {
				return false // Never started, just mark stopped
			}
			continue // State changed, retry
		case StateStopping:
			return false // Already stopping
		case StateStarting, StateRunning:
			if !b.state.CompareAndSwap(int32(currentState), int32(StateStopping)) {
				continue // State changed, retry
			}
			// Cancel context to signal goroutines
			if b.cancel != nil {
				b.cancel()
			}
			return true
		default:
			return false
		}
	}
}

// TransitionToStopped marks the server as fully stopped.
// Must be called after all goroutines have exited.
func (b *Base) TransitionToStopped() {
	b.state.Store(int32(StateStopped))
}

// WaitForReady blocks until the server is ready or context is cancelled.
// Returns nil if server is running, wrapped ctx.Err() if cancelled.
func (b *Base) WaitForReady(ctx context.Context) error {
	select {
	case <-b.startedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for server ready: %w", ctx.Err())
	}
}

// WaitForShutdown blocks until all goroutines tracked by WG have completed.
func (b *Base) WaitForShutdown() {
	b.wg.Wait()
}

// Context returns the server's context for use in goroutines.
// Returns nil if the server hasn't started.
func (b *Base) Context() context.Context {
	return b.ctx
}

// AddGoroutine increments the WaitGroup counter.
// Must be called before starting a goroutine.
func (b *Base) AddGoroutine() {
	b.wg.Add(1)
}

// DoneGoroutine decrements the WaitGroup counter.
// Must be deferred at the start of each goroutine.
func (b *Base) DoneGoroutine() {
	b.wg.Done()
}

// SendError sends an error to the error channel (non-blocking).
// If the channel is full, the error is dropped.
func (b *Base) SendError(err error) {
	select {
	case b.errCh <- err:
	default:
	}
}

// CloseErrChannel closes the error channel to signal consumers.
// Should be called when the server is fully stopped.
func (b *Base) CloseErrChannel() {
	close(b.errCh)
}

// StartedChannel returns the started channel for custom waiting logic.
// The channel is closed when the server transitions to Running.
func (b *Base) StartedChannel() <-chan struct{} {
	return b.startedCh
}
