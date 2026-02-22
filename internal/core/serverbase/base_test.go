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
		ctx := context.Background()
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

		ctx := context.Background()
		if err := b.TransitionToStarting(ctx); err != nil {
			t.Fatalf("TransitionToStarting failed: %v", err)
		}

		// Transition to Failed
		testErr := context.DeadlineExceeded
		b.TransitionToFailed(testErr)

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
	})
}

// T010: Race condition tests (run with -race flag)
func TestRaceConditions(t *testing.T) {
	t.Parallel()

	t.Run("concurrent state reads during transitions", func(t *testing.T) {
		t.Parallel()

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
		ctx := context.Background()
		_ = b.TransitionToStarting(ctx)
		b.TransitionToRunning()
		b.TransitionToStopping()
		b.TransitionToStopped()

		wg.Wait()
	})

	t.Run("concurrent Stop calls", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := context.Background()
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
	})
}

// T011: Double Start/Stop idempotency tests
func TestIdempotency(t *testing.T) {
	t.Parallel()

	t.Run("double Start returns error", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := context.Background()
		if err := b.TransitionToStarting(ctx); err != nil {
			t.Fatalf("first TransitionToStarting failed: %v", err)
		}

		// Second Start should fail
		err := b.TransitionToStarting(ctx)
		if err == nil {
			t.Error("expected error on second TransitionToStarting")
		}
	})

	t.Run("double Stop is safe", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := context.Background()
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
	})

	t.Run("Stop without Start is safe", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		// Stop on Created server should transition to Stopped
		if b.TransitionToStopping() {
			t.Error("TransitionToStopping from Created should return false")
		}

		if b.State() != StateStopped {
			t.Errorf("expected StateStopped, got %s", b.State())
		}
	})

	t.Run("Stop on Failed is safe", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := context.Background()
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
	})
}

// T012: Cancelled context tests
func TestCancelledContext(t *testing.T) {
	t.Parallel()

	t.Run("Start with already cancelled context fails immediately", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := b.TransitionToStarting(ctx)
		if err == nil {
			t.Error("expected error with cancelled context")
		}

		if b.State() != StateFailed {
			t.Errorf("expected StateFailed, got %s", b.State())
		}
	})

	t.Run("WaitForReady respects context cancellation", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := context.Background()
		if err := b.TransitionToStarting(ctx); err != nil {
			t.Fatalf("TransitionToStarting failed: %v", err)
		}

		// Create a context that will be cancelled
		waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// Don't transition to Running, so WaitForReady should timeout
		err := b.WaitForReady(waitCtx)
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("WaitForReady succeeds when server becomes ready", func(t *testing.T) {
		t.Parallel()

		b := NewBase()

		ctx := context.Background()
		if err := b.TransitionToStarting(ctx); err != nil {
			t.Fatalf("TransitionToStarting failed: %v", err)
		}

		// Transition to Running in a goroutine
		go func() {
			b.TransitionToRunning()
		}()

		waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := b.WaitForReady(waitCtx)
		if err != nil {
			t.Errorf("WaitForReady failed: %v", err)
		}
	})
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

// Test goroutine tracking
func TestGoroutineTracking(t *testing.T) {
	t.Parallel()

	b := NewBase()

	ctx := context.Background()
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

	ctx := context.Background()
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

func TestState_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state   State
		want    bool
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
			isValid, errs := tt.state.IsValid()
			if isValid != tt.want {
				t.Errorf("State(%d).IsValid() = %v, want %v", tt.state, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("State(%d).IsValid() returned no errors, want error", tt.state)
				}
				if !errors.Is(errs[0], ErrInvalidState) {
					t.Errorf("error should wrap ErrInvalidState, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("State(%d).IsValid() returned unexpected errors: %v", tt.state, errs)
			}
		})
	}
}
