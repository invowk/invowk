// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/pkg/types"

	"github.com/charmbracelet/x/xpty"
)

func runInteractiveCmd(ctx context.Context, opts tui.InteractiveOptions, cmd *exec.Cmd) (result *tui.InteractiveResult, err error) {
	width, height := tui.TerminalDimension(80), tui.TerminalDimension(24)
	if w, h, termErr := tui.TerminalSize(); termErr == nil {
		width, height = w, h
	}

	pty, err := startInteractivePty(cmd, width, height)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := pty.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	return tui.RunInteractiveSession(ctx, opts, pty, func(waitCtx context.Context) tui.InteractiveResult {
		startTime := time.Now()
		waitErr := xpty.WaitProcess(waitCtx, cmd)
		result := tui.InteractiveResult{Duration: time.Since(startTime)}
		if waitErr == nil {
			return result
		}
		if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
			result.ExitCode = validatedInteractiveExitCode(exitErr)
			return result
		}
		result.Error = waitErr
		result.ExitCode = types.ExitCode(1)
		return result
	})
}

func validatedInteractiveExitCode(exitErr *exec.ExitError) types.ExitCode {
	if exitErr == nil {
		return types.ExitCode(1)
	}
	exitCode := types.ExitCode(exitErr.ExitCode())
	if err := exitCode.Validate(); err != nil {
		return types.ExitCode(1)
	}
	return exitCode
}

// startInteractivePty creates the command's pseudo-terminal at the command
// area of a viewer drawn in a terminal of width x height, gives it to cmd as
// its terminal (on Unix also as its controlling terminal, in a new session:
// prepareInteractiveCmd) and starts cmd on it. The caller owns the returned
// PTY and closes it once the session ends.
func startInteractivePty(cmd *exec.Cmd, width, height tui.TerminalDimension) (xpty.Pty, error) {
	cols, rows := tui.InteractiveCommandArea(width, height)
	pty, err := xpty.NewPty(int(cols), int(rows))
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}
	if err := prepareInteractiveCmd(cmd, pty); err != nil {
		_ = pty.Close()
		return nil, err
	}
	if err := pty.Start(cmd); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("failed to start command on PTY: %w", err)
	}
	return pty, nil
}
