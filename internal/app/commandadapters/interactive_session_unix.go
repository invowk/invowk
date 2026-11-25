// SPDX-License-Identifier: MPL-2.0

//go:build unix

package commandadapters

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/xpty"
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
	unixPty, ok := pty.(*xpty.UnixPty)
	if !ok {
		return fmt.Errorf("interactive terminal is %T, want a Unix PTY", pty)
	}
	slave := unixPty.Slave()
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
	unixPty, ok := pty.(*xpty.UnixPty)
	if !ok {
		return fmt.Errorf("interactive terminal is %T, want a Unix PTY", pty)
	}
	return unixPty.Slave().Close()
}
