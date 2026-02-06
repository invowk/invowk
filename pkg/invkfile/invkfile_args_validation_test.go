// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Positional Arguments Validation
// ============================================================================

func TestParseArgsValidation_InvalidName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argName string
	}{
		{"starts with number", "1arg"},
		{"contains space", "my arg"},
		{"special characters", "arg@name"},
		{"empty name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "` + tt.argName + `", description: "Test arg"},
		]
	}
]
`
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject arg with invalid name %q", tt.argName)
			}
		})
	}
}

func TestParseArgsValidation_ValidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argName string
	}{
		{"simple lowercase", "name"},
		{"with hyphen", "output-file"},
		{"with underscore", "output_file"},
		{"with numbers", "file1"},
		{"mixed case", "outputFile"},
		{"uppercase start", "Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "` + tt.argName + `", description: "Test arg"},
		]
	}
]
`
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			inv, err := Parse(invkfilePath)
			if err != nil {
				t.Errorf("Parse() should accept arg with valid name %q, got error: %v", tt.argName, err)
				return
			}

			if len(inv.Commands[0].Args) != 1 {
				t.Errorf("Expected 1 arg, got %d", len(inv.Commands[0].Args))
			}
		})
	}
}

func TestParseArgsValidation_EmptyDescription(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "myarg", description: "   "},
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
		t.Error("Parse() should reject arg with empty/whitespace-only description")
	}
}

func TestParseArgsValidation_DuplicateNames(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "name", description: "First argument"},
			{name: "name", description: "Duplicate argument"},
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
		t.Error("Parse() should reject duplicate arg names")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Error should mention 'duplicate', got: %v", err)
	}
}

func TestParseArgsValidation_RequiredAfterOptional(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "optional", description: "Optional arg"},
			{name: "required", description: "Required arg", required: true},
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
		t.Error("Parse() should reject required arg after optional arg")
	}
	if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "optional") {
		t.Errorf("Error should mention required/optional ordering, got: %v", err)
	}
}

func TestParseArgsValidation_VariadicNotLast(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "files", description: "Source files", required: true, variadic: true},
			{name: "dest", description: "Destination", required: true},
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
		t.Error("Parse() should reject variadic arg that is not last")
	}
	if !strings.Contains(err.Error(), "variadic") {
		t.Errorf("Error should mention variadic constraint, got: %v", err)
	}
}

func TestParseArgsValidation_RequiredWithDefaultValue(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "myarg", description: "Test arg", required: true, default_value: "foo"},
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
		t.Error("Parse() should reject arg that is both required and has default_value")
	}
	if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "default_value") {
		t.Errorf("Error should mention required/default_value conflict, got: %v", err)
	}
}

func TestParseArgsValidation_InvalidType(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "myarg", description: "Test arg", type: "invalid"},
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
		t.Error("Parse() should reject invalid arg type")
	}
}

func TestParseArgsValidation_TypeIncompatibleWithDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		argType      string
		defaultValue string
	}{
		{"int with non-number", "int", "abc"},
		{"int with float", "int", "3.14"},
		{"float with non-number", "float", "abc"},
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
			}
		]
		args: [
			{name: "myarg", description: "Test arg", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.argType, tt.defaultValue)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject arg with type %q and incompatible default_value %q", tt.argType, tt.defaultValue)
			}
		})
	}
}

func TestParseArgsValidation_InvalidRegex(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "myarg", description: "Test arg", validation: "[invalid(regex"},
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
		t.Error("Parse() should reject invalid validation regex")
	}
}

func TestParseArgsValidation_DefaultNotMatchingValidation(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
		args: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "invalid"},
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
		t.Error("Parse() should reject default_value that doesn't match validation pattern")
	}
}

func TestValidateArgumentValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		arg        Argument
		value      string
		wantErr    bool
		errContain string
	}{
		{
			name:    "string type accepts any value",
			arg:     Argument{Name: "test", Type: ArgumentTypeString},
			value:   "hello world",
			wantErr: false,
		},
		{
			name:    "int type accepts positive",
			arg:     Argument{Name: "test", Type: ArgumentTypeInt},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "int type accepts zero",
			arg:     Argument{Name: "test", Type: ArgumentTypeInt},
			value:   "0",
			wantErr: false,
		},
		{
			name:    "int type accepts negative",
			arg:     Argument{Name: "test", Type: ArgumentTypeInt},
			value:   "-10",
			wantErr: false,
		},
		{
			name:       "int type rejects non-integer",
			arg:        Argument{Name: "test", Type: ArgumentTypeInt},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:       "int type rejects float",
			arg:        Argument{Name: "test", Type: ArgumentTypeInt},
			value:      "3.14",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:    "float type accepts positive",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "3.14",
			wantErr: false,
		},
		{
			name:    "float type accepts negative",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "-2.5",
			wantErr: false,
		},
		{
			name:    "float type accepts zero",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "0.0",
			wantErr: false,
		},
		{
			name:    "float type accepts integer-like",
			arg:     Argument{Name: "test", Type: ArgumentTypeFloat},
			value:   "42",
			wantErr: false,
		},
		{
			name:       "float type rejects non-number",
			arg:        Argument{Name: "test", Type: ArgumentTypeFloat},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid floating-point number",
		},
		{
			name:    "validation regex passes",
			arg:     Argument{Name: "env", Type: ArgumentTypeString, Validation: "^(dev|staging|prod)$"},
			value:   "prod",
			wantErr: false,
		},
		{
			name:       "validation regex fails",
			arg:        Argument{Name: "env", Type: ArgumentTypeString, Validation: "^(dev|staging|prod)$"},
			value:      "invalid",
			wantErr:    true,
			errContain: "does not match required pattern",
		},
		{
			name:    "empty type defaults to string",
			arg:     Argument{Name: "test"},
			value:   "anything",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.arg.ValidateArgumentValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateArgumentValue() should return error")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateArgumentValue() error = %v, should contain %q", err, tt.errContain)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateArgumentValue() unexpected error = %v", err)
				}
			}
		})
	}
}
