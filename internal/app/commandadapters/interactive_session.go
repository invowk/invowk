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
	width, height := 80, 24
	if w, h, termErr := tui.TerminalSize(); termErr == nil {
		width, height = int(w), int(h)
	}

	pty, err := xpty.NewPty(width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}
	defer func() {
		if closeErr := pty.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err = pty.Start(cmd); err != nil {
		return nil, fmt.Errorf("failed to start command on PTY: %w", err)
	}

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
