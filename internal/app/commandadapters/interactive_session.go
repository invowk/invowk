// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/pkg/types"

	"github.com/charmbracelet/x/xpty"
)

// interactiveTerminal is the viewer's side of the command's terminal, which
// the adapter closes once the session ends.
type interactiveTerminal interface {
	tui.InteractiveTerminal
	Close() error
}

func runInteractiveCmd(ctx context.Context, opts tui.InteractiveOptions, cmd *exec.Cmd) (result *tui.InteractiveResult, err error) {
	width, height := tui.InteractiveTerminalSize()
	pty, err := startInteractivePty(cmd, width, height)
	if err != nil {
		return nil, err
	}
	terminal := viewerTerminal(pty)
	defer func() {
		// On Unix the slave copy was already closed after start, so the
		// second close of it reports os.ErrClosed, which is not a failure.
		if closeErr := terminal.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) && err == nil {
			err = closeErr
		}
	}()

	return tui.RunInteractiveSession(ctx, opts, terminal, func(waitCtx context.Context) tui.InteractiveResult {
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
	if err := releaseParentSlave(pty); err != nil {
		// The command already runs: end and reap it before giving up.
		_ = cmd.Process.Kill() //nolint:errcheck // best effort: the start already failed
		_ = cmd.Wait()         //nolint:errcheck // reaps the killed process; its status is moot
		_ = pty.Close()
		return nil, fmt.Errorf("failed to release the PTY slave: %w", err)
	}
	return pty, nil
}
