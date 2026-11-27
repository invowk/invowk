// SPDX-License-Identifier: MPL-2.0

//go:build windows

package cmd

import "os"

func openControllingTerminal() (*os.File, error) {
	return os.OpenFile("CONOUT$", os.O_WRONLY, 0)
}
