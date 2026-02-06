// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Tests for Enhanced Flags Feature - Type handling
// ============================================================================

func TestParseFlags_WithType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		flagType     string
		defaultValue string
		wantType     FlagType
	}{
		{"string type explicit", "string", "hello", FlagTypeString},
		{"bool type with true", "bool", "true", FlagTypeBool},
		{"bool type with false", "bool", "false", FlagTypeBool},
		{"int type with positive", "int", "42", FlagTypeInt},
		{"int type with zero", "int", "0", FlagTypeInt},
		{"int type with negative", "int", "-10", FlagTypeInt},
		{"float type with positive", "float", "3.14", FlagTypeFloat},
		{"float type with negative", "float", "-2.5", FlagTypeFloat},
		{"float type with integer-like", "float", "10.0", FlagTypeFloat},
		{"float type with scientific notation", "float", "1.5e-3", FlagTypeFloat},
		{"float type with zero", "float", "0.0", FlagTypeFloat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.flagType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			flag := inv.Commands[0].Flags[0]
			if flag.GetType() != tt.wantType {
				t.Errorf("Flag.GetType() = %v, want %v", flag.GetType(), tt.wantType)
			}
			if flag.DefaultValue != tt.defaultValue {
				t.Errorf("Flag.DefaultValue = %v, want %v", flag.DefaultValue, tt.defaultValue)
			}
		})
	}
}

func TestParseFlags_TypeDefaultsToString(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "myflag", description: "Test flag"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.Type != "" {
		t.Errorf("Flag.Type should be empty (unset), got %q", flag.Type)
	}
	if flag.GetType() != FlagTypeString {
		t.Errorf("Flag.GetType() should default to 'string', got %v", flag.GetType())
	}
}

func TestParseFlagsValidation_InvalidType(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", type: "invalid"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject invalid type")
	}
}

func TestParseFlagsValidation_TypeIncompatibleWithDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		flagType     string
		defaultValue string
	}{
		{"bool with non-bool value", "bool", "yes"},
		{"bool with number", "bool", "1"},
		{"int with non-number", "int", "abc"},
		{"int with float", "int", "3.14"},
		{"float with non-number", "float", "abc"},
		{"float with multiple dots", "float", "3.14.15"},
		{"float with invalid suffix", "float", "3.14abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "myflag", description: "Test flag", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.flagType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject flag with type %q and incompatible default_value %q", tt.flagType, tt.defaultValue)
			}
		})
	}
}
