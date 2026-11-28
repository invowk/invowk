// SPDX-License-Identifier: MPL-2.0

//go:build !unix && !windows

package tui

import "os"

func openInteractiveStatusTTY() (*os.File, error) {
	return nil, os.ErrNotExist
}
