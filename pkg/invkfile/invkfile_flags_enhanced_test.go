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
// Tests for Enhanced Flags Feature (type, required, short, validation)
// ============================================================================

func TestParseFlags_WithType(t *testing.T) {
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

func TestParseFlags_RequiredFlag(t *testing.T) {
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
		flags: [
			{name: "myflag", description: "Test flag", required: true},
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
	if !flag.Required {
		t.Errorf("Flag.Required = false, want true")
	}
}

func TestParseFlagsValidation_RequiredWithDefaultValue(t *testing.T) {
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
		flags: [
			{name: "myflag", description: "Test flag", required: true, default_value: "foo"},
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
		t.Errorf("Parse() should reject flag that is both required and has default_value")
	}
	if err != nil && !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "default_value") {
		t.Errorf("Error message should mention required and default_value conflict, got: %v", err)
	}
}

func TestParseFlags_ShortAlias(t *testing.T) {
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
		flags: [
			{name: "verbose", description: "Enable verbose output", short: "v"},
			{name: "quiet", description: "Quiet mode", short: "q"},
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

	flags := inv.Commands[0].Flags
	if flags[0].Short != "v" {
		t.Errorf("Flag[0].Short = %q, want %q", flags[0].Short, "v")
	}
	if flags[1].Short != "q" {
		t.Errorf("Flag[1].Short = %q, want %q", flags[1].Short, "q")
	}
}

func TestParseFlagsValidation_InvalidShortAlias(t *testing.T) {
	tests := []struct {
		name  string
		short string
	}{
		{"multiple chars", "ab"},
		{"digit", "1"},
		{"special char", "-"},
		{"empty string is valid", ""}, // Should NOT cause error - empty means no short alias
	}

	for _, tt := range tests {
		if tt.short == "" {
			continue // Skip empty - it's valid
		}
		t.Run(tt.name, func(t *testing.T) {
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
		flags: [
			{name: "myflag", description: "Test flag", short: "%s"},
		]
	}
]
`, tt.short)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject invalid short alias %q", tt.short)
			}
		})
	}
}

func TestParseFlagsValidation_DuplicateShortAlias(t *testing.T) {
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
		flags: [
			{name: "verbose", description: "Enable verbose output", short: "v"},
			{name: "version", description: "Show version", short: "v"},
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
		t.Errorf("Parse() should reject duplicate short alias")
	}
	if err != nil && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "short") {
		t.Errorf("Error message should mention duplicate short alias, got: %v", err)
	}
}

func TestParseFlags_ValidationRegex(t *testing.T) {
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
		flags: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$"},
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
	if flag.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Flag.Validation = %q, want %q", flag.Validation, "^(dev|staging|prod)$")
	}
}

func TestParseFlagsValidation_InvalidRegex(t *testing.T) {
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
		flags: [
			{name: "myflag", description: "Test flag", validation: "[invalid(regex"},
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
		t.Errorf("Parse() should reject invalid validation regex")
	}
}

func TestParseFlagsValidation_DefaultNotMatchingValidation(t *testing.T) {
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
		flags: [
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
		t.Errorf("Parse() should reject default_value that doesn't match validation pattern")
	}
}

func TestParseFlags_DefaultMatchesValidation(t *testing.T) {
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
		flags: [
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "prod"},
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
		t.Fatalf("Parse() should accept default_value that matches validation, got error: %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.DefaultValue != "prod" {
		t.Errorf("Flag.DefaultValue = %q, want %q", flag.DefaultValue, "prod")
	}
}

func TestValidateFlagValue(t *testing.T) {
	tests := []struct {
		name       string
		flag       Flag
		value      string
		wantErr    bool
		errContain string
	}{
		{
			name:    "string type accepts any value",
			flag:    Flag{Name: "test", Type: FlagTypeString},
			value:   "hello world",
			wantErr: false,
		},
		{
			name:    "bool type accepts true",
			flag:    Flag{Name: "test", Type: FlagTypeBool},
			value:   "true",
			wantErr: false,
		},
		{
			name:    "bool type accepts false",
			flag:    Flag{Name: "test", Type: FlagTypeBool},
			value:   "false",
			wantErr: false,
		},
		{
			name:       "bool type rejects invalid",
			flag:       Flag{Name: "test", Type: FlagTypeBool},
			value:      "yes",
			wantErr:    true,
			errContain: "must be 'true' or 'false'",
		},
		{
			name:    "int type accepts positive",
			flag:    Flag{Name: "test", Type: FlagTypeInt},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "int type accepts zero",
			flag:    Flag{Name: "test", Type: FlagTypeInt},
			value:   "0",
			wantErr: false,
		},
		{
			name:    "int type accepts negative",
			flag:    Flag{Name: "test", Type: FlagTypeInt},
			value:   "-10",
			wantErr: false,
		},
		{
			name:       "int type rejects non-integer",
			flag:       Flag{Name: "test", Type: FlagTypeInt},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:       "int type rejects float",
			flag:       Flag{Name: "test", Type: FlagTypeInt},
			value:      "3.14",
			wantErr:    true,
			errContain: "must be a valid integer",
		},
		{
			name:    "float type accepts positive",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "3.14",
			wantErr: false,
		},
		{
			name:    "float type accepts negative",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "-2.5",
			wantErr: false,
		},
		{
			name:    "float type accepts zero",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "0.0",
			wantErr: false,
		},
		{
			name:    "float type accepts integer-like",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "float type accepts scientific notation",
			flag:    Flag{Name: "test", Type: FlagTypeFloat},
			value:   "1.5e-3",
			wantErr: false,
		},
		{
			name:       "float type rejects non-number",
			flag:       Flag{Name: "test", Type: FlagTypeFloat},
			value:      "abc",
			wantErr:    true,
			errContain: "must be a valid floating-point number",
		},
		{
			name:       "float type rejects multiple dots",
			flag:       Flag{Name: "test", Type: FlagTypeFloat},
			value:      "3.14.15",
			wantErr:    true,
			errContain: "must be a valid floating-point number",
		},
		{
			name:    "validation regex passes",
			flag:    Flag{Name: "env", Type: FlagTypeString, Validation: "^(dev|staging|prod)$"},
			value:   "prod",
			wantErr: false,
		},
		{
			name:       "validation regex fails",
			flag:       Flag{Name: "env", Type: FlagTypeString, Validation: "^(dev|staging|prod)$"},
			value:      "invalid",
			wantErr:    true,
			errContain: "does not match required pattern",
		},
		{
			name:    "empty type defaults to string",
			flag:    Flag{Name: "test"},
			value:   "anything",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.flag.ValidateFlagValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateFlagValue() should return error")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateFlagValue() error = %v, should contain %q", err, tt.errContain)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFlagValue() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseFlags_AllEnhancedFeatures(t *testing.T) {
	// Test a flag with all enhanced features together
	cueContent := `
cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
			}
		]
		flags: [
			{
				name: "environment"
				description: "Target environment"
				type: "string"
				required: true
				short: "t"
				validation: "^(dev|staging|prod)$"
			},
			{
				name: "dry-run"
				description: "Perform a dry run"
				type: "bool"
				default_value: "false"
				short: "d"
			},
			{
				name: "replicas"
				description: "Number of replicas"
				type: "int"
				default_value: "3"
				short: "r"
			},
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

	flags := inv.Commands[0].Flags
	if len(flags) != 3 {
		t.Fatalf("Expected 3 flags, got %d", len(flags))
	}

	// Check environment flag
	envFlag := flags[0]
	if envFlag.Name != "environment" {
		t.Errorf("Flag[0].Name = %q, want %q", envFlag.Name, "environment")
	}
	if envFlag.GetType() != FlagTypeString {
		t.Errorf("Flag[0].GetType() = %v, want %v", envFlag.GetType(), FlagTypeString)
	}
	if !envFlag.Required {
		t.Errorf("Flag[0].Required = false, want true")
	}
	if envFlag.Short != "t" {
		t.Errorf("Flag[0].Short = %q, want %q", envFlag.Short, "t")
	}
	if envFlag.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Flag[0].Validation = %q, want %q", envFlag.Validation, "^(dev|staging|prod)$")
	}

	// Check dry-run flag
	dryRunFlag := flags[1]
	if dryRunFlag.GetType() != FlagTypeBool {
		t.Errorf("Flag[1].GetType() = %v, want %v", dryRunFlag.GetType(), FlagTypeBool)
	}
	if dryRunFlag.DefaultValue != "false" {
		t.Errorf("Flag[1].DefaultValue = %q, want %q", dryRunFlag.DefaultValue, "false")
	}
	if dryRunFlag.Short != "d" {
		t.Errorf("Flag[1].Short = %q, want %q", dryRunFlag.Short, "d")
	}

	// Check replicas flag
	replicasFlag := flags[2]
	if replicasFlag.GetType() != FlagTypeInt {
		t.Errorf("Flag[2].GetType() = %v, want %v", replicasFlag.GetType(), FlagTypeInt)
	}
	if replicasFlag.DefaultValue != "3" {
		t.Errorf("Flag[2].DefaultValue = %q, want %q", replicasFlag.DefaultValue, "3")
	}
}

// ============================================================================
// Tests for Reserved Flag Names and Short Aliases
// ============================================================================

func TestValidateFlags_ReservedEnvFileName(t *testing.T) {
	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
			}
		]
		flags: [
			{name: "env-file", description: "This should fail - reserved flag name"}
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
		t.Fatal("Parse() should fail for reserved flag name 'env-file', got nil error")
	}

	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention 'reserved', got: %v", err)
	}
}

func TestValidateFlags_ReservedShortAliasE(t *testing.T) {
	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
			}
		]
		flags: [
			{name: "environment", short: "e", description: "This should fail - reserved short alias"}
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
		t.Fatal("Parse() should fail for reserved short alias 'e', got nil error")
	}

	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention 'reserved', got: %v", err)
	}
}
