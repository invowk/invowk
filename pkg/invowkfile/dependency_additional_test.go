// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestDependencyValidators_InvalidCases(t *testing.T) {
	t.Parallel()

	invalidCode := types.ExitCode(-1)

	tests := []struct {
		name     string
		err      error
		sentinel error
		checkAs  func(t *testing.T, err error)
	}{
		{
			name:     "tool dependency",
			err:      ToolDependency{Alternatives: []BinaryName{""}}.Validate(),
			sentinel: ErrInvalidToolDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidToolDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "command dependency",
			err:      CommandDependency{Alternatives: []CommandName{""}}.Validate(),
			sentinel: ErrInvalidCommandDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCommandDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "capability dependency",
			err:      CapabilityDependency{Alternatives: []CapabilityName{"bogus"}}.Validate(),
			sentinel: ErrInvalidCapabilityDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCapabilityDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "env var check",
			err:      EnvVarCheck{Name: "", Validation: "["}.Validate(),
			sentinel: ErrInvalidEnvVarCheck,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidEnvVarCheckError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "env var dependency",
			err:      EnvVarDependency{Alternatives: []EnvVarCheck{{Name: ""}}}.Validate(),
			sentinel: ErrInvalidEnvVarDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidEnvVarDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name:     "filepath dependency",
			err:      FilepathDependency{Alternatives: []FilesystemPath{""}}.Validate(),
			sentinel: ErrInvalidFilepathDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidFilepathDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "custom check",
			err: CustomCheck{
				Name:           "",
				CheckScript:    "   ",
				ExpectedCode:   &invalidCode,
				ExpectedOutput: "[",
			}.Validate(),
			sentinel: ErrInvalidCustomCheck,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCustomCheckError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "custom check dependency direct fields",
			err: CustomCheckDependency{
				Name:           "",
				CheckScript:    "   ",
				ExpectedCode:   &invalidCode,
				ExpectedOutput: "[",
			}.Validate(),
			sentinel: ErrInvalidCustomCheckDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCustomCheckDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "custom check dependency alternatives",
			err: CustomCheckDependency{
				Alternatives: []CustomCheck{{Name: "", CheckScript: "   "}},
			}.Validate(),
			sentinel: ErrInvalidCustomCheckDependency,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidCustomCheckDependencyError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
		{
			name: "depends_on",
			err: DependsOn{
				Tools:        []ToolDependency{{Alternatives: []BinaryName{""}}},
				Commands:     []CommandDependency{{Alternatives: []CommandName{""}}},
				Filepaths:    []FilepathDependency{{Alternatives: []FilesystemPath{""}}},
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{"bogus"}}},
				CustomChecks: []CustomCheckDependency{{Name: "", CheckScript: "   "}},
				EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: ""}}}},
			}.Validate(),
			sentinel: ErrInvalidDependsOn,
			checkAs: func(t *testing.T, err error) {
				t.Helper()
				var typed *InvalidDependsOnError
				if !errors.As(err, &typed) {
					t.Fatalf("errors.As(%T) = false", &typed)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err == nil {
				t.Fatal("Validate() returned nil, want error")
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", tt.err, tt.sentinel)
			}
			tt.checkAs(t, tt.err)
		})
	}
}

func TestDependencyValidators_ValidCases(t *testing.T) {
	t.Parallel()

	validCode := types.ExitCode(0)
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "tool dependency",
			err:  ToolDependency{Alternatives: []BinaryName{"git"}}.Validate(),
		},
		{
			name: "command dependency",
			err:  CommandDependency{Alternatives: []CommandName{"build"}}.Validate(),
		},
		{
			name: "capability dependency",
			err:  CapabilityDependency{Alternatives: []CapabilityName{CapabilityInternet}}.Validate(),
		},
		{
			name: "env var check",
			err:  EnvVarCheck{Name: "PATH", Validation: "^.+$"}.Validate(),
		},
		{
			name: "env var dependency",
			err:  EnvVarDependency{Alternatives: []EnvVarCheck{{Name: "PATH"}}}.Validate(),
		},
		{
			name: "filepath dependency",
			err:  FilepathDependency{Alternatives: []FilesystemPath{"scripts/install.sh"}}.Validate(),
		},
		{
			name: "custom check",
			err: CustomCheck{
				Name:           "shellcheck",
				CheckScript:    "echo ok",
				ExpectedCode:   &validCode,
				ExpectedOutput: "^ok$",
			}.Validate(),
		},
		{
			name: "custom check dependency direct",
			err: CustomCheckDependency{
				Name:           "shellcheck",
				CheckScript:    "echo ok",
				ExpectedCode:   &validCode,
				ExpectedOutput: "^ok$",
			}.Validate(),
		},
		{
			name: "custom check dependency alternatives",
			err: CustomCheckDependency{
				Alternatives: []CustomCheck{{Name: "shellcheck", CheckScript: "echo ok"}},
			}.Validate(),
		},
		{
			name: "depends_on",
			err: DependsOn{
				Tools:        []ToolDependency{{Alternatives: []BinaryName{"git"}}},
				Commands:     []CommandDependency{{Alternatives: []CommandName{"build"}}},
				Filepaths:    []FilepathDependency{{Alternatives: []FilesystemPath{"scripts/install.sh"}}},
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityTTY}}},
				CustomChecks: []CustomCheckDependency{{Name: "shellcheck", CheckScript: "echo ok"}},
				EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "PATH"}}}},
			}.Validate(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err != nil {
				t.Fatalf("Validate() error = %v, want nil", tt.err)
			}
		})
	}
}

func TestDependencyErrorStringsAndUnwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		sentinel  error
		wantError string
	}{
		{
			name:      "invalid binary name",
			err:       &InvalidBinaryNameError{Value: "/bin/sh", Reason: "must not contain path separators"},
			sentinel:  ErrInvalidBinaryName,
			wantError: `invalid binary name "/bin/sh": must not contain path separators`,
		},
		{
			name:      "invalid check name",
			err:       &InvalidCheckNameError{Value: ""},
			sentinel:  ErrInvalidCheckName,
			wantError: `invalid check name "": must be non-empty and not whitespace-only`,
		},
		{
			name:      "invalid script content",
			err:       &InvalidScriptContentError{Value: "   "},
			sentinel:  ErrInvalidScriptContent,
			wantError: `invalid script content: non-empty value must not be whitespace-only (got "   ")`,
		},
		{
			name:      "invalid tool dependency",
			err:       &InvalidToolDependencyError{FieldErrors: []error{errors.New("one"), errors.New("two")}},
			sentinel:  ErrInvalidToolDependency,
			wantError: "invalid tool dependency: 2 field error(s)",
		},
		{
			name:      "invalid command dependency",
			err:       &InvalidCommandDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCommandDependency,
			wantError: "invalid command dependency: 1 field error(s)",
		},
		{
			name:      "invalid capability dependency",
			err:       &InvalidCapabilityDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCapabilityDependency,
			wantError: "invalid capability dependency: 1 field error(s)",
		},
		{
			name:      "invalid env var check",
			err:       &InvalidEnvVarCheckError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidEnvVarCheck,
			wantError: "invalid env var check: 1 field error(s)",
		},
		{
			name:      "invalid env var dependency",
			err:       &InvalidEnvVarDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidEnvVarDependency,
			wantError: "invalid env var dependency: 1 field error(s)",
		},
		{
			name:      "invalid filepath dependency",
			err:       &InvalidFilepathDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidFilepathDependency,
			wantError: "invalid filepath dependency: 1 field error(s)",
		},
		{
			name:      "invalid custom check",
			err:       &InvalidCustomCheckError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCustomCheck,
			wantError: "invalid custom check: 1 field error(s)",
		},
		{
			name:      "invalid custom check dependency",
			err:       &InvalidCustomCheckDependencyError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidCustomCheckDependency,
			wantError: "invalid custom check dependency: 1 field error(s)",
		},
		{
			name:      "invalid depends_on",
			err:       &InvalidDependsOnError{FieldErrors: []error{errors.New("one")}},
			sentinel:  ErrInvalidDependsOn,
			wantError: "invalid depends_on: 1 field error(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err.Error() != tt.wantError {
				t.Fatalf("Error() = %q, want %q", tt.err.Error(), tt.wantError)
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", tt.err, tt.sentinel)
			}
		})
	}
}
