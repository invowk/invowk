// SPDX-License-Identifier: MPL-2.0

package tui

import "fmt"

const (
	// maxEscapeHoldBytes caps the bytes buffered while waiting for an unclosed
	// escape or control-string sequence to finish, matching
	// interactiveInputQueueBytes so pathological output never grows unboundedly.
	maxEscapeHoldBytes = 64 << 10

	// sgrResetSequence resets graphic rendition attributes around delegated
	// styled writes so neither stream bleeds styling into the other.
	sgrResetSequence = "\x1b[0m"

	asciiBEL      byte = 0x07
	asciiBS       byte = 0x08
	asciiCR       byte = 0x0d
	asciiCAN      byte = 0x18
	asciiSUB      byte = 0x1a
	asciiESC      byte = 0x1b
	asciiSpace    byte = 0x20
	asciiInterMin byte = 0x20
	asciiInterMax byte = 0x2f
	asciiParamMin byte = 0x30
	asciiParamMax byte = 0x3f
	asciiFinalMin byte = 0x40
	asciiFinalMax byte = 0x7e
	asciiDEL      byte = 0x7f
	c1Min         byte = 0x80
	c1SS2         byte = 0x8e
	c1SS3         byte = 0x8f
	c1DCS         byte = 0x90
	c1SOS         byte = 0x98
	c1CSI         byte = 0x9b
	c1ST          byte = 0x9c
	c1OSC         byte = 0x9d
	c1PM          byte = 0x9e
	c1APC         byte = 0x9f
	utf82Min      byte = 0xc2
	utf82Max      byte = 0xdf
	utf83Min      byte = 0xe0
	utf83Max      byte = 0xef
	utf84Min      byte = 0xf0
	utf84Max      byte = 0xf4

	utf8Rem2Bytes = 1
	utf8Rem3Bytes = 2
	utf8Rem4Bytes = 3

	ansiStateGround             ansiParserState = 0
	ansiStateEscape             ansiParserState = 1
	ansiStateEscapeIntermediate ansiParserState = 2
	ansiStateCSIEntry           ansiParserState = 3
	ansiStateCSIParam           ansiParserState = 4
	ansiStateCSIIntermediate    ansiParserState = 5
	ansiStateDCSEntry           ansiParserState = 6
	ansiStateDCSParam           ansiParserState = 7
	ansiStateDCSIntermediate    ansiParserState = 8
	ansiStateDCSString          ansiParserState = 9
	ansiStateOSCString          ansiParserState = 10
	ansiStateSOSString          ansiParserState = 11
	ansiStatePMString           ansiParserState = 12
	ansiStateAPCString          ansiParserState = 13
	ansiStateSingleShift        ansiParserState = 14
	ansiStateUTF8               ansiParserState = 15
)

type (
	// ansiParserState tracks DEC ANSI and UTF-8 parser state aligned with
	// charmbracelet/x/vt across PTY reads and emulator writes.
	ansiParserState int

	// invalidANSIParserStateError reports an unrecognized ansiParserState.
	invalidANSIParserStateError struct {
		value ansiParserState
	}

	// ansiStreamBuffer holds incomplete trailing ANSI/VT escape sequences and
	// multi-byte UTF-8 runes across PTY chunks while flushing complete prefixes
	// immediately at GroundState.
	//
	// Invariant: whenever buf is empty, (state, utf8Remaining) equals
	// (emuState, emuUTF8Remaining); whenever buf is non-empty, (state,
	// utf8Remaining) is scanANSIState(emuState, emuUTF8Remaining, buf).
	//
	//goplint:ignore -- low-level ANSI byte-stream buffer operating on raw PTY octets and parser counters.
	ansiStreamBuffer struct {
		buf              []byte
		state            ansiParserState
		utf8Remaining    int
		emuState         ansiParserState
		emuUTF8Remaining int
		seq              uint64
	}
)

// String returns the human-readable name of the parser state.
func (s ansiParserState) String() string {
	switch s {
	case ansiStateGround:
		return "ground"
	case ansiStateEscape:
		return "escape"
	case ansiStateEscapeIntermediate:
		return "escape_intermediate"
	case ansiStateCSIEntry:
		return "csi_entry"
	case ansiStateCSIParam:
		return "csi_param"
	case ansiStateCSIIntermediate:
		return "csi_intermediate"
	case ansiStateDCSEntry:
		return "dcs_entry"
	case ansiStateDCSParam:
		return "dcs_param"
	case ansiStateDCSIntermediate:
		return "dcs_intermediate"
	case ansiStateDCSString:
		return "dcs_string"
	case ansiStateOSCString:
		return "osc_string"
	case ansiStateSOSString:
		return "sos_string"
	case ansiStatePMString:
		return "pm_string"
	case ansiStateAPCString:
		return "apc_string"
	case ansiStateSingleShift:
		return "single_shift"
	case ansiStateUTF8:
		return "utf8"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

func (s ansiParserState) validate() error {
	switch s {
	case ansiStateGround,
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
		ansiStateUTF8:
		return nil
	default:
		return &invalidANSIParserStateError{value: s}
	}
}

func (e *invalidANSIParserStateError) Error() string {
	return fmt.Sprintf("invalid ANSI parser state: %d", int(e.value))
}

// push appends data, scans for the furthest GroundState boundary, and returns
// the complete prefix ready to write to the emulator. Any trailing incomplete
// sequence remains buffered unless buf exceeds maxEscapeHoldBytes, in which
// case everything is flushed and emuState records the resulting non-ground
// state in the emulator.
//
//goplint:ignore -- raw PTY byte slice boundary scanner.
func (b *ansiStreamBuffer) push(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	start := len(b.buf)
	b.buf = append(b.buf, data...)

	lastSafeEnd := 0
	for i := start; i < len(b.buf); i++ {
		b.state, b.utf8Remaining = advanceANSIState(b.state, b.utf8Remaining, b.buf[i])
		if b.state == ansiStateGround {
			lastSafeEnd = i + 1
		}
	}

	var ready []byte
	if lastSafeEnd > 0 {
		ready = append([]byte(nil), b.buf[:lastSafeEnd]...)
		remaining := copy(b.buf, b.buf[lastSafeEnd:])
		b.buf = b.buf[:remaining]
		b.emuState = ansiStateGround
		b.emuUTF8Remaining = 0
	}

	if len(b.buf) > maxEscapeHoldBytes {
		ready = append(ready, b.buf...)
		b.buf = b.buf[:0]
		b.emuState = b.state
		b.emuUTF8Remaining = b.utf8Remaining
		b.seq++
	}

	return ready
}

// flush returns all currently buffered bytes and records the parser state left
// inside the emulator after those bytes are written.
//
//goplint:ignore -- returns raw terminal byte slice buffered for emulator flush.
func (b *ansiStreamBuffer) flush() []byte {
	if len(b.buf) == 0 {
		return nil
	}
	pending := append([]byte(nil), b.buf...)
	b.buf = b.buf[:0]
	b.emuState = b.state
	b.emuUTF8Remaining = b.utf8Remaining
	b.seq++
	return pending
}

func (b *ansiStreamBuffer) hasPending() bool {
	return len(b.buf) > 0
}

//goplint:ignore -- monotonic internal timer generation counter.
func (b *ansiStreamBuffer) nextFlushSeq() uint64 {
	b.seq++
	return b.seq
}

// resetEmulatorState returns the recovery bytes needed to restore the emulator
// to GroundState when a prior forced flush left it mid-sequence, without
// touching any still-buffered incomplete command sequence in buf.
//
//goplint:ignore -- returns raw CAN recovery bytes for the terminal emulator.
func (b *ansiStreamBuffer) resetEmulatorState() []byte {
	if b.emuState == ansiStateGround {
		return nil
	}
	recovery := groundRecoveryBytes(b.emuState)
	b.emuState = ansiStateGround
	b.emuUTF8Remaining = 0
	b.state, b.utf8Remaining = scanANSIState(ansiStateGround, 0, b.buf)
	return recovery
}

//goplint:ignore -- raw ANSI byte scanner state helper.
func scanANSIState(start ansiParserState, utf8Rem int, data []byte) (endState ansiParserState, endRem int) {
	endState, endRem = start, utf8Rem
	for _, c := range data {
		endState, endRem = advanceANSIState(endState, endRem, c)
	}
	return endState, endRem
}

// isolateStyledText wraps body with SGR resets and neutralizes any trailing
// incomplete escape or UTF-8 state inside body before the closing reset so
// neither stream bleeds styling or parser state into the other.
//
//goplint:ignore -- operates on raw ANSI rendering string and byte slice.
func isolateStyledText(body string) []byte {
	endState, _ := scanANSIState(ansiStateGround, 0, []byte(body))
	recovery := groundRecoveryBytes(endState)

	out := make([]byte, 0, len(sgrResetSequence)+len(body)+len(recovery)+len(sgrResetSequence))
	out = append(out, sgrResetSequence...)
	out = append(out, body...)
	out = append(out, recovery...)
	out = append(out, sgrResetSequence...)
	return out
}

//goplint:ignore -- returns raw CAN recovery bytes.
func groundRecoveryBytes(state ansiParserState) []byte {
	switch state {
	case ansiStateGround:
		return nil
	case ansiStateUTF8:
		// Up to 3 continuation bytes can remain for a 4-byte UTF-8 rune; 3 CAN
		// bytes complete the rune as U+FFFD and any extra CAN bytes are ignored
		// once back in GroundState.
		return []byte{asciiCAN, asciiCAN, asciiCAN}
	case ansiStateEscape,
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
		ansiStateSingleShift:
		return []byte{asciiCAN}
	default:
		return []byte{asciiCAN}
	}
}

//goplint:ignore -- DEC ANSI state transition operates on raw stream bytes and UTF-8 remainder counts.
func advanceANSIState(state ansiParserState, utf8Rem int, c byte) (nextState ansiParserState, nextRem int) {
	switch state {
	case ansiStateUTF8:
		if utf8Rem > 1 {
			return ansiStateUTF8, utf8Rem - 1
		}
		return ansiStateGround, 0

	case ansiStateOSCString:
		switch c {
		case asciiESC:
			return ansiStateEscape, 0
		case asciiBEL, asciiCAN, asciiSUB, c1ST:
			return ansiStateGround, 0
		default:
			return ansiStateOSCString, 0
		}

	case ansiStateDCSString:
		switch c {
		case asciiESC:
			return ansiStateEscape, 0
		case asciiCAN, asciiSUB, c1ST:
			return ansiStateGround, 0
		default:
			return ansiStateDCSString, 0
		}

	case ansiStateSOSString, ansiStatePMString, ansiStateAPCString:
		if isAnywhereControl(c) {
			return transitionAnywhere(c)
		}
		return state, 0

	case ansiStateDCSEntry:
		if c == asciiESC {
			// Screen/tmux DCS passthrough enters DCSString on ESC directly
			// from DCSEntry (matching charmbracelet/x/ansi transition_table).
			return ansiStateDCSString, 0
		}
		if isAnywhereControl(c) {
			return transitionAnywhere(c)
		}
		return advanceDCSEntry(c), 0

	case ansiStateGround,
		ansiStateEscape,
		ansiStateEscapeIntermediate,
		ansiStateCSIEntry,
		ansiStateCSIParam,
		ansiStateCSIIntermediate,
		ansiStateDCSParam,
		ansiStateDCSIntermediate,
		ansiStateSingleShift:
		if isAnywhereControl(c) {
			return transitionAnywhere(c)
		}
		return advanceStateByte(state, c), 0

	default:
		return ansiStateGround, 0
	}
}

//goplint:ignore -- raw byte classifier for DEC ANSI anywhere-control range.
func isAnywhereControl(c byte) bool {
	return c == asciiCAN || c == asciiSUB || c == asciiESC || c >= c1Min
}

//goplint:ignore -- DEC ANSI anywhere-control transition on raw byte.
func transitionAnywhere(c byte) (nextState ansiParserState, nextRem int) {
	switch {
	case c == asciiESC:
		return ansiStateEscape, 0
	case c == c1SS2, c == c1SS3:
		return ansiStateSingleShift, 0
	case c == c1DCS:
		return ansiStateDCSEntry, 0
	case c == c1SOS:
		return ansiStateSOSString, 0
	case c == c1CSI:
		return ansiStateCSIEntry, 0
	case c == c1OSC:
		return ansiStateOSCString, 0
	case c == c1PM:
		return ansiStatePMString, 0
	case c == c1APC:
		return ansiStateAPCString, 0
	case c >= utf82Min && c <= utf82Max:
		return ansiStateUTF8, utf8Rem2Bytes
	case c >= utf83Min && c <= utf83Max:
		return ansiStateUTF8, utf8Rem3Bytes
	case c >= utf84Min && c <= utf84Max:
		return ansiStateUTF8, utf8Rem4Bytes
	default:
		return ansiStateGround, 0
	}
}

//goplint:ignore -- DEC ANSI state transition on raw byte.
func advanceStateByte(state ansiParserState, c byte) ansiParserState {
	switch state {
	case ansiStateGround:
		return ansiStateGround
	case ansiStateEscape:
		return advanceEscape(c)
	case ansiStateEscapeIntermediate:
		if c >= asciiParamMin && c <= asciiFinalMax {
			return ansiStateGround
		}
		return ansiStateEscapeIntermediate
	case ansiStateCSIEntry:
		return advanceCSIEntry(c)
	case ansiStateCSIParam:
		return advanceCSIParam(c)
	case ansiStateCSIIntermediate:
		if c >= asciiParamMin && c <= asciiFinalMax {
			return ansiStateGround
		}
		return ansiStateCSIIntermediate
	case ansiStateDCSParam:
		return advanceDCSParam(c)
	case ansiStateDCSIntermediate:
		if c >= asciiParamMin && c <= asciiFinalMax {
			return ansiStateDCSString
		}
		return ansiStateDCSIntermediate
	case ansiStateSingleShift:
		if c >= asciiSpace && c <= asciiFinalMax {
			return ansiStateGround
		}
		return ansiStateSingleShift
	case ansiStateDCSEntry,
		ansiStateDCSString,
		ansiStateOSCString,
		ansiStateSOSString,
		ansiStatePMString,
		ansiStateAPCString,
		ansiStateUTF8:
		return state
	default:
		return ansiStateGround
	}
}

//goplint:ignore -- DEC ANSI escape byte transition.
func advanceEscape(c byte) ansiParserState {
	if c >= asciiInterMin && c <= asciiInterMax {
		return ansiStateEscapeIntermediate
	}
	switch c {
	case '[':
		return ansiStateCSIEntry
	case ']':
		return ansiStateOSCString
	case 'P':
		return ansiStateDCSEntry
	case 'X':
		return ansiStateSOSString
	case '^':
		return ansiStatePMString
	case '_':
		return ansiStateAPCString
	case 'N', 'O':
		return ansiStateSingleShift
	}
	if c >= asciiParamMin && c <= asciiFinalMax {
		return ansiStateGround
	}
	return ansiStateEscape
}

//goplint:ignore -- DEC ANSI CSI entry byte transition.
func advanceCSIEntry(c byte) ansiParserState {
	if c >= asciiInterMin && c <= asciiInterMax {
		return ansiStateCSIIntermediate
	}
	if c >= asciiParamMin && c <= asciiParamMax {
		return ansiStateCSIParam
	}
	if c >= asciiFinalMin && c <= asciiFinalMax {
		return ansiStateGround
	}
	return ansiStateCSIEntry
}

//goplint:ignore -- DEC ANSI CSI param byte transition.
func advanceCSIParam(c byte) ansiParserState {
	if c >= asciiInterMin && c <= asciiInterMax {
		return ansiStateCSIIntermediate
	}
	if c >= asciiFinalMin && c <= asciiFinalMax {
		return ansiStateGround
	}
	return ansiStateCSIParam
}

//goplint:ignore -- DEC ANSI DCS entry byte transition.
func advanceDCSEntry(c byte) ansiParserState {
	if c >= asciiBS && c <= asciiCR {
		return ansiStateDCSString
	}
	if c >= asciiInterMin && c <= asciiInterMax {
		return ansiStateDCSIntermediate
	}
	if c >= asciiParamMin && c <= asciiParamMax {
		return ansiStateDCSParam
	}
	if c >= asciiFinalMin && c <= asciiFinalMax {
		return ansiStateDCSString
	}
	return ansiStateDCSEntry
}

//goplint:ignore -- DEC ANSI DCS param byte transition.
func advanceDCSParam(c byte) ansiParserState {
	if c >= asciiInterMin && c <= asciiInterMax {
		return ansiStateDCSIntermediate
	}
	if c >= asciiFinalMin && c <= asciiFinalMax {
		return ansiStateDCSString
	}
	return ansiStateDCSParam
}
