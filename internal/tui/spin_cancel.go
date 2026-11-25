// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"fmt"
	"time"
)

const (
	// SpinCancelWait bounds how long SpinWithAction waits for its action after
	// it cancelled the action's context because the spinner stopped early
	// (Ctrl+C, SIGINT or a Bubble Tea quit). Callers that kill a process on
	// cancellation should cut its I/O off well before this bound
	// (exec.Cmd.WaitDelay).
	SpinCancelWait = 15 * time.Second

	// spinCancelledErrMsg is the message of ErrSpinCancelled.
	spinCancelledErrMsg = "spin: command cancelled"

	// spinStillRunningErrFmt formats the message of ErrSpinActionStillRunning
	// with SpinCancelWait.
	spinStillRunningErrFmt = "spin: the command did not stop within %s of being cancelled and may still be running"
)

var (
	// ErrSpinCancelled is returned by SpinWithAction when the spinner stopped
	// before the action ended, the action's context was cancelled, and the
	// action then ended within SpinCancelWait. It wraps context.Canceled.
	ErrSpinCancelled error = spinCancelledError{}

	// ErrSpinActionStillRunning is returned by SpinWithAction when the action
	// had not ended SpinCancelWait after its context was cancelled. The action
	// goroutine is abandoned: the caller must not read anything it writes.
	ErrSpinActionStillRunning error = spinStillRunningError{}
)

type (
	// spinCancelledError is the type of ErrSpinCancelled.
	spinCancelledError struct{}

	// spinStillRunningError is the type of ErrSpinActionStillRunning.
	spinStillRunningError struct{}
)

// Error implements the error interface.
func (spinCancelledError) Error() string { return spinCancelledErrMsg }

// Unwrap returns context.Canceled, so errors.Is(ErrSpinCancelled, context.Canceled) holds.
func (spinCancelledError) Unwrap() error { return context.Canceled }

// Error implements the error interface.
func (spinStillRunningError) Error() string {
	return fmt.Sprintf(spinStillRunningErrFmt, SpinCancelWait)
}
