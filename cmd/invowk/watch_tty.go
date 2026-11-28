// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"io"
	"os"
)

type (
	// WatchTerminalOpener resolves an interactive terminal writer for watch-mode
	// terminal protocol sequences (OSC 7577 and OSC 7501).
	WatchTerminalOpener interface {
		OpenWatchTerminal() (w io.Writer, cleanup func(), ok bool)
	}

	defaultWatchTerminalOpener struct {
		stdout io.Writer
	}
)

func (o defaultWatchTerminalOpener) OpenWatchTerminal() (io.Writer, func(), bool) {
	// Only probe the controlling TTY when stdout was left at the process
	// default (os.Stdout). Unit tests passing io.Discard or *bytes.Buffer
	// must never accidentally open the developer's /dev/tty.
	if f, ok := o.stdout.(*os.File); ok && f == os.Stdout {
		if tty, err := openControllingTerminal(); err == nil {
			return tty, func() { _ = tty.Close() }, true
		}
	}
	if stdoutIsTerminal(o.stdout) {
		return o.stdout, func() {}, true
	}
	return nil, func() {}, false
}
