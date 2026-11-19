// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import (
	"errors"
	"os"
	"testing"
)

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
