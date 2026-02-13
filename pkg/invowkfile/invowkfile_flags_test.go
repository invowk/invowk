// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Flags Feature (Basic Parsing and Generation)
// ============================================================================

func TestParseFlags(t *testing.T) {
	t.Parallel()

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
			{name: "env", description: "Target environment"},
			{name: "dry-run", description: "Perform a dry run without making changes", default_value: "false"},
			{name: "verbose", description: "Enable verbose output"},
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Flags) != 3 {
		t.Fatalf("Expected 3 flags, got %d", len(cmd.Flags))
	}

	// First flag - no default value
	flag0 := cmd.Flags[0]
	if flag0.Name != "env" {
		t.Errorf("Flag[0].Name = %q, want %q", flag0.Name, "env")
	}
	if flag0.Description != "Target environment" {
		t.Errorf("Flag[0].Description = %q, want %q", flag0.Description, "Target environment")
	}
	if flag0.DefaultValue != "" {
		t.Errorf("Flag[0].DefaultValue = %q, want empty string", flag0.DefaultValue)
	}

	// Second flag - with default value
	flag1 := cmd.Flags[1]
	if flag1.Name != "dry-run" {
		t.Errorf("Flag[1].Name = %q, want %q", flag1.Name, "dry-run")
	}
	if flag1.Description != "Perform a dry run without making changes" {
		t.Errorf("Flag[1].Description = %q, want %q", flag1.Description, "Perform a dry run without making changes")
	}
	if flag1.DefaultValue != "false" {
		t.Errorf("Flag[1].DefaultValue = %q, want %q", flag1.DefaultValue, "false")
	}

	// Third flag
	flag2 := cmd.Flags[2]
	if flag2.Name != "verbose" {
		t.Errorf("Flag[2].Name = %q, want %q", flag2.Name, "verbose")
	}
}

func TestParseFlagsValidation_InvalidName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagName string
	}{
		{"starts with number", "1flag"},
		{"contains space", "my flag"},
		{"special characters", "flag@name"},
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
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "` + tt.flagName + `", description: "Test flag"},
		]
	}
]
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invowkfile: %v", writeErr)
			}

			_, err = Parse(invowkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject flag with invalid name %q", tt.flagName)
			}
		})
	}
}

func TestParseFlagsValidation_ValidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flagName string
	}{
		{"simple lowercase", "verbose"},
		{"with hyphen", "dry-run"},
		{"with underscore", "output_file"},
		{"with numbers", "retry3"},
		{"mixed case", "outputFile"},
		{"uppercase start", "Verbose"},
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
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "` + tt.flagName + `", description: "Test flag"},
		]
	}
]
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invowkfile: %v", writeErr)
			}

			inv, err := Parse(invowkfilePath)
			if err != nil {
				t.Errorf("Parse() should accept flag with valid name %q, got error: %v", tt.flagName, err)
				return
			}

			if len(inv.Commands[0].Flags) != 1 {
				t.Errorf("Expected 1 flag, got %d", len(inv.Commands[0].Flags))
			}
		})
	}
}

func TestParseFlagsValidation_EmptyDescription(t *testing.T) {
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
			{name: "verbose", description: "   "},
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err = Parse(invowkfilePath)
	if err == nil {
		t.Error("Parse() should reject flag with empty/whitespace-only description")
	}
}

func TestParseFlagsValidation_DuplicateNames(t *testing.T) {
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
			{name: "verbose", description: "Enable verbose output"},
			{name: "verbose", description: "Duplicate flag"},
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err = Parse(invowkfilePath)
	if err == nil {
		t.Error("Parse() should reject duplicate flag names")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Error should mention 'duplicate', got: %v", err)
	}
}

func TestGenerateCUE_WithFlags(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "deploy",
				Implementations: []Implementation{
					{Script: "echo deploy", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}},
				},
				Flags: []Flag{
					{Name: "env", Description: "Target environment"},
					{Name: "dry-run", Description: "Perform dry run", DefaultValue: "false"},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, "flags:") {
		t.Error("GenerateCUE should contain 'flags:'")
	}

	if !strings.Contains(output, `name: "env"`) {
		t.Error("GenerateCUE should contain flag name 'env'")
	}

	if !strings.Contains(output, `description: "Target environment"`) {
		t.Error("GenerateCUE should contain flag description")
	}

	if !strings.Contains(output, `default_value: "false"`) {
		t.Error("GenerateCUE should contain default_value for flags that have one")
	}
}

func TestGenerateCUE_WithoutFlags(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "build",
				Implementations: []Implementation{
					{Script: "echo build", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if strings.Contains(output, "flags:") {
		t.Error("GenerateCUE should not contain 'flags:' when there are no flags")
	}
}

func TestParseFlags_EmptyList(t *testing.T) {
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
		flags: []
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands[0].Flags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(inv.Commands[0].Flags))
	}
}

func TestParseFlags_NoFlagsField(t *testing.T) {
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
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands[0].Flags) != 0 {
		t.Errorf("Expected nil or empty flags, got %v", inv.Commands[0].Flags)
	}
}
