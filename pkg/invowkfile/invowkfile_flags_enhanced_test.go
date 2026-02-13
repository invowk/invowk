// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Enhanced Flags Feature - Validation and Integration
// ============================================================================

func TestParseFlags_ValidationRegex(t *testing.T) {
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
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Flag.Validation = %q, want %q", flag.Validation, "^(dev|staging|prod)$")
	}
}

func TestParseFlagsValidation_InvalidRegex(t *testing.T) {
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
			{name: "myflag", description: "Test flag", validation: "[invalid(regex"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject invalid validation regex")
	}
}

func TestParseFlagsValidation_DefaultNotMatchingValidation(t *testing.T) {
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
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "invalid"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject default_value that doesn't match validation pattern")
	}
}

func TestParseFlags_DefaultMatchesValidation(t *testing.T) {
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
			{name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "prod"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() should accept default_value that matches validation, got error: %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if flag.DefaultValue != "prod" {
		t.Errorf("Flag.DefaultValue = %q, want %q", flag.DefaultValue, "prod")
	}
}

func TestValidateFlagValue(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

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
				platforms: [{name: "linux"}]
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
				short: "n"
			},
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
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
