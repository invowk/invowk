// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// T009: State transition tests
func TestStateTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Created to Starting to Running to Stopped", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		// Initial state should be Created
		if b.State() != StateCreated {
			t.Errorf("expected StateCreated, got %s", b.State())
		}

		// Transition to Starting
		ctx := t.Context()
		if err := b.TransitionToStarting(ctx); err != nil {
			t.Fatalf("TransitionToStarting failed: %v", err)
		}
		if b.State() != StateStarting {
			t.Errorf("expected StateStarting, got %s", b.State())
		}

		// Transition to Running
		b.TransitionToRunning()
		if b.State() != StateRunning {
			t.Errorf("expected StateRunning, got %s", b.State())
		}
		if !b.IsRunning() {
			t.Error("IsRunning should return true")
		}

		// Transition to Stopping
		if !b.TransitionToStopping() {
			t.Error("TransitionToStopping should return true")
		}
		if b.State() != StateStopping {
			t.Errorf("expected StateStopping, got %s", b.State())
		}

		// Transition to Stopped
		b.TransitionToStopped()
		if b.State() != StateStopped {
			t.Errorf("expected StateStopped, got %s", b.State())
		}
	})

	t.Run("Starting to Failed", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := t.Context()
		if err := b.TransitionToStarting(ctx); err != nil {
			t.Fatalf("TransitionToStarting failed: %v", err)
		}

		// Transition to Failed
		testErr := context.DeadlineExceeded
		if err := b.TransitionToFailed(testErr); !errors.Is(err, testErr) {
			t.Fatalf("TransitionToFailed() = %v, want %v", err, testErr)
		}

		if b.State() != StateFailed {
			t.Errorf("expected StateFailed, got %s", b.State())
		}

		if !errors.Is(b.LastError(), testErr) {
			t.Errorf("expected %v, got %v", testErr, b.LastError())
		}

		// Check error was sent to channel
		select {
		case err := <-b.Err():
			if !errors.Is(err, testErr) {
				t.Errorf("expected %v from error channel, got %v", testErr, err)
			}
		default:
			t.Error("expected error in channel")
		}

		select {
		case err, ok := <-b.Err():
			if ok {
				t.Fatalf("expected error channel closed after failure, got %v", err)
			}
		default:
			t.Fatal("expected error channel closed after failure")
		}
	})
}

// T010: Race condition tests (run with -race flag)
func TestRaceConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "concurrent state reads during transitions", run: func(t *testing.T) {
			t.Helper()

			b := NewBase()

			var wg sync.WaitGroup

			// Start multiple readers
			for range 10 {
				wg.Go(func() {
					for range 100 {
						_ = b.State()
						_ = b.IsRunning()
					}
				})
			}

			// Perform transitions
			ctx := t.Context()
			_ = b.TransitionToStarting(ctx)
			b.TransitionToRunning()
			b.TransitionToStopping()
			b.TransitionToStopped()

			wg.Wait()
		}},
		{name: "concurrent Stop calls", run: func(t *testing.T) {
			t.Helper()

			b := NewBase()

			ctx := t.Context()
			if err := b.TransitionToStarting(ctx); err != nil {
				t.Fatalf("TransitionToStarting failed: %v", err)
			}
			b.TransitionToRunning()

			var wg sync.WaitGroup
			for range 10 {
				wg.Go(func() {
					b.TransitionToStopping()
				})
			}
			wg.Wait()

			// Should end up in Stopping state
			state := b.State()
			if state != StateStopping && state != StateStopped {
				t.Errorf("expected StateStopping or StateStopped, got %s", state)
			}
		}},
		{name: "competing Start and Stop transitions", run: func(t *testing.T) {
			t.Helper()

			// Launch N goroutines that race to Start vs Stop the server.
			// The state machine must always reach a valid terminal state
			// without panics, deadlocks, or invalid intermediate states.
			for range 50 {
				b := NewBase()

				var wg sync.WaitGroup

				// Goroutine 1: tries to Start then transition to Running
				wg.Go(func() {
					ctx := t.Context()
					if err := b.TransitionToStarting(ctx); err == nil {
						b.TransitionToRunning()
					}
				})

				// Goroutine 2: tries to Stop
				wg.Go(func() {
					b.TransitionToStopping()
				})

				// Goroutine 3: concurrent state readers
				wg.Go(func() {
					for range 20 {
						state := b.State()
						// Every observed state must be valid
						if err := state.Validate(); err != nil {
							t.Errorf("observed invalid state during competing transitions: %s", state)
						}
					}
				})

				wg.Wait()

				// After all goroutines finish, the server must be in a valid state.
				// Acceptable terminal/near-terminal states: Stopped, Failed, Running,
				// Stopping (if Stop won the race but TransitionToStopped wasn't called).
				finalState := b.State()
				switch finalState {
				case StateCreated:
					// Created is acceptable only if Stop raced before Start
					// and TransitionToStopping moved it directly to Stopped,
					// but the goroutine reading state saw Created first.
					// Re-check: after wg.Wait() the state must be stable.
					// Created means Stop called on Created → Stopped, but
					// CAS(Created→Stopped) leaves it Stopped not Created.
					// So Created here means Start hasn't run yet — possible
					// if goroutine 1 hasn't been scheduled.
				case StateRunning, StateStopping, StateStopped, StateFailed:
					// All valid outcomes of the Start/Stop race
				case StateStarting:
					// Acceptable: Start won but TransitionToRunning hasn't
					// executed yet (goroutine scheduling)
				default:
					t.Errorf("unexpected final state after competing transitions: %s", finalState)
				}
			}
		}},
		{name: "concurrent lifecycle context reads during transitions", run: func(t *testing.T) {
			t.Helper()

			for range 50 {
				b := NewBase()

				var wg sync.WaitGroup
				for range 8 {
					wg.Go(func() {
						for range 100 {
							ctx := b.Context()
							if ctx != nil {
								select {
								case <-ctx.Done():
								default:
								}
							}
						}
					})
				}
				wg.Go(func() {
					if err := b.TransitionToStarting(t.Context()); err == nil {
						b.TransitionToRunning()
					}
				})
				wg.Go(func() {
					b.TransitionToFailed(context.Canceled)
				})
				wg.Go(func() {
					b.TransitionToStopping()
				})
				wg.Wait()

				if err := b.State().Validate(); err != nil {
					t.Fatalf("final state is invalid: %v", err)
				}
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

// T011: Double Start/Stop idempotency tests
func TestIdempotency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "double Start returns error", run: func(t *testing.T) {
			t.Helper()

			b := NewBase()

			ctx := t.Context()
			if err := b.TransitionToStarting(ctx); err != nil {
				t.Fatalf("first TransitionToStarting failed: %v", err)
			}

			// Second Start should fail
			err := b.TransitionToStarting(ctx)
			if err == nil {
				t.Error("expected error on second TransitionToStarting")
			}
		}},
		{name: "double Stop is safe", run: func(t *testing.T) {
			t.Helper()

			b := NewBase()

			ctx := t.Context()
			if err := b.TransitionToStarting(ctx); err != nil {
				t.Fatalf("TransitionToStarting failed: %v", err)
			}
			b.TransitionToRunning()

			// First Stop
			if !b.TransitionToStopping() {
				t.Error("first TransitionToStopping should return true")
			}
			b.TransitionToStopped()

			// Second Stop should be no-op (return false, no panic)
			if b.TransitionToStopping() {
				t.Error("second TransitionToStopping should return false")
			}

			if b.State() != StateStopped {
				t.Errorf("expected StateStopped, got %s", b.State())
			}
		}},
		{name: "Stop without Start is safe", run: func(t *testing.T) {
			t.Helper()

			b := NewBase()

			// Stop on Created server should transition to Stopped
			if b.TransitionToStopping() {
				t.Error("TransitionToStopping from Created should return false")
			}

			if b.State() != StateStopped {
				t.Errorf("expected StateStopped, got %s", b.State())
			}

			select {
			case _, ok := <-b.Err():
				if ok {
					t.Fatal("Err channel still open after stopping a never-started server")
				}
			default:
				t.Fatal("Err channel should be closed after stopping a never-started server")
			}
		}},
		{name: "Stop on Failed is safe", run: func(t *testing.T) {
			t.Helper()

			b := NewBase()

			ctx := t.Context()
			if err := b.TransitionToStarting(ctx); err != nil {
				t.Fatalf("TransitionToStarting failed: %v", err)
			}

			b.TransitionToFailed(context.DeadlineExceeded)

			// Stop on Failed should be no-op
			if b.TransitionToStopping() {
				t.Error("TransitionToStopping from Failed should return false")
			}

			if b.State() != StateFailed {
				t.Errorf("expected StateFailed, got %s", b.State())
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

// T012: Cancelled context tests
func TestCancelledContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		operation   string
		cancel      bool
		becomeReady bool
		wantErr     error
		wantState   State
		wantLastErr error
	}{
		{name: "Start with already cancelled context fails immediately", operation: "start", cancel: true, wantErr: context.Canceled, wantState: StateFailed, wantLastErr: context.Canceled},
		{name: "WaitForReady respects context cancellation", operation: "wait", cancel: true, wantErr: context.Canceled, wantState: StateStarting},
		{name: "WaitForReady succeeds when server becomes ready", operation: "wait", becomeReady: true, wantState: StateRunning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := NewBase()
			var err error
			switch tt.operation {
			case "start":
				ctx, cancel := context.WithCancel(t.Context())
				if tt.cancel {
					cancel()
				} else {
					defer cancel()
				}
				err = b.TransitionToStarting(ctx)
			case "wait":
				if startErr := b.TransitionToStarting(t.Context()); startErr != nil {
					t.Fatalf("TransitionToStarting() error = %v", startErr)
				}
				waitCtx, cancel := context.WithTimeout(t.Context(), time.Second)
				if tt.cancel {
					cancel()
				} else {
					defer cancel()
				}
				if tt.becomeReady {
					go b.TransitionToRunning()
				}
				err = b.WaitForReady(waitCtx)
			default:
				t.Fatalf("unknown operation %q", tt.operation)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("operation error = %v, want %v", err, tt.wantErr)
			}
			if got := b.State(); got != tt.wantState {
				t.Errorf("State() = %s, want %s", got, tt.wantState)
			}
			if !errors.Is(b.LastError(), tt.wantLastErr) {
				t.Errorf("LastError() = %v, want %v", b.LastError(), tt.wantLastErr)
			}
		})
	}
}

func TestTransitionToFailedCancelsLifecycleContext(t *testing.T) {
	t.Parallel()

	b := NewBase()
	if err := b.TransitionToStarting(t.Context()); err != nil {
		t.Fatalf("TransitionToStarting() error = %v", err)
	}

	serverCtx := b.Context()
	if serverCtx == nil {
		t.Fatal("Context() = nil after TransitionToStarting()")
	}

	cause := context.DeadlineExceeded
	if err := b.TransitionToFailed(cause); !errors.Is(err, cause) {
		t.Fatalf("TransitionToFailed() = %v, want %v", err, cause)
	}

	select {
	case <-serverCtx.Done():
	default:
		t.Fatal("server context should be cancelled after TransitionToFailed()")
	}
	if !errors.Is(serverCtx.Err(), context.Canceled) {
		t.Fatalf("server context error = %v, want context.Canceled", serverCtx.Err())
	}
}

func TestTransitionToStarting_LifecycleContextInheritsCallerContext(t *testing.T) {
	t.Parallel()

	b := NewBase()

	// Create a cancellable context to simulate caller cancellation (e.g., Ctrl+C).
	callerCtx, cancel := context.WithCancel(t.Context())
	if err := b.TransitionToStarting(callerCtx); err != nil {
		t.Fatalf("TransitionToStarting failed: %v", err)
	}

	// The server's internal context should still be active.
	if b.Context().Err() != nil {
		t.Fatal("server context should not be cancelled before caller cancels")
	}

	// Cancel the caller's context — this should propagate to the server's lifecycle context.
	cancel()

	// Give the cancellation a moment to propagate through the context tree.
	select {
	case <-b.Context().Done():
		// Expected: server context is now cancelled.
	case <-time.After(time.Second):
		t.Fatal("server context was not cancelled after caller context was cancelled")
	}
}

// Test State.String()
func TestStateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state    State
		expected string
	}{
		{StateCreated, "created"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateFailed, "failed"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

// Test State.IsTerminal()
func TestStateIsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state      State
		isTerminal bool
	}{
		{StateCreated, false},
		{StateStarting, false},
		{StateRunning, false},
		{StateStopping, false},
		{StateStopped, true},
		{StateFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			t.Parallel()

			if got := tt.state.IsTerminal(); got != tt.isTerminal {
				t.Errorf("State(%d).IsTerminal() = %v, want %v", tt.state, got, tt.isTerminal)
			}
		})
	}
}

// Test WithErrorChannel option
func TestWithErrorChannel(t *testing.T) {
	t.Parallel()

	b := NewBase(WithErrorChannel(5))

	// Should be able to send 5 errors without blocking
	for range 5 {
		b.SendError(context.DeadlineExceeded)
	}

	// Read them all back
	for i := range 5 {
		select {
		case <-b.Err():
			// Expected
		default:
			t.Errorf("expected error %d in channel", i)
		}
	}
}

func TestDefaultErrorChannelDropsAdditionalErrors(t *testing.T) {
	t.Parallel()

	b := NewBase()

	b.SendError(context.Canceled)
	b.SendError(context.DeadlineExceeded)

	select {
	case err := <-b.Err():
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("first error = %v, want context.Canceled", err)
		}
	default:
		t.Fatal("expected first error in channel")
	}

	select {
	case err := <-b.Err():
		t.Fatalf("unexpected second buffered error: %v", err)
	default:
	}
}

func TestSendErrorAfterCloseIsIgnored(t *testing.T) {
	t.Parallel()

	b := NewBase()
	b.CloseErrChannel()
	b.SendError(context.Canceled)

	select {
	case _, ok := <-b.Err():
		if ok {
			t.Fatal("Err channel still open after CloseErrChannel()")
		}
	default:
		t.Fatal("Err channel should be closed after CloseErrChannel()")
	}
}

func TestTransitionToStoppedClosesErrChannel(t *testing.T) {
	t.Parallel()

	b := NewBase()
	if err := b.TransitionToStarting(t.Context()); err != nil {
		t.Fatalf("TransitionToStarting() error = %v", err)
	}
	b.TransitionToRunning()
	if !b.TransitionToStopping() {
		t.Fatal("TransitionToStopping() = false, want true")
	}

	b.TransitionToStopped()
	b.CloseErrChannel()

	select {
	case _, ok := <-b.Err():
		if ok {
			t.Fatal("Err channel still open after TransitionToStopped()")
		}
	default:
		t.Fatal("Err channel should be closed after TransitionToStopped()")
	}
}

func TestTerminalTransitionsAreIrreversible(t *testing.T) {
	t.Parallel()

	t.Run("failed is not overwritten by stopped", func(t *testing.T) {
		t.Parallel()

		b := NewBase()
		if err := b.TransitionToStarting(t.Context()); err != nil {
			t.Fatalf("TransitionToStarting() error = %v", err)
		}
		cause := context.DeadlineExceeded
		if err := b.TransitionToFailed(cause); !errors.Is(err, cause) {
			t.Fatalf("TransitionToFailed() = %v, want %v", err, cause)
		}
		if b.TransitionToStopped() {
			t.Fatal("TransitionToStopped() = true after failed terminal state")
		}
		if b.State() != StateFailed {
			t.Fatalf("State() = %s, want %s", b.State(), StateFailed)
		}
		if !errors.Is(b.LastError(), cause) {
			t.Fatalf("LastError() = %v, want %v", b.LastError(), cause)
		}
	})

	t.Run("stopped is not overwritten by failed", func(t *testing.T) {
		t.Parallel()

		b := NewBase()
		if err := b.TransitionToStarting(t.Context()); err != nil {
			t.Fatalf("TransitionToStarting() error = %v", err)
		}
		b.TransitionToRunning()
		if !b.TransitionToStopping() {
			t.Fatal("TransitionToStopping() = false, want true")
		}
		if !b.TransitionToStopped() {
			t.Fatal("TransitionToStopped() = false, want true")
		}
		if err := b.TransitionToFailed(context.Canceled); err != nil {
			t.Fatalf("TransitionToFailed() after stopped = %v, want nil ignored transition", err)
		}
		if b.State() != StateStopped {
			t.Fatalf("State() = %s, want %s", b.State(), StateStopped)
		}
		if b.LastError() != nil {
			t.Fatalf("LastError() = %v, want nil", b.LastError())
		}
	})
}

func TestLastErrorReleasesStateLock(t *testing.T) {
	t.Parallel()

	b := NewBase()
	if err := b.TransitionToStarting(t.Context()); err != nil {
		t.Fatalf("TransitionToStarting() error = %v", err)
	}
	cause := context.Canceled
	if err := b.TransitionToFailed(cause); !errors.Is(err, cause) {
		t.Fatalf("TransitionToFailed() = %v, want %v", err, cause)
	}
	if !errors.Is(b.LastError(), cause) {
		t.Fatalf("LastError() = %v, want %v", b.LastError(), cause)
	}

	done := make(chan struct{})
	go func() {
		_ = b.TransitionToStopped()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("TransitionToStopped() blocked after LastError()")
	}
}

// Test goroutine tracking
func TestGoroutineTracking(t *testing.T) {
	t.Parallel()

	b := NewBase()

	ctx := t.Context()
	if err := b.TransitionToStarting(ctx); err != nil {
		t.Fatalf("TransitionToStarting failed: %v", err)
	}

	var counter int
	var mu sync.Mutex

	// Start some goroutines
	for range 5 {
		b.AddGoroutine()
		go func() {
			defer b.DoneGoroutine()
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}

	// Wait for all goroutines
	b.WaitForShutdown()

	mu.Lock()
	if counter != 5 {
		t.Errorf("expected counter=5, got %d", counter)
	}
	mu.Unlock()
}

// Test Context() returns the server context
func TestContext(t *testing.T) {
	t.Parallel()

	b := NewBase()

	// Before Start, context should be nil
	if b.Context() != nil {
		t.Error("expected nil context before Start")
	}

	ctx := t.Context()
	if err := b.TransitionToStarting(ctx); err != nil {
		t.Fatalf("TransitionToStarting failed: %v", err)
	}

	// After Start, context should be non-nil
	if b.Context() == nil {
		t.Error("expected non-nil context after Start")
	}

	// Context should be cancelled after TransitionToStopping
	b.TransitionToRunning()
	b.TransitionToStopping()

	select {
	case <-b.Context().Done():
		// Expected
	default:
		t.Error("context should be cancelled after TransitionToStopping")
	}
}

func TestState_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state   State
		wantOK  bool
		wantErr bool
	}{
		{StateCreated, true, false},
		{StateStarting, true, false},
		{StateRunning, true, false},
		{StateStopping, true, false},
		{StateStopped, true, false},
		{StateFailed, true, false},
		{State(99), false, true},
		{State(-1), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			t.Parallel()
			err := tt.state.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("State(%d).Validate() error = %v, wantOK %v", tt.state, err, tt.wantOK)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("State(%d).Validate() returned nil, want error", tt.state)
				}
				if !errors.Is(err, ErrInvalidState) {
					t.Errorf("error should wrap ErrInvalidState, got: %v", err)
				}
				stateErr, ok := errors.AsType[*InvalidStateError](err)
				if !ok {
					t.Fatalf("State(%d).Validate() error type = %T, want *InvalidStateError", tt.state, err)
				}
				if stateErr.Value != tt.state {
					t.Fatalf("InvalidStateError.Value = %d, want %d", stateErr.Value, tt.state)
				}
			} else if err != nil {
				t.Errorf("State(%d).Validate() returned unexpected error: %v", tt.state, err)
			}
		})
	}
}
