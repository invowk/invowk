// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"time"
)

// SpinCancelWait bounds how long SpinWithAction waits for its action after it
// cancelled the action's context because the spinner stopped early (Ctrl+C,
// SIGINT or a Bubble Tea quit). Callers that kill a process on cancellation
// should cut its I/O off well before this bound (exec.Cmd.WaitDelay).
const SpinCancelWait = 15 * time.Second

var (
	// ErrSpinCancelled is returned by SpinWithAction when the spinner stopped
	// before the action ended, the action's context was cancelled, and the
	// action then ended within SpinCancelWait. It wraps context.Canceled.
	ErrSpinCancelled error = spinCancelledError{}

	// ErrSpinActionStillRunning is returned by SpinWithAction when the action
	// had not ended SpinCancelWait after its context was cancelled. The action
	// goroutine is abandoned: the caller must not read anything it writes.
	ErrSpinActionStillRunning = errors.New("spin: the command did not stop within " +
		SpinCancelWait.String() + " of being cancelled and may still be running")
)

// spinCancelledError is the type of ErrSpinCancelled.
type spinCancelledError struct{}

// Error implements the error interface.
func (spinCancelledError) Error() string { return "spin: command cancelled" }

// Unwrap returns context.Canceled, so errors.Is(ErrSpinCancelled, context.Canceled) holds.
func (spinCancelledError) Unwrap() error { return context.Canceled }
