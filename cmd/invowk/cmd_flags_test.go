// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// User environment capture tests
// ---------------------------------------------------------------------------

func TestCaptureUserEnv(t *testing.T) {
	// Set a test environment variable
	testKey := "INVOWK_TEST_CAPTURE_ENV_VAR"
	testValue := "test_value_12345"
	cleanup := testutil.MustSetenv(t, testKey, testValue)
	defer cleanup()

	env := captureUserEnv()

	if env[testKey] != testValue {
		t.Errorf("captureUserEnv() should capture env var, got %q, want %q", env[testKey], testValue)
	}

	// Verify PATH is captured (should exist on all systems)
	if _, exists := env["PATH"]; !exists {
		t.Error("captureUserEnv() should capture PATH environment variable")
	}
}

// ---------------------------------------------------------------------------
// Flag tests
// ---------------------------------------------------------------------------

func TestFlagNameToEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "verbose",
			expected: "INVOWK_FLAG_VERBOSE",
		},
		{
			name:     "name with hyphen",
			input:    "output-file",
			expected: "INVOWK_FLAG_OUTPUT_FILE",
		},
		{
			name:     "name with multiple hyphens",
			input:    "dry-run-mode",
			expected: "INVOWK_FLAG_DRY_RUN_MODE",
		},
		{
			name:     "name with underscore",
			input:    "output_file",
			expected: "INVOWK_FLAG_OUTPUT_FILE",
		},
		{
			name:     "mixed case preserved as uppercase",
			input:    "outputFile",
			expected: "INVOWK_FLAG_OUTPUTFILE",
		},
		{
			name:     "already uppercase",
			input:    "VERBOSE",
			expected: "INVOWK_FLAG_VERBOSE",
		},
		{
			name:     "single character",
			input:    "v",
			expected: "INVOWK_FLAG_V",
		},
		{
			name:     "numeric suffix",
			input:    "level2",
			expected: "INVOWK_FLAG_LEVEL2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := invowkfile.FlagNameToEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("invowkfile.FlagNameToEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunCommandWithFlags_FlagsInjectedAsEnvVars(t *testing.T) {
	// This test verifies that flag values are correctly converted to environment variables
	// We'll test the FlagNameToEnvVar conversion more extensively since
	// the actual runCommandWithFlags requires a full invowkfile setup

	// Test that the conversion is consistent
	flagName := "my-custom-flag"
	envVar := invowkfile.FlagNameToEnvVar(flagName)

	if envVar != "INVOWK_FLAG_MY_CUSTOM_FLAG" {
		t.Errorf("invowkfile.FlagNameToEnvVar(%q) = %q, expected INVOWK_FLAG_MY_CUSTOM_FLAG", flagName, envVar)
	}

	// Verify the pattern: INVOWK_FLAG_ prefix, uppercase, hyphens replaced with underscores
	if !strings.HasPrefix(envVar, "INVOWK_FLAG_") {
		t.Error("FlagNameToEnvVar result should have INVOWK_FLAG_ prefix")
	}

	if strings.Contains(envVar, "-") {
		t.Error("FlagNameToEnvVar result should not contain hyphens")
	}

	if envVar != strings.ToUpper(envVar) {
		t.Error("FlagNameToEnvVar result should be all uppercase")
	}
}

func TestFlagNameToEnvVar_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "INVOWK_FLAG_",
		},
		{
			name:     "only hyphens",
			input:    "---",
			expected: "INVOWK_FLAG____",
		},
		{
			name:     "starts with hyphen",
			input:    "-flag",
			expected: "INVOWK_FLAG__FLAG",
		},
		{
			name:     "ends with hyphen",
			input:    "flag-",
			expected: "INVOWK_FLAG_FLAG_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := invowkfile.FlagNameToEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("invowkfile.FlagNameToEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
