// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"os"
	"testing"
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
	tests := []struct {
		name     string
		config   Config
		envValue string
		expected bool
	}{
		{
			name:     "config accessible false, not nested",
			config:   Config{Accessible: false},
			envValue: "",
			expected: false,
		},
		{
			name:     "config accessible true, not nested",
			config:   Config{Accessible: true},
			envValue: "",
			expected: true,
		},
		{
			name:     "config accessible false, nested",
			config:   Config{Accessible: false},
			envValue: "1",
			expected: true,
		},
		{
			name:     "config accessible true, nested",
			config:   Config{Accessible: true},
			envValue: "1",
			expected: true,
		},
		{
			name:     "zero config, nested",
			config:   Config{},
			envValue: "1",
			expected: true,
		},
		{
			name:     "zero config, not nested",
			config:   Config{},
			envValue: "",
			expected: false,
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
			if result != tt.expected {
				t.Errorf("shouldUseAccessible() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig_NestedInteractive(t *testing.T) {
	tests := []struct {
		name               string
		envValue           string
		expectedAccessible bool
	}{
		{
			name:               "not nested",
			envValue:           "",
			expectedAccessible: false,
		},
		{
			name:               "nested",
			envValue:           "1",
			expectedAccessible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			// Clear ACCESSIBLE env to test only INVOWK_INTERACTIVE
			os.Unsetenv("ACCESSIBLE")

			// Set test value
			if tt.envValue == "" {
				os.Unsetenv("INVOWK_INTERACTIVE")
			} else {
				os.Setenv("INVOWK_INTERACTIVE", tt.envValue)
			}

			config := DefaultConfig()
			if config.Accessible != tt.expectedAccessible {
				t.Errorf("DefaultConfig().Accessible = %v, want %v", config.Accessible, tt.expectedAccessible)
			}
		})
	}
}
