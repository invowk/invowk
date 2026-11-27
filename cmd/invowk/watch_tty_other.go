// SPDX-License-Identifier: MPL-2.0

//go:build !unix && !windows

package cmd

import "os"

func openControllingTerminal() (*os.File, error) {
	return nil, os.ErrNotExist
}
