// SPDX-License-Identifier: EPL-2.0

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package tui

import (
	"golang.org/x/sys/unix"
)

// getTerminalSizeFromFd returns the terminal size for the given file descriptor.
func getTerminalSizeFromFd(fd int) (width, height int, err error) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}
