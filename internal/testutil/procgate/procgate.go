// SPDX-License-Identifier: MPL-2.0

// Package procgate parks a simulated process at a seam of real code until
// the test opens the gate. Concurrency characterisation tests (the
// ConcurrentModuleEdits replays) use it to interleave processes at exactly
// the step a counterexample names, synchronising only through channels.
package procgate

import (
	"sync"
	"testing"
)

type (
	// Gate parks one process at a seam until the test opens it.
	Gate struct {
		once    sync.Once
		reached chan struct{}
		release chan struct{}
	}

	// Outcome is what a process run by Run returned.
	Outcome[T any] struct {
		Val T
		Err error
	}
)

// New returns a closed gate: the first Park blocks until Open.
func New() *Gate {
	return &Gate{reached: make(chan struct{}), release: make(chan struct{})}
}

// Run starts fn as a process and returns a channel that receives its outcome.
func Run[T any](fn func() (T, error)) <-chan Outcome[T] {
	done := make(chan Outcome[T], 1)
	go func() {
		val, err := fn()
		done <- Outcome[T]{Val: val, Err: err}
	}()
	return done
}

// AwaitParked waits until the gated process reaches its seam, failing the
// test if the process returns first.
func AwaitParked[T any](t *testing.T, g *Gate, done <-chan Outcome[T]) {
	t.Helper()
	select {
	case <-g.reached:
	case o := <-done:
		t.Fatalf("gated process returned before its seam: %v", o.Err)
	}
}

// Park signals that the process reached the seam, then blocks until Open.
func (g *Gate) Park() {
	g.once.Do(func() { close(g.reached) })
	<-g.release
}

// Open releases the parked process.
func (g *Gate) Open() { close(g.release) }
