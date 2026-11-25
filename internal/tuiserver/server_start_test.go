// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/invowk/invowk/internal/core/serverbase"
)

// cancelledStartContext returns a context that is already cancelled.
func cancelledStartContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

func newStartTestServer(t *testing.T) *Server {
	t.Helper()
	server, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return server
}

// TestServerSecondStartWithCancelledContextOnRunningServer checks that a
// second Start with a cancelled context leaves a running TUI server serving
// (spec server-lifecycle-base).
func TestServerSecondStartWithCancelledContextOnRunningServer(t *testing.T) {
	t.Parallel()

	server := newStartTestServer(t)
	if err := server.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	err := server.Start(cancelledStartContext(t))
	if !errors.Is(err, serverbase.ErrStartNotCreated) {
		t.Fatalf("second Start() error = %v, want serverbase.ErrStartNotCreated", err)
	}
	if st := server.State(); st != serverbase.StateRunning {
		t.Fatalf("State() = %s after a second cancelled Start, want %s", st, serverbase.StateRunning)
	}
	if server.LastError() != nil {
		t.Errorf("LastError() = %v, want nil", server.LastError())
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, string(server.URL())+"/", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request after a second cancelled Start error = %v, want the server still serving", err)
	}
	_ = resp.Body.Close()
	if err := server.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if st := server.State(); st != serverbase.StateStopped {
		t.Errorf("State() = %s after Stop, want %s", st, serverbase.StateStopped)
	}
}

// TestServerSecondStartWithCancelledContextWhileStarting checks that a
// cancelled Start does not fail a server whose first Start is in progress.
// tuiserver.Start has no hold point, so the test puts the base in Starting as
// the first Start's TransitionToStarting does.
func TestServerSecondStartWithCancelledContextWhileStarting(t *testing.T) {
	t.Parallel()

	server := newStartTestServer(t)
	if err := server.base.TransitionToStarting(t.Context()); err != nil {
		t.Fatalf("TransitionToStarting() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	err := server.Start(cancelledStartContext(t))
	if !errors.Is(err, serverbase.ErrStartNotCreated) {
		t.Fatalf("second Start() error = %v, want serverbase.ErrStartNotCreated", err)
	}
	if st := server.State(); st != serverbase.StateStarting {
		t.Fatalf("State() = %s, want %s (not marked Failed by the second Start)", st, serverbase.StateStarting)
	}
	if n := len(server.ListenAddrs()); n != 0 {
		t.Errorf("second Start bound %d listeners, want 0", n)
	}
	if ctx := server.base.Context(); ctx == nil || ctx.Err() != nil {
		t.Errorf("server context = %v, want the first Start's live context", ctx)
	}
}

// TestServerStartWithCancelledContextAfterStop checks that a cancelled Start
// on a server that Stop moved Created -> Stopped binds nothing and returns an
// error. The base returned the recorded nil there and Start went on to bind.
func TestServerStartWithCancelledContextAfterStop(t *testing.T) {
	t.Parallel()

	server := newStartTestServer(t)
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() on a fresh server error = %v", err)
	}

	err := server.Start(cancelledStartContext(t))
	if err == nil {
		t.Fatal("Start() on a stopped server = nil, want an error")
	}
	if !errors.Is(err, serverbase.ErrStartNotCreated) {
		t.Errorf("Start() error = %v, want serverbase.ErrStartNotCreated", err)
	}
	if n := len(server.ListenAddrs()); n != 0 {
		t.Errorf("Start bound %d listeners on a stopped server, want 0", n)
		closeListeners(server.listeners)
	}
	if st := server.State(); st != serverbase.StateStopped {
		t.Errorf("State() = %s, want %s", st, serverbase.StateStopped)
	}
}

// TestServerCancelledStartRacesStop races a cancelled Start with Stop on a
// Created server, under -race: Start never returns nil, the server ends
// Failed or Stopped, and nothing is bound.
func TestServerCancelledStartRacesStop(t *testing.T) {
	t.Parallel()

	ctx := cancelledStartContext(t)
	for i := range 50 {
		server := newStartTestServer(t)
		release := make(chan struct{})
		startCh := make(chan error, 1)
		stopCh := make(chan error, 1)
		go func() {
			<-release
			startCh <- server.Start(ctx)
		}()
		go func() {
			<-release
			stopCh <- server.Stop()
		}()
		close(release)
		startErr, stopErr := <-startCh, <-stopCh

		if startErr == nil {
			t.Errorf("iteration %d: Start() = nil, want an error", i)
		}
		if stopErr != nil {
			t.Errorf("iteration %d: Stop() error = %v, want nil", i, stopErr)
		}
		if st := server.State(); st != serverbase.StateFailed && st != serverbase.StateStopped {
			t.Errorf("iteration %d: State() = %s, want Failed or Stopped", i, st)
		}
		if n := len(server.ListenAddrs()); n != 0 {
			t.Errorf("iteration %d: %d listeners bound, want 0", i, n)
			closeListeners(server.listeners)
		}
	}
}
