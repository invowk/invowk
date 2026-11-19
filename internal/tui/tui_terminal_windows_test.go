// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"errors"
	"os"
	"testing"
	"time"
)

// waitClosed fails unless f is closed within a generous deadline: the releaser
// closes a separately opened input in the background.
func waitClosed(t *testing.T, f *os.File) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := f.Stat(); errors.Is(err, os.ErrClosed) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: still open 10s after release, want it closed in the background", f.Name())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestConsoleProbe_RegularFileFails(t *testing.T) {
	t.Parallel()

	f := tempTerminalFile(t, "not-a-console")
	if err := windowsConsoleProbe(f); err == nil {
		t.Fatalf("windowsConsoleProbe(%s) = nil, want an error for a regular file", f.Name())
	}
}

func TestResolveComponentTerminal_RegularFileConsoleIsNoTerminal(t *testing.T) {
	t.Parallel()

	in, out := tempTerminalFile(t, "conin"), tempTerminalFile(t, "conout")
	r := terminalResolver{
		open:       func() (*os.File, *os.File, error) { return in, out, nil },
		probe:      platformConsoleProbe(),
		isTerminal: noTerminals,
	}

	_, err := r.resolve(ComponentTypeInput)
	if !errors.Is(err, ErrNoTerminal) {
		t.Fatalf("resolve() error = %v, want ErrNoTerminal for handles that are not consoles", err)
	}
	assertClosed(t, in)
	assertClosed(t, out)
}

func TestPlatformTerminalReleaser_KeepsStdinAndClosesOutput(t *testing.T) {
	t.Parallel()

	out := tempTerminalFile(t, "conout")
	if err := platformTerminalReleaser(os.Stdin, out)(); err != nil {
		t.Fatalf("release() error = %v", err)
	}
	assertClosed(t, out)
	if _, err := os.Stdin.Stat(); err != nil {
		t.Fatalf("os.Stdin.Stat() after release = %v, want stdin left open", err)
	}
}

func TestPlatformTerminalReleaser_ClosesSeparateInputAndOutput(t *testing.T) {
	t.Parallel()

	in, out := tempTerminalFile(t, "conin"), tempTerminalFile(t, "conout")
	if err := platformTerminalReleaser(in, out)(); err != nil {
		t.Fatalf("release() error = %v", err)
	}
	assertClosed(t, out)
	waitClosed(t, in)
}

func TestPlatformTerminalReleaser_HalfOpenClosesInput(t *testing.T) {
	t.Parallel()

	in := tempTerminalFile(t, "conin")
	if err := platformTerminalReleaser(in, nil)(); err != nil {
		t.Fatalf("release() error = %v, want nil when only CONIN$ was opened", err)
	}
	waitClosed(t, in)
}
