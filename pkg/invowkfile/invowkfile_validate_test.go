// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestInvowkfile_ValidateFields_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value Invowkfile has all zero-valid fields — should pass.
	inv := Invowkfile{}
	if err := inv.ValidateFields(); err != nil {
		t.Fatalf("Invowkfile{}.ValidateFields() should pass, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_Valid(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		DefaultShell: "/bin/bash",
		WorkDir:      "src",
		Commands: []Command{
			{
				Name: "build",
				Implementations: []Implementation{
					{
						Script:    ImplementationScript{Content: "go build ./..."},
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: AllPlatformConfigs(),
					},
				},
			},
		},
		Env: &EnvConfig{
			Vars: map[EnvVarName]string{"MY_VAR": "value"},
		},
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{"go"}}},
		},
	}
	if err := inv.ValidateFields(); err != nil {
		t.Fatalf("valid Invowkfile.ValidateFields() returned error: %v", err)
	}
}

func TestInvowkfile_ValidateFields_InvalidDefaultShell(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		DefaultShell: "   ", // whitespace-only
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with whitespace-only DefaultShell should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_Validate_IncludesFieldValidation(t *testing.T) {
	t.Parallel()

	validCommand := func() Command {
		return Command{
			Name: "build",
			Implementations: []Implementation{
				{
					Script:    ImplementationScript{Content: "go build ./..."},
					Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
					Platforms: []PlatformConfig{{Name: PlatformLinux}},
				},
			},
		}
	}

	tests := []struct {
		name string
		inv  Invowkfile
		want error
	}{
		{
			name: "default shell",
			inv:  Invowkfile{DefaultShell: "   ", Commands: []Command{validCommand()}},
			want: ErrInvalidShellPath,
		},
		{
			name: "root workdir",
			inv:  Invowkfile{WorkDir: "   ", Commands: []Command{validCommand()}},
			want: ErrInvalidWorkDir,
		},
		{
			name: "metadata",
			inv: Invowkfile{
				Commands: []Command{validCommand()},
				Metadata: &ModuleMetadata{
					module:  invowkmod.ModuleID(""),
					version: invowkmod.SemVer("1.0.0"),
				},
			},
			want: ErrInvalidModuleMetadata,
		},
		{
			name: "file path",
			inv:  Invowkfile{FilePath: "   ", Commands: []Command{validCommand()}},
			want: ErrInvalidFilesystemPath,
		},
		{
			name: "module path",
			inv:  Invowkfile{ModulePath: "   ", Commands: []Command{validCommand()}},
			want: ErrInvalidFilesystemPath,
		},
		{
			name: "command workdir",
			inv: Invowkfile{Commands: []Command{
				func() Command {
					cmd := validCommand()
					cmd.WorkDir = "   "
					return cmd
				}(),
			}},
			want: ErrInvalidWorkDir,
		},
		{
			name: "implementation workdir",
			inv: Invowkfile{Commands: []Command{
				func() Command {
					cmd := validCommand()
					cmd.Implementations[0].WorkDir = "   "
					return cmd
				}(),
			}},
			want: ErrInvalidWorkDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := (&tt.inv).Validate()
			if !errors.Is(errs, tt.want) {
				t.Fatalf("Validate() errors = %v, want errors.Is(..., %v)", errs, tt.want)
			}
			if !validationErrorsIncludeValidator(errs, fieldValidatorName) {
				t.Fatalf("Validate() errors = %v, want fields validator error", errs)
			}
		})
	}
}

func TestInvowkfile_ValidateFields_InvalidWorkDir(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		WorkDir: "   ", // whitespace-only
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with whitespace-only WorkDir should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_InvalidEnv(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		Env: &EnvConfig{
			Vars: map[EnvVarName]string{"123-BAD": "value"},
		},
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with invalid Env should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_InvalidDependsOn(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{""}}},
		},
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with invalid DependsOn should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_InvalidMetadata(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		Metadata: &ModuleMetadata{
			module:  invowkmod.ModuleID(""),
			version: invowkmod.SemVer("1.0.0"),
		},
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with invalid Metadata should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
	var invErr *InvalidInvowkfileError
	if !errors.As(err, &invErr) {
		t.Fatalf("error should be *InvalidInvowkfileError, got: %T", err)
	}
	if len(invErr.FieldErrors) != 1 || !errors.Is(invErr.FieldErrors[0], ErrInvalidModuleMetadata) {
		t.Fatalf("field errors = %v, want ErrInvalidModuleMetadata", invErr.FieldErrors)
	}
}

func TestInvowkfile_ValidateFields_InvalidCommand(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		Commands: []Command{{Name: ""}}, // invalid empty name
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with invalid Command should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_InvalidFilePath(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		FilePath: "   ", // whitespace-only
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with whitespace-only FilePath should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_InvalidModulePath(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		ModulePath: "   ", // whitespace-only
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with whitespace-only ModulePath should fail")
	}
	if !errors.Is(err, ErrInvalidInvowkfile) {
		t.Errorf("error should wrap ErrInvalidInvowkfile, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_NilOptionalFields(t *testing.T) {
	t.Parallel()
	// nil Env and DependsOn, empty FilePath and ModulePath should all pass.
	inv := Invowkfile{}
	if err := inv.ValidateFields(); err != nil {
		t.Fatalf("Invowkfile with nil optional fields should pass, got: %v", err)
	}
}

func TestInvowkfile_ValidateFields_MultipleErrors(t *testing.T) {
	t.Parallel()
	inv := Invowkfile{
		DefaultShell: "   ",                 // invalid
		WorkDir:      "   ",                 // invalid
		Commands:     []Command{{Name: ""}}, // invalid
		FilePath:     "   ",                 // invalid
	}
	err := inv.ValidateFields()
	if err == nil {
		t.Fatal("Invowkfile with multiple invalid fields should fail")
	}
	var invErr *InvalidInvowkfileError
	if !errors.As(err, &invErr) {
		t.Fatalf("error should be *InvalidInvowkfileError, got: %T", err)
	}
	if len(invErr.FieldErrors) < 3 {
		t.Errorf("expected at least 3 field errors, got %d", len(invErr.FieldErrors))
	}
}

func TestInvalidInvowkfileError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidInvowkfileError{FieldErrors: []error{errors.New("a"), errors.New("b")}}
	got := e.Error()
	want := "invalid invowkfile: 2 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidInvowkfileError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidInvowkfileError{}
	if !errors.Is(e, ErrInvalidInvowkfile) {
		t.Error("Unwrap() should return ErrInvalidInvowkfile")
	}
}

func validationErrorsIncludeValidator(errs ValidationErrors, validator ValidatorName) bool {
	for _, err := range errs {
		if err.Validator == validator {
			return true
		}
	}
	return false
}
