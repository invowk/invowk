// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"os"
	"path/filepath"
	"runtime"
)

const windowsSystemRootDefault = `C:\Windows`

// FixedShellCommand returns a shell binary path and arguments for running a
// short inline script without relying on PATH lookup.
func FixedShellCommand(script string) (shellPath string, shellArgs []string) {
	if runtime.GOOS == "windows" {
		systemRoot := os.Getenv("SystemRoot")
		if systemRoot == "" {
			systemRoot = windowsSystemRootDefault
		}
		return filepath.Join(systemRoot, "System32", "cmd.exe"), []string{"/c", script}
	}
	return "/bin/sh", []string{"-c", script}
}
