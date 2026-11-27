// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestWatchShutdownSignalChannel(t *testing.T) {
	t.Parallel()

	t.Run("SIGHUP sets preserveResume and cancels context", func(t *testing.T) {
		t.Parallel()

		sigCh := make(chan os.Signal, 1)
		ctx, stop, preserve := watchShutdownSignalChannel(t.Context(), sigCh)
		defer stop()

		sigCh <- syscall.SIGHUP

		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for watch context cancellation on SIGHUP")
		}
		if !preserve() {
			t.Fatal("preserveResume() = false after SIGHUP, want true")
		}
	})

	t.Run("SIGTERM sets preserveResume and cancels context", func(t *testing.T) {
		t.Parallel()

		sigCh := make(chan os.Signal, 1)
		ctx, stop, preserve := watchShutdownSignalChannel(t.Context(), sigCh)
		defer stop()

		sigCh <- syscall.SIGTERM

		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for watch context cancellation on SIGTERM")
		}
		if !preserve() {
			t.Fatal("preserveResume() = false after SIGTERM, want true")
		}
	})

	t.Run("parent cancel without SIGHUP/SIGTERM clears resume record", func(t *testing.T) {
		t.Parallel()

		sigCh := make(chan os.Signal, 1)
		ctx, cancelParent := context.WithCancel(t.Context())
		watchCtx, stop, preserve := watchShutdownSignalChannel(ctx, sigCh)
		defer stop()

		cancelParent()

		select {
		case <-watchCtx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for watch context cancellation")
		}
		stop()
		if preserve() {
			t.Fatal("preserveResume() = true on parent cancel, want false")
		}
	})
}
