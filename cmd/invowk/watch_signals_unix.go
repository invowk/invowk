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
// SIGTERM) while watch mode runs. Unlike SIGINT (handled via fang's context),
// receiving SIGHUP or SIGTERM sets preserveResume to true so watch mode exits
// cleanly without emitting OSC 7577 clear=1, allowing the terminal to resume it.
func watchShutdownSignals(parent context.Context) (ctx context.Context, stop func(), preserveResume func() bool) {
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
		}
	}()

	stop = func() {
		stopOnce.Do(func() {
			close(stopCh)
			cancel()
			<-done
		})
	}
	return ctx, stop, preserve.Load
}
