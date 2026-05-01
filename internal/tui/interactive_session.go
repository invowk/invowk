// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
)

const interruptedExitCode types.ExitCode = 130

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
	p := tea.NewProgram(m, tea.WithContext(ctx))
	if opts.OnProgramReady != nil {
		opts.OnProgramReady(p)
	}

	go streamInteractiveOutput(p, terminal)
	go func() {
		p.Send(doneMsg{result: wait(ctx)})
	}()

	finalModel, err := p.Run()
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

func streamInteractiveOutput(program *tea.Program, terminal InteractiveTerminal) {
	buf := make([]byte, 4096)
	for {
		n, readErr := terminal.Read(buf)
		if n > 0 {
			content := stripOSCSequences(string(buf[:n]))
			if content != "" {
				program.Send(outputMsg{content: content})
			}
		}
		if readErr != nil {
			break
		}
	}
}
