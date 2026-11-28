// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

// watchShutdownSignals listens for terminal close / restart signals (SIGHUP,
// SIGTERM) while watch mode runs. When parent already carries a root
// signalTracker, watchShutdownSignals observes parent cancellation directly so
// preserveResume never races between <-parent.Done() and a second signal.Notify
// channel. Receiving SIGHUP or SIGTERM sets preserveResume to true so watch mode
// exits cleanly without emitting OSC 7577 clear=1, allowing the terminal to
// resume it.
func watchShutdownSignals(parent context.Context) (ctx context.Context, stop func(), preserveResume func() bool) {
	if tracker := signalTrackerFromContext(parent); tracker != nil {
		return watchShutdownTrackedParent(parent, tracker)
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	ctx, stopInner, preserve := watchShutdownSignalChannel(parent, sigCh)
	var stopOnce sync.Once
	return ctx, func() {
		stopOnce.Do(func() {
			signal.Stop(sigCh)
			stopInner()
		})
	}, preserve
}

func watchShutdownTrackedParent(parent context.Context, tracker *signalTracker) (ctx context.Context, stop func(), preserveResume func() bool) {
	ctx, cancel := context.WithCancel(parent)
	var preserve atomic.Bool
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer close(done)
		<-ctx.Done()
		if isWatchPreserveSignal(tracker.Signal()) {
			preserve.Store(true)
		}
	}()

	stop = func() {
		stopOnce.Do(func() {
			cancel()
			<-done
		})
	}
	return ctx, stop, func() bool {
		return preserve.Load() || isWatchPreserveSignal(tracker.Signal())
	}
}

func watchShutdownSignalChannel(parent context.Context, sigCh <-chan os.Signal) (ctx context.Context, stop func(), preserveResume func() bool) {
	ctx, cancel := context.WithCancel(parent)
	stopCh := make(chan struct{})
	done := make(chan struct{})
	var preserve atomic.Bool
	var stopOnce sync.Once

	go func() {
		defer close(done)
		select {
		case <-sigCh:
			preserve.Store(true)
			cancel()
		case <-stopCh:
		case <-parent.Done():
			if isWatchPreserveSignal(terminationSignalFromContext(parent)) {
				preserve.Store(true)
			}
		}
	}()

	stop = func() {
		stopOnce.Do(func() {
			close(stopCh)
			cancel()
			<-done
		})
	}
	return ctx, stop, func() bool {
		return preserve.Load() || isWatchPreserveSignal(terminationSignalFromContext(parent))
	}
}
