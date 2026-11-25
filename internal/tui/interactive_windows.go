// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"golang.org/x/sys/windows"
)

// getTerminalSizeFromFd returns the terminal size for the given file
// descriptor, a console handle on Windows. It uses the handle directly: an
// os.File wrapping it would close the process's standard handle when the
// garbage collector finalizes that wrapper.
func getTerminalSizeFromFd(fd int) (width, height int, err error) {
	handle := windows.Handle(uintptr(fd))
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(handle, &info); err != nil {
		return 0, 0, err
	}
	width = int(info.Window.Right - info.Window.Left + 1)
	height = int(info.Window.Bottom - info.Window.Top + 1)
	return width, height, nil
}
