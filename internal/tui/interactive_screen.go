// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"
	"sync/atomic"

	"github.com/charmbracelet/x/vt"
)

const (
	// interactiveScrollbackLines is the history kept above the command's
	// screen: lines that scroll off its top (vt's default, set explicitly).
	interactiveScrollbackLines = vt.DefaultScrollbackSize
	// interactiveInputQueueBytes bounds the replies and keys waiting for the
	// command to read its input. What does not fit is dropped (design D5).
	interactiveInputQueueBytes = 64 << 10
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
		input io.Closer

		queue chan []byte
		// queued counts the bytes in queue; dropped counts the input bytes
		// dropped because the queue was full.
		queued     atomic.Int64
		dropped    atomic.Int64
		pumpDone   chan struct{}
		writerDone chan struct{}
	}
)

// newEmulatedScreen creates the emulator at width x height and starts the
// reply pump and the writer, which writes queued input to sink.
func newEmulatedScreen(width, height TerminalDimension, sink io.Writer) *emulatedScreen {
	emu := vt.NewEmulator(max(int(width), 1), max(int(height), 1))
	emu.SetScrollbackSize(interactiveScrollbackLines)
	s := &emulatedScreen{
		emu:        emu,
		queue:      make(chan []byte, interactiveInputQueueChunks),
		pumpDone:   make(chan struct{}),
		writerDone: make(chan struct{}),
	}
	if closer, ok := emu.InputPipe().(io.Closer); ok {
		s.input = closer
	}
	if sink == nil {
		sink = io.Discard
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
	for {
		buf := make([]byte, interactiveInputReadSize)
		n, err := s.emu.Read(buf)
		if n > 0 {
			size := int64(n)
			if s.queued.Load()+size > interactiveInputQueueBytes {
				s.dropped.Add(size)
			} else {
				select {
				case s.queue <- buf[:n]:
					s.queued.Add(size)
				default:
					s.dropped.Add(size)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// write delivers queued chunks to the command's terminal. It is the only
// party that blocks when the command does not read its input. It ends when
// the queue is closed and drained, or at the first failed write.
func (s *emulatedScreen) write(sink io.Writer) {
	defer close(s.writerDone)
	for chunk := range s.queue {
		s.queued.Add(-int64(len(chunk)))
		if _, err := sink.Write(chunk); err != nil {
			return
		}
	}
}

// close ends the pump by closing the emulator's input pipe. It never calls
// Emulator.Close, whose unguarded field the pump reads. Later replies fail
// inside vt with io.ErrClosedPipe, which vt ignores. The writer is not
// waited for: it may be blocked on input the command never reads.
func (s *emulatedScreen) close() {
	if s.input != nil {
		_ = s.input.Close()
	}
}
