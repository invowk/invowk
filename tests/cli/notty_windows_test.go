// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// noTerminalProcAttr starts the child detached from any console, so CONIN$
// and CONOUT$ do not open and its standard handles are the redirected ones.
func noTerminalProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
}
