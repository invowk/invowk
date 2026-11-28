// SPDX-License-Identifier: MPL-2.0

//go:build windows

package commandadapters

import "golang.org/x/sys/windows"

// isProcessElevated reports whether the current process token is elevated.
func isProcessElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
