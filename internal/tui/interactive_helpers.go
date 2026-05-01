// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"os"
	"regexp"
)

// oscSequenceRe matches all OSC (Operating System Command) escape sequences.
// OSC sequences are used for terminal features like window titles, hyperlinks,
// color queries, clipboard operations, etc.
//
// In the context of invowk's interactive pager, these sequences don't function
// (hyperlinks aren't clickable, window titles don't apply, etc.) and can appear
// as visual garbage when fragmented across PTY read buffers.
//
// Format: ESC ] <content> <terminator>
// Terminators: BEL (\x07), ST (\x1b\\), or backslash alone (\)
//
// Also matches partial/fragmented sequences where leading ESC was consumed.
var oscSequenceRe = regexp.MustCompile(
	`\x1b\][^\x07\x1b\\]*(?:\x07|\x1b\\|\\)` + // Full OSC: ESC ] ... terminator
		`|` +
		`\][^\x07\x1b\\]*(?:\x07|\x1b\\|\\)`, // Partial: ] ... terminator (missing ESC)
)

// stripOSCSequences removes all OSC escape sequences from output.
// In invowk's interactive pager context, OSC features don't function anyway
// (the pager is a text viewport, not a full terminal emulator), and fragmented
// sequences appear as visual garbage.
func stripOSCSequences(s string) string {
	return oscSequenceRe.ReplaceAllString(s, "")
}

func newInteractiveModel(opts InteractiveOptions, terminal InteractiveTerminal) *interactiveModel {
	return &interactiveModel{
		title:    opts.Title,
		cmdName:  opts.CommandName,
		state:    stateExecuting,
		terminal: terminal,
	}
}

// getTerminalSize attempts to get the current terminal size.
func getTerminalSize() (width, height int, err error) {
	// Try to get size from stdout
	fd := int(os.Stdout.Fd())
	width, height, err = getTerminalSizeFromFd(fd)
	if err == nil {
		return width, height, nil
	}

	// Fallback: try stderr
	fd = int(os.Stderr.Fd())
	width, height, err = getTerminalSizeFromFd(fd)
	if err == nil {
		return width, height, nil
	}

	// Fallback: try stdin
	fd = int(os.Stdin.Fd())
	return getTerminalSizeFromFd(fd)
}

// TerminalSize returns the current terminal size for runtime adapters that
// prepare interactive terminal sessions.
func TerminalSize() (width, height TerminalDimension, err error) {
	w, h, err := getTerminalSize()
	return TerminalDimension(w), TerminalDimension(h), err //goplint:ignore -- getTerminalSize returns non-negative terminal dimensions from the OS.
}
