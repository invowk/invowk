// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingConn is a net.Conn stand-in that records whether it was closed.
type recordingConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *recordingConn) Close() error {
	c.closed.Store(true)
	return nil
}

// TestRevokeTokenClosesAuthenticatedConnection replays the fix for finding F7
// (HostCallbackToken.sessionLifetimeWithRevokeClosing) with a real SSH client:
// revoking the token closes the connection it authenticated, so neither the
// open connection nor a new session outlives the execution.
func TestRevokeTokenClosesAuthenticatedConnection(t *testing.T) {
	t.Parallel()

	srv := mustNew(t, DefaultConfig())
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	token, err := srv.GenerateToken("exec-f7")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	client := dialWithToken(t, srv, token.Value)

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession() before revocation error = %v", err)
	}
	_ = session.Close()

	closed := closedSignal(client)
	srv.RevokeToken(token.Value)

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("connection still open 5s after its token was revoked (F7)")
	}
	if extra, sessionErr := client.NewSession(); sessionErr == nil {
		_ = extra.Close()
		t.Fatal("NewSession() after revocation succeeded; want the connection closed")
	}
}

// TestRevocationRacesAuthenticationSafely checks the atomicity the model
// assumes: a login racing its token's revocation either fails or has its
// connection closed by the revocation, never an open connection for a
// revoked token.
func TestRevocationRacesAuthenticationSafely(t *testing.T) {
	t.Parallel()

	srv := mustNew(t, DefaultConfig())
	for range 2000 {
		token, err := srv.GenerateToken("exec-race")
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		raw := &recordingConn{}
		conn := &tokenConn{Conn: raw, server: srv}

		var admitted bool
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Go(func() { <-start; _, admitted = srv.admitConn(token.Value, conn) })
		wg.Go(func() { <-start; srv.RevokeToken(token.Value) })
		close(start)
		wg.Wait()

		if admitted && !raw.closed.Load() {
			t.Fatal("login admitted and its connection left open after the token was revoked")
		}
	}
}

// TestRevokeTokensForCommandClosesConnectionsOfExpiredTokens: an expired
// token is dropped from the token map, but its connections stay bound to the
// execution and close when the execution revokes by command.
func TestRevokeTokensForCommandClosesConnectionsOfExpiredTokens(t *testing.T) {
	t.Parallel()

	srv := mustNew(t, DefaultConfig())
	token, err := srv.GenerateToken("exec-expired")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	raw := &recordingConn{}
	if _, ok := srv.admitConn(token.Value, &tokenConn{Conn: raw, server: srv}); !ok {
		t.Fatal("admitConn() = false for a fresh token")
	}
	srv.tokenMu.Lock()
	delete(srv.tokens, token.Value) // as the expiry sweep does
	srv.tokenMu.Unlock()

	srv.RevokeTokensForCommand("exec-expired")
	if !raw.closed.Load() {
		t.Fatal("RevokeTokensForCommand() left open a connection of an expired token")
	}
}
