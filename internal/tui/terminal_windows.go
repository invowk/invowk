// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// platformConsoleProbe returns the Windows console check: GetConsoleMode fails
// on a handle that is not a console, for example when no console is attached.
func platformConsoleProbe() consoleProbe {
	return windowsConsoleProbe
}

// windowsConsoleProbe reports an error when f is not a console handle.
func windowsConsoleProbe(f *os.File) error {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil {
		return fmt.Errorf("GetConsoleMode: %w", err)
	}
	return nil
}

// platformTerminalOpener opens CONIN$ and CONOUT$. Unlike Bubble Tea's
// OpenTTY, it closes CONIN$ when CONOUT$ cannot be opened.
func platformTerminalOpener() (in, out *os.File, err error) {
	in, err = os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open CONIN$: %w", err)
	}
	out, err = os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		// Release the half-opened console.
		return nil, nil, errors.Join(fmt.Errorf("open CONOUT$: %w", err), in.Close())
	}
	return in, out, nil
}
