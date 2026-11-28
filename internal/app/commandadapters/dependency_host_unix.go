// SPDX-License-Identifier: MPL-2.0

//go:build unix

package commandadapters

import "os"

// isProcessElevated reports whether the current process runs with effective UID 0.
func isProcessElevated() bool {
	return os.Geteuid() == 0
}
