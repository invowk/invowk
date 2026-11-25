// SPDX-License-Identifier: MPL-2.0

//go:build !unix

package commandadapters

import (
	"os/exec"

	"github.com/charmbracelet/x/xpty"
)

// prepareInteractiveCmd does nothing outside Unix: ConPTY attaches the
// process to the pseudo-console it is spawned on, and CONIN$/CONOUT$ already
// open that console.
func prepareInteractiveCmd(*exec.Cmd, xpty.Pty) error {
	return nil
}
