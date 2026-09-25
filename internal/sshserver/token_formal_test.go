// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/testutil"
)

const (
	tokenNone tokenModelState = iota
	tokenValid
	tokenRevoked
)

type (
	tokenModelState int

	// modelSession is an authenticated connection: the server's wrapper and
	// the recording connection underneath it.
	modelSession struct {
		raw  *recordingConn
		conn *tokenConn
	}
)

// TestHostCallbackToken_LifecycleMatchesModel binds formal/tla/HostCallbackToken.tla
// to the real token API: random sequences of generate, login, revoke, session
// end, and clock advances past the TTL must agree with the model's token
// states. A revoked or expired token never logs in again (NoAuthAfterExecution),
// revocation closes every connection the token authenticated
// (NoSessionAfterExecution), and expiry alone closes none. Connections are
// recording stand-ins; TestRevokeTokenClosesAuthenticatedConnection covers a
// real SSH client.
func TestHostCallbackToken_LifecycleMatchesModel(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		clock := testutil.NewFakeClock(time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
		cfg := DefaultConfig()
		srv, err := NewWithClock(cfg, clock)
		if err != nil {
			rt.Fatalf("NewWithClock() error = %v", err)
		}

		const executions = 3
		model := make([]tokenModelState, executions)
		issuedAt := make([]time.Time, executions)
		tokens := make([]*Token, executions)
		sessions := make([][]modelSession, executions)

		steps := rapid.IntRange(1, 20).Draw(rt, "steps")
		for step := range steps {
			e := rapid.IntRange(0, executions-1).Draw(rt, fmt.Sprintf("exec%d", step))
			switch action := rapid.IntRange(0, 4).Draw(rt, fmt.Sprintf("action%d", step)); action {
			case 0: // an execution starts and generates its token
				if model[e] != tokenNone {
					continue
				}
				tok, genErr := srv.GenerateToken(CommandID(fmt.Sprintf("exec-%d", e)))
				if genErr != nil {
					rt.Fatalf("GenerateToken() error = %v", genErr)
				}
				tokens[e], model[e], issuedAt[e] = tok, tokenValid, clock.Now()
			case 1: // a process in the container tries to log in
				if tokens[e] == nil {
					continue
				}
				expired := clock.Now().After(issuedAt[e].Add(cfg.TokenTTL))
				want := model[e] == tokenValid && !expired
				raw := &recordingConn{}
				conn := &tokenConn{Conn: raw, server: srv}
				if _, got := srv.admitConn(tokens[e].Value, conn); got != want {
					rt.Fatalf("step %d: login(exec %d) = %v, model %v (state %d, expired %v)", step, e, got, want, model[e], expired)
				} else if got {
					sessions[e] = append(sessions[e], modelSession{raw: raw, conn: conn})
				}
				if expired {
					model[e] = tokenRevoked // expired tokens are dropped at login
				}
				for _, open := range sessions[e] {
					if open.raw.closed.Load() {
						rt.Fatalf("step %d: expiry closed a session of exec %d; only revocation may", step, e)
					}
				}
			case 2: // the execution ends on any path; the deferred cleanup revokes
				if tokens[e] == nil {
					continue
				}
				srv.RevokeToken(tokens[e].Value)
				model[e] = tokenRevoked
				for _, open := range sessions[e] {
					if !open.raw.closed.Load() {
						rt.Fatalf("step %d: a session of exec %d outlived its revoked token (F7)", step, e)
					}
				}
				sessions[e] = nil
			case 3: // the process in the container ends a session itself
				if len(sessions[e]) == 0 {
					continue
				}
				srv.tokenMu.RLock()
				tracked := len(srv.conns[tokens[e].Value])
				srv.tokenMu.RUnlock()
				if tracked != len(sessions[e]) {
					rt.Fatalf("step %d: server tracks %d sessions of exec %d, model %d", step, tracked, e, len(sessions[e]))
				}
				last := sessions[e][len(sessions[e])-1]
				sessions[e] = sessions[e][:len(sessions[e])-1]
				_ = last.conn.Close()
			default: // time passes beyond the TTL
				clock.Advance(cfg.TokenTTL + time.Second)
			}
		}
	})
}

// TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen replays finding
// F11 (HostCallbackToken.findingF11StopKeepsSessions) with a real SSH client:
// Stop closes the listener, but ssh.Server.Shutdown only waits for open
// connections and closes none, so an authenticated client stays connected
// after Stop returns, and Stop reports the shutdown deadline. The fix inverts
// this test.
func TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ShutdownTimeout = 100 * time.Millisecond
	srv := mustNew(t, cfg)
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	token, err := srv.GenerateToken("exec-f11")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	client := dialWithToken(t, srv, token.Value)
	closed := closedSignal(client)

	stopErr := srv.Stop()
	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Fatalf("Stop() error = %v, want context.DeadlineExceeded while a connection is open", stopErr)
	}
	select {
	case <-closed:
		t.Fatal("Stop() closed the authenticated connection; F11 is fixed, invert this test")
	case <-time.After(200 * time.Millisecond):
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession() after Stop error = %v, want the connection still usable (F11)", err)
	}
	_ = session.Close()
}

// dialToken logs in to srv over loopback with a real SSH client.
func dialToken(srv *Server, token TokenValue) (*gossh.Client, error) {
	client, err := gossh.Dial("tcp", srv.Address(), &gossh.ClientConfig{
		User:            "invowk",
		Auth:            []gossh.AuthMethod{gossh.Password(string(token))},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test client for a loopback server with an ephemeral host key
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", srv.Address(), err)
	}
	return client, nil
}

// dialWithToken is dialToken that fails the test on error and closes the
// client when the test ends.
func dialWithToken(t *testing.T, srv *Server, token TokenValue) *gossh.Client {
	t.Helper()
	client, err := dialToken(srv, token)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// closedSignal returns a channel closed once the client's connection closed
// (gossh.Client.Wait returned).
func closedSignal(client *gossh.Client) <-chan struct{} {
	closed := make(chan struct{})
	go func() {
		_ = client.Wait()
		close(closed)
	}()
	return closed
}
