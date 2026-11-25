// SPDX-License-Identifier: MPL-2.0

//go:build unix

package commandadapters

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/unix"
)

var (
	// errInteractiveStdioPreset is returned when a prepared interactive
	// command already has a standard stream: its controlling terminal would
	// not be the viewer's PTY, or the stream the runtime chose would be lost.
	errInteractiveStdioPreset = errors.New("interactive command must not set stdin, stdout or stderr before it starts on the viewer's terminal")
	// errInteractiveSysProcAttrPreset is returned when a prepared interactive
	// command already has process attributes, which the session setup would
	// replace.
	errInteractiveSysProcAttrPreset = errors.New("interactive command must not set SysProcAttr before it starts on the viewer's terminal")
)

// nonBlockingTerminal is the viewer's side of a Unix PTY (design D8): Read
// and Write go through a non-blocking duplicate of the master, which the
// runtime poller wakes with os.ErrClosed when it is closed. With the
// blocking master that creack/pty leaves, a Close left the descriptor open
// while the viewer's writer was blocked on input the command never read, or
// its reader on output the command never wrote, so the command got SIGHUP
// only when invowk exited.
//
// O_NONBLOCK belongs to the open file description, which the duplicate
// shares with the original master: the original is never read or written
// (it would see EAGAIN), and nothing may call Fd() on either file, since
// os.File.Fd switches the description back to blocking. So this type
// exposes no file, and Resize goes through xpty's Resize, which uses
// Control, never Fd.
type nonBlockingTerminal struct {
	pty    *xpty.UnixPty
	master *os.File
}

// prepareInteractiveCmd starts cmd in a new session whose controlling
// terminal is the PTY's slave, which is also its stdin, stdout and stderr.
// A process of the command that opens /dev/tty then gets the viewer's PTY,
// never invowk's own terminal; Ctrl+C, resizes and the hang-up at the end
// reach the command through that terminal as SIGINT, SIGWINCH and SIGHUP.
//
// Ctty is a descriptor number in the child (Go 1.15+): 0 names the slave
// because stdin is set to it here. A command that already set a stream or
// SysProcAttr is refused instead of silently getting another terminal.
func prepareInteractiveCmd(cmd *exec.Cmd, pty xpty.Pty) error {
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		return errInteractiveStdioPreset
	}
	if cmd.SysProcAttr != nil {
		return errInteractiveSysProcAttrPreset
	}
	slave, err := ptySlave(pty)
	if err != nil {
		return err
	}
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	return nil
}

// releaseParentSlave closes invowk's own copy of the slave once the command
// started on it. The master read then ends (EIO) as soon as every process
// of the command has closed the slave and its output was read, which is
// what lets the viewer show the completion banner after the last output
// (design D8). UnixPty.Close closes the slave again later; the adapter
// ignores the os.ErrClosed of that second close.
func releaseParentSlave(pty xpty.Pty) error {
	slave, err := ptySlave(pty)
	if err != nil {
		return err
	}
	return slave.Close()
}

// viewerTerminal returns the terminal the viewer uses for pty: on Unix a
// nonBlockingTerminal, or pty itself when the master cannot be duplicated
// (the session then works as before, without the early hang-up).
func viewerTerminal(pty xpty.Pty) interactiveTerminal {
	unixPty, ok := pty.(*xpty.UnixPty)
	if !ok {
		return pty
	}
	dup := -1
	var dupErr error
	if err := unixPty.Control(func(fd uintptr) {
		dup, dupErr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
	}); err != nil || dupErr != nil {
		return pty
	}
	if err := unix.SetNonblock(dup, true); err != nil {
		_ = unix.Close(dup) //nolint:errcheck // best effort: the duplicate was never used
		return pty
	}
	return &nonBlockingTerminal{pty: unixPty, master: os.NewFile(uintptr(dup), unixPty.Name())}
}

// Read reads the command's output from the non-blocking master.
func (t *nonBlockingTerminal) Read(b []byte) (int, error) {
	return t.master.Read(b)
}

// Write writes the viewer's input to the non-blocking master.
func (t *nonBlockingTerminal) Write(b []byte) (int, error) {
	return t.master.Write(b)
}

// Resize sets the command's terminal size through xpty's Control-based Resize.
//
//goplint:ignore -- tui.InteractiveTerminal and xpty take plain int columns and rows.
func (t *nonBlockingTerminal) Resize(width, height int) error {
	return t.pty.Resize(width, height)
}

// Close closes the duplicate first, which wakes a blocked Read or Write and
// closes its descriptor at once, then the PTY (the original master and the
// slave copy), so the command's terminal hangs up as the viewer ends.
func (t *nonBlockingTerminal) Close() error {
	return errors.Join(t.master.Close(), t.pty.Close())
}

// ptySlave returns the slave side of the viewer's Unix PTY.
func ptySlave(pty xpty.Pty) (*os.File, error) {
	unixPty, ok := pty.(*xpty.UnixPty)
	if !ok {
		return nil, fmt.Errorf("interactive terminal is %T, want a Unix PTY", pty)
	}
	return unixPty.Slave(), nil
}
