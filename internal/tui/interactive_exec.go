// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/xpty"
)

// RunInteractiveCmd executes a command in interactive mode with alternate screen buffer.
// It creates a PTY for the command, forwards keyboard input during execution,
// and allows output review after completion.
func RunInteractiveCmd(ctx context.Context, opts InteractiveOptions, cmd *exec.Cmd) (result *InteractiveResult, err error) {
	// Get terminal size for initial PTY dimensions
	width, height := 80, 24
	if w, h, termErr := getTerminalSize(); termErr == nil {
		width, height = w, h
	}

	// Create a PTY using xpty (cross-platform)
	pty, err := xpty.NewPty(width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}
	defer func() {
		if closeErr := pty.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Start the command on the PTY
	if err = pty.Start(cmd); err != nil {
		return nil, fmt.Errorf("failed to start command on PTY: %w", err)
	}

	// Create the model
	m := newInteractiveModel(opts, pty)

	// Create the Bubble Tea program; alt screen and mouse mode are declared in View().
	p := tea.NewProgram(m)

	// Notify the caller that the program is ready.
	// This allows them to access the program for terminal control.
	if opts.OnProgramReady != nil {
		opts.OnProgramReady(p)
	}

	// Read PTY output in a goroutine and send to the program
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := pty.Read(buf)
			if n > 0 {
				// Strip OSC sequences that don't function in the pager context
				// and appear as visual garbage when fragmented across buffers.
				content := stripOSCSequences(string(buf[:n]))
				if content != "" {
					p.Send(outputMsg{content: content})
				}
			}
			if readErr != nil {
				// Non-EOF errors are ignored; PTY read errors during command
				// execution are typically transient and don't warrant crashing
				break
			}
		}
	}()

	// Wait for the command to complete in a goroutine
	go func() {
		startTime := time.Now()
		waitErr := xpty.WaitProcess(ctx, cmd)
		duration := time.Since(startTime)

		result := InteractiveResult{
			Duration: duration,
		}

		if waitErr != nil {
			if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
				exitCode := types.ExitCode(exitErr.ExitCode())
				if validateErr := exitCode.Validate(); validateErr != nil {
					result.ExitCode = 1
				} else {
					result.ExitCode = exitCode
				}
			} else {
				result.Error = waitErr
				result.ExitCode = 1
			}
		}

		p.Send(doneMsg{result: result})
	}()

	// Run the TUI
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if im, ok := finalModel.(*interactiveModel); ok && im.result != nil {
		return im.result, nil
	}

	// If we get here without a result, the user force-quit
	return &InteractiveResult{
		ExitCode: 130, // Standard exit code for Ctrl+C
		Error:    fmt.Errorf("execution interrupted"),
	}, nil
}
