// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"
	"os"
)

const (
	// interactiveHeaderRows is the viewer's title line plus its separator.
	interactiveHeaderRows = 2
	// interactiveFooterRows is the viewer's status line.
	interactiveFooterRows = 1

	// interactiveDefaultWidth and interactiveDefaultHeight are the size
	// assumed when the terminal's size cannot be read.
	interactiveDefaultWidth  TerminalDimension = 80
	interactiveDefaultHeight TerminalDimension = 24
)

// InteractiveCommandArea returns the size of the command area of an
// interactive viewer drawn in a terminal of the given size: the full width,
// and the height without the viewer's header and footer rows (at least one
// row). Runtime adapters create the command's terminal at this size, so the
// command sees no size change when the viewer draws its first frame.
func InteractiveCommandArea(width, height TerminalDimension) (cols, rows TerminalDimension) {
	return width, max(height-interactiveHeaderRows-interactiveFooterRows, 1)
}

// InteractiveTerminalSize returns the size of the terminal the interactive
// viewer draws in, or 80x24 when it cannot be read. The runtime adapter
// sizes the command's PTY and the viewer sizes its emulated screen from it,
// so both start at the same command area.
func InteractiveTerminalSize() (width, height TerminalDimension) {
	if w, h, err := TerminalSize(); err == nil && w > 0 && h > 0 {
		return w, h
	}
	return interactiveDefaultWidth, interactiveDefaultHeight
}

// newInteractiveModel creates the viewer model with its emulated screen at
// the command area of InteractiveTerminalSize, the size the runtime adapter
// gave the command's PTY. The screen writes replies and keys to terminal
// (discarded when terminal is nil).
func newInteractiveModel(opts InteractiveOptions, terminal InteractiveTerminal) *interactiveModel {
	cols, rows := InteractiveCommandArea(InteractiveTerminalSize())
	sink := io.Discard
	if terminal != nil {
		sink = terminal
	}
	return &interactiveModel{
		screen:   newEmulatedScreen(cols, rows, sink),
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
