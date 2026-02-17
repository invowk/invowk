// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// Positional argument tests
// ---------------------------------------------------------------------------

func TestArgNameToEnvVar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "env",
			expected: "INVOWK_ARG_ENV",
		},
		{
			name:     "name with hyphen",
			input:    "output-file",
			expected: "INVOWK_ARG_OUTPUT_FILE",
		},
		{
			name:     "name with multiple hyphens",
			input:    "my-config-path",
			expected: "INVOWK_ARG_MY_CONFIG_PATH",
		},
		{
			name:     "mixed case",
			input:    "myArg",
			expected: "INVOWK_ARG_MYARG",
		},
		{
			name:     "already uppercase",
			input:    "VERBOSE",
			expected: "INVOWK_ARG_VERBOSE",
		},
		{
			name:     "single character",
			input:    "v",
			expected: "INVOWK_ARG_V",
		},
		{
			name:     "numeric suffix",
			input:    "arg1",
			expected: "INVOWK_ARG_ARG1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "INVOWK_ARG_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := invowkfile.ArgNameToEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("invowkfile.ArgNameToEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildCommandUsageString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmdPart  string
		args     []invowkfile.Argument
		expected string
	}{
		{
			name:     "no arguments",
			cmdPart:  "deploy",
			args:     []invowkfile.Argument{},
			expected: "deploy",
		},
		{
			name:    "single required argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: true},
			},
			expected: "deploy <env>",
		},
		{
			name:    "single optional argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: false},
			},
			expected: "deploy [env]",
		},
		{
			name:    "required and optional arguments",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: true},
				{Name: "replicas", Required: false},
			},
			expected: "deploy <env> [replicas]",
		},
		{
			name:    "required variadic argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "services", Required: true, Variadic: true},
			},
			expected: "deploy <services>...",
		},
		{
			name:    "optional variadic argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "services", Required: false, Variadic: true},
			},
			expected: "deploy [services]...",
		},
		{
			name:    "multiple args with variadic",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: true},
				{Name: "replicas", Required: false},
				{Name: "services", Required: false, Variadic: true},
			},
			expected: "deploy <env> [replicas] [services]...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildCommandUsageString(tt.cmdPart, tt.args)
			if result != tt.expected {
				t.Errorf("buildCommandUsageString(%q, ...) = %q, want %q", tt.cmdPart, result, tt.expected)
			}
		})
	}
}

func TestBuildArgsDocumentation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []invowkfile.Argument
		shouldHave    []string
		shouldNotHave []string
	}{
		{
			name: "required argument",
			args: []invowkfile.Argument{
				{Name: "env", Description: "Target environment", Required: true},
			},
			shouldHave: []string{"env", "(required)", "Target environment"},
		},
		{
			name: "optional with default",
			args: []invowkfile.Argument{
				{Name: "replicas", Description: "Number of replicas", DefaultValue: "1"},
			},
			shouldHave: []string{"replicas", `(default: "1")`, "Number of replicas"},
		},
		{
			name: "optional without default",
			args: []invowkfile.Argument{
				{Name: "tag", Description: "Image tag"},
			},
			shouldHave: []string{"tag", "(optional)", "Image tag"},
		},
		{
			name: "typed argument",
			args: []invowkfile.Argument{
				{Name: "count", Description: "Count value", Type: invowkfile.ArgumentTypeInt},
			},
			shouldHave: []string{"count", "[int]", "Count value"},
		},
		{
			name: "variadic argument",
			args: []invowkfile.Argument{
				{Name: "services", Description: "Services to deploy", Variadic: true},
			},
			shouldHave: []string{"services", "(variadic)", "Services to deploy"},
		},
		{
			name: "string type not shown",
			args: []invowkfile.Argument{
				{Name: "name", Description: "Name", Type: invowkfile.ArgumentTypeString},
			},
			shouldHave:    []string{"name", "Name"},
			shouldNotHave: []string{"[string]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildArgsDocumentation(tt.args)

			for _, s := range tt.shouldHave {
				if !strings.Contains(result, s) {
					t.Errorf("buildArgsDocumentation() should contain %q, got: %q", s, result)
				}
			}

			for _, s := range tt.shouldNotHave {
				if strings.Contains(result, s) {
					t.Errorf("buildArgsDocumentation() should NOT contain %q, got: %q", s, result)
				}
			}
		})
	}
}

func TestArgumentValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ArgumentValidationError
		expected string
	}{
		{
			name: "missing required",
			err: &ArgumentValidationError{
				Type:         ArgErrMissingRequired,
				CommandName:  "deploy",
				MinArgs:      2,
				ProvidedArgs: []string{"prod"},
			},
			expected: "missing required arguments for command 'deploy': expected at least 2, got 1",
		},
		{
			name: "too many",
			err: &ArgumentValidationError{
				Type:         ArgErrTooMany,
				CommandName:  "deploy",
				MaxArgs:      2,
				ProvidedArgs: []string{"prod", "3", "extra"},
			},
			expected: "too many arguments for command 'deploy': expected at most 2, got 3",
		},
		{
			name: "invalid value",
			err: &ArgumentValidationError{
				Type:         ArgErrInvalidValue,
				CommandName:  "deploy",
				InvalidArg:   "replicas",
				InvalidValue: "abc",
				ValueError:   fmt.Errorf("not a valid integer"),
			},
			expected: "invalid value for argument 'replicas': not a valid integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateArguments_NoDefinitions(t *testing.T) {
	t.Parallel()

	// When no arg definitions exist, any arguments should be allowed (backward compatible)
	err := validateArguments("test-cmd", []string{"foo", "bar", "baz"}, nil)
	if err != nil {
		t.Errorf("Expected nil error when no arg definitions, got: %v", err)
	}

	err = validateArguments("test-cmd", []string{}, nil)
	if err != nil {
		t.Errorf("Expected nil error when no args and no definitions, got: %v", err)
	}

	err = validateArguments("test-cmd", []string{"anything"}, []invowkfile.Argument{})
	if err != nil {
		t.Errorf("Expected nil error when empty arg definitions, got: %v", err)
	}
}

func TestValidateArguments_MissingRequired(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
		{Name: "region", Description: "AWS region", Required: true},
	}

	// No arguments provided when 2 are required
	err := validateArguments("deploy", []string{}, argDefs)
	if err == nil {
		t.Fatal("Expected error for missing required arguments")
	}

	argErr, ok := errors.AsType[*ArgumentValidationError](err)
	if !ok {
		t.Fatalf("Expected ArgumentValidationError, got: %T", err)
	}

	if argErr.Type != ArgErrMissingRequired {
		t.Errorf("Expected ArgErrMissingRequired, got: %v", argErr.Type)
	}
	if argErr.MinArgs != 2 {
		t.Errorf("Expected MinArgs=2, got: %d", argErr.MinArgs)
	}

	// Only 1 argument provided when 2 are required
	err = validateArguments("deploy", []string{"prod"}, argDefs)
	if err == nil {
		t.Fatal("Expected error for missing required arguments")
	}
}

func TestValidateArguments_TooManyArgs(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
	}

	// 3 arguments provided when only 1 is expected
	err := validateArguments("deploy", []string{"prod", "extra1", "extra2"}, argDefs)
	if err == nil {
		t.Fatal("Expected error for too many arguments")
	}

	argErr, ok := errors.AsType[*ArgumentValidationError](err)
	if !ok {
		t.Fatalf("Expected ArgumentValidationError, got: %T", err)
	}

	if argErr.Type != ArgErrTooMany {
		t.Errorf("Expected ArgErrTooMany, got: %v", argErr.Type)
	}
	if argErr.MaxArgs != 1 {
		t.Errorf("Expected MaxArgs=1, got: %d", argErr.MaxArgs)
	}
}

func TestValidateArguments_VariadicAllowsExtra(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
		{Name: "services", Description: "Services to deploy", Variadic: true},
	}

	// Many arguments should be allowed when last arg is variadic
	err := validateArguments("deploy", []string{"prod", "svc1", "svc2", "svc3", "svc4"}, argDefs)
	if err != nil {
		t.Errorf("Expected no error with variadic args, got: %v", err)
	}

	// Just the required arg should also be valid
	err = validateArguments("deploy", []string{"prod"}, argDefs)
	if err != nil {
		t.Errorf("Expected no error with just required arg, got: %v", err)
	}
}

func TestValidateArguments_InvalidValue(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "replicas", Description: "Number of replicas", Type: invowkfile.ArgumentTypeInt, Required: true},
	}

	// Invalid integer value
	err := validateArguments("scale", []string{"not-a-number"}, argDefs)
	if err == nil {
		t.Fatal("Expected error for invalid argument value")
	}

	argErr, ok := errors.AsType[*ArgumentValidationError](err)
	if !ok {
		t.Fatalf("Expected ArgumentValidationError, got: %T", err)
	}

	if argErr.Type != ArgErrInvalidValue {
		t.Errorf("Expected ArgErrInvalidValue, got: %v", argErr.Type)
	}
	if argErr.InvalidArg != "replicas" {
		t.Errorf("Expected InvalidArg='replicas', got: %q", argErr.InvalidArg)
	}
	if argErr.InvalidValue != "not-a-number" {
		t.Errorf("Expected InvalidValue='not-a-number', got: %q", argErr.InvalidValue)
	}
}

func TestValidateArguments_ValidTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argDefs []invowkfile.Argument
		args    []string
		wantErr bool
	}{
		{
			name: "valid integer",
			argDefs: []invowkfile.Argument{
				{Name: "count", Type: invowkfile.ArgumentTypeInt, Required: true},
			},
			args:    []string{"42"},
			wantErr: false,
		},
		{
			name: "valid float",
			argDefs: []invowkfile.Argument{
				{Name: "scale", Type: invowkfile.ArgumentTypeFloat, Required: true},
			},
			args:    []string{"3.14"},
			wantErr: false,
		},
		{
			name: "string allows anything",
			argDefs: []invowkfile.Argument{
				{Name: "message", Type: invowkfile.ArgumentTypeString, Required: true},
			},
			args:    []string{"Hello, World!"},
			wantErr: false,
		},
		{
			name: "default type is string",
			argDefs: []invowkfile.Argument{
				{Name: "input", Required: true}, // No type specified
			},
			args:    []string{"any value works"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateArguments("test-cmd", tt.args, tt.argDefs)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArguments() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateArguments_OptionalWithDefault(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
		{Name: "replicas", Description: "Number of replicas", DefaultValue: "3"},
	}

	// Just the required argument
	err := validateArguments("deploy", []string{"prod"}, argDefs)
	if err != nil {
		t.Errorf("Expected no error when optional args have defaults, got: %v", err)
	}

	// Both arguments provided
	err = validateArguments("deploy", []string{"prod", "5"}, argDefs)
	if err != nil {
		t.Errorf("Expected no error when all args provided, got: %v", err)
	}
}

func TestRenderArgumentValidationError_MissingRequired(t *testing.T) {
	t.Parallel()

	err := &ArgumentValidationError{
		Type:        ArgErrMissingRequired,
		CommandName: "deploy",
		ArgDefs: []invowkfile.Argument{
			{Name: "env", Description: "Target environment", Required: true},
			{Name: "replicas", Description: "Number of replicas", DefaultValue: "1"},
		},
		ProvidedArgs: []string{},
		MinArgs:      1,
	}

	output := RenderArgumentValidationError(err)

	if !strings.Contains(output, "Missing required arguments") {
		t.Error("Should contain 'Missing required arguments'")
	}
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}
	if !strings.Contains(output, "env") {
		t.Error("Should contain argument name 'env'")
	}
	if !strings.Contains(output, "(required)") {
		t.Error("Should indicate required arguments")
	}
	if !strings.Contains(output, "--help") {
		t.Error("Should contain help hint")
	}
}

func TestRenderArgumentValidationError_TooMany(t *testing.T) {
	t.Parallel()

	err := &ArgumentValidationError{
		Type:        ArgErrTooMany,
		CommandName: "deploy",
		ArgDefs: []invowkfile.Argument{
			{Name: "env", Description: "Target environment"},
		},
		ProvidedArgs: []string{"prod", "extra1", "extra2"},
		MaxArgs:      1,
	}

	output := RenderArgumentValidationError(err)

	if !strings.Contains(output, "Too many arguments") {
		t.Error("Should contain 'Too many arguments'")
	}
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}
	if !strings.Contains(output, "Provided:") {
		t.Error("Should show provided arguments")
	}
}

func TestRenderArgumentValidationError_InvalidValue(t *testing.T) {
	t.Parallel()

	err := &ArgumentValidationError{
		Type:         ArgErrInvalidValue,
		CommandName:  "deploy",
		InvalidArg:   "replicas",
		InvalidValue: "abc",
		ValueError:   fmt.Errorf("must be a valid integer"),
	}

	output := RenderArgumentValidationError(err)

	if !strings.Contains(output, "Invalid argument value") {
		t.Error("Should contain 'Invalid argument value'")
	}
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}
	if !strings.Contains(output, "'replicas'") {
		t.Error("Should contain argument name")
	}
	if !strings.Contains(output, "abc") {
		t.Error("Should contain invalid value")
	}
	if !strings.Contains(output, "must be a valid integer") {
		t.Error("Should contain error message")
	}
}

func TestRenderArgsSubcommandConflictError(t *testing.T) {
	t.Parallel()

	err := &discovery.ArgsSubcommandConflictError{
		CommandName: "deploy",
		Args: []invowkfile.Argument{
			{Name: "env", Description: "Target environment"},
			{Name: "replicas", Description: "Number of replicas"},
		},
		Subcommands: []string{"deploy status", "deploy logs"},
		FilePath:    "/test/invowkfile.cue",
	}

	output := RenderArgsSubcommandConflictError(err)

	// Check header - now uses error styling with "Invalid command structure"
	if !strings.Contains(output, "Invalid command structure") {
		t.Error("Should contain 'Invalid command structure' header")
	}

	// Check command name
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}

	// Check file path is shown
	if !strings.Contains(output, "/test/invowkfile.cue") {
		t.Error("Should contain file path")
	}

	// Check args are listed
	if !strings.Contains(output, "env") {
		t.Error("Should list argument 'env'")
	}
	if !strings.Contains(output, "replicas") {
		t.Error("Should list argument 'replicas'")
	}

	// Check subcommands are listed
	if !strings.Contains(output, "deploy status") {
		t.Error("Should list subcommand 'deploy status'")
	}
	if !strings.Contains(output, "deploy logs") {
		t.Error("Should list subcommand 'deploy logs'")
	}

	// Check hint
	if !strings.Contains(output, "Remove either the 'args' field or the subcommands") {
		t.Error("Should contain resolution hint")
	}
}
