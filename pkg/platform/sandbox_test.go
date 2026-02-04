// SPDX-License-Identifier: MPL-2.0

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSandbox_NoSandbox(t *testing.T) {
	// Reset detection cache
	resetSandboxDetection()

	// Save original env and restore after test
	origSnapName := os.Getenv("SNAP_NAME")
	t.Cleanup(func() {
		os.Setenv("SNAP_NAME", origSnapName)
		resetSandboxDetection()
	})

	// Clear SNAP_NAME to ensure no Snap detection
	os.Unsetenv("SNAP_NAME")

	// detectSandboxInternal checks /.flatpak-info which we can't control
	// but in most test environments it won't exist, so test the logic
	result := detectSandboxInternal()

	// If /.flatpak-info exists (running in actual Flatpak), we'll detect it
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		if result != SandboxFlatpak {
			t.Errorf("expected SandboxFlatpak when /.flatpak-info exists, got %q", result)
		}
	} else if result != SandboxNone {
		t.Errorf("expected SandboxNone when no sandbox indicators, got %q", result)
	}
}

func TestDetectSandbox_Snap(t *testing.T) {
	// Reset detection cache
	resetSandboxDetection()

	// Save original env and restore after test
	origSnapName := os.Getenv("SNAP_NAME")
	t.Cleanup(func() {
		os.Setenv("SNAP_NAME", origSnapName)
		resetSandboxDetection()
	})

	// Set SNAP_NAME to simulate Snap environment
	os.Setenv("SNAP_NAME", "test-snap")

	result := detectSandboxInternal()

	// If /.flatpak-info exists, Flatpak takes precedence
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		if result != SandboxFlatpak {
			t.Errorf("expected SandboxFlatpak (takes precedence), got %q", result)
		}
	} else if result != SandboxSnap {
		t.Errorf("expected SandboxSnap, got %q", result)
	}
}

func TestIsInSandbox(t *testing.T) {
	// Reset detection cache
	resetSandboxDetection()

	// Save original env and restore after test
	origSnapName := os.Getenv("SNAP_NAME")
	t.Cleanup(func() {
		os.Setenv("SNAP_NAME", origSnapName)
		resetSandboxDetection()
	})

	// Clear environment
	os.Unsetenv("SNAP_NAME")

	// Test depends on actual environment
	inSandbox := IsInSandbox()

	// Verify consistency with DetectSandbox
	if inSandbox != (DetectSandbox() != SandboxNone) {
		t.Error("IsInSandbox inconsistent with DetectSandbox")
	}
}

func TestGetSpawnCommand(t *testing.T) {
	tests := []struct {
		name     string
		sandbox  SandboxType
		expected string
	}{
		{
			name:     "no sandbox",
			sandbox:  SandboxNone,
			expected: "",
		},
		{
			name:     "flatpak",
			sandbox:  SandboxFlatpak,
			expected: "flatpak-spawn",
		},
		{
			name:     "snap",
			sandbox:  SandboxSnap,
			expected: "snap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset and set detected sandbox directly for testing
			resetSandboxDetection()
			sandboxOnce.Do(func() {
				detectedSandbox = tt.sandbox
			})
			t.Cleanup(resetSandboxDetection)

			result := GetSpawnCommand()
			if result != tt.expected {
				t.Errorf("GetSpawnCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetSpawnArgs(t *testing.T) {
	tests := []struct {
		name     string
		sandbox  SandboxType
		expected []string
	}{
		{
			name:     "no sandbox",
			sandbox:  SandboxNone,
			expected: nil,
		},
		{
			name:     "flatpak",
			sandbox:  SandboxFlatpak,
			expected: []string{"--host"},
		},
		{
			name:     "snap",
			sandbox:  SandboxSnap,
			expected: []string{"run", "--shell"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset and set detected sandbox directly for testing
			resetSandboxDetection()
			sandboxOnce.Do(func() {
				detectedSandbox = tt.sandbox
			})
			t.Cleanup(resetSandboxDetection)

			result := GetSpawnArgs()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("GetSpawnArgs() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("GetSpawnArgs() = %v, want %v", result, tt.expected)
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("GetSpawnArgs()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestDetectSandboxCaching(t *testing.T) {
	// Reset detection cache
	resetSandboxDetection()

	// Save original env and restore after test
	origSnapName := os.Getenv("SNAP_NAME")
	t.Cleanup(func() {
		os.Setenv("SNAP_NAME", origSnapName)
		resetSandboxDetection()
	})

	// Clear environment for first detection
	os.Unsetenv("SNAP_NAME")

	// First detection
	first := DetectSandbox()

	// Change environment
	os.Setenv("SNAP_NAME", "test-snap")

	// Second detection should return cached result
	second := DetectSandbox()

	if first != second {
		t.Errorf("DetectSandbox should return cached result: first=%q, second=%q", first, second)
	}
}

func TestSandboxTypeConstants(t *testing.T) {
	// Verify type constants are distinct
	types := []SandboxType{SandboxNone, SandboxFlatpak, SandboxSnap}
	seen := make(map[SandboxType]bool)

	for _, st := range types {
		if seen[st] {
			t.Errorf("duplicate SandboxType constant: %q", st)
		}
		seen[st] = true
	}

	// Verify SandboxNone is empty string for boolean-like checks
	if SandboxNone != "" {
		t.Errorf("SandboxNone should be empty string, got %q", SandboxNone)
	}
}

func TestDetectSandbox_FlatpakFile(t *testing.T) {
	// This test can only verify behavior if we can create /.flatpak-info
	// which requires root. Instead, we test the detection logic separately.

	// Create a temp directory to simulate root
	tmpDir := t.TempDir()
	flatpakInfoPath := filepath.Join(tmpDir, ".flatpak-info")

	// Create the file
	if err := os.WriteFile(flatpakInfoPath, []byte("[Application]\nname=test\n"), 0o644); err != nil {
		t.Fatalf("failed to create test flatpak-info: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(flatpakInfoPath); err != nil {
		t.Fatalf("flatpak-info file should exist: %v", err)
	}

	// We can't actually test DetectSandbox with this because it checks /.flatpak-info
	// but this confirms our file creation logic works for documentation purposes.
}
