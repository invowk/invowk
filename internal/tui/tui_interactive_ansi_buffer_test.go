// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestANSIStreamBuffer_ImmediateCompleteAndSplitSequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chunks   []string
		wantOuts []string
		wantPend string
		wantSt   ansiParserState
	}{
		{
			name:     "plain ASCII flushes immediately",
			chunks:   []string{"hello world\r\n"},
			wantOuts: []string{"hello world\r\n"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "lone ESC held",
			chunks:   []string{"abc\x1b"},
			wantOuts: []string{"abc"},
			wantPend: "\x1b",
			wantSt:   ansiStateEscape,
		},
		{
			name:     "split 2-byte ESC sequence",
			chunks:   []string{"before\x1b", "7after"},
			wantOuts: []string{"before", "\x1b7after"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split intermediate ESC sequence",
			chunks:   []string{"a\x1b(", "Bb"},
			wantOuts: []string{"a", "\x1b(Bb"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split CSI SGR sequence",
			chunks:   []string{"prefix\x1b[3", "1mRED\x1b[0m"},
			wantOuts: []string{"prefix", "\x1b[31mRED\x1b[0m"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split C1 CSI sequence",
			chunks:   []string{"p\x9b3", "2;1m"},
			wantOuts: []string{"p", "\x9b32;1m"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split CSI intermediate sequence",
			chunks:   []string{"p\x1b[1 ", "qdone"},
			wantOuts: []string{"p", "\x1b[1 qdone"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split OSC with BEL",
			chunks:   []string{"a\x1b]0;my", " title\x07b"},
			wantOuts: []string{"a", "\x1b]0;my title\x07b"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split OSC with ESC backslash ST across ESC and backslash",
			chunks:   []string{"a\x1b]2;title\x1b", "\\b"},
			wantOuts: []string{"a", "\x1b]2;title\x1b\\b"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split C1 OSC with C1 ST and UTF-8 title",
			chunks:   []string{"a\x9d0;título 😀", "\x9cb"},
			wantOuts: []string{"a", "\x9d0;título 😀\x9cb"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split DCS with ST",
			chunks:   []string{"a\x1bP1$q", "data\x1b", "\\b"},
			wantOuts: []string{"a", "", "\x1bP1$qdata\x1b\\b"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "DCSEntry ESC passthrough splits across ESC and backslash ST",
			chunks:   []string{"a\x1bP\x1b", "[31m\x1b", "\\b"},
			wantOuts: []string{"a", "", "\x1bP\x1b[31m\x1b\\b"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split C1 DCS",
			chunks:   []string{"a\x90payload", "\x9cb"},
			wantOuts: []string{"a", "\x90payload\x9cb"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split SOS PM APC sequences",
			chunks:   []string{"1\x1bXsos\x1b", "\\2\x1b^pm", "\x9c3\x1b_apc\x1b", "\\4"},
			wantOuts: []string{"1", "\x1bXsos\x1b\\2", "\x1b^pm\x9c3", "\x1b_apc\x1b\\4"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split C1 SOS PM APC sequences",
			chunks:   []string{"1\x98s", "\x9c2\x9ep", "\x9c3\x9fa", "\x9c4"},
			wantOuts: []string{"1", "\x98s\x9c2", "\x9ep\x9c3", "\x9fa\x9c4"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "7-bit SS2 and SS3 hold target character",
			chunks:   []string{"a\x1bN", "xb\x1bO", "yc"},
			wantOuts: []string{"a", "\x1bNxb", "\x1bOyc"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "8-bit SS2 and SS3 hold UTF-8 target character",
			chunks:   []string{"a\x8e\xc3", "\xa9b\x8f\xe2\x82", "\xacc"},
			wantOuts: []string{"a", "\x8e\xc3\xa9b", "\x8f\xe2\x82\xacc"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split 2-byte UTF-8",
			chunks:   []string{"hi \xc3", "\xa9!"},
			wantOuts: []string{"hi ", "\xc3\xa9!"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split 3-byte UTF-8",
			chunks:   []string{"cost \xe2", "\x82", "\xac5"},
			wantOuts: []string{"cost ", "", "\xe2\x82\xac5"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "split 4-byte UTF-8",
			chunks:   []string{"go \xf0\x9f", "\x9a\x80!"},
			wantOuts: []string{"go ", "\xf0\x9f\x9a\x80!"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "interrupted CSI by new ESC then completed",
			chunks:   []string{"a\x1b[3", "\x1b[32mok"},
			wantOuts: []string{"a", "\x1b[3\x1b[32mok"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "interrupted OSC by CAN",
			chunks:   []string{"a\x1b]0;abc", "\x18ok"},
			wantOuts: []string{"a", "\x1b]0;abc\x18ok"},
			wantSt:   ansiStateGround,
		},
		{
			name:     "interrupted OSC by SUB",
			chunks:   []string{"a\x1b]0;abc", "\x1aok"},
			wantOuts: []string{"a", "\x1b]0;abc\x1aok"},
			wantSt:   ansiStateGround,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf ansiStreamBuffer
			for i, chunk := range tt.chunks {
				got := string(buf.push([]byte(chunk)))
				if got != tt.wantOuts[i] {
					t.Errorf("chunk %d output = %q, want %q", i, got, tt.wantOuts[i])
				}
			}
			if string(buf.buf) != tt.wantPend {
				t.Errorf("pending = %q, want %q", string(buf.buf), tt.wantPend)
			}
			if buf.state != tt.wantSt {
				t.Errorf("state = %s, want %s", buf.state, tt.wantSt)
			}
		})
	}
}

func TestANSIStreamBuffer_ByteByByteMatchesSingleWrite(t *testing.T) {
	t.Parallel()

	stream := strings.Join([]string{
		"plain ",
		"\x1b[38;2;255;100;0mcolor\x1b[0m ",
		"\x1b(0lqqk\x1b(B ",
		"\x1b]8;;https://invowk.io\x1b\\link\x1b]8;;\x07 ",
		"\x1bP1$qok\x1b\\ ",
		"\x1bP\x1b[33mpass\x1b\\ ",
		"\x1bXsos\x1b\\ \x1b^pm\x9c \x1b_apc\x1b\\ ",
		"\x8ea \x1bOb ",
		"café € 🚀",
	}, "")

	var buf ansiStreamBuffer
	var collected bytes.Buffer
	for i := range len(stream) {
		if out := buf.push([]byte{stream[i]}); len(out) > 0 {
			collected.Write(out)
		}
	}
	if buf.hasPending() {
		t.Fatalf("unexpected trailing pending bytes %q after complete stream", string(buf.buf))
	}
	if got := collected.String(); got != stream {
		t.Fatalf("byte-by-byte reconstructed output = %q, want %q", got, stream)
	}
	if buf.emuState != ansiStateGround {
		t.Fatalf("emuState = %s, want ground", buf.emuState)
	}
}

func TestANSIStreamBuffer_OverflowCapFlushAndRecovery(t *testing.T) {
	t.Parallel()

	var buf ansiStreamBuffer
	prefix := "lead"
	payload := "\x1b]0;" + strings.Repeat("x", maxEscapeHoldBytes)
	out := buf.push([]byte(prefix + payload))
	if string(out) != prefix+payload {
		t.Fatalf("overflow output len = %d, want %d", len(out), len(prefix+payload))
	}
	if buf.hasPending() {
		t.Fatalf("buffer still pending after overflow flush: %d bytes", len(buf.buf))
	}
	if buf.emuState != ansiStateOSCString {
		t.Fatalf("emuState = %s, want osc_string", buf.emuState)
	}

	recovery := buf.resetEmulatorState()
	if !bytes.Equal(recovery, []byte{asciiCAN}) {
		t.Fatalf("recovery = %q, want CAN", recovery)
	}
	if buf.emuState != ansiStateGround || buf.state != ansiStateGround {
		t.Fatalf("states after recovery = (%s, %s), want ground", buf.emuState, buf.state)
	}
}

func TestANSIStreamBuffer_FlushResetAndSubsequentChunks(t *testing.T) {
	t.Parallel()

	t.Run("forced flush followed by completing chunk returns to ground", func(t *testing.T) {
		t.Parallel()

		var buf ansiStreamBuffer
		if out := buf.push([]byte("\x1b[3")); len(out) != 0 {
			t.Fatalf("unexpected immediate flush: %q", out)
		}
		if flushed := string(buf.flush()); flushed != "\x1b[3" {
			t.Fatalf("flush() = %q, want %q", flushed, "\x1b[3")
		}
		if buf.emuState != ansiStateCSIParam {
			t.Fatalf("emuState = %s, want csi_param", buf.emuState)
		}

		// Completing bytes flush immediately and restore emuState to ground.
		if out := string(buf.push([]byte("1mRED"))); out != "1mRED" {
			t.Fatalf("push(completing) = %q, want %q", out, "1mRED")
		}
		if buf.emuState != ansiStateGround || buf.state != ansiStateGround {
			t.Fatalf("states = (%s, %s), want ground", buf.emuState, buf.state)
		}
	})

	t.Run("UTF-8 forced flush recovery sends triple CAN and resyncs pending tail", func(t *testing.T) {
		t.Parallel()

		var buf ansiStreamBuffer
		if out := buf.push([]byte("\xf0\x9f")); len(out) != 0 {
			t.Fatalf("unexpected flush: %q", out)
		}
		if flushed := buf.flush(); !bytes.Equal(flushed, []byte("\xf0\x9f")) {
			t.Fatalf("flush() = %q, want lead bytes", flushed)
		}
		if buf.emuState != ansiStateUTF8 {
			t.Fatalf("emuState = %s, want utf8", buf.emuState)
		}

		// A single continuation byte arrives while emu is still in Utf8State.
		if out := buf.push([]byte("\x9a")); len(out) != 0 {
			t.Fatalf("unexpected flush for partial continuation: %q", out)
		}

		// Recovery resets emulator to GroundState and re-evaluates buffered tail.
		recovery := buf.resetEmulatorState()
		if !bytes.Equal(recovery, []byte{asciiCAN, asciiCAN, asciiCAN}) {
			t.Fatalf("recovery = %q, want 3x CAN", recovery)
		}
		if buf.emuState != ansiStateGround {
			t.Fatalf("emuState = %s, want ground", buf.emuState)
		}
		// 0x9A in GroundState resolves back to GroundState.
		if buf.state != ansiStateGround {
			t.Fatalf("recomputed state = %s, want ground", buf.state)
		}
		if flushed := buf.flush(); !bytes.Equal(flushed, []byte("\x9a")) {
			t.Fatalf("subsequent flush() = %q, want 0x9a", flushed)
		}
	})

	t.Run("resetEmulatorState is no-op when already in ground", func(t *testing.T) {
		t.Parallel()

		var buf ansiStreamBuffer
		_ = buf.push([]byte("ok\x1b[3"))
		if got := buf.resetEmulatorState(); len(got) != 0 {
			t.Fatalf("resetEmulatorState() = %q, want nil while emu is ground", got)
		}
		if string(buf.buf) != "\x1b[3" {
			t.Fatalf("held command sequence = %q, want preserved", string(buf.buf))
		}
	})
}

func TestANSIParserState_StringAndValidate(t *testing.T) {
	t.Parallel()

	allStates := []ansiParserState{
		ansiStateGround,
		ansiStateEscape,
		ansiStateEscapeIntermediate,
		ansiStateCSIEntry,
		ansiStateCSIParam,
		ansiStateCSIIntermediate,
		ansiStateDCSEntry,
		ansiStateDCSParam,
		ansiStateDCSIntermediate,
		ansiStateDCSString,
		ansiStateOSCString,
		ansiStateSOSString,
		ansiStatePMString,
		ansiStateAPCString,
		ansiStateSingleShift,
		ansiStateUTF8,
	}
	for _, st := range allStates {
		if err := st.validate(); err != nil {
			t.Errorf("%s validate() unexpected error: %v", st, err)
		}
		if strings.HasPrefix(st.String(), "unknown(") {
			t.Errorf("state %d missing String() name: %q", st, st.String())
		}
	}

	invalid := ansiParserState(99)
	if got := invalid.String(); got != "unknown(99)" {
		t.Errorf("invalid String() = %q, want unknown(99)", got)
	}
	err := invalid.validate()
	if err == nil {
		t.Fatal("invalid validate() = nil, want error")
	}
	var stateErr *invalidANSIParserStateError
	if !errors.As(err, &stateErr) || stateErr.value != invalid || stateErr.Error() != "invalid ANSI parser state: 99" {
		t.Errorf("unexpected error = %v", err)
	}
}
