// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/invowk/invowk/internal/core/serverbase"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/tlatrace"
)

const (
	// tokenTraceCloseBound bounds the wait for a revoked or closed connection.
	tokenTraceCloseBound = 5 * time.Second
	// tokenTraceStopSettle is the fixed settle after Stop. Under finding F11
	// Stop closes no connection, so the long close bound would only add wall
	// time; the fix switches Stop back to tokenTraceCloseBound.
	tokenTraceStopSettle = 200 * time.Millisecond
	// tokenTraceStagger separates token issue times so exactly one token can
	// expire per step (a merged expiry is a record the spec rejects).
	tokenTraceStagger = time.Second

	tokenPhaseIdle    = "idle"
	tokenPhaseRunning = "running"
	tokenPhaseEnded   = "ended"
)

type (
	// tokenTraceClient is a real SSH client and the signal that its
	// connection closed (gossh.Client.Wait returned).
	tokenTraceClient struct {
		client *gossh.Client
		closed <-chan struct{}
	}

	// tokenTraceExec is the harness's view of one execution. phase is the
	// only projected field that is harness bookkeeping, not ground truth: it
	// stands for the execution, which lives outside internal/sshserver.
	tokenTraceExec struct {
		commandID CommandID
		token     TokenValue
		expiresAt time.Time
		phase     string
		clients   []*tokenTraceClient
	}

	tokenTraceRun struct {
		t     *testing.T
		srv   *Server
		clock *testutil.FakeClock
		execs [2]*tokenTraceExec
		rec   tlatrace.Recorder
	}
)

// tokenTraceRecord renders HostCallbackTokenTrace.tla's flat Proj.
func tokenTraceRecord(e1Exec string, e1Usable, e1Session bool, e2Exec string, e2Usable, e2Session bool, server string) tlatrace.Record {
	return tlatrace.Record{
		"e1_exec": tlatrace.Str(e1Exec), "e1_usable": tlatrace.Bool(e1Usable), "e1_session": tlatrace.Bool(e1Session),
		"e2_exec": tlatrace.Str(e2Exec), "e2_usable": tlatrace.Bool(e2Usable), "e2_session": tlatrace.Bool(e2Session),
		"server": tlatrace.Str(server),
	}
}

// TestHostCallbackToken_TraceHarness records traces of a started Server on a
// fake clock, with real SSH clients logging in over loopback, for trace
// validation against HostCallbackToken.tla, plus targeted mutations that
// validation must reject.
func TestHostCallbackToken_TraceHarness(t *testing.T) {
	t.Parallel()
	tlatrace.Dir(t) // skip before doing any work when trace output is off

	sequences := tokenTraceSequences()
	accepted := tlatrace.RecordEach(t, sequences, runTokenTrace)
	if accepted == nil {
		return
	}
	recorded := make(map[string][]tlatrace.Record, len(sequences))
	for i, name := range sequences {
		recorded[name] = accepted[i]
	}
	tlatrace.WriteSuite(t, "HostCallbackToken", tlatrace.Traces{Accepted: accepted, Rejected: tokenTraceMutations(t, recorded)})
}

// tokenTraceSequences returns every sequence of Start(e1) followed by up to
// three operations on e1, plus curated two-execution sequences. An operation
// is its action letter and execution number: S1 is Start(e1), P is Stop.
func tokenTraceSequences() []string {
	sequences := []string{"S1"}
	frontier := []string{"S1"}
	for range 3 {
		var next []string
		for _, prefix := range frontier {
			for _, op := range []string{"A1", "E1", "C1", "X1", "P"} {
				next = append(next, prefix+" "+op)
			}
		}
		sequences = append(sequences, next...)
		frontier = next
	}
	return append(sequences,
		"S1 S2 A1 A2 E1 E2",       // overlapping executions
		"S1 S2 A1 A2 E1 C2",       // revoking one execution while the other's session is open
		"S1 A1 S2 A2 E2 A1 C1 E1", // a second login of e1 after e2 ended
		"S1 S2 X1 X2",             // staggered expiry
		"S1 S2 A1 X1 A2 X2 E1 E2", // expiry closes nothing; revocation does
		"S1 S2 X2 A2 A1 E1",       // e2 expires first: a login with it fails
		"S1 S2 A1 A2 P",           // Stop with both executions active (F11)
		"S1 S2 A1 P C1 E2 S2 A2",  // after Stop: no new execution, no new login
		"S1 A1 S2 E1 P E2 X2",     // Stop after one execution ended
	)
}

func runTokenTrace(t *testing.T, sequence string) []tlatrace.Record {
	t.Helper()
	cfg := DefaultConfig()
	cfg.ShutdownTimeout = 100 * time.Millisecond
	clock := testutil.NewFakeClock(time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC))
	srv, err := NewWithClock(cfg, clock)
	if err != nil {
		t.Fatalf("NewWithClock() error = %v", err)
	}
	if err = srv.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	run := &tokenTraceRun{t: t, srv: srv, clock: clock}
	for e := range run.execs {
		run.execs[e] = &tokenTraceExec{commandID: CommandID(fmt.Sprintf("exec-%d", e+1)), phase: tokenPhaseIdle}
	}
	run.observe()
	for op := range strings.FieldsSeq(sequence) {
		run.apply(op)
		run.observe()
	}
	return run.rec.Trace()
}

func (r *tokenTraceRun) apply(op string) {
	r.t.Helper()
	if op == "P" {
		r.stop()
		return
	}
	e := r.execs[op[1]-'1']
	switch op[:1] {
	case "S":
		r.start(e)
	case "A":
		r.login(e)
	case "E":
		// Every end path revokes the token. Revoking an ended execution again
		// is a call the code allows; the model has no step for it.
		if e.phase == tokenPhaseIdle {
			return
		}
		r.srv.RevokeToken(e.token)
		e.phase = tokenPhaseEnded
		r.awaitClosed(e)
	case "C":
		for _, c := range e.clients {
			_ = c.client.Close()
		}
		r.awaitClosed(e)
	case "X":
		r.expire(e)
	default:
		r.t.Fatalf("unknown token trace operation %q", op)
	}
}

func (r *tokenTraceRun) start(e *tokenTraceExec) {
	r.t.Helper()
	if e.phase != tokenPhaseIdle {
		return // an execution starts once
	}
	if later := r.clock.Now().Add(tokenTraceStagger); !r.expiresAnother(later, e) {
		r.clock.Set(later)
	}
	info, err := r.srv.GetConnectionInfo(e.commandID)
	if !r.srv.IsRunning() {
		if err == nil {
			r.t.Fatal("GetConnectionInfo() succeeded on a stopped server")
		}
		return // the model's Start is disabled once the server stopped
	}
	if err != nil {
		r.t.Fatalf("GetConnectionInfo() error = %v", err)
	}
	e.token, e.expiresAt, e.phase = info.Token, info.ExpireAt, tokenPhaseRunning
}

// expiresAnother reports whether moving the clock to at would expire an
// admissible token other than except's, which would merge two expiries into
// one step.
func (r *tokenTraceRun) expiresAnother(at time.Time, except *tokenTraceExec) bool {
	for _, other := range r.execs {
		if other != except && r.usable(other) && at.After(other.expiresAt) {
			return true
		}
	}
	return false
}

// login dials with the execution's token. A login the model allows must
// succeed; a login it forbids that succeeds anyway shows up as a session step
// no action allows, and validation rejects the trace.
func (r *tokenTraceRun) login(e *tokenTraceExec) {
	r.t.Helper()
	if e.phase == tokenPhaseIdle {
		return // no token to log in with
	}
	allowed := r.srv.IsRunning() && r.usable(e)
	client, err := dialToken(r.srv, e.token)
	if err != nil {
		if allowed {
			r.t.Fatalf("login with an admissible token of %s failed: %v", e.commandID, err)
		}
		return
	}
	r.t.Cleanup(func() { _ = client.Close() })
	e.clients = append(e.clients, &tokenTraceClient{client: client, closed: closedSignal(client)})
}

// expire sets the clock just past one admissible token's expiry. When that
// would also expire another admissible token, the step would merge two
// expiries, so the harness skips it.
func (r *tokenTraceRun) expire(e *tokenTraceExec) {
	if target := e.expiresAt.Add(time.Millisecond); r.usable(e) && !r.expiresAnother(target, e) {
		r.clock.Set(target)
	}
}

// stop stops the server. Shutdown closes the listener and waits for open
// connections until its deadline, closing none (finding F11), so Stop returns
// context.DeadlineExceeded while an authenticated client is connected.
func (r *tokenTraceRun) stop() {
	r.t.Helper()
	open := openClients(r.execs[0]) + openClients(r.execs[1])
	if err := r.srv.Stop(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		r.t.Fatalf("Stop() error = %v", err)
	}
	if open > 0 {
		time.Sleep(tokenTraceStopSettle) // observe then cross-checks the connection map
	}
}

// awaitClosed waits up to tokenTraceCloseBound for every client of e to see
// its connection close and for the server to forget them. On timeout the
// still-open state is recorded, and validation rejects the trace.
func (r *tokenTraceRun) awaitClosed(e *tokenTraceExec) {
	testutil.PollUntil(r.t, tokenTraceCloseBound, 5*time.Millisecond, func() bool {
		return openClients(e) == 0 && r.trackedConns(e) == 0
	})
}

func openClients(e *tokenTraceExec) int {
	open := 0
	for _, c := range e.clients {
		select {
		case <-c.closed:
		default:
			open++
		}
	}
	return open
}

// usable reports, without side effects, whether the server would admit the
// execution's token now; ValidateToken would drop an expired entry.
func (r *tokenTraceRun) usable(e *tokenTraceExec) bool {
	if e.token == "" {
		return false
	}
	r.srv.tokenMu.RLock()
	defer r.srv.tokenMu.RUnlock()
	token, ok := r.srv.tokens[e.token]
	return ok && !r.clock.Now().After(token.ExpiresAt)
}

func (r *tokenTraceRun) trackedConns(e *tokenTraceExec) int {
	r.srv.tokenMu.RLock()
	defer r.srv.tokenMu.RUnlock()
	return len(r.srv.conns[e.token])
}

func (r *tokenTraceRun) observe() {
	r.t.Helper()
	var server string
	switch state := r.srv.State(); state {
	case serverbase.StateRunning:
		server = "running"
	case serverbase.StateStopped:
		server = "stopped"
	case serverbase.StateCreated, serverbase.StateStarting, serverbase.StateStopping, serverbase.StateFailed:
		r.t.Fatalf("server state %s has no model counterpart", state)
	}
	e1, e2 := r.execs[0], r.execs[1]
	for _, e := range r.execs {
		if tracked, open := r.trackedConns(e), openClients(e); tracked != open {
			r.t.Fatalf("server tracks %d connections of %s, but %d clients are connected", tracked, e.commandID, open)
		}
	}
	r.rec.Observe(tokenTraceRecord(
		e1.phase, r.usable(e1), openClients(e1) > 0,
		e2.phase, r.usable(e2), openClients(e2) > 0,
		server))
}

// tokenTraceMutations edits accepted traces one step at a time into
// behaviours HostCallbackToken.tla forbids. A connection left open after Stop
// is not among them: the base configuration accepts it (open finding F11).
func tokenTraceMutations(t *testing.T, recorded map[string][]tlatrace.Record) [][]tlatrace.Record {
	t.Helper()
	trace := func(sequence string) []tlatrace.Record {
		tr, ok := recorded[sequence]
		if !ok {
			t.Fatalf("mutation base %q was not recorded", sequence)
		}
		return tr
	}
	yes, no := tlatrace.Bool(true), tlatrace.Bool(false)
	return [][]tlatrace.Record{
		// F7 (NoSessionAfterExecution): revocation leaves the connection open.
		tlatrace.Edit(trace("S1 A1 E1"), -1, tlatrace.Record{"e1_session": yes}),
		// NoAuthAfterExecution: a login succeeds after revocation.
		tlatrace.Extend(trace("S1 E1"), tlatrace.Record{"e1_session": yes}),
		// Expiry closes a connection: only revocation may.
		tlatrace.Edit(trace("S1 A1 X1"), -1, tlatrace.Record{"e1_session": no}),
		// Revoking e1 closes e2's connection.
		tlatrace.Edit(trace("S1 S2 A1 A2 E1 E2"), 5, tlatrace.Record{"e2_session": no}),
		// A revoked token becomes admissible again.
		tlatrace.Extend(trace("S1 E1"), tlatrace.Record{"e1_usable": yes}),
		// Two tokens expire in one record.
		tlatrace.Edit(trace("S1 S2 X1 X2"), 3, tlatrace.Record{"e2_usable": no}),
		// A stopped server issues a token for a new execution.
		tlatrace.Extend(trace("S1 P"), tlatrace.Record{"e2_exec": tlatrace.Str(tokenPhaseRunning), "e2_usable": yes}),
	}
}
