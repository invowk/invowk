// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"
)

// platformConsoleProbe returns nil on Unix: an opened /dev/tty is the terminal.
func platformConsoleProbe() consoleProbe {
	return nil
}

// platformTerminalReleaser closes /dev/tty. Bubble Tea's Unix input reader is
// cancellable, so no read is pending once the program has exited.
func platformTerminalReleaser(in, out *os.File) func() error {
	return closeTerminalFiles(in, out)
}

// platformTerminalOpener opens /dev/tty (one file used for input and output).
func platformTerminalOpener() (in, out *os.File, err error) {
	return tea.OpenTTY()
}
