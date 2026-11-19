// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package cli

import "syscall"

// noTerminalProcAttr starts the child in a new session, which has no
// controlling terminal, so it cannot open /dev/tty.
func noTerminalProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
