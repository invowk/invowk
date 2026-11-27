// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import "os"

func openControllingTerminal() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_WRONLY, 0)
}
