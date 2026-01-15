// SPDX-License-Identifier: EPL-2.0

// Package platform provides cross-platform compatibility utilities.
package platform

import "strings"

// WindowsReservedNames are filenames that cannot be used on Windows.
// These names are reserved by the operating system regardless of file extension.
var WindowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// IsWindowsReservedName checks if a filename is a Windows reserved name.
// It handles filenames with extensions by checking just the base name portion.
func IsWindowsReservedName(name string) bool {
	// Get the base name without extension
	upper := strings.ToUpper(name)
	// Remove extension if present
	if idx := strings.LastIndex(upper, "."); idx != -1 {
		upper = upper[:idx]
	}
	return WindowsReservedNames[upper]
}
