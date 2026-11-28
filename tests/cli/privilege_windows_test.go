// SPDX-License-Identifier: MPL-2.0

//go:build windows

package cli

import "golang.org/x/sys/windows"

func isTestProcessElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
