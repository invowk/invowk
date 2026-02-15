// SPDX-License-Identifier: MPL-2.0

package platform

import "testing"

func TestIsWindowsReservedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Reserved names (various cases)
		{"CON lowercase", "con", true},
		{"CON uppercase", "CON", true},
		{"CON mixed case", "Con", true},
		{"PRN", "prn", true},
		{"AUX", "aux", true},
		{"NUL", "nul", true},
		{"COM1", "com1", true},
		{"COM9", "com9", true},
		{"LPT1", "lpt1", true},
		{"LPT9", "lpt9", true},

		// Reserved names with extensions
		{"CON.txt", "con.txt", true},
		{"NUL.exe", "NUL.exe", true},
		{"COM1.log", "com1.log", true},

		// Non-reserved names
		{"normal file", "myfile", false},
		{"normal with extension", "myfile.txt", false},
		{"contains reserved", "confile", false},
		{"COM10", "com10", false},
		{"LPT10", "lpt10", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsWindowsReservedName(tt.input)
			if result != tt.expected {
				t.Errorf("IsWindowsReservedName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWindowsReservedNames(t *testing.T) {
	t.Parallel()

	// Verify all expected reserved names are in the map
	expectedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}

	for _, name := range expectedNames {
		if !WindowsReservedNames[name] {
			t.Errorf("WindowsReservedNames missing %q", name)
		}
	}

	// Verify count is correct (22 reserved names)
	expectedCount := 22
	if len(WindowsReservedNames) != expectedCount {
		t.Errorf("WindowsReservedNames has %d entries, want %d", len(WindowsReservedNames), expectedCount)
	}
}
