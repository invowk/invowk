// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

const screenTestTimeout = 2 * time.Second

// ansiSequencePattern matches the CSI and OSC sequences the emulator renders.
var ansiSequencePattern = regexp.MustCompile(`\x1b\[[0-9;:?<=>]*[\x20-\x2f]*[\x40-\x7e]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

type (
	// recordingTerminal records what the viewer writes to the command's
	// terminal. Its Read fails at once, as a terminal whose command ended.
	recordingTerminal struct {
		mu  sync.Mutex
		buf bytes.Buffer
	}

	// blockingTerminal never completes a Write until it is closed, as the
	// input of a command that never reads it.
	blockingTerminal struct {
		closed chan struct{}
		once   sync.Once
	}
)

func newRecordingTerminal() *recordingTerminal { return &recordingTerminal{} }

func (*recordingTerminal) Read([]byte) (int, error) { return 0, errors.New("closed") }

func (r *recordingTerminal) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

func (*recordingTerminal) Resize(int, int) error { return nil }

func (r *recordingTerminal) written() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// waitFor waits until the terminal received bytes that contain want.
func (r *recordingTerminal) waitFor(t *testing.T, want string) string {
	t.Helper()
	deadline := time.Now().Add(screenTestTimeout)
	for time.Now().Before(deadline) {
		if got := r.written(); strings.Contains(got, want) {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("terminal received %q, want it to contain %q", r.written(), want)
	return ""
}

func newBlockingTerminal() *blockingTerminal { return &blockingTerminal{closed: make(chan struct{})} }

func (*blockingTerminal) Read([]byte) (int, error) { return 0, errors.New("closed") }

func (b *blockingTerminal) Write([]byte) (int, error) {
	<-b.closed
	return 0, errors.New("closed")
}

func (*blockingTerminal) Resize(int, int) error { return nil }

func (b *blockingTerminal) Close() { b.once.Do(func() { close(b.closed) }) }

// newSizedInteractiveModel returns a model whose viewer is 80x24 (an 80x21
// command screen); its screen is closed at cleanup.
func newSizedInteractiveModel(t *testing.T, term InteractiveTerminal) *interactiveModel {
	t.Helper()
	model := newInteractiveModel(InteractiveOptions{Title: "T"}, term)
	t.Cleanup(model.screen.close)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return model
}

// stripANSI removes styling sequences from rendered text.
func stripANSI(s string) string {
	return ansiSequencePattern.ReplaceAllString(s, "")
}

// screenText returns the command screen as plain text.
func screenText(m *interactiveModel) string {
	return m.screen.emu.String()
}

func writeOutput(m *interactiveModel, chunks ...string) {
	for _, c := range chunks {
		m.Update(outputMsg{data: []byte(c)})
	}
}

func TestInteractiveScreenRepaintLeavesOneFrame(t *testing.T) {
	t.Parallel()

	model := newSizedInteractiveModel(t, nil)
	// An inline prompt draws two lines, then moves up and repaints them.
	writeOutput(model,
		"Pick one\r\n> alpha\r\n  beta",
		"\x1b[2A\r\x1b[J",
		"Pick one\r\n  alpha\r\n> beta",
	)
	text := screenText(model)
	if n := strings.Count(text, "Pick one"); n != 1 {
		t.Fatalf("screen has %d frames, want one:\n%s", n, text)
	}
	if !strings.Contains(text, "> beta") || strings.Contains(text, "> alpha") {
		t.Fatalf("screen does not show the repainted choice:\n%s", text)
	}
	if model.screen.emu.ScrollbackLen() != 0 {
		t.Fatalf("history holds %d lines, want none", model.screen.emu.ScrollbackLen())
	}
}

func TestInteractiveScreenCarriageReturnProgress(t *testing.T) {
	t.Parallel()

	model := newSizedInteractiveModel(t, nil)
	writeOutput(model, "step 1\rstep 2\rdone  \r\n")
	text := screenText(model)
	if strings.Contains(text, "step") || !strings.HasPrefix(text, "done") {
		t.Fatalf("screen = %q, want only done on the row", text)
	}
}

func TestInteractiveScreenSplitOSCDrawsNothing(t *testing.T) {
	t.Parallel()

	model := newSizedInteractiveModel(t, nil)
	writeOutput(model, "a\x1b]11;rgb:30", "30/0a0a/2424\x07b")
	if text := screenText(model); !strings.HasPrefix(text, "ab") || strings.Contains(text, "rgb") {
		t.Fatalf("screen = %q, want ab and no OSC text", text)
	}
}

func TestInteractiveScreenShowsInView(t *testing.T) {
	t.Parallel()

	model := newSizedInteractiveModel(t, nil)
	writeOutput(model, "\x1b[31mred text\x1b[0m\r\n")
	view := model.View()
	if !strings.Contains(view.Content, "red text") {
		t.Fatalf("view lacks the screen text:\n%s", view.Content)
	}
	lines := strings.Split(view.Content, "\n")
	if len(lines) != 24 {
		t.Fatalf("view has %d lines, want 24 (header, 21 screen rows, footer)", len(lines))
	}
	if view.Cursor == nil || view.Cursor.Y != interactiveHeaderRows+1 || view.Cursor.X != 0 {
		t.Fatalf("cursor = %+v, want column 0 of the second screen row", view.Cursor)
	}
	writeOutput(model, "\x1b[?25l")
	if model.View().Cursor != nil {
		t.Fatal("hidden cursor is still shown")
	}
}

func TestInteractiveScreenAnswersQueries(t *testing.T) {
	t.Parallel()

	t.Run("primary device attributes", func(t *testing.T) {
		t.Parallel()
		term := newRecordingTerminal()
		model := newSizedInteractiveModel(t, term)
		writeOutput(model, "\x1b[c")
		got := term.waitFor(t, "c")
		if !strings.HasPrefix(got, "\x1b[?") || !strings.HasSuffix(got, "c") {
			t.Fatalf("DA1 reply = %q, want ESC [ ? ... c", got)
		}
	})

	t.Run("cursor position", func(t *testing.T) {
		t.Parallel()
		term := newRecordingTerminal()
		model := newSizedInteractiveModel(t, term)
		writeOutput(model, "\x1b[2J\x1b[H\x1b[6n")
		if got := term.waitFor(t, "R"); got != "\x1b[1;1R" {
			t.Fatalf("CPR reply = %q, want ESC [ 1 ; 1 R", got)
		}
	})
}

// TestInteractiveScreenStaysResponsive: with a command that never reads its
// input, queries and keys neither block Update nor the force quit (spec
// "The viewer stays responsive when the command does not read its input").
func TestInteractiveScreenStaysResponsive(t *testing.T) {
	t.Parallel()

	term := newBlockingTerminal()
	model := newInteractiveModel(InteractiveOptions{}, term)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 10000 {
			model.Update(outputMsg{data: []byte("\x1b[c")})
		}
		for range 1000 {
			model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
		}
	}()
	select {
	case <-done:
	case <-time.After(screenTestTimeout):
		t.Fatal("queries and keys blocked Update")
	}
	if model.screen.dropped.Load() == 0 {
		t.Fatal("drop count = 0, want undeliverable input dropped")
	}
	_, cmd := model.Update(tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+\\ returned no command, want force quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl+\\ command is not tea.QuitMsg")
	}

	model.screen.close()
	select {
	case <-model.screen.pumpDone:
	case <-time.After(screenTestTimeout):
		t.Fatal("pump did not end with the session")
	}
	term.Close()
	select {
	case <-model.screen.writerDone:
	case <-time.After(screenTestTimeout):
		t.Fatal("writer did not end once the terminal closed")
	}
}

func TestInteractiveStyledTextOnScreen(t *testing.T) {
	t.Parallel()

	model := newSizedInteractiveModel(t, nil)
	responseCh := make(chan ComponentResponse, 1)
	model.handleTUIComponentRequest(TUIComponentMsg{
		Component:  ComponentTypeWrite,
		Options:    StyledTextOptions{Text: "line one\nline two"},
		ResponseCh: responseCh,
	})
	<-responseCh
	text := screenText(model)
	if !strings.HasPrefix(text, "line one\nline two") {
		t.Fatalf("screen = %q, want both lines at the left margin", text)
	}
}

// completedLines returns the full completed view content as plain lines.
func completedLines(m *interactiveModel) []string {
	return strings.Split(stripANSI(m.viewport.GetContent()), "\n")
}

func seqOutput(from, to int) string {
	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, "%d\r\n", i)
	}
	return b.String()
}

func indexOfLine(lines []string, want string) int {
	for i, l := range lines {
		if strings.TrimSpace(l) == want {
			return i
		}
	}
	return -1
}

func lastIndexOfLine(lines []string, want string) int {
	for i, l := range slices.Backward(lines) {
		if strings.TrimSpace(l) == want {
			return i
		}
	}
	return -1
}

func TestInteractiveCompletedHistory(t *testing.T) {
	t.Parallel()

	t.Run("seq 500", func(t *testing.T) {
		t.Parallel()
		model := newSizedInteractiveModel(t, nil)
		writeOutput(model, seqOutput(1, 500))
		model.Update(doneMsg{result: InteractiveResult{}})
		lines := completedLines(model)
		first, last := indexOfLine(lines, "1"), indexOfLine(lines, "500")
		banner := indexOfLine(lines, "COMPLETED SUCCESSFULLY")
		if first != 0 || last <= first || banner <= last {
			t.Fatalf("lines 1/500/banner at %d/%d/%d, want 0 < 500 < banner", first, last, banner)
		}
		if got := last - first; got != 499 {
			t.Fatalf("history spans %d lines between 1 and 500, want 499", got)
		}
		model.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
		if top := strings.TrimSpace(strings.Split(stripANSI(model.viewport.View()), "\n")[0]); top != "1" {
			t.Fatalf("top line after g = %q, want 1", top)
		}
	})

	t.Run("seq 20000 keeps 10000", func(t *testing.T) {
		t.Parallel()
		model := newSizedInteractiveModel(t, nil)
		writeOutput(model, seqOutput(1, 20000))
		if n := model.screen.emu.ScrollbackLen(); n != interactiveScrollbackLines {
			t.Fatalf("history holds %d lines, want %d", n, interactiveScrollbackLines)
		}
		model.Update(doneMsg{result: InteractiveResult{}})
		lines := completedLines(model)
		if indexOfLine(lines, "1") != -1 {
			t.Fatal("line 1 is still browsable past the history limit")
		}
		last, banner := indexOfLine(lines, "20000"), indexOfLine(lines, "COMPLETED SUCCESSFULLY")
		if last < 0 || banner <= last {
			t.Fatalf("line 20000 at %d, banner at %d, want 20000 above the banner", last, banner)
		}
	})

	t.Run("alternate screen absent", func(t *testing.T) {
		t.Parallel()
		model := newSizedInteractiveModel(t, nil)
		writeOutput(model, "before\r\n\x1b[?1049h", seqOutput(1, 100), "ALT-PAGE")
		model.Update(doneMsg{result: InteractiveResult{ExitCode: 1}})
		lines := completedLines(model)
		if indexOfLine(lines, "ALT-PAGE") != -1 || indexOfLine(lines, "50") != -1 {
			t.Fatalf("completed view holds alternate-screen content:\n%s", strings.Join(lines, "\n"))
		}
		if indexOfLine(lines, "before") != 0 || indexOfLine(lines, "EXITED WITH CODE 1") < 0 {
			t.Fatalf("completed view = %q, want main screen then banner", lines)
		}
	})
}

// TestInteractiveLateOutputAboveBanner: output after completion is drawn
// above the banner; the banner stays last, and the scroll offset is kept or
// the bottom followed (design D8).
func TestInteractiveLateOutputAboveBanner(t *testing.T) {
	t.Parallel()

	for _, scrolled := range []bool{false, true} {
		t.Run(fmt.Sprintf("scrolled_up=%t", scrolled), func(t *testing.T) {
			t.Parallel()
			model := newSizedInteractiveModel(t, nil)
			writeOutput(model, seqOutput(1, 100))
			model.Update(doneMsg{result: InteractiveResult{}})
			if scrolled {
				model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
			}
			offset := model.viewport.YOffset()
			_, cmd := model.Update(outputMsg{data: []byte("LATE-LINE\r\n")})
			if cmd == nil {
				t.Fatal("late output scheduled no rebuild")
			}
			// A second chunk in the same frame does not schedule another one.
			if _, again := model.Update(outputMsg{data: []byte("LATE-TWO\r\n")}); again != nil {
				t.Fatal("second late chunk scheduled a second rebuild")
			}
			model.Update(completedRebuildMsg{})
			lines := completedLines(model)
			late, banner := indexOfLine(lines, "LATE-TWO"), lastIndexOfLine(lines, "Press Enter to return to terminal...")
			if late < 0 || indexOfLine(lines, "COMPLETED SUCCESSFULLY") <= late {
				t.Fatalf("late output at %d, banner at %d, want it above the banner", late, banner)
			}
			if banner != len(lines)-2 {
				t.Fatalf("banner ends at %d of %d lines, want the banner last", banner, len(lines))
			}
			if scrolled && model.viewport.YOffset() != offset {
				t.Fatalf("offset = %d, want kept at %d", model.viewport.YOffset(), offset)
			}
			if !scrolled && !model.viewport.AtBottom() {
				t.Fatal("view left the bottom, want it to follow")
			}
		})
	}
}
