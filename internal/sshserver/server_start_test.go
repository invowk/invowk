// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/invowk/invowk/internal/core/serverbase"
)

// cancelledContext returns a context that is already cancelled.
func cancelledContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

// TestServerSecondStartWithCancelledContextOnRunningServer checks that a
// second Start with a cancelled context leaves a running server running
// (spec server-lifecycle-base). The base marked it Failed and cancelled it.
func TestServerSecondStartWithCancelledContextOnRunningServer(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, nil)
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	stopOnCleanup(t, srv)
	addr := srv.Address()

	err := srv.Start(cancelledContext(t))
	if !errors.Is(err, serverbase.ErrStartNotCreated) {
		t.Fatalf("second Start() error = %v, want serverbase.ErrStartNotCreated", err)
	}
	if st := srv.State(); st != serverbase.StateRunning {
		t.Fatalf("State() = %s after a second cancelled Start, want %s", st, serverbase.StateRunning)
	}
	if srv.LastError() != nil {
		t.Errorf("LastError() = %v, want nil", srv.LastError())
	}
	if !dialConnects(t.Context(), addr) {
		t.Fatalf("dial %s refused after a second cancelled Start, want the server still serving", addr)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if st := srv.State(); st != serverbase.StateStopped {
		t.Errorf("State() = %s after Stop, want %s", st, serverbase.StateStopped)
	}
	assertTornDown(t, srv)
}

// TestServerSecondStartWithCancelledContextDuringStart checks that a second
// Start with a cancelled context during a held first Start changes nothing:
// the first Start still reaches readiness and Stop leaves nothing running.
func TestServerSecondStartWithCancelledContextDuringStart(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, nil)
	hold := newLifecycleHold()
	capture := &addrCapture{}
	srv.hooks = &lifecycleHooks{beforePublish: func(addr net.Addr) {
		capture.record(addr)
		hold.hold()
	}}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "beforePublish")

	err := srv.Start(cancelledContext(t))
	if !errors.Is(err, serverbase.ErrStartNotCreated) {
		t.Fatalf("second Start() error = %v, want serverbase.ErrStartNotCreated", err)
	}
	if fired := capture.fired.Load(); fired != 1 {
		t.Errorf("beforePublish fired %d times, want 1 (the second Start bound nothing)", fired)
	}
	requireStillStarting(t, srv, startCh)

	hold.Release()
	if err := awaitErr(t, startCh, callTimeout, "first Start"); err != nil {
		t.Fatalf("first Start() error = %v, want nil", err)
	}
	if st := srv.State(); st != serverbase.StateRunning {
		t.Fatalf("State() = %s, want %s", st, serverbase.StateRunning)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
	assertTornDown(t, srv)
}

// assertTornDown checks that the one teardown ran and closed the published
// listener. It reads the server's fields rather than dialling the port: a
// parallel test may bind the freed ephemeral port again.
func assertTornDown(t *testing.T, srv *Server) {
	t.Helper()
	srv.srvMu.Lock()
	defer srv.srvMu.Unlock()
	if !srv.tornDown || srv.listener != nil || srv.srv != nil {
		t.Errorf("after Stop: tornDown = %v, listener = %v, srv = %v; want the teardown to have closed and cleared them", srv.tornDown, srv.listener, srv.srv)
	}
}
