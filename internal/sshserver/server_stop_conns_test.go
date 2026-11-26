// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/testutil"
)

// stopConnsServer starts a server whose password handler holds every login
// at hold, and whose teardown closes connsClosed once it has closed the
// accepted connections.
func stopConnsServer(t *testing.T, shutdownTimeout time.Duration, hold *lifecycleHold) (srv *Server, connsClosed <-chan struct{}) {
	t.Helper()
	srv = newTestServer(t, func(cfg *Config) { cfg.ShutdownTimeout = shutdownTimeout })
	closedCh := make(chan struct{})
	var closeOnce sync.Once
	srv.hooks = &lifecycleHooks{
		beforeAdmit: hold.hold,
		connsClosed: func() { closeOnce.Do(func() { close(closedCh) }) },
	}
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	stopOnCleanup(t, srv, hold)
	return srv, closedCh
}

// dialAsyncToken logs in with token in its own goroutine and reports the
// login's error.
func dialAsyncToken(srv *Server, token TokenValue) <-chan error {
	ch := make(chan error, 1)
	go func() {
		client, err := dialToken(srv, token)
		if err == nil {
			_ = client.Close()
		}
		ch <- err
	}()
	return ch
}

func requireNoIndexedConns(t *testing.T, srv *Server) {
	t.Helper()
	testutil.RequirePollUntil(t, waitTimeout, 5*time.Millisecond, "connections still indexed by token after Stop", func() bool {
		srv.tokenMu.RLock()
		defer srv.tokenMu.RUnlock()
		return len(srv.conns) == 0
	})
	srv.connMu.Lock()
	defer srv.connMu.Unlock()
	if n := len(srv.accepted); n != 0 {
		t.Errorf("server holds %d accepted connections after Stop, want 0", n)
	}
}

// TestServerStopDuringHandshake holds a login inside the password check,
// runs Stop, and releases the hold once Stop has closed the connections while
// it still waits: the login fails, nothing is tracked, and Stop returns nil
// well inside ShutdownTimeout. Under -race it is the regression test of the
// library-wait race gliderlabs/ssh#117, which Stop no longer depends on: with
// the library's graceful Shutdown, a handshake that completed while Shutdown
// waited added to its connection WaitGroup concurrently with Wait. The base
// may pass this test when that race does not trigger.
func TestServerStopDuringHandshake(t *testing.T) {
	t.Parallel()

	const shutdownTimeout = 5 * time.Second
	hold := newLifecycleHold()
	srv, connsClosed := stopConnsServer(t, shutdownTimeout, hold)
	token, err := srv.GenerateToken("exec-handshake")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	login := dialAsyncToken(srv, token.Value)
	awaitClosed(t, hold.reached, hookTimeout, "password check")

	began := time.Now()
	stopCh := stopAsync(srv)
	awaitClosed(t, connsClosed, hookTimeout, "teardown closing the connections")
	requireNotReturned(t, stopCh, "Stop")
	hold.Release()

	if err := awaitErr(t, stopCh, callTimeout, "Stop"); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if elapsed := time.Since(began); elapsed >= shutdownTimeout {
		t.Errorf("Stop() took %v, want well inside ShutdownTimeout %v", elapsed, shutdownTimeout)
	}
	if err := awaitErr(t, login, callTimeout, "login"); err == nil {
		t.Error("login held across Stop succeeded, want it to fail")
	}
	requireNoIndexedConns(t, srv)
}

// TestServerStopTimesOutOnHeldLogin holds a login inside the password check
// until after Stop returned: Stop returns an error at ShutdownTimeout, and the
// login, released afterwards, fails.
func TestServerStopTimesOutOnHeldLogin(t *testing.T) {
	t.Parallel()

	const shutdownTimeout = 300 * time.Millisecond
	hold := newLifecycleHold()
	srv, connsClosed := stopConnsServer(t, shutdownTimeout, hold)
	token, err := srv.GenerateToken("exec-held-login")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	login := dialAsyncToken(srv, token.Value)
	awaitClosed(t, hold.reached, hookTimeout, "password check")

	stopErr := awaitErr(t, stopAsync(srv), callTimeout, "Stop")
	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Errorf("Stop() error = %v, want context.DeadlineExceeded while a login is held", stopErr)
	}
	awaitClosed(t, connsClosed, hookTimeout, "teardown closing the connections")

	hold.Release()
	if err := awaitErr(t, login, callTimeout, "login"); err == nil {
		t.Error("login released after Stop succeeded, want it to fail")
	}
	requireNoIndexedConns(t, srv)
}

// TestServerRevocationRacesStop revokes an authenticated client's token while
// Stop runs: both return, the connection is closed once, and -race reports
// nothing.
func TestServerRevocationRacesStop(t *testing.T) {
	t.Parallel()

	for i := range 10 {
		srv := newTestServer(t, func(cfg *Config) { cfg.ShutdownTimeout = 5 * time.Second })
		if err := srv.Start(t.Context()); err != nil {
			t.Fatalf("iteration %d: Start() error = %v", i, err)
		}
		token, err := srv.GenerateToken("exec-revoke-stop")
		if err != nil {
			t.Fatalf("iteration %d: GenerateToken() error = %v", i, err)
		}
		client := dialWithToken(t, srv, token.Value)
		closed := closedSignal(client)
		waitUntilServerTracksConn(t, client)

		release := make(chan struct{})
		revoked := make(chan struct{})
		go func() {
			<-release
			srv.RevokeToken(token.Value)
			close(revoked)
		}()
		stopCh := make(chan error, 1)
		go func() {
			<-release
			stopCh <- srv.Stop()
		}()
		close(release)

		if err := awaitErr(t, stopCh, callTimeout, "Stop"); err != nil {
			t.Errorf("iteration %d: Stop() error = %v, want nil", i, err)
		}
		awaitClosed(t, revoked, callTimeout, "RevokeToken")
		awaitClosed(t, closed, callTimeout, "client connection close")
		requireNoIndexedConns(t, srv)
	}
}
