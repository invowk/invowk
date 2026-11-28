// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"

	"github.com/invowk/invowk/internal/runtime"
)

type (
	signalTrackerContextKey struct{}

	// signalTracker records the first trapped OS termination signal before the
	// associated context is cancelled, giving downstream handlers a race-free
	// happens-before edge once ctx.Done() closes.
	signalTracker struct {
		mu  sync.Mutex
		sig os.Signal
	}
)

// Signal returns the first recorded termination signal, or nil if none arrived.
func (t *signalTracker) Signal() os.Signal {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	sig := t.sig
	t.mu.Unlock()
	return sig
}

func (t *signalTracker) record(sig os.Signal) {
	if t == nil || sig == nil {
		return
	}
	t.mu.Lock()
	if t.sig == nil {
		t.sig = sig
	}
	t.mu.Unlock()
}

func notifyTerminationSignals(parent context.Context) (ctx context.Context, stop func(), tracker *signalTracker) {
	sigCh := make(chan os.Signal, 1)
	signals := trappedTerminationSignals()
	if len(signals) > 0 {
		signal.Notify(sigCh, signals...)
	}
	return notifyTerminationSignalChannel(parent, sigCh, func() {
		signal.Stop(sigCh)
	})
}

func notifyTerminationSignalChannel(
	parent context.Context,
	sigCh <-chan os.Signal,
	stopNotify func(),
) (ctx context.Context, stop func(), tracker *signalTracker) {
	tracker = &signalTracker{}
	trackedParent := context.WithValue(parent, signalTrackerContextKey{}, tracker)
	trackedParent = runtime.ContextWithTerminationSignalLookup(trackedParent, tracker.Signal)

	ctx, cancel := context.WithCancel(trackedParent)
	stopCh := make(chan struct{})
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer close(done)
		select {
		case sig, ok := <-sigCh:
			if ok && sig != nil {
				// Restore the default OS signal handler once the first signal is
				// received so a second signal terminates immediately during cleanup.
				if stopNotify != nil {
					stopNotify()
				}
				tracker.record(sig)
				cancel()
			}
		case <-stopCh:
		case <-parent.Done():
		}
	}()

	stop = func() {
		stopOnce.Do(func() {
			if stopNotify != nil {
				stopNotify()
			}
			close(stopCh)
			cancel()
			<-done
		})
	}
	return ctx, stop, tracker
}

func signalTrackerFromContext(ctx context.Context) *signalTracker {
	if ctx == nil {
		return nil
	}
	tracker, _ := ctx.Value(signalTrackerContextKey{}).(*signalTracker)
	return tracker
}

func terminationSignalFromContext(ctx context.Context) os.Signal {
	if tracker := signalTrackerFromContext(ctx); tracker != nil {
		return tracker.Signal()
	}
	return runtime.TerminationSignalFromContext(ctx)
}

func isSignalCancelledContext(ctx context.Context, err error) bool {
	if err == nil || terminationSignalFromContext(ctx) == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return errors.Is(err, context.Canceled)
}

func isSignalHandledCancellation(tracker *signalTracker, err error) bool {
	if tracker == nil || tracker.Signal() == nil {
		return false
	}
	if _, ok := errors.AsType[*tuiSpinExitCodeError](err); ok { //nolint:errcheck // type match only; preserved error is handled by caller
		return false
	}
	return !errors.Is(err, context.DeadlineExceeded)
}

func mapSignalTerminationError(tracker *signalTracker, err error) error {
	if !isSignalHandledCancellation(tracker, err) {
		return err
	}
	code, ok := signalExitCode(tracker.Signal())
	if !ok {
		return err
	}
	return &ExitError{Code: code}
}
