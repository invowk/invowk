// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for CUE Schema Validation
// ============================================================================

// TestCUESchema_RejectsCustomCheckWithBothNameAndAlternatives verifies that the CUE schema
// rejects custom checks that have both direct fields (name, script) AND alternatives
func TestCUESchema_RejectsCustomCheckWithBothNameAndAlternatives(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			custom_checks: [
				{
					name: "should-not-have-both"
					script: {content: "echo test"}
					alternatives: [
						{name: "alt1", script: {content: "echo alt1"}}
					]
				}
			]
		}
	}
]
`
	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(FilesystemPath(invowkfilePath))
	if err == nil {
		t.Error("Parse() should reject custom check with both direct fields and alternatives")
	}
	// The error could be about conflicting fields or disjunction not matching
	if !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "not allowed") {
		t.Logf("Warning: Error message may not be ideal, got: %v", err)
	}
}

func TestParse_InvowkfileWithoutModule_IsValid(t *testing.T) {
	t.Parallel()

	// invowkfile.cue now contains only commands - module metadata is in invowkmod.cue
	// An invowkfile without module field should be valid (module is not allowed in invowkfile.cue)
	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Parse() should accept invowkfile without module field: %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(inv.Commands))
	}
}

func TestGetFullCommandName(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Metadata: &ModuleMetadata{module: "my.module"},
	}

	tests := []struct {
		name     string
		cmdName  CommandName
		expected CommandName
	}{
		{"simple command", "build", "my.module build"},
		{"subcommand with space", "test unit", "my.module test unit"},
		{"nested subcommand", "db migrate up", "my.module db migrate up"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := inv.GetFullCommandName(tt.cmdName)
			if result != tt.expected {
				t.Errorf("GetFullCommandName(%q) = %q, want %q", tt.cmdName, result, tt.expected)
			}
		})
	}
}

func TestListCommands_WithModule(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Metadata: &ModuleMetadata{module: "mymodule"},
		Commands: []Command{
			{Name: "build"},
			{Name: "test"},
			{Name: "deploy prod"},
		},
	}

	names := inv.ListCommands()

	expected := []string{"mymodule build", "mymodule test", "mymodule deploy prod"}
	if len(names) != len(expected) {
		t.Fatalf("ListCommands() returned %d names, want %d", len(names), len(expected))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ListCommands()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestFlattenCommands_WithModule(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Metadata: &ModuleMetadata{module: "mymodule"},
		Commands: []Command{
			{Name: "build", Description: "Build command"},
			{Name: "test unit", Description: "Unit tests"},
		},
	}

	flat := inv.FlattenCommands()

	if len(flat) != 2 {
		t.Fatalf("FlattenCommands() returned %d commands, want 2", len(flat))
	}

	// Check that keys are prefixed with module
	if _, ok := flat["mymodule build"]; !ok {
		t.Error("FlattenCommands() should have key 'mymodule build'")
	}

	if _, ok := flat["mymodule test unit"]; !ok {
		t.Error("FlattenCommands() should have key 'mymodule test unit'")
	}

	// Check that commands are correct
	if flat["mymodule build"].Description != "Build command" {
		t.Errorf("flat['mymodule build'].Description = %q, want %q", flat["mymodule build"].Description, "Build command")
	}
}

func TestGenerateCUE_OutputsCommandContent(t *testing.T) {
	t.Parallel()

	// GenerateCUE only generates command content (invowkfile.cue)
	// Module metadata is generated separately for invowkmod.cue
	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "test",
				Implementations: []Implementation{
					{Script: ImplementationScript{Content: "echo test"}, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Should contain commands section
	if !strings.Contains(output, "cmds:") {
		t.Error("GenerateCUE should contain 'cmds:'")
	}

	// Should NOT contain module (module is in invowkmod.cue, not invowkfile.cue)
	if strings.Contains(output, "module:") {
		t.Error("GenerateCUE should NOT contain 'module:' - module metadata goes in invowkmod.cue")
	}
}

// ============================================================================
// Tests for Interpreter Validation (empty/whitespace rejection)
// ============================================================================

// TestCUESchema_RejectsEmptyInterpreter verifies that the CUE schema rejects
// empty interpreter values when the field is explicitly declared.
func TestCUESchema_RejectsEmptyInterpreter(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test", interpreter: ""}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(FilesystemPath(invowkfilePath))
	if err == nil {
		t.Error("Parse() should reject empty interpreter when explicitly declared")
	}
}

// TestCUESchema_RejectsWhitespaceOnlyInterpreter verifies that the CUE schema rejects
// whitespace-only interpreter values when the field is explicitly declared.
func TestCUESchema_RejectsWhitespaceOnlyInterpreter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interpreter string
	}{
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tab", "\t"},
		{"mixed whitespace", "  \t  "},
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
				script: {content: "echo test", interpreter: "` + tt.interpreter + `"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
			tmpDir := t.TempDir()
			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invowkfile: %v", err)
			}

			_, err := Parse(FilesystemPath(invowkfilePath))
			if err == nil {
				t.Errorf("Parse() should reject whitespace-only interpreter %q", tt.interpreter)
			}
		})
	}
}

// TestCUESchema_RejectsEmptyInterpreterForContainer verifies that the CUE schema
// rejects empty interpreter for container runtime as well.
func TestCUESchema_RejectsEmptyInterpreterForContainer(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test", interpreter: ""}
				runtimes: [{name: "container", image: "debian:stable-slim"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(FilesystemPath(invowkfilePath))
	if err == nil {
		t.Error("Parse() should reject empty interpreter for container runtime")
	}
}

// TestValidateImplementationScript_ValidInterpreters tests that script validation accepts valid interpreters.
// Note: Whitespace-only interpreter rejection is now handled by CUE schema validation:
// interpreter?: string & =~"^\\s*\\S.*$" (requires at least one non-whitespace char)
// See TestParse_RejectsEmptyInterpreter_NativeRuntime and TestParse_RejectsEmptyInterpreter_ContainerRuntime
// for CUE-level validation tests.
func TestValidateImplementationScript_ValidInterpreters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interpreter string
	}{
		{"empty string is valid (field not declared)", ""},
		{"valid interpreter - auto", "auto"},
		{"valid interpreter - python3", "python3"},
		{"valid interpreter - with leading space", " python3"}, // Has non-whitespace content
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			script := ImplementationScript{
				Content:     "echo test",
				Interpreter: InterpreterSpec(tt.interpreter),
			}

			err := script.Validate()
			if err != nil {
				t.Errorf("ImplementationScript.Validate() unexpected error for interpreter %q: %v", tt.interpreter, err)
			}
		})
	}
}

func TestValidateRuntimeConfig_EnvInheritMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    EnvInheritMode
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"none", EnvInheritNone, false},
		{"allow", EnvInheritAllow, false},
		{"all", EnvInheritAll, false},
		{"invalid", EnvInheritMode("nope"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := &RuntimeConfig{
				Name:           RuntimeNative,
				EnvInheritMode: tt.mode,
			}

			err := validateRuntimeConfig(rt, "test-cmd", 1)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateRuntimeConfig() should return error for env_inherit_mode %q", tt.mode)
				}
			} else if err != nil {
				t.Errorf("validateRuntimeConfig() unexpected error for env_inherit_mode %q: %v", tt.mode, err)
			}
		})
	}
}

func TestValidateRuntimeConfig_EnvInheritNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rt      *RuntimeConfig
		wantErr bool
	}{
		{
			name: "valid allow and deny lists",
			rt: &RuntimeConfig{
				Name:            RuntimeNative,
				EnvInheritMode:  EnvInheritAllow,
				EnvInheritAllow: []EnvVarName{"TERM", "LANG", "MY_VAR1"},
				EnvInheritDeny:  []EnvVarName{"AWS_SECRET_ACCESS_KEY"},
			},
			wantErr: false,
		},
		{
			name: "invalid allow name",
			rt: &RuntimeConfig{
				Name:            RuntimeNative,
				EnvInheritAllow: []EnvVarName{"TERM", "BAD-VAR"},
			},
			wantErr: true,
		},
		{
			name: "invalid deny name",
			rt: &RuntimeConfig{
				Name:           RuntimeNative,
				EnvInheritDeny: []EnvVarName{"OK", "NO=PE"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateRuntimeConfig(tt.rt, "test-cmd", 1)
			if tt.wantErr && err == nil {
				t.Errorf("validateRuntimeConfig() should return error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateRuntimeConfig() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRuntimeConfig_EnvInheritAllowRequiresAllowMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    EnvInheritMode
		wantErr bool
	}{
		{"omitted mode", "", true},
		{"none mode", EnvInheritNone, true},
		{"all mode", EnvInheritAll, true},
		{"allow mode", EnvInheritAllow, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := &RuntimeConfig{
				Name:            RuntimeNative,
				EnvInheritMode:  tt.mode,
				EnvInheritAllow: []EnvVarName{"PATH"},
			}

			err := validateRuntimeConfig(rt, "test-cmd", 1)
			if tt.wantErr {
				if err == nil {
					t.Fatal("validateRuntimeConfig() error = nil, want allow mode requirement error")
				}
				if !strings.Contains(err.Error(), `env_inherit_allow requires env_inherit_mode: "allow"`) {
					t.Fatalf("validateRuntimeConfig() error = %v, want allow mode requirement", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateRuntimeConfig() error = %v, want nil", err)
			}
		})
	}
}

func TestParse_RejectsEnvInheritAllowWithoutAllowMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode string
	}{
		{"omitted mode", ""},
		{"none mode", `env_inherit_mode: "none"`},
		{"all mode", `env_inherit_mode: "all"`},
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
				script: {content: "echo test"}
				runtimes: [{
					name: "native"
					` + tt.mode + `
					env_inherit_allow: ["PATH"]
				}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
			tmpDir := t.TempDir()
			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invowkfile: %v", err)
			}

			_, err := Parse(FilesystemPath(invowkfilePath))
			if err == nil {
				t.Fatal("Parse() error = nil, want allow mode requirement error")
			}
			if !strings.Contains(err.Error(), `env_inherit_allow requires env_inherit_mode: "allow"`) {
				t.Fatalf("Parse() error = %v, want allow mode requirement", err)
			}
		})
	}
}

func TestParse_AcceptsEnvInheritAllowWithAllowMode(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test"}
				runtimes: [{
					name: "native"
					env_inherit_mode: "allow"
					env_inherit_allow: ["PATH", "TERM"]
				}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	if _, err := Parse(FilesystemPath(invowkfilePath)); err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
}

func TestParse_RejectsFlagMissingDescription(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [{name: "verbose"}]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	_, err := Parse(FilesystemPath(invowkfilePath))
	if err == nil {
		t.Fatal("Parse() error = nil, want missing flag description error")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Fatalf("Parse() error = %v, want description error", err)
	}
}

func TestParse_RejectsArgumentMissingDescription(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo test"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		args: [{name: "file"}]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
		t.Fatalf("Failed to write invowkfile: %v", err)
	}

	_, err := Parse(FilesystemPath(invowkfilePath))
	if err == nil {
		t.Fatal("Parse() error = nil, want missing argument description error")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Fatalf("Parse() error = %v, want description error", err)
	}
}

// TestParseInterpreter_ValidValues verifies that valid interpreter values work correctly.
func TestParseInterpreter_ValidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interpreter string
	}{
		{"auto", "auto"},
		{"simple command", "python3"},
		{"with path", "/usr/bin/python3"},
		{"with args", "python3 -u"},
		{"shebang-style", "/usr/bin/env python3"},
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
				script: {content: "print('hello')", interpreter: "` + tt.interpreter + `"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
			tmpDir := t.TempDir()
			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invowkfile: %v", err)
			}

			inv, err := Parse(FilesystemPath(invowkfilePath))
			if err != nil {
				t.Fatalf("Parse() should accept valid interpreter %q, got error: %v", tt.interpreter, err)
			}

			script := inv.Commands[0].Implementations[0].Script
			if script.Interpreter != InterpreterSpec(tt.interpreter) {
				t.Errorf("ImplementationScript.Interpreter = %q, want %q", script.Interpreter, tt.interpreter)
			}
		})
	}
}

// TestParseInterpreter_OmittedFieldIsValid verifies that omitting the interpreter
// field entirely is valid (defaults to auto-detection).
func TestParseInterpreter_OmittedFieldIsValid(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: {content: "echo hello"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Parse() should accept omitted interpreter field, got error: %v", err)
	}

	script := inv.Commands[0].Implementations[0].Script
	if script.Interpreter != "" {
		t.Errorf("ImplementationScript.Interpreter should be empty when omitted, got %q", script.Interpreter)
	}
}
