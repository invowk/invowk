// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"os"
	"runtime"
	"testing"
)

const osWindows = "windows"

func TestSetHomeDir_Linux(t *testing.T) {
	if runtime.GOOS == osWindows {
		t.Skip("skipping Linux-specific test on Windows")
	}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")

	cleanup := SetHomeDir(t, tmpDir)

	// Verify HOME is set to tmpDir
	if got := os.Getenv("HOME"); got != tmpDir {
		t.Errorf("HOME = %q, want %q", got, tmpDir)
	}

	// Cleanup should restore original
	cleanup()

	if got := os.Getenv("HOME"); got != originalHome {
		t.Errorf("After cleanup, HOME = %q, want %q", got, originalHome)
	}
}

func TestSetHomeDir_Windows(t *testing.T) {
	if runtime.GOOS != osWindows {
		t.Skip("skipping Windows-specific test on non-Windows")
	}

	tmpDir := t.TempDir()
	originalUserProfile := os.Getenv("USERPROFILE")

	cleanup := SetHomeDir(t, tmpDir)

	// Verify USERPROFILE is set to tmpDir
	if got := os.Getenv("USERPROFILE"); got != tmpDir {
		t.Errorf("USERPROFILE = %q, want %q", got, tmpDir)
	}

	// Cleanup should restore original
	cleanup()

	if got := os.Getenv("USERPROFILE"); got != originalUserProfile {
		t.Errorf("After cleanup, USERPROFILE = %q, want %q", got, originalUserProfile)
	}
}

func TestSetHomeDir_WithTCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	var envVar string
	if runtime.GOOS == osWindows {
		envVar = "USERPROFILE"
	} else {
		envVar = "HOME"
	}

	original := os.Getenv(envVar)

	// Use t.Cleanup pattern
	t.Run("subtest", func(t *testing.T) {
		t.Cleanup(SetHomeDir(t, tmpDir))

		if got := os.Getenv(envVar); got != tmpDir {
			t.Errorf("%s = %q, want %q", envVar, got, tmpDir)
		}
	})

	// After subtest, should be restored
	if got := os.Getenv(envVar); got != original {
		t.Errorf("After subtest, %s = %q, want %q", envVar, got, original)
	}
}

func TestSetHomeDir_EmptyDir(t *testing.T) {
	// Setting to empty string should work (though unusual)
	var envVar string
	if runtime.GOOS == osWindows {
		envVar = "USERPROFILE"
	} else {
		envVar = "HOME"
	}

	original := os.Getenv(envVar)

	cleanup := SetHomeDir(t, "")

	if got := os.Getenv(envVar); got != "" {
		t.Errorf("%s = %q, want empty string", envVar, got)
	}

	cleanup()

	if got := os.Getenv(envVar); got != original {
		t.Errorf("After cleanup, %s = %q, want %q", envVar, got, original)
	}
}
