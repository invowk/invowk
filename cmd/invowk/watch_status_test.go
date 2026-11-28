// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/watch"
	"github.com/invowk/invowk/pkg/types"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestWatchStatusReporter_LifecycleAndIsolation(t *testing.T) {
	t.Parallel()

	t.Run("emits working idle error and clear to TTY writer", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		closed := false
		app := &App{
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
				return &termBuf, func() { closed = true }, true
			}),
		}

		r := newWatchStatusReporter(app, "build\napp")
		// Clear before any emit must be a no-op so outer status isn't wiped on setup errors.
		r.Clear()
		if termBuf.Len() != 0 {
			t.Fatalf("Clear() before emit wrote %q, want empty", termBuf.String())
		}

		r.Working()
		r.Error()
		r.Working()
		r.Idle()
		r.Clear()
		r.Clear()
		r.Close()
		r.Close()

		if !closed {
			t.Fatal("Close() did not run cleanup")
		}

		wantWorking := ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateWorking, Title: "build app", Message: "Running...",
		}))
		wantError := ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateError, Title: "build app", Message: "Command failed",
		}))
		wantIdle := ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateIdle, Title: "build app", Message: "Watching for changes",
		}))

		got := termBuf.String()
		want := wantWorking + wantError + wantWorking + wantIdle + ansi.ClearProgramStatus
		if got != want {
			t.Fatalf("status sequence = %q, want %q", got, want)
		}
	})

	t.Run("emits nothing on non-TTY buffer or pipe os.File", func(t *testing.T) {
		t.Parallel()

		var stdoutBuf bytes.Buffer
		app := &App{stdout: &stdoutBuf}
		r := newWatchStatusReporter(app, "build")
		r.Working()
		r.Idle()
		r.Error()
		r.Clear()
		r.Close()

		if stdoutBuf.Len() != 0 {
			t.Fatalf("non-TTY writer received %q", stdoutBuf.String())
		}

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			_ = pr.Close()
			_ = pw.Close()
		})

		pipeApp := &App{stdout: pw}
		pipeReporter := newWatchStatusReporter(pipeApp, "build")
		pipeReporter.Working()
		pipeReporter.Clear()
		pipeReporter.Close()
		if closeErr := pw.Close(); closeErr != nil {
			t.Fatalf("pw.Close() error = %v", closeErr)
		}
		pipeBytes, err := io.ReadAll(pr)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		if len(pipeBytes) != 0 {
			t.Fatalf("non-TTY os.File pipe received %q", string(pipeBytes))
		}
	})

	t.Run("is safe under concurrent emits and Close", func(t *testing.T) {
		t.Parallel()

		var termBuf syncBuffer
		app := &App{
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
				return &termBuf, func() {}, true
			}),
		}
		r := newWatchStatusReporter(app, "serve")

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				r.Working()
				r.Idle()
				r.Error()
			})
		}
		wg.Wait()
		r.Clear()
		r.Close()

		if !strings.HasSuffix(termBuf.String(), ansi.ClearProgramStatus) {
			t.Fatalf("missing trailing clear sequence in %q", termBuf.String())
		}
	})
}

func TestRunWatchMode_ProgramStatusTransitions(t *testing.T) {
	t.Parallel()

	t.Run("working error working idle clear", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		commands := &fakeWatchCommandService{
			cmdInfo:   newResolvedWatchCommand(t),
			exitCodes: []types.ExitCode{1, 0},
		}
		watchers := &fakeWatchFactory{
			run: func(ctx context.Context, cfg watch.Config) error {
				return cfg.OnChange(ctx, []string{"main.go"})
			},
		}
		app := &App{
			Config:        &staticConfigProvider{cfg: config.DefaultConfig()},
			Commands:      commands,
			Watchers:      watchers,
			Diagnostics:   &defaultDiagnosticRenderer{},
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) { return &termBuf, func() {}, true }),
			stdout:        &bytes.Buffer{},
			stderr:        &bytes.Buffer{},
		}

		if err := runWatchMode(newWatchTestCmd(t), app, &rootFlagValues{}, &cmdFlagValues{}, []string{"build"}); err != nil {
			t.Fatalf("runWatchMode() error = %v", err)
		}

		want := ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateWorking, Title: "build", Message: "Running...",
		})) + ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateError, Title: "build", Message: "Command failed",
		})) + ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateWorking, Title: "build", Message: "Running...",
		})) + ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateIdle, Title: "build", Message: "Watching for changes",
		})) + ansi.ClearProgramStatus

		if got := termBuf.String(); got != want {
			t.Fatalf("status sequence = %q, want %q", got, want)
		}
	})

	t.Run("fatal change handler error skips idle or error flash before clear", func(t *testing.T) {
		t.Parallel()

		infraErr := errors.New("broken pipe")
		var termBuf bytes.Buffer
		commands := &fakeWatchCommandService{
			cmdInfo:     newResolvedWatchCommand(t),
			executeErrs: []error{nil, infraErr, infraErr, infraErr},
		}
		watchers := &fakeWatchFactory{
			run: func(ctx context.Context, cfg watch.Config) error {
				for range 3 {
					if err := cfg.OnChange(ctx, []string{"main.go"}); err != nil {
						return err
					}
				}
				return nil
			},
		}
		app := &App{
			Config:        &staticConfigProvider{cfg: config.DefaultConfig()},
			Commands:      commands,
			Watchers:      watchers,
			Diagnostics:   &defaultDiagnosticRenderer{},
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) { return &termBuf, func() {}, true }),
			stdout:        &bytes.Buffer{},
			stderr:        &bytes.Buffer{},
		}

		err := runWatchMode(newWatchTestCmd(t), app, &rootFlagValues{}, &cmdFlagValues{}, []string{"build"})
		if !errors.Is(err, infraErr) {
			t.Fatalf("runWatchMode() error = %v, want %v", err, infraErr)
		}

		workingSeq := ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateWorking, Title: "build", Message: "Running...",
		}))
		errorSeq := ansi.SetProgramStatus(*tui.NormalizeProgramStatus(ansi.ProgramStatus{
			State: ansi.ProgramStateError, Title: "build", Message: "Command failed",
		}))
		got := termBuf.String()
		if !strings.HasSuffix(got, workingSeq+ansi.ClearProgramStatus) {
			t.Fatalf("expected working followed directly by clear on fatal exit; got %q (errorSeq=%q)", got, errorSeq)
		}
	})
}
