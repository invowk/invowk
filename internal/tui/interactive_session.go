// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
)

const (
	interruptedExitCode types.ExitCode = 130

	// interactiveOutputGrace is how long the waiter waits, after the command
	// ended, for the output reader to reach the end of the command's output
	// before it shows the completion banner (design D8). Processes the
	// command left running can keep the terminal open, and the Windows
	// pseudo-console's output does not end with the process, so the timer
	// is the normal path there.
	interactiveOutputGrace = 250 * time.Millisecond
)

// RunInteractiveSession renders an already-started interactive terminal
// session. Runtime adapters own PTY/process creation and waiting.
func RunInteractiveSession(
	ctx context.Context,
	opts InteractiveOptions,
	terminal InteractiveTerminal,
	wait InteractiveWaitFunc,
) (*InteractiveResult, error) {
	if terminal == nil {
		return nil, errors.New("interactive terminal is required")
	}
	if wait == nil {
		return nil, errors.New("interactive wait function is required")
	}

	m := newInteractiveModel(opts, terminal)
	// Ends the reply pump; the writer may stay blocked on input the command
	// never reads until the adapter closes the terminal.
	defer m.screen.close()

	programOpts := append([]tea.ProgramOption{tea.WithContext(ctx)}, opts.ProgramOptions...)
	p := tea.NewProgram(m, programOpts...)
	if opts.OnProgramReady != nil {
		opts.OnProgramReady(p)
	}

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		streamInteractiveOutput(p, terminal)
	}()
	go func() {
		result := wait(ctx)
		awaitOutputEnd(readerDone, interactiveOutputGrace)
		p.Send(doneMsg{result: result})
	}()

	finalModel, err := p.Run()
	if needsInteractiveStatusClear(finalModel, err) {
		clearInteractiveProgramStatus(opts)
	}
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}
	if im, ok := finalModel.(*interactiveModel); ok && im.result != nil {
		return im.result, nil
	}
	return &InteractiveResult{
		ExitCode: interruptedExitCode,
		Error:    errors.New("execution interrupted"),
	}, nil
}

// awaitOutputEnd returns once the output reader has ended or the grace
// period has passed. The reader's last Send happens before it ends, so all
// output it read before that is delivered before the completion message.
func awaitOutputEnd(readerDone <-chan struct{}, grace time.Duration) {
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-readerDone:
	case <-timer.C:
	}
}

func needsInteractiveStatusClear(finalModel tea.Model, runErr error) bool {
	if runErr != nil {
		return errors.Is(runErr, tea.ErrProgramKilled) || errors.Is(runErr, tea.ErrInterrupted)
	}
	im, ok := finalModel.(*interactiveModel)
	return !ok || !im.quitting
}

func clearInteractiveProgramStatus(opts InteractiveOptions) {
	if opts.StatusClearWriter != nil {
		writeClearProgramStatus(opts.StatusClearWriter)
		return
	}
	if tty, err := openInteractiveStatusTTY(); err == nil {
		writeClearProgramStatus(tty)
		_ = tty.Close()
		return
	}
	if isTerminalFile(os.Stdout) {
		writeClearProgramStatus(os.Stdout)
	}
}

// streamInteractiveOutput sends everything the command writes to its
// terminal to the model, in order, until the terminal read fails. Output
// read after completion is still sent: the model draws it above the banner.
func streamInteractiveOutput(program *tea.Program, terminal InteractiveTerminal) {
	buf := make([]byte, 4096)
	for {
		n, readErr := terminal.Read(buf)
		if n > 0 {
			program.Send(outputMsg{data: append([]byte(nil), buf[:n]...)})
		}
		if readErr != nil {
			program.Send(outputEndMsg{})
			return
		}
	}
}
