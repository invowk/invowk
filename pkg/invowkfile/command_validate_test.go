// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestCommand_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value Command has empty Name which is nonzero-required — should fail.
	c := Command{}
	if c.Validate() == nil {
		t.Fatal("Command{}.Validate() should fail (empty Name is nonzero-required)")
	}
}

func TestCommand_Validate_Valid(t *testing.T) {
	t.Parallel()
	c := Command{
		Name:        "build",
		Description: "Build the project",
		Category:    "dev",
		Implementations: []Implementation{
			{
				Script:    "go build ./...",
				Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
				Platforms: AllPlatformConfigs(),
			},
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid Command.Validate() returned error: %v", err)
	}
}

func TestCommand_Validate_ValidMinimal(t *testing.T) {
	t.Parallel()
	// Only Name is required.
	c := Command{Name: "build"}
	if err := c.Validate(); err != nil {
		t.Fatalf("minimal Command.Validate() returned error: %v", err)
	}
}

func TestCommand_Validate_InvalidName(t *testing.T) {
	t.Parallel()
	c := Command{Name: ""}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with empty name should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
	var cmdErr *InvalidCommandError
	if !errors.As(err, &cmdErr) {
		t.Errorf("error should be *InvalidCommandError, got: %T", err)
	} else if len(cmdErr.FieldErrors) == 0 {
		t.Error("InvalidCommandError.FieldErrors should not be empty")
	}
}

func TestCommand_Validate_InvalidDescription(t *testing.T) {
	t.Parallel()
	c := Command{
		Name:        "build",
		Description: "   ", // whitespace-only
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with whitespace-only description should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidCategory(t *testing.T) {
	t.Parallel()
	c := Command{
		Name:     "build",
		Category: "   ", // whitespace-only
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with whitespace-only category should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidImplementation(t *testing.T) {
	t.Parallel()
	c := Command{
		Name: "build",
		Implementations: []Implementation{
			{Script: "   "}, // whitespace-only script
		},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with invalid implementation should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidEnv(t *testing.T) {
	t.Parallel()
	c := Command{
		Name: "build",
		Env: &EnvConfig{
			Vars: map[EnvVarName]string{"123-BAD": "value"},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with invalid Env should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidWorkDir(t *testing.T) {
	t.Parallel()
	c := Command{
		Name:    "build",
		WorkDir: "   ", // whitespace-only
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with whitespace-only WorkDir should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidDependsOn(t *testing.T) {
	t.Parallel()
	c := Command{
		Name: "build",
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{""}}},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with invalid DependsOn should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidFlags(t *testing.T) {
	t.Parallel()
	c := Command{
		Name:  "build",
		Flags: []Flag{{Name: ""}}, // invalid empty flag name
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with invalid flags should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidArgs(t *testing.T) {
	t.Parallel()
	c := Command{
		Name: "build",
		Args: []Argument{{Name: ""}}, // invalid empty arg name
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with invalid args should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_InvalidWatch(t *testing.T) {
	t.Parallel()
	c := Command{
		Name: "build",
		Watch: &WatchConfig{
			Patterns: []GlobPattern{"[invalid"},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with invalid Watch should fail")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("error should wrap ErrInvalidCommand, got: %v", err)
	}
}

func TestCommand_Validate_NilOptionalFields(t *testing.T) {
	t.Parallel()
	// nil Env, DependsOn, and Watch should all pass.
	c := Command{
		Name: "build",
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Command with nil optional fields should pass, got: %v", err)
	}
}

func TestCommand_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()
	c := Command{
		Name:     "",    // invalid
		Category: "   ", // invalid
		Flags:    []Flag{{Name: ""}},
		Args:     []Argument{{Name: ""}},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Command with multiple invalid fields should fail")
	}
	var cmdErr *InvalidCommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error should be *InvalidCommandError, got: %T", err)
	}
	if len(cmdErr.FieldErrors) < 3 {
		t.Errorf("expected at least 3 field errors, got %d", len(cmdErr.FieldErrors))
	}
}

func TestInvalidCommandError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidCommandError{FieldErrors: []error{errors.New("a")}}
	got := e.Error()
	want := "invalid command: 1 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidCommandError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidCommandError{}
	if !errors.Is(e, ErrInvalidCommand) {
		t.Error("Unwrap() should return ErrInvalidCommand")
	}
}
