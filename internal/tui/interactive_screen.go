// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/vt"
)

const (
	// interactiveScrollbackLines is the history kept above the command's
	// screen: lines that scroll off its top (vt's default, set explicitly).
	interactiveScrollbackLines = vt.DefaultScrollbackSize
	// interactiveInputQueueBytes bounds the replies and keys waiting for the
	// command to read its input. What does not fit is dropped (design D5).
	interactiveInputQueueBytes = 64 << 10
	// interactivePasteQueueBytes bounds the pastes waiting for the command to
	// read its input. Each paste is queued or dropped whole; one paste is
	// always accepted when no other one waits, whatever its size.
	interactivePasteQueueBytes = 1 << 20
	// interactiveInputQueueChunks bounds the number of queued chunks.
	interactiveInputQueueChunks = 1024
	// interactiveInputReadSize is the pump's read size.
	interactiveInputReadSize = 4096
)

type (
	// emulatedScreen is the terminal screen the viewer shows for the command:
	// a vt emulator plus the two goroutines that carry its replies and the
	// encoded keys to the command's terminal without ever blocking the
	// Bubble Tea event loop (design D5).
	//
	// All emulator calls happen on the event loop; the pump only reads the
	// emulator's input pipe, which vt allows concurrently.
	emulatedScreen struct {
		emu *vt.Emulator
		// input is the emulator's input pipe writer; closing it ends the pump.
		input io.WriteCloser

		// pasting is set while paste collects the bytes of one paste: the
		// pump then appends what it reads to pasteBuf instead of queuing it.
		pasting  atomic.Bool
		pasteMu  sync.Mutex
		pasteBuf bytes.Buffer
		// closed is set when the session ended: the writer then stops, so
		// it never writes to a terminal the adapter is closing.
		closed atomic.Bool

		queue chan queuedInput
		// queued and pasteQueued count the bytes of replies and keys, and of
		// pastes, in queue; dropped counts the input bytes dropped because
		// the queue was full.
		queued      atomic.Int64
		pasteQueued atomic.Int64
		dropped     atomic.Int64
		pumpDone    chan struct{}
		writerDone  chan struct{}
	}

	// queuedInput is one chunk waiting for the writer: replies and keys, or
	// one whole paste.
	queuedInput struct {
		data  []byte
		paste bool
	}
)

// newEmulatedScreen creates the emulator at width x height and starts the
// reply pump and the writer, which writes queued input to sink (non-nil).
func newEmulatedScreen(width, height TerminalDimension, sink io.Writer) *emulatedScreen {
	emu := vt.NewEmulator(max(int(width), 1), max(int(height), 1))
	emu.SetScrollbackSize(interactiveScrollbackLines)
	s := &emulatedScreen{
		emu:        emu,
		queue:      make(chan queuedInput, interactiveInputQueueChunks),
		pumpDone:   make(chan struct{}),
		writerDone: make(chan struct{}),
	}
	if pipe, ok := emu.InputPipe().(io.WriteCloser); ok {
		s.input = pipe
	}
	go s.pump()
	go s.write(sink)
	return s
}

// pump moves everything the emulator writes to its input (query replies,
// keys, paste) into the bounded queue. It never waits on anything but the
// emulator's pipe, so the emulator call that produced the bytes returns at
// once; a chunk that does not fit is dropped and counted.
func (s *emulatedScreen) pump() {
	defer close(s.pumpDone)
	defer close(s.queue)
	buf := make([]byte, interactiveInputReadSize)
	for {
		n, err := s.emu.Read(buf)
		if n > 0 {
			if s.pasting.Load() {
				s.pasteMu.Lock()
				s.pasteBuf.Write(buf[:n])
				s.pasteMu.Unlock()
			} else {
				s.enqueue(queuedInput{data: append([]byte(nil), buf[:n]...)})
			}
		}
		if err != nil {
			return
		}
	}
}

// enqueue hands a chunk to the writer without blocking, or drops and counts
// it when its part of the queue is full: replies and keys share one byte
// bound, pastes another, so a waiting paste never makes keys drop.
func (s *emulatedScreen) enqueue(in queuedInput) {
	size := int64(len(in.data))
	counter, limit := &s.queued, int64(interactiveInputQueueBytes)
	if in.paste {
		counter, limit = &s.pasteQueued, interactivePasteQueueBytes
	}
	waiting := counter.Load()
	if waiting+size > limit && (!in.paste || waiting > 0) {
		s.dropped.Add(size)
		return
	}
	counter.Add(size)
	select {
	case s.queue <- in:
	default:
		counter.Add(-size)
		s.dropped.Add(size)
	}
}

// paste forwards pasted text, bracketed when the command enabled bracketed
// paste, as one chunk that is queued or dropped whole: a partial paste could
// lose its end marker and leave the command in paste mode. An empty write to
// the input pipe returns only once the pump called Read again, that is, once
// it handled every byte written before; so the first one keeps earlier keys
// out of the paste and the second one ends the collection after the paste's
// last byte. The pump never blocks meanwhile (design D5).
func (s *emulatedScreen) paste(msg tea.PasteMsg) {
	if s.input == nil {
		s.emu.Paste(msg.Content)
		return
	}
	_, _ = s.input.Write(nil) //nolint:errcheck // fails only once the session ended
	s.pasting.Store(true)
	s.emu.Paste(msg.Content)
	_, _ = s.input.Write(nil) //nolint:errcheck // fails only once the session ended
	s.pasting.Store(false)

	s.pasteMu.Lock()
	chunk := bytes.Clone(s.pasteBuf.Bytes())
	s.pasteBuf.Reset()
	s.pasteMu.Unlock()
	if len(chunk) > 0 && !s.closed.Load() {
		s.enqueue(queuedInput{data: chunk, paste: true})
	}
}

// write delivers queued chunks to the command's terminal. It is the only
// party that blocks when the command does not read its input. It ends when
// the queue is closed and drained, at the first failed write, or before the
// next write once the session ended.
func (s *emulatedScreen) write(sink io.Writer) {
	defer close(s.writerDone)
	for in := range s.queue {
		if in.paste {
			s.pasteQueued.Add(-int64(len(in.data)))
		} else {
			s.queued.Add(-int64(len(in.data)))
		}
		if s.closed.Load() {
			return
		}
		if _, err := sink.Write(in.data); err != nil {
			return
		}
	}
}

// close ends the pump by closing the emulator's input pipe, and stops the
// writer before its next write, since the adapter closes the terminal next
// and a Windows pseudo-console write has no guard against a closed handle.
// It never calls Emulator.Close, whose unguarded field the pump reads.
// Later replies fail inside vt with io.ErrClosedPipe, which vt ignores. The
// writer is not waited for: it may be blocked on input the command never
// reads.
func (s *emulatedScreen) close() {
	s.closed.Store(true)
	if s.input != nil {
		_ = s.input.Close()
	}
}
