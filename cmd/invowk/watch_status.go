// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"io"
	"sync"

	"github.com/charmbracelet/x/ansi"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type watchStatusReporter struct {
	enabled    bool
	closed     bool
	reported   bool
	mu         sync.Mutex
	writer     io.Writer
	cleanup    func()
	closeOnce  sync.Once
	baseStatus ansi.ProgramStatus
}

func newWatchStatusReporter(app *App, cmdName invowkfile.CommandName) *watchStatusReporter {
	writer, cleanup, ok := openAppWatchTerminal(app)
	if !ok || writer == nil {
		if cleanup != nil {
			cleanup()
		}
		return &watchStatusReporter{}
	}
	return &watchStatusReporter{
		enabled: true,
		writer:  writer,
		cleanup: cleanup,
		baseStatus: ansi.ProgramStatus{
			Title: string(cmdName),
		},
	}
}

func (r *watchStatusReporter) Working() {
	if r == nil || !r.enabled {
		return
	}
	st := r.baseStatus
	st.State = ansi.ProgramStateWorking
	st.Message = "Running..."
	r.writeStatus(st)
}

func (r *watchStatusReporter) Idle() {
	if r == nil || !r.enabled {
		return
	}
	st := r.baseStatus
	st.State = ansi.ProgramStateIdle
	st.Message = "Watching for changes"
	r.writeStatus(st)
}

func (r *watchStatusReporter) Error() {
	if r == nil || !r.enabled {
		return
	}
	st := r.baseStatus
	st.State = ansi.ProgramStateError
	st.Message = "Command failed"
	r.writeStatus(st)
}

func (r *watchStatusReporter) Clear() {
	if r == nil || !r.enabled {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || !r.reported {
		return
	}
	if _, err := io.WriteString(r.writer, ansi.ClearProgramStatus); err != nil {
		return
	}
	r.reported = false
}

func (r *watchStatusReporter) Close() {
	if r == nil || r.cleanup == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.closeOnce.Do(r.cleanup)
}

func (r *watchStatusReporter) writeStatus(status ansi.ProgramStatus) {
	normalized := tui.NormalizeProgramStatus(status)
	if normalized == nil {
		return
	}
	seq := ansi.SetProgramStatus(*normalized)
	if seq == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	if _, err := io.WriteString(r.writer, seq); err != nil {
		return
	}
	r.reported = true
}
