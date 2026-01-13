// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"os"
	"testing"

	"golang.org/x/term"
)

func TestIsNestedInteractive(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "not set",
			envValue: "",
			expected: false,
		},
		{
			name:     "set to 1",
			envValue: "1",
			expected: true,
		},
		{
			name:     "set to true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "set to any value",
			envValue: "yes",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original env
			original := os.Getenv("INVOWK_INTERACTIVE")
			defer func() {
				if original == "" {
					os.Unsetenv("INVOWK_INTERACTIVE")
				} else {
					os.Setenv("INVOWK_INTERACTIVE", original)
				}
			}()

			// Set test value
			if tt.envValue == "" {
				os.Unsetenv("INVOWK_INTERACTIVE")
			} else {
				os.Setenv("INVOWK_INTERACTIVE", tt.envValue)
			}

			result := IsNestedInteractive()
			if result != tt.expected {
				t.Errorf("IsNestedInteractive() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestShouldUseAccessible(t *testing.T) {
	// Get the current TTY state - this affects expected results
	// When stdin is not a terminal (e.g., in CI), shouldUseAccessible always returns true
	isTerminal := isInputTerminal()

	tests := []struct {
		name      string
		config    Config
		envValue  string
		wantIfTTY bool // expected when stdin IS a terminal
		wantNoTTY bool // expected when stdin is NOT a terminal
	}{
		{
			name:      "config accessible false, not nested",
			config:    Config{Accessible: false},
			envValue:  "",
			wantIfTTY: false,
			wantNoTTY: true, // no TTY means accessible mode
		},
		{
			name:      "config accessible true, not nested",
			config:    Config{Accessible: true},
			envValue:  "",
			wantIfTTY: true,
			wantNoTTY: true,
		},
		{
			name:      "config accessible false, nested",
			config:    Config{Accessible: false},
			envValue:  "1",
			wantIfTTY: true,
			wantNoTTY: true,
		},
		{
			name:      "config accessible true, nested",
			config:    Config{Accessible: true},
			envValue:  "1",
			wantIfTTY: true,
			wantNoTTY: true,
		},
		{
			name:      "zero config, nested",
			config:    Config{},
			envValue:  "1",
			wantIfTTY: true,
			wantNoTTY: true,
		},
		{
			name:      "zero config, not nested",
			config:    Config{},
			envValue:  "",
			wantIfTTY: false,
			wantNoTTY: true, // no TTY means accessible mode
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original env
			original := os.Getenv("INVOWK_INTERACTIVE")
			defer func() {
				if original == "" {
					os.Unsetenv("INVOWK_INTERACTIVE")
				} else {
					os.Setenv("INVOWK_INTERACTIVE", original)
				}
			}()

			// Set test value
			if tt.envValue == "" {
				os.Unsetenv("INVOWK_INTERACTIVE")
			} else {
				os.Setenv("INVOWK_INTERACTIVE", tt.envValue)
			}

			result := shouldUseAccessible(tt.config)

			// Expected value depends on whether stdin is a terminal
			expected := tt.wantIfTTY
			if !isTerminal {
				expected = tt.wantNoTTY
			}

			if result != expected {
				t.Errorf("shouldUseAccessible() = %v, want %v (stdin is terminal: %v)",
					result, expected, isTerminal)
			}
		})
	}
}

func TestDefaultConfig_NestedInteractive(t *testing.T) {
	// When INVOWK_INTERACTIVE is set, accessible mode should be enabled
	// regardless of whether stdin is a terminal
	t.Run("nested always uses accessible mode and stderr", func(t *testing.T) {
		// Save and restore original env
		originalInteractive := os.Getenv("INVOWK_INTERACTIVE")
		originalAccessible := os.Getenv("ACCESSIBLE")
		defer func() {
			if originalInteractive == "" {
				os.Unsetenv("INVOWK_INTERACTIVE")
			} else {
				os.Setenv("INVOWK_INTERACTIVE", originalInteractive)
			}
			if originalAccessible == "" {
				os.Unsetenv("ACCESSIBLE")
			} else {
				os.Setenv("ACCESSIBLE", originalAccessible)
			}
		}()

		os.Unsetenv("ACCESSIBLE")
		os.Setenv("INVOWK_INTERACTIVE", "1")

		config := DefaultConfig()
		if !config.Accessible {
			t.Error("DefaultConfig().Accessible should be true when nested")
		}
		if config.Output != os.Stderr {
			t.Error("DefaultConfig().Output should be os.Stderr when nested")
		}
	})
}

func TestIsInputTerminal(t *testing.T) {
	// This test verifies that isInputTerminal returns consistent results
	// The actual value depends on how the tests are run (terminal vs CI)
	result := isInputTerminal()
	expected := term.IsTerminal(int(os.Stdin.Fd()))

	if result != expected {
		t.Errorf("isInputTerminal() = %v, want %v", result, expected)
	}
}

func TestDefaultConfig_NoTTY_UsesAccessible(t *testing.T) {
	// When stdin is not a terminal, accessible mode should be enabled
	// We can't easily test this without mocking stdin, but we can verify
	// the logic is consistent with isInputTerminal
	originalInteractive := os.Getenv("INVOWK_INTERACTIVE")
	originalAccessible := os.Getenv("ACCESSIBLE")
	defer func() {
		if originalInteractive == "" {
			os.Unsetenv("INVOWK_INTERACTIVE")
		} else {
			os.Setenv("INVOWK_INTERACTIVE", originalInteractive)
		}
		if originalAccessible == "" {
			os.Unsetenv("ACCESSIBLE")
		} else {
			os.Setenv("ACCESSIBLE", originalAccessible)
		}
	}()

	os.Unsetenv("INVOWK_INTERACTIVE")
	os.Unsetenv("ACCESSIBLE")

	config := DefaultConfig()
	isTerminal := isInputTerminal()

	// When stdin is not a terminal, accessible should be true
	// When stdin is a terminal, accessible should be false (unless env is set)
	expectedAccessible := !isTerminal
	if config.Accessible != expectedAccessible {
		t.Errorf("DefaultConfig().Accessible = %v, want %v (stdin is terminal: %v)",
			config.Accessible, expectedAccessible, isTerminal)
	}

	// Output should be stderr when accessible, stdout otherwise
	if config.Accessible {
		if config.Output != os.Stderr {
			t.Error("DefaultConfig().Output should be os.Stderr when accessible")
		}
	} else {
		if config.Output != os.Stdout {
			t.Error("DefaultConfig().Output should be os.Stdout when not accessible")
		}
	}
}
