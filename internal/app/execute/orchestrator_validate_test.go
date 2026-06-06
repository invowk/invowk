// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestBuildExecutionContextOptions_Validate(t *testing.T) {
	t.Parallel()

	validSelection := RuntimeSelectionOf(invowkfile.RuntimeNative, &invowkfile.Implementation{})

	tests := []struct {
		name    string
		opts    BuildExecutionContextOptions
		wantErr bool
	}{
		{
			name: "valid with required fields",
			opts: BuildExecutionContextOptions{
				Selection: validSelection,
			},
			wantErr: false,
		},
		{
			name: "invalid selection (zero value)",
			opts: BuildExecutionContextOptions{
				Selection: RuntimeSelection{},
			},
			wantErr: true,
		},
		{
			name: "valid with source ID and platform",
			opts: BuildExecutionContextOptions{
				Selection: validSelection,
				SourceID:  discovery.SourceIDInvowkfile,
				Platform:  invowkfile.PlatformLinux,
			},
			wantErr: false,
		},
		{
			name: "invalid env inherit mode",
			opts: BuildExecutionContextOptions{
				Selection:      validSelection,
				EnvInheritMode: invowkfile.EnvInheritMode("bogus"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildExecutionContextOptions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildExecutionContextOptions_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	opts := BuildExecutionContextOptions{
		Selection: RuntimeSelection{},
	}
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected error for zero-value selection")
	}

	if !errors.Is(err, ErrInvalidBuildExecutionContextOptions) {
		t.Errorf("errors.Is(err, ErrInvalidBuildExecutionContextOptions) = false, want true")
	}

	var invalidErr *InvalidBuildExecutionContextOptionsError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidBuildExecutionContextOptionsError) = false, want true")
	}
	if len(invalidErr.FieldErrors) == 0 {
		t.Error("expected non-empty FieldErrors")
	}
}

func TestBuildExecutionContextOptionsValidateMutationContracts(t *testing.T) {
	t.Parallel()

	opts := BuildExecutionContextOptions{
		Selection:       RuntimeSelection{mode: invowkfile.RuntimeNative, platform: invowkfile.PlatformLinux, impl: &invowkfile.Implementation{}},
		Workdir:         " \t ",
		ContainerName:   "Bad",
		EnvFiles:        []invowkfile.DotenvFilePath{" \t "},
		EnvInheritMode:  "bogus",
		EnvInheritAllow: []invowkfile.EnvVarName{"1BAD"},
		EnvInheritDeny:  []invowkfile.EnvVarName{"-NOPE"},
		CommandFullName: "1bad",
		SourceID:        "1bad",
		Platform:        "plan9",
	}

	err := opts.Validate()
	invalidErr := requireInvalidBuildExecutionContextOptionsError(t, err)
	if got, want := invalidErr.Error(), "invalid build execution context options: 10 field error(s)"; got != want {
		t.Fatalf("InvalidBuildExecutionContextOptionsError.Error() = %q, want %q", got, want)
	}
	if got, want := len(invalidErr.FieldErrors), 10; got != want {
		t.Fatalf("FieldErrors length = %d, want %d", got, want)
	}
	for _, want := range []error{
		invowkfile.ErrInvalidWorkDir,
		invowkfile.ErrInvalidContainerName,
		invowkfile.ErrInvalidDotenvFilePath,
		invowkfile.ErrInvalidEnvInheritMode,
		invowkfile.ErrInvalidEnvVarName,
		invowkfile.ErrInvalidCommandName,
		discovery.ErrInvalidSourceID,
		invowkfile.ErrInvalidPlatform,
	} {
		if !executeFieldErrorsContain(invalidErr.FieldErrors, want) {
			t.Fatalf("FieldErrors should contain %v, got %#v", want, invalidErr.FieldErrors)
		}
	}
	if !executeFieldErrorsContainString(invalidErr.FieldErrors, "platform must match runtime selection platform") {
		t.Fatalf("FieldErrors should contain platform mismatch error, got %#v", invalidErr.FieldErrors)
	}
}

func requireInvalidBuildExecutionContextOptionsError(
	t *testing.T,
	err error,
) *InvalidBuildExecutionContextOptionsError {
	t.Helper()

	if !errors.Is(err, ErrInvalidBuildExecutionContextOptions) {
		t.Fatalf("error = %v, want ErrInvalidBuildExecutionContextOptions", err)
	}
	var invalidErr *InvalidBuildExecutionContextOptionsError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error type = %T, want *InvalidBuildExecutionContextOptionsError", err)
	}
	return invalidErr
}

func executeFieldErrorsContain(fieldErrors []error, target error) bool {
	for _, err := range fieldErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func executeFieldErrorsContainString(fieldErrors []error, target string) bool {
	for _, err := range fieldErrors {
		if err.Error() == target {
			return true
		}
	}
	return false
}
