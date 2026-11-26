// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ErrStartNotCreated is matched by the error of a TransitionToStarting on a
// server that is not in Created: only a never-started server can start.
var ErrStartNotCreated = errors.New("cannot start server")

// Base provides common fields and lifecycle infrastructure for servers.
// Concrete server implementations embed this struct.
//
// A server instance is single-use: once stopped or failed, create a new instance.
type Base struct {
	//plint:internal -- atomic lifecycle state; managed by TransitionTo*() methods
	state atomic.Int32

	//plint:internal -- protects state transitions; not user-configurable
	stateMu sync.Mutex

	//plint:internal -- lifecycle context; created in TransitionToStarting()
	ctx context.Context
	//plint:internal -- lifecycle cancel func; paired with ctx
	cancel context.CancelFunc
	//plint:internal -- tracks goroutines for graceful shutdown
	wg sync.WaitGroup
	//plint:internal -- closed on TransitionToRunning(); signals readiness
	startedCh chan struct{}
	//plint:internal -- async error channel; created in NewBase()
	errCh chan error
	//plint:internal -- ensures errCh is closed once at terminal stop
	errCloseOnce sync.Once
	//plint:internal -- stateMu protects errCh close/send ordering
	errClosed bool
	//plint:internal -- last error that caused Failed state
	lastErr error
}

// NewBase creates a new Base with the given options.
// The error channel has a buffer of 1, which allows the server goroutine to report
// one error without blocking. Additional errors are silently dropped via non-blocking
// send (select/default) — callers should read Err() promptly or use LastError() after
// the server reaches a terminal state.
func NewBase(opts ...Option) *Base {
	b := &Base{
		startedCh: make(chan struct{}),
		errCh:     make(chan error, 1),
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// State returns the current server state (atomic, lock-free read).
func (b *Base) State() State {
	state := State(b.state.Load())
	if err := state.Validate(); err != nil {
		return StateFailed
	}
	return state
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
	err := b.lastErr
	b.stateMu.Unlock()
	return err
}

// --- Lifecycle helpers for concrete implementations ---

// TransitionToStarting attempts to transition from Created to Starting.
// Only a server in Created changes state: on any other state it returns an
// error naming that state (matching ErrStartNotCreated) and changes nothing,
// whatever ctx says. On Created with an already-cancelled ctx it moves the
// server to Failed and returns an error wrapping ctx's error.
// Must be called at the beginning of Start().
func (b *Base) TransitionToStarting(ctx context.Context) error {
	// Both branches change the state only by one compare-and-swap from
	// Created, inside one stateMu critical section, so the state check
	// precedes every effect of the context check: a second Start with a
	// cancelled ctx can no longer fail a server another Start owns, and a
	// lock-free TransitionToStopping that moved Created -> Stopped meanwhile
	// only makes the CAS lose (decision 120; formal/tla/Serverbase.tla,
	// CancelledStartChangesOnlyCreated). The cancelled branch does not use
	// TransitionToFailed: its CAS loop fails any non-terminal state, and its
	// lost CAS returns the recorded error, nil after Created -> Stopped.
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	// Check for an already-cancelled context BEFORE any setup, so the serve
	// goroutine can never reach StateRunning on a cancelled start.
	if ctxErr := ctx.Err(); ctxErr != nil {
		if !b.state.CompareAndSwap(int32(StateCreated), int32(StateFailed)) {
			return b.notCreatedErrorLocked()
		}
		// No server context exists yet, so there is nothing to cancel.
		err := fmt.Errorf("context cancelled before start: %w", ctxErr)
		b.lastErr = err
		b.sendErrorLocked(err)
		b.closeErrChannelLocked()
		return err
	}

	// The Created -> Starting CAS and the ctx/cancel store happen in one
	// stateMu critical section. TransitionToStopping CASes lock-free and then
	// reads cancel under stateMu, so it always observes the stored cancel func
	// and cancels the server context; storing after an unlocked CAS let a
	// concurrent stopper read a nil cancel and leave the context live
	// (formal/tla/Serverbase.tla, finding F1).
	//
	// The internal context is a child of the caller's context so that parent
	// cancellation (e.g., Ctrl+C) propagates to server goroutines. Shutdown
	// (Stop) uses its own independent context for graceful cleanup.
	if !b.state.CompareAndSwap(int32(StateCreated), int32(StateStarting)) {
		return b.notCreatedErrorLocked()
	}
	b.ctx, b.cancel = context.WithCancel(ctx)
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
// Returns the recorded failure error so adapters can return the canonical
// lifecycle error without reaching back into LastError().
func (b *Base) TransitionToFailed(err error) error {
	b.stateMu.Lock()
	if !b.casToTerminalLocked(StateFailed) {
		recordedErr := b.lastErr
		b.stateMu.Unlock()
		return recordedErr
	}
	b.lastErr = err
	cancel := b.cancel
	b.sendErrorLocked(err)
	b.closeErrChannelLocked()
	b.stateMu.Unlock()

	if cancel != nil {
		cancel()
	}
	return err
}

// TransitionToStopping attempts to transition to Stopping state.
// Returns true if transition occurred, false if already stopped/stopping.
// Cancels the context and signals shutdown.
//
// Uses a CAS-retry loop because multiple goroutines may race to stop the server
// (e.g., signal handler + error handler). The loop ensures exactly one caller
// succeeds and performs cleanup (context cancellation); others return false.
func (b *Base) TransitionToStopping() bool {
	for {
		currentState := b.State()
		switch currentState {
		case StateStopped, StateFailed:
			return false // Already stopped
		case StateCreated:
			if b.state.CompareAndSwap(int32(StateCreated), int32(StateStopped)) {
				b.CloseErrChannel()
				return false // Never started, just mark stopped
			}
			continue // State changed, retry
		case StateStopping:
			return false // Already stopping
		case StateStarting, StateRunning:
			if !b.state.CompareAndSwap(int32(currentState), int32(StateStopping)) {
				continue // State changed, retry
			}
			// Cancel context to signal goroutines.
			// stateMu protects cancel field against concurrent TransitionToStarting.
			b.stateMu.Lock()
			cancel := b.cancel
			b.stateMu.Unlock()
			if cancel != nil {
				cancel()
			}
			return true
		default:
			return false
		}
	}
}

// TransitionToStopped marks the server as fully stopped.
// Must be called after all goroutines have exited.
func (b *Base) TransitionToStopped() bool {
	b.stateMu.Lock()
	if !b.casToTerminalLocked(StateStopped) {
		b.stateMu.Unlock()
		return false
	}
	b.closeErrChannelLocked()
	b.stateMu.Unlock()
	return true
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
	b.stateMu.Lock()
	ctx := b.ctx
	b.stateMu.Unlock()
	return ctx
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
	b.stateMu.Lock()
	b.sendErrorLocked(err)
	b.stateMu.Unlock()
}

// CloseErrChannel closes the error channel to signal consumers.
// Should be called when the server is fully stopped.
func (b *Base) CloseErrChannel() {
	b.stateMu.Lock()
	b.closeErrChannelLocked()
	b.stateMu.Unlock()
}

// StartedChannel returns the started channel for custom waiting logic.
// The channel is closed when the server transitions to Running.
func (b *Base) StartedChannel() <-chan struct{} {
	return b.startedCh
}

// notCreatedErrorLocked is the error of a TransitionToStarting that found the
// server out of Created. The state it names is never Created again: no
// transition returns to Created. The caller holds stateMu.
func (b *Base) notCreatedErrorLocked() error {
	return fmt.Errorf("%w in state %s", ErrStartNotCreated, b.State())
}

// casToTerminalLocked moves a non-terminal state to target and reports
// whether it did. It compare-and-swaps from the observed state, re-reading on
// failure: TransitionToStopping changes the state lock-free (for example
// Created -> Stopped), and an unconditional Store here could overwrite that
// terminal state (formal/tla/Serverbase.tla, finding F2). The caller holds
// stateMu.
func (b *Base) casToTerminalLocked(target State) bool {
	for {
		current := b.State()
		if current.IsTerminal() {
			return false
		}
		if b.state.CompareAndSwap(int32(current), int32(target)) {
			return true
		}
	}
}

func (b *Base) sendErrorLocked(err error) {
	if b.errClosed {
		return
	}
	select {
	case b.errCh <- err:
	default:
	}
}

func (b *Base) closeErrChannelLocked() {
	b.errCloseOnce.Do(func() {
		b.errClosed = true
		close(b.errCh)
	})
}
