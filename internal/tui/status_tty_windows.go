// SPDX-License-Identifier: MPL-2.0

//go:build windows

package tui

import "os"

func openInteractiveStatusTTY() (*os.File, error) {
	return os.OpenFile("CONOUT$", os.O_WRONLY, 0)
}
