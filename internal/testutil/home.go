// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"runtime"
	"testing"
)

// SetHomeDir sets the appropriate HOME environment variable based on platform
// and returns a cleanup function to restore the original value.
//
// This consolidates the duplicated setHomeDirEnv() functions from:
//   - cmd/invowk/cmd_test.go
//   - internal/config/config_test.go
//   - internal/discovery/discovery_test.go
//
// Platform handling:
//   - Windows: Sets USERPROFILE
//   - Linux/macOS: Sets HOME
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    tmpDir := t.TempDir()
//	    cleanup := testutil.SetHomeDir(t, tmpDir)
//	    defer cleanup()
//
//	    // Test code that uses home directory...
//	}
//
// Or with t.Cleanup:
//
//	func TestSomething(t *testing.T) {
//	    tmpDir := t.TempDir()
//	    t.Cleanup(testutil.SetHomeDir(t, tmpDir))
//
//	    // Test code...
//	}
func SetHomeDir(t testing.TB, dir string) func() {
	t.Helper()

	switch runtime.GOOS {
	case "windows":
		return MustSetenv(t, "USERPROFILE", dir)
	default:
		return MustSetenv(t, "HOME", dir)
	}
}
