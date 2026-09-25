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

type tokenModelState int

// TestHostCallbackToken_LifecycleMatchesModel binds formal/tla/HostCallbackToken.tla
// to the real token API: random sequences of generate, validate, revoke, and
// clock advances past the TTL must agree with the model's token states. A
// revoked or expired token never validates again (NoAuthAfterExecution); the
// model's session properties need a live SSH server and stay model-only.
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

		steps := rapid.IntRange(1, 20).Draw(rt, "steps")
		for step := range steps {
			e := rapid.IntRange(0, executions-1).Draw(rt, fmt.Sprintf("exec%d", step))
			switch action := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("action%d", step)); action {
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
				if _, got := srv.ValidateToken(tokens[e].Value); got != want {
					rt.Fatalf("step %d: ValidateToken(exec %d) = %v, model %v (state %d, expired %v)", step, e, got, want, model[e], expired)
				}
				if expired {
					model[e] = tokenRevoked // ValidateToken revokes expired tokens
				}
			case 2: // the execution ends on any path; the deferred cleanup revokes
				if tokens[e] == nil {
					continue
				}
				srv.RevokeToken(tokens[e].Value)
				model[e] = tokenRevoked
			default: // time passes beyond the TTL
				clock.Advance(cfg.TokenTTL + time.Second)
			}
		}
	})
}
