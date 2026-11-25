// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

const sessionTestTimeout = 10 * time.Second

type (
	// scriptedTerminal delivers the chunks sent on its channel to the
	// session's reader and ends the reader when the channel is closed.
	scriptedTerminal struct {
		chunks chan string
		// ended is closed when Read reported the end of the output.
		ended   chan struct{}
		endOnce sync.Once
	}

	// sessionProbe runs RunInteractiveSession without a terminal and records
	// the order in which the model received output and completion.
	sessionProbe struct {
		mu       sync.Mutex
		events   []string
		model    *interactiveModel
		doneSeen chan struct{}
		rebuilt  chan struct{}
		// sized is closed when the program delivers its first window size.
		sized    chan struct{}
		doneOnce sync.Once
		rebOnce  sync.Once
		sizeOnce sync.Once
		program  *tea.Program
		ready    chan struct{}
	}
)

func newScriptedTerminal() *scriptedTerminal {
	return &scriptedTerminal{chunks: make(chan string, 1024), ended: make(chan struct{})}
}

func (s *scriptedTerminal) Read(p []byte) (int, error) {
	chunk, ok := <-s.chunks
	if !ok {
		s.endOnce.Do(func() { close(s.ended) })
		return 0, io.EOF
	}
	return copy(p, chunk), nil
}

func (*scriptedTerminal) Write(p []byte) (int, error) { return len(p), nil }

func (*scriptedTerminal) Resize(int, int) error { return nil }

func newSessionProbe() *sessionProbe {
	return &sessionProbe{
		doneSeen: make(chan struct{}),
		rebuilt:  make(chan struct{}),
		sized:    make(chan struct{}),
		ready:    make(chan struct{}),
	}
}

func (p *sessionProbe) filter(m tea.Model, msg tea.Msg) tea.Msg {
	p.mu.Lock()
	defer p.mu.Unlock()
	if im, ok := m.(*interactiveModel); ok {
		p.model = im
	}
	switch msg := msg.(type) {
	case outputMsg:
		p.events = append(p.events, "output:"+string(msg.data))
	case doneMsg:
		p.events = append(p.events, "done")
		p.doneOnce.Do(func() { close(p.doneSeen) })
	case completedRebuildMsg:
		p.rebOnce.Do(func() { close(p.rebuilt) })
	case tea.WindowSizeMsg:
		p.sizeOnce.Do(func() { close(p.sized) })
	}
	return msg
}

func (p *sessionProbe) options() InteractiveOptions {
	return InteractiveOptions{
		OnProgramReady: func(prog *tea.Program) {
			p.program = prog
			close(p.ready)
		},
		ProgramOptions: []tea.ProgramOption{
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(80, 24),
			tea.WithFilter(p.filter),
		},
	}
}

// run starts the session and returns a function that, once the test let it
// reach its end, presses Enter and waits for the session to return.
func (p *sessionProbe) run(t *testing.T, term InteractiveTerminal, wait InteractiveWaitFunc) func(after <-chan struct{}) *InteractiveResult {
	t.Helper()
	type outcome struct {
		result *InteractiveResult
		err    error
	}
	out := make(chan outcome, 1)
	go func() {
		r, err := RunInteractiveSession(t.Context(), p.options(), term, wait)
		out <- outcome{r, err}
	}()
	return func(after <-chan struct{}) *InteractiveResult {
		t.Helper()
		select {
		case <-after:
		case <-time.After(sessionTestTimeout):
			t.Fatalf("session did not reach the awaited point; events %q", p.eventList())
		}
		<-p.ready
		p.program.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
		select {
		case o := <-out:
			if o.err != nil {
				t.Fatalf("RunInteractiveSession() error = %v", o.err)
			}
			return o.result
		case <-time.After(sessionTestTimeout):
			t.Fatal("session did not return after Enter")
			return nil
		}
	}
}

func (p *sessionProbe) eventList() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.events...)
}

func (p *sessionProbe) completedContent() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return strings.Split(stripANSI(p.model.viewport.GetContent()), "\n")
}

func immediateWait(context.Context) InteractiveResult { return InteractiveResult{} }

// TestInteractiveSessionOutputOrder: everything the reader read reaches the
// model in order, and the completion message comes after the reader's end
// (it replaces the old concurrent-writes test: output is now applied on the
// event loop only).
func TestInteractiveSessionOutputOrder(t *testing.T) {
	t.Parallel()

	term := newScriptedTerminal()
	for i := range 200 {
		term.chunks <- fmt.Sprintf("line %d\r\n", i)
	}
	close(term.chunks)
	probe := newSessionProbe()
	finish := probe.run(t, term, func(context.Context) InteractiveResult {
		<-term.ended
		return InteractiveResult{}
	})
	finish(probe.doneSeen)

	events := probe.eventList()
	if len(events) != 201 || events[200] != "done" {
		t.Fatalf("got %d events ending with %q, want 200 outputs then done", len(events), events[len(events)-1])
	}
	for i := range 200 {
		if want := fmt.Sprintf("output:line %d\r\n", i); events[i] != want {
			t.Fatalf("event %d = %q, want %q", i, events[i], want)
		}
	}
}

// TestInteractiveSessionLastChunkWithinGrace: a last chunk that arrives
// after the command ended, but within the grace period, precedes the banner.
func TestInteractiveSessionLastChunkWithinGrace(t *testing.T) {
	t.Parallel()

	term := newScriptedTerminal()
	term.chunks <- "first\r\n"
	probe := newSessionProbe()
	waited := make(chan struct{})
	finish := probe.run(t, term, func(context.Context) InteractiveResult {
		// The command ends once the viewer is laid out, so the completed
		// view is built when the banner arrives.
		<-probe.sized
		close(waited)
		return InteractiveResult{}
	})
	go func() {
		// No sleep: the chunk follows the command's end with the whole
		// grace period as its margin, so a slow scheduler cannot reorder it.
		<-waited
		term.chunks <- "LAST-LINE\r\n"
		close(term.chunks)
	}()
	finish(probe.doneSeen)

	events := probe.eventList()
	if len(events) < 2 || events[len(events)-1] != "done" || events[len(events)-2] != "output:LAST-LINE\r\n" {
		t.Fatalf("events = %q, want LAST-LINE right before done", events)
	}
	lines := probe.completedContent()
	if last, banner := indexOfLine(lines, "LAST-LINE"), indexOfLine(lines, "COMPLETED SUCCESSFULLY"); last < 0 || banner <= last {
		t.Fatalf("LAST-LINE at %d, banner at %d, want it above the banner", last, banner)
	}
}

// TestInteractiveSessionLateOutputAfterBanner: output that arrives after the
// banner is drawn above it, and the banner stays last.
func TestInteractiveSessionLateOutputAfterBanner(t *testing.T) {
	t.Parallel()

	term := newScriptedTerminal()
	term.chunks <- "early\r\n"
	probe := newSessionProbe()
	finish := probe.run(t, term, immediateWait)
	go func() {
		<-probe.doneSeen
		term.chunks <- "LATE-LINE\r\n"
		close(term.chunks)
	}()
	finish(probe.rebuilt)

	lines := probe.completedContent()
	late, banner := indexOfLine(lines, "LATE-LINE"), indexOfLine(lines, "COMPLETED SUCCESSFULLY")
	if late < 0 || banner <= late {
		t.Fatalf("LATE-LINE at %d, banner at %d, want it above the banner:\n%s", late, banner, strings.Join(lines, "\n"))
	}
	if end := lastIndexOfLine(lines, "Press Enter to return to terminal..."); end != len(lines)-2 {
		t.Fatalf("banner ends at %d of %d lines, want the banner last", end, len(lines))
	}
}

// TestInteractiveSessionReaderNeverEnds: a terminal whose output never ends
// (a process left holding it, the Windows pseudo-console) gets the banner
// after the grace period.
func TestInteractiveSessionReaderNeverEnds(t *testing.T) {
	t.Parallel()

	term := newScriptedTerminal()
	t.Cleanup(func() { close(term.chunks) })
	probe := newSessionProbe()
	waitedAt := make(chan time.Time, 1)
	finish := probe.run(t, term, func(context.Context) InteractiveResult {
		waitedAt <- time.Now()
		return InteractiveResult{}
	})
	<-probe.doneSeen
	elapsed := time.Since(<-waitedAt)
	result := finish(probe.doneSeen)
	if elapsed < interactiveOutputGrace/2 {
		t.Fatalf("banner after %s, want it to wait for the grace period", elapsed)
	}
	if result == nil || result.ExitCode != 0 {
		t.Fatalf("result = %+v, want success", result)
	}
}
