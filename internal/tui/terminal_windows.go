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

// platformTerminalOpener opens the console. Keys come from stdin when stdin is
// a console: Bubble Tea can cancel a read only on the stdin handle, so input
// from a separately opened CONIN$ would leave a read pending after the program
// exits. Otherwise it opens CONIN$. Output always goes to CONOUT$. Unlike
// Bubble Tea's OpenTTY, it closes CONIN$ when CONOUT$ cannot be opened.
// When stdin is chosen, openProbedTerminal probes it a second time; that is
// one extra GetConsoleMode call, kept so the opener and the probe stay
// independent.
func platformTerminalOpener() (in, out *os.File, err error) {
	if windowsConsoleProbe(os.Stdin) == nil {
		in = os.Stdin
	} else {
		in, err = os.OpenFile("CONIN$", os.O_RDWR, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("open CONIN$: %w", err)
		}
	}
	out, err = os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("open CONOUT$: %w", err), platformTerminalReleaser(in, nil)())
	}
	return in, out, nil
}

// platformTerminalReleaser closes CONOUT$ and, unless the input is stdin
// (which the process owns), CONIN$. Bubble Tea reads a separately opened
// CONIN$ with a reader it cannot cancel, and os.File.Close waits for a pending
// read, which would block until the next key press. CONIN$ is therefore
// closed in the background, where its error has no caller to reach: the
// component has already returned its result.
func platformTerminalReleaser(in, out *os.File) func() error {
	return windowsTerminalReleaser(in, out, closeInBackground)
}

// windowsTerminalReleaser closes out synchronously and hands a separately
// opened in (never os.Stdin) to closeInput. Tests pass a synchronous closeInput.
func windowsTerminalReleaser(in, out *os.File, closeInput func(*os.File)) func() error {
	return func() error {
		var err error
		if out != nil {
			err = out.Close()
		}
		if in != nil && in != os.Stdin {
			closeInput(in)
		}
		return err
	}
}

// closeInBackground closes f without waiting for a pending read to finish.
func closeInBackground(f *os.File) {
	go func() { _ = f.Close() }()
}
