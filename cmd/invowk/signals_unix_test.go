// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/pkg/types"
)

func TestSignalExitCode_UnixMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sig  os.Signal
		want types.ExitCode
	}{
		{name: "SIGHUP maps to 129", sig: syscall.SIGHUP, want: 129},
		{name: "SIGINT maps to 130", sig: os.Interrupt, want: 130},
		{name: "SIGTERM maps to 143", sig: syscall.SIGTERM, want: 143},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sigCh := make(chan os.Signal, 1)
			sigCtx, stop, tracker := notifyTerminationSignalChannel(t.Context(), sigCh, nil)
			defer stop()

			var stdout, stderr bytes.Buffer
			app, err := NewApp(Dependencies{
				Config: &staticConfigProvider{cfg: config.DefaultConfig()},
				Stdout: &stdout,
				Stderr: &stderr,
				Getenv: func(string) string { return "" },
			})
			if err != nil {
				t.Fatalf("NewApp() error = %v", err)
			}

			rootCmd := NewRootCommand(app)
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			started := make(chan struct{})
			rootCmd.AddCommand(&cobra.Command{
				Use: "wait",
				RunE: func(cmd *cobra.Command, _ []string) error {
					close(started)
					<-cmd.Context().Done()
					return cmd.Context().Err()
				},
			})
			rootCmd.SetArgs([]string{"wait"})

			go func() {
				<-started
				sigCh <- tt.sig
			}()

			runErr := fang.Execute(
				sigCtx,
				rootCmd,
				fang.WithErrorHandler(newFangErrorHandlerWithTracker(app, rootCmd, tracker)),
			)
			mapped := mapSignalTerminationError(tracker, runErr)
			if got := exitCodeForError(mapped); got != tt.want {
				t.Fatalf("exitCodeForError() = %d, want %d (err=%v)", got, tt.want, mapped)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty (no redundant ERROR box)", stderr.String())
			}
		})
	}
}

func TestSignalExitCode_ExceptionsAndTimeouts(t *testing.T) {
	t.Parallel()

	t.Run("tui spin keeps exit code 1 on signal", func(t *testing.T) {
		t.Parallel()

		sigCh := make(chan os.Signal, 1)
		sigCtx, stop, tracker := notifyTerminationSignalChannel(t.Context(), sigCh, nil)
		defer stop()

		var stdout, stderr bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &stdout,
			Stderr: &stderr,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		rootCmd := NewRootCommand(app)
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		sigCh <- syscall.SIGTERM
		select {
		case <-sigCtx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for signal context cancellation")
		}

		spinErr := &tuiSpinExitCodeError{err: tui.ErrSpinActionStillRunning}
		handler := newFangErrorHandlerWithTracker(app, rootCmd, tracker)
		handler(&stderr, fang.Styles{}, spinErr)
		if !strings.Contains(stderr.String(), "did not stop") {
			t.Fatalf("stderr = %q, want spin still-running warning", stderr.String())
		}

		mapped := mapSignalTerminationError(tracker, spinErr)
		if got := exitCodeForError(mapped); got != 1 {
			t.Fatalf("exitCodeForError(spinErr) = %d, want 1", got)
		}
		if !errors.Is(mapped, tui.ErrSpinActionStillRunning) {
			t.Fatalf("mapped error = %v, want ErrSpinActionStillRunning", mapped)
		}
	})

	t.Run("explicit deadline exceeded keeps exit code 1", func(t *testing.T) {
		t.Parallel()

		sigCh := make(chan os.Signal, 1)
		sigCtx, stop, tracker := notifyTerminationSignalChannel(t.Context(), sigCh, nil)
		defer stop()

		var stdout, stderr bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &stdout,
			Stderr: &stderr,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		rootCmd := NewRootCommand(app)
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		_ = sigCtx

		deadlineErr := context.DeadlineExceeded
		handler := newFangErrorHandlerWithTracker(app, rootCmd, tracker)
		handler(&stderr, fang.Styles{}, deadlineErr)
		if !strings.Contains(stderr.String(), "context deadline exceeded") {
			t.Fatalf("stderr = %q, want deadline exceeded message", stderr.String())
		}

		mapped := mapSignalTerminationError(tracker, deadlineErr)
		if got := exitCodeForError(mapped); got != 1 {
			t.Fatalf("exitCodeForError(deadlineErr) = %d, want 1", got)
		}
	})

	t.Run("verbose prints single plain signal exit status line", func(t *testing.T) {
		t.Parallel()

		sigCh := make(chan os.Signal, 1)
		sigCtx, stop, tracker := notifyTerminationSignalChannel(t.Context(), sigCh, nil)
		defer stop()

		sigCh <- syscall.SIGTERM
		select {
		case <-sigCtx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for signal context cancellation")
		}

		var stdout, stderr bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &stdout,
			Stderr: &stderr,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		rootCmd := NewRootCommand(app)
		if err := rootCmd.PersistentFlags().Set("ivk-verbose", "true"); err != nil {
			t.Fatalf("Set(ivk-verbose) error = %v", err)
		}
		handler := newFangErrorHandlerWithTracker(app, rootCmd, tracker)
		handler(&stderr, fang.Styles{}, context.Canceled)

		if got := stderr.String(); got != "invowk: command exited with status 143\n" {
			t.Fatalf("stderr = %q, want single status 143 line", got)
		}
	})
}
