// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"os"

	"golang.org/x/sys/windows"
)

// getTerminalSizeFromFd returns the terminal size for the given file descriptor.
func getTerminalSizeFromFd(fd int) (width, height int, err error) {
	handle := windows.Handle(os.NewFile(uintptr(fd), "").Fd())
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(handle, &info); err != nil {
		return 0, 0, err
	}
	width = int(info.Window.Right - info.Window.Left + 1)
	height = int(info.Window.Bottom - info.Window.Top + 1)
	return width, height, nil
}
