// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"fmt"
	"testing"
	"time"

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
