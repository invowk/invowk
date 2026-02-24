// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Positional Arguments Feature (Basic Parsing)
// ============================================================================

func TestParseArgs_Basic(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "greet"
		description: "Greet a person"
		implementations: [
			{
				script: "echo Hello $INVOWK_ARG_NAME"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		args: [
			{name: "name", description: "The name to greet", required: true},
			{name: "greeting", description: "The greeting to use", default_value: "Hello"},
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
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(cmd.Args))
	}

	// First arg - required
	arg0 := cmd.Args[0]
	if arg0.Name != "name" {
		t.Errorf("Arg[0].Name = %q, want %q", arg0.Name, "name")
	}
	if arg0.Description != "The name to greet" {
		t.Errorf("Arg[0].Description = %q, want %q", arg0.Description, "The name to greet")
	}
	if !arg0.Required {
		t.Error("Arg[0].Required = false, want true")
	}

	// Second arg - optional with default
	arg1 := cmd.Args[1]
	if arg1.Name != "greeting" {
		t.Errorf("Arg[1].Name = %q, want %q", arg1.Name, "greeting")
	}
	if arg1.Required {
		t.Error("Arg[1].Required = true, want false")
	}
	if arg1.DefaultValue != "Hello" {
		t.Errorf("Arg[1].DefaultValue = %q, want %q", arg1.DefaultValue, "Hello")
	}
}

func TestParseArgs_WithTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		argType      string
		defaultValue string
		wantType     ArgumentType
	}{
		{"string type explicit", "string", "hello", ArgumentTypeString},
		{"int type with positive", "int", "42", ArgumentTypeInt},
		{"int type with zero", "int", "0", ArgumentTypeInt},
		{"int type with negative", "int", "-10", ArgumentTypeInt},
		{"float type with positive", "float", "3.14", ArgumentTypeFloat},
		{"float type with negative", "float", "-2.5", ArgumentTypeFloat},
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
		args: [
			{name: "myarg", description: "Test arg", type: "%s", default_value: "%s"},
		]
	}
]
`, tt.argType, tt.defaultValue)

			tmpDir := t.TempDir()
			invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
			if err := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invowkfile: %v", err)
			}

			inv, err := Parse(FilesystemPath(invowkfilePath))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			arg := inv.Commands[0].Args[0]
			if arg.GetType() != tt.wantType {
				t.Errorf("Arg.GetType() = %v, want %v", arg.GetType(), tt.wantType)
			}
			if arg.DefaultValue != tt.defaultValue {
				t.Errorf("Arg.DefaultValue = %v, want %v", arg.DefaultValue, tt.defaultValue)
			}
		})
	}
}

func TestParseArgs_TypeDefaultsToString(t *testing.T) {
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
		args: [
			{name: "myarg", description: "Test arg"},
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
		t.Fatalf("Parse() error = %v", err)
	}

	arg := inv.Commands[0].Args[0]
	if arg.Type != "" {
		t.Errorf("Arg.Type should be empty (unset), got %q", arg.Type)
	}
	if arg.GetType() != ArgumentTypeString {
		t.Errorf("Arg.GetType() should default to 'string', got %v", arg.GetType())
	}
}

func TestParseArgs_Variadic(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "copy"
		description: "Copy files to a destination"
		implementations: [
			{
				script: "cp $INVOWK_ARG_FILES $INVOWK_ARG_DEST"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		args: [
			{name: "dest", description: "Destination directory", required: true},
			{name: "files", description: "Source files", required: true, variadic: true},
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
		t.Fatalf("Parse() error = %v", err)
	}

	args := inv.Commands[0].Args
	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}

	// First arg - non-variadic
	if args[0].Variadic {
		t.Error("Arg[0].Variadic = true, want false")
	}

	// Second arg - variadic
	if !args[1].Variadic {
		t.Error("Arg[1].Variadic = false, want true")
	}
}

func TestParseArgs_ValidationRegex(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying to $INVOWK_ARG_ENV"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		args: [
			{name: "env", description: "Target environment", required: true, validation: "^(dev|staging|prod)$"},
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
		t.Fatalf("Parse() error = %v", err)
	}

	arg := inv.Commands[0].Args[0]
	if arg.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Arg.Validation = %q, want %q", arg.Validation, "^(dev|staging|prod)$")
	}
}

func TestParseArgs_EmptyList(t *testing.T) {
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
		args: []
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
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands[0].Args) != 0 {
		t.Errorf("Expected 0 args, got %d", len(inv.Commands[0].Args))
	}
}

func TestParseArgs_NoArgsField(t *testing.T) {
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
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands[0].Args) != 0 {
		t.Errorf("Expected nil or empty args, got %v", inv.Commands[0].Args)
	}
}

func TestParseArgs_AllFeatures(t *testing.T) {
	t.Parallel()

	// Test an arg with all features together
	cueContent := `
cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying to $INVOWK_ARG_ENV with $INVOWK_ARG_REPLICAS replicas"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		args: [
			{
				name: "env"
				description: "Target environment"
				type: "string"
				required: true
				validation: "^(dev|staging|prod)$"
			},
			{
				name: "replicas"
				description: "Number of replicas"
				type: "int"
				default_value: "3"
			},
			{
				name: "extra-args"
				description: "Extra arguments to pass"
				variadic: true
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

	inv, err := Parse(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	args := inv.Commands[0].Args
	if len(args) != 3 {
		t.Fatalf("Expected 3 args, got %d", len(args))
	}

	// Check env arg
	envArg := args[0]
	if envArg.Name != "env" {
		t.Errorf("Arg[0].Name = %q, want %q", envArg.Name, "env")
	}
	if envArg.GetType() != ArgumentTypeString {
		t.Errorf("Arg[0].GetType() = %v, want %v", envArg.GetType(), ArgumentTypeString)
	}
	if !envArg.Required {
		t.Error("Arg[0].Required = false, want true")
	}
	if envArg.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Arg[0].Validation = %q, want %q", envArg.Validation, "^(dev|staging|prod)$")
	}

	// Check replicas arg
	replicasArg := args[1]
	if replicasArg.GetType() != ArgumentTypeInt {
		t.Errorf("Arg[1].GetType() = %v, want %v", replicasArg.GetType(), ArgumentTypeInt)
	}
	if replicasArg.DefaultValue != "3" {
		t.Errorf("Arg[1].DefaultValue = %q, want %q", replicasArg.DefaultValue, "3")
	}
	if replicasArg.Required {
		t.Error("Arg[1].Required = true, want false")
	}

	// Check extra-args arg
	extraArg := args[2]
	if !extraArg.Variadic {
		t.Error("Arg[2].Variadic = false, want true")
	}
}

// ============================================================================
// Tests for CUE Generation with Args
// ============================================================================

func TestGenerateCUE_WithArgs(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy to environment",
				Implementations: []Implementation{
					{
						Script:    "echo deploying",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
				Args: []Argument{
					{
						Name:        "env",
						Description: "Target environment",
						Required:    true,
						Validation:  "^(dev|staging|prod)$",
					},
					{
						Name:         "replicas",
						Description:  "Number of replicas",
						Type:         ArgumentTypeInt,
						DefaultValue: "3",
					},
					{
						Name:        "services",
						Description: "Services to deploy",
						Variadic:    true,
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check args section exists
	if !strings.Contains(output, "args: [") {
		t.Error("GenerateCUE should contain 'args: ['")
	}

	// Check required arg
	if !strings.Contains(output, `name: "env"`) {
		t.Error("GenerateCUE should contain arg name 'env'")
	}
	if !strings.Contains(output, `description: "Target environment"`) {
		t.Error("GenerateCUE should contain arg description")
	}
	if !strings.Contains(output, "required: true") {
		t.Error("GenerateCUE should contain required: true for required arg")
	}
	if !strings.Contains(output, `validation: "^(dev|staging|prod)$"`) {
		t.Error("GenerateCUE should contain validation pattern")
	}

	// Check typed arg with default
	if !strings.Contains(output, `name: "replicas"`) {
		t.Error("GenerateCUE should contain arg name 'replicas'")
	}
	if !strings.Contains(output, `type: "int"`) {
		t.Error("GenerateCUE should contain type: int")
	}
	if !strings.Contains(output, `default_value: "3"`) {
		t.Error("GenerateCUE should contain default_value: 3")
	}

	// Check variadic arg
	if !strings.Contains(output, `name: "services"`) {
		t.Error("GenerateCUE should contain arg name 'services'")
	}
	if !strings.Contains(output, "variadic: true") {
		t.Error("GenerateCUE should contain variadic: true")
	}
}

func TestGenerateCUE_WithArgs_StringTypeNotIncluded(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name:        "greet",
				Description: "Greet someone",
				Implementations: []Implementation{
					{
						Script:    "echo hello",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
				Args: []Argument{
					{
						Name:        "name",
						Description: "Name to greet",
						Type:        ArgumentTypeString, // Default type
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// String type should NOT be explicitly included (it's the default)
	if strings.Contains(output, `type: "string"`) {
		t.Error("GenerateCUE should NOT include type: string (it's the default)")
	}

	// But the arg should still be there
	if !strings.Contains(output, `name: "name"`) {
		t.Error("GenerateCUE should contain the arg")
	}
}

func TestGenerateCUE_WithArgs_RoundTrip(t *testing.T) {
	t.Parallel()

	// Create an invowkfile with args, generate CUE, parse it back, and verify
	original := &Invowkfile{
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy application",
				Implementations: []Implementation{
					{
						Script:    "echo deploying",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
				Args: []Argument{
					{
						Name:        "env",
						Description: "Target environment",
						Required:    true,
					},
					{
						Name:         "count",
						Description:  "Replica count",
						Type:         ArgumentTypeInt,
						DefaultValue: "1",
					},
					{
						Name:        "extras",
						Description: "Extra params",
						Variadic:    true,
					},
				},
			},
		},
	}

	// Generate CUE
	cueContent := GenerateCUE(original)

	// Write to temp file and parse back
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	// Verify parsed args match original
	if len(parsed.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(parsed.Commands))
	}

	args := parsed.Commands[0].Args
	if len(args) != 3 {
		t.Fatalf("Expected 3 args, got %d", len(args))
	}

	// Check first arg
	if args[0].Name != "env" {
		t.Errorf("Args[0].Name = %q, want %q", args[0].Name, "env")
	}
	if !args[0].Required {
		t.Error("Args[0].Required should be true")
	}

	// Check second arg
	if args[1].Name != "count" {
		t.Errorf("Args[1].Name = %q, want %q", args[1].Name, "count")
	}
	if args[1].GetType() != ArgumentTypeInt {
		t.Errorf("Args[1].Type = %q, want %q", args[1].GetType(), ArgumentTypeInt)
	}
	if args[1].DefaultValue != "1" {
		t.Errorf("Args[1].DefaultValue = %q, want %q", args[1].DefaultValue, "1")
	}

	// Check third arg
	if args[2].Name != "extras" {
		t.Errorf("Args[2].Name = %q, want %q", args[2].Name, "extras")
	}
	if !args[2].Variadic {
		t.Error("Args[2].Variadic should be true")
	}
}
