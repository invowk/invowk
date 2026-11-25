// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"context"
	"errors"
	"strings"
	"testing"
)

var errBaseStartTest = errors.New("base start test failure")

// TestTransitionToStarting_NotCreatedChangesNothing checks that a Start on a
// server that is not in Created returns an error naming the state and leaves
// the state, the recorded failure and the server context alone, whether or
// not its own context is cancelled (decision 120). The base let a cancelled
// Start fail a Starting or Running server and return nil on a stopped one.
func TestTransitionToStarting_NotCreatedChangesNothing(t *testing.T) {
	t.Parallel()

	setups := []struct {
		state State
		setup func(t *testing.T, b *Base)
	}{
		{StateStarting, func(t *testing.T, b *Base) { t.Helper(); mustStart(t, b) }},
		{StateRunning, func(t *testing.T, b *Base) { t.Helper(); mustStart(t, b); b.TransitionToRunning() }},
		{StateStopping, func(t *testing.T, b *Base) {
			t.Helper()
			mustStart(t, b)
			if !b.TransitionToStopping() {
				t.Fatal("TransitionToStopping() = false on a Starting server")
			}
		}},
		{StateStopped, func(t *testing.T, b *Base) { t.Helper(); b.TransitionToStopping() }},
		{StateFailed, func(t *testing.T, b *Base) { t.Helper(); mustStart(t, b); _ = b.TransitionToFailed(errBaseStartTest) }},
	}

	for _, setup := range setups {
		for _, cancelled := range []bool{true, false} {
			name := setup.state.String() + "/live context"
			if cancelled {
				name = setup.state.String() + "/cancelled context"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				b := NewBase()
				setup.setup(t, b)
				if got := b.State(); got != setup.state {
					t.Fatalf("setup state = %s, want %s", got, setup.state)
				}
				lastErr, serverCtx := b.LastError(), b.Context()
				errClosed := errChannelClosed(b)

				ctx, cancel := context.WithCancel(t.Context())
				if cancelled {
					cancel()
				} else {
					defer cancel()
				}
				err := b.TransitionToStarting(ctx)

				if !errors.Is(err, ErrStartNotCreated) {
					t.Fatalf("TransitionToStarting() error = %v, want ErrStartNotCreated", err)
				}
				if !strings.Contains(err.Error(), setup.state.String()) {
					t.Errorf("TransitionToStarting() error = %q, want it to name state %s", err, setup.state)
				}
				if got := b.State(); got != setup.state {
					t.Errorf("State() = %s after TransitionToStarting, want %s unchanged", got, setup.state)
				}
				if got := b.LastError(); !errors.Is(got, lastErr) || (lastErr == nil) != (got == nil) {
					t.Errorf("LastError() = %v after TransitionToStarting, want %v unchanged", got, lastErr)
				}
				if got := b.Context(); got != serverCtx {
					t.Error("Context() changed after TransitionToStarting")
				}
				if (setup.state == StateStarting || setup.state == StateRunning) && serverCtx.Err() != nil {
					t.Errorf("server context cancelled by TransitionToStarting: %v", serverCtx.Err())
				}
				if got := errChannelClosed(b); got != errClosed {
					t.Errorf("Err() closed = %v after TransitionToStarting, want %v unchanged", got, errClosed)
				}
			})
		}
	}
}

// TestTransitionToStarting_CancelledOnCreatedFails checks that the first
// Start with a cancelled context still fails a Created server.
func TestTransitionToStarting_CancelledOnCreatedFails(t *testing.T) {
	t.Parallel()

	b := NewBase()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := b.TransitionToStarting(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("TransitionToStarting() error = %v, want context.Canceled", err)
	}
	if errors.Is(err, ErrStartNotCreated) {
		t.Errorf("TransitionToStarting() error = %v, want no ErrStartNotCreated on Created", err)
	}
	if got := b.State(); got != StateFailed {
		t.Errorf("State() = %s, want %s", got, StateFailed)
	}
	if !errors.Is(b.LastError(), context.Canceled) {
		t.Errorf("LastError() = %v, want context.Canceled", b.LastError())
	}
	select {
	case got, open := <-b.Err():
		if !open || !errors.Is(got, context.Canceled) {
			t.Errorf("first Err() receive = %v (open %t), want the context.Canceled failure", got, open)
		}
	default:
		t.Error("Err() delivered nothing after a cancelled first start")
	}
	if !errChannelClosed(b) {
		t.Error("Err() still open after a cancelled first start")
	}
}

// TestTransitionToStarting_CancelledRacesStop races a cancelled Start with a
// Stop on a Created server: the server ends Failed or Stopped, never moves
// from one to the other, and the Start never returns nil.
func TestTransitionToStarting_CancelledRacesStop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for i := range 200 {
		b := NewBase()
		release := make(chan struct{})
		startErr := make(chan error, 1)
		stopped := make(chan struct{})
		go func() {
			<-release
			startErr <- b.TransitionToStarting(ctx)
		}()
		go func() {
			<-release
			b.TransitionToStopping()
			close(stopped)
		}()
		close(release)
		err := <-startErr
		<-stopped

		if err == nil {
			t.Fatalf("iteration %d: TransitionToStarting() = nil, want an error", i)
		}
		switch got := b.State(); got {
		case StateFailed:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("iteration %d: Failed with error %v, want context.Canceled", i, err)
			}
		case StateStopped:
			if !errors.Is(err, ErrStartNotCreated) {
				t.Fatalf("iteration %d: Stopped with error %v, want ErrStartNotCreated", i, err)
			}
			if b.LastError() != nil {
				t.Fatalf("iteration %d: LastError() = %v on a stopped server, want nil", i, b.LastError())
			}
		case StateCreated, StateStarting, StateRunning, StateStopping:
			t.Fatalf("iteration %d: State() = %s, want Failed or Stopped", i, got)
		default:
			t.Fatalf("iteration %d: State() = %s, want Failed or Stopped", i, got)
		}
		if !errChannelClosed(b) {
			t.Fatalf("iteration %d: Err() still open", i)
		}
	}
}

func mustStart(t *testing.T, b *Base) {
	t.Helper()
	if err := b.TransitionToStarting(t.Context()); err != nil {
		t.Fatalf("TransitionToStarting() error = %v", err)
	}
}
