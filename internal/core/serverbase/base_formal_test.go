// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"pgregory.net/rapid"
)

// This file binds formal/tla/Serverbase.tla to the real Base.
//
// The TLA+ model checks every interleaving of two concurrent transitions. The
// tests here cover what Go tests can reach without new seams:
//   - TestServerbase_SequentialMatchesModel replays random sequences of the
//     model's operations and compares each result with the model's sequential
//     semantics.
//   - TestServerbase_ConcurrentSafetyInvariants races two operations and checks
//     only the invariants the model proves (ErrClosedAtMostOnce,
//     NoSendAfterClose, StartedClosedByWinner).
//
// TerminalAbsorbing (finding F2) and StopCancelsContext (finding F1) have
// recorded TLC counterexamples. They need interleavings inside a single
// transition, which the race detector does not flag and a stress test hits
// too rarely to assert, so they stay model-only evidence until the fix change
// adds a seam.

const (
	opStart serverbaseOp = iota
	opStartCancelled
	opRun
	opFail
	opStop
	opStopped
	opSend
)

var errFormalTest = errors.New("formal test failure")

type serverbaseOp int

// sequentialStep is the model's sequential semantics: the state after op runs
// alone from `from`, and whether TransitionToStopping reports a win.
func sequentialStep(from State, op serverbaseOp) (next State, stopWon bool) {
	switch op {
	case opStart:
		if from == StateCreated {
			return StateStarting, false
		}
		return from, false
	case opStartCancelled, opFail:
		if from.IsTerminal() {
			return from, false
		}
		return StateFailed, false
	case opRun:
		if from == StateStarting {
			return StateRunning, false
		}
		return from, false
	case opStop:
		switch from {
		case StateCreated:
			return StateStopped, false
		case StateStarting, StateRunning:
			return StateStopping, true
		case StateStopping, StateStopped, StateFailed:
			return from, false
		default:
			return from, false
		}
	case opStopped:
		if from.IsTerminal() {
			return from, false
		}
		return StateStopped, false
	case opSend:
		return from, false
	default:
		return from, false
	}
}

func applyServerbaseOp(ctx context.Context, b *Base, op serverbaseOp) (stopWon bool) {
	switch op {
	case opStart:
		_ = b.TransitionToStarting(ctx)
	case opStartCancelled:
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_ = b.TransitionToStarting(cancelled)
	case opRun:
		b.TransitionToRunning()
	case opFail:
		_ = b.TransitionToFailed(errFormalTest)
	case opStop:
		return b.TransitionToStopping()
	case opStopped:
		b.TransitionToStopped()
	case opSend:
		b.SendError(errFormalTest)
	}
	return false
}

// errChannelClosed drains buffered errors and reports whether Err() is closed.
func errChannelClosed(b *Base) bool {
	for {
		select {
		case _, ok := <-b.Err():
			if !ok {
				return true
			}
		default:
			return false
		}
	}
}

func startedChannelClosed(b *Base) bool {
	select {
	case <-b.StartedChannel():
		return true
	default:
		return false
	}
}

func TestServerbase_SequentialMatchesModel(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		b := NewBase()
		ref := StateCreated
		reachedRunning := false
		steps := rapid.SliceOfN(rapid.IntRange(int(opStart), int(opSend)), 1, 12).Draw(rt, "ops")
		for i, raw := range steps {
			op := serverbaseOp(raw)
			want, wantWon := sequentialStep(ref, op)
			gotWon := applyServerbaseOp(t.Context(), b, op)
			if got := b.State(); got != want {
				rt.Fatalf("step %d op %d: state %s, model %s", i, op, got, want)
			}
			if gotWon != wantWon {
				rt.Fatalf("step %d op %d: TransitionToStopping returned %v, model %v", i, op, gotWon, wantWon)
			}
			ref = want
			reachedRunning = reachedRunning || want == StateRunning
			// StartedClosedByWinner: readiness is signalled exactly when Running was reached.
			if startedChannelClosed(b) != reachedRunning {
				rt.Fatalf("step %d: started channel closed=%v, Running reached=%v", i, !reachedRunning, reachedRunning)
			}
			// The error channel is closed exactly when the state is terminal.
			if closed := errChannelClosed(b); closed != ref.IsTerminal() {
				rt.Fatalf("step %d: error channel closed=%v in state %s", i, closed, ref)
			}
		}
	})
}

// TestServerbase_ConcurrentSafetyInvariants races two operations on a fresh
// Base. A double close or a send on a closed channel panics, so completing
// without a panic checks ErrClosedAtMostOnce and NoSendAfterClose; the started
// channel may only be closed if Running was reached.
func TestServerbase_ConcurrentSafetyInvariants(t *testing.T) {
	t.Parallel()

	rounds := 2000
	if testing.Short() {
		rounds = 200
	}
	ops := []serverbaseOp{opStart, opStartCancelled, opRun, opFail, opStop, opStopped, opSend}
	for round := range rounds {
		for _, first := range ops {
			second := ops[round%len(ops)]
			b := NewBase()
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); applyServerbaseOp(t.Context(), b, first) }()
			go func() { defer wg.Done(); applyServerbaseOp(t.Context(), b, second) }()
			wg.Wait()
			if startedChannelClosed(b) && b.State() == StateCreated {
				t.Fatalf("ops %d/%d: started channel closed although Running was never reached", first, second)
			}
		}
	}
}
