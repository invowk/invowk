// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestInvowkfile_Validate_CollectsAllErrors(t *testing.T) {
	t.Parallel()

	// Create an invowkfile with multiple validation errors
	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name:        "test",
				Description: strings.Repeat("x", MaxDescriptionLength+1), // Error: too long
				Implementations: []Implementation{
					{
						Script:   "", // Error: empty script
						Runtimes: []RuntimeConfig{},
					},
				},
				Flags: []Flag{
					{Name: "verbose", Description: ""}, // Error: empty description
				},
			},
		},
	}

	errs := inv.Validate()

	// Should have multiple errors, not just the first one
	// Expected errors: description too long, empty script, no runtimes, empty flag description
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors to be collected, got %d: %v", len(errs), errs)
	}
}

func TestInvowkfile_Validate_DefaultValidators(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name:        "build",
				Description: "Build the project",
				Implementations: []Implementation{
					{
						Script: "echo 'building'",
						Runtimes: []RuntimeConfig{
							{Name: RuntimeNative},
						},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	errs := inv.Validate()
	if len(errs) > 0 {
		t.Errorf("valid invowkfile should have no errors, got: %v", errs)
	}
}

func TestInvowkfile_Validate_WithStrictMode(t *testing.T) {
	t.Parallel()

	// Create a custom validator that returns a warning
	warningValidator := &mockValidator{
		name: "warning-validator",
		errors: []ValidationError{
			{Validator: "warning-validator", Message: "this is a warning", Severity: SeverityWarning},
		},
	}

	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name: "test",
				Implementations: []Implementation{
					{
						Script:    "echo test",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	// Without strict mode, the warning should remain a warning
	errs := inv.Validate(WithValidators(warningValidator))
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity without strict mode, got %v", errs[0].Severity)
	}

	// With strict mode, the warning should become an error
	errs = inv.Validate(WithValidators(warningValidator), WithStrictMode(true))
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Severity != SeverityError {
		t.Errorf("expected error severity with strict mode, got %v", errs[0].Severity)
	}
}

func TestInvowkfile_Validate_WithFS(t *testing.T) {
	t.Parallel()

	// Create a test filesystem
	testFS := fstest.MapFS{
		"Containerfile": &fstest.MapFile{Data: []byte("FROM debian:stable-slim")},
	}

	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name: "container-cmd",
				Implementations: []Implementation{
					{
						Script: "echo test",
						Runtimes: []RuntimeConfig{
							{
								Name:          RuntimeContainer,
								Containerfile: "Containerfile",
							},
						},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	// The FS option should be passed to the validation context
	errs := inv.Validate(WithFS(testFS))

	// The validation should pass (no errors about containerfile path)
	if len(errs) > 0 {
		t.Errorf("expected no errors with valid containerfile, got: %v", errs)
	}
}

func TestInvowkfile_Validate_WithAdditionalValidators(t *testing.T) {
	t.Parallel()

	customValidator := &mockValidator{
		name: "custom",
		errors: []ValidationError{
			{Validator: "custom", Message: "custom validation error", Severity: SeverityError},
		},
	}

	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name: "test",
				Implementations: []Implementation{
					{
						Script:    "echo test",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	// With additional validators, should include both default and custom errors
	errs := inv.Validate(WithAdditionalValidators(customValidator))

	// Should have the custom error
	found := false
	for _, e := range errs {
		if e.Message == "custom validation error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected custom validation error, got: %v", errs)
	}
}

func TestInvowkfile_Validate_NoCommands(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{}, // Empty
	}

	errs := inv.Validate()
	if len(errs) == 0 {
		t.Error("expected error for invowkfile with no commands")
	}
	if !strings.Contains(errs.Error(), "no commands") {
		t.Errorf("expected 'no commands' error, got: %v", errs)
	}
}

func TestInvowkfile_Validate_ContainerRuntimeErrors(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name: "container-test",
				Implementations: []Implementation{
					{
						Script: "echo test",
						Runtimes: []RuntimeConfig{
							{
								Name:          RuntimeContainer,
								Containerfile: "",
								Image:         "", // Both empty - should error
							},
						},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	errs := inv.Validate()
	if len(errs) == 0 {
		t.Error("expected error for container runtime without image or containerfile")
	}

	errStr := errs.Error()
	if !strings.Contains(errStr, "container runtime requires") {
		t.Errorf("expected container requirement error, got: %v", errStr)
	}
}

func TestInvowkfile_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()

	// Test with multiple different validation issues
	inv := &Invowkfile{
		FilePath: "/test/invowkfile.cue",
		Commands: []Command{
			{
				Name: "test",
				Implementations: []Implementation{
					{
						Script: "echo test",
						Runtimes: []RuntimeConfig{
							{Name: RuntimeNative},
						},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
				Flags: []Flag{
					{Name: "verbose", Description: ""}, // Empty description
					{Name: "", Description: "test"},    // Empty name
				},
			},
		},
	}

	errs := inv.Validate()

	// Should collect multiple errors from flags
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}

	// Verify we can filter errors and warnings
	errors := errs.Errors()
	for _, e := range errors {
		if e.Severity != SeverityError {
			t.Errorf("Errors() returned non-error: %v", e)
		}
	}
}

func TestDefaultValidators(t *testing.T) {
	t.Parallel()

	validators := DefaultValidators()
	if len(validators) == 0 {
		t.Error("DefaultValidators() should return at least one validator")
	}

	// Check that StructureValidator is included
	found := false
	for _, v := range validators {
		if v.Name() == "structure" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DefaultValidators() should include StructureValidator")
	}
}

func TestNewDefaultValidator(t *testing.T) {
	t.Parallel()

	cv := NewDefaultValidator()
	if cv == nil {
		t.Fatal("NewDefaultValidator() returned nil")
	}
	if cv.Count() == 0 {
		t.Error("NewDefaultValidator() should have at least one validator")
	}
}
