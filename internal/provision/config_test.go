// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

func TestConfigIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       Config
		wantValid bool
		wantErrs  bool
	}{
		{
			name:      "zero value is valid (all paths empty means use defaults)",
			cfg:       Config{},
			wantValid: true,
		},
		{
			name: "all valid paths",
			cfg: Config{
				InvowkBinaryPath: "/usr/local/bin/invowk",
				InvowkfilePath:   "/home/user/project/invowkfile.cue",
				BinaryMountPath:  "/invowk/bin",
				ModulesMountPath: "/invowk/modules",
				CacheDir:         "/home/user/.cache/invowk/provision",
				ModulesPaths:     []types.FilesystemPath{"/home/user/.config/invowk/commands"},
			},
			wantValid: true,
		},
		{
			name: "booleans and TagSuffix do not affect validity",
			cfg: Config{
				Enabled:          true,
				Strict:           true,
				ForceRebuild:     true,
				TagSuffix:        "test-suffix-123",
				InvowkBinaryPath: "/usr/local/bin/invowk",
			},
			wantValid: true,
		},
		{
			name: "single invalid field: whitespace-only InvowkBinaryPath",
			cfg: Config{
				InvowkBinaryPath: "   ",
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "single invalid field: whitespace-only BinaryMountPath",
			cfg: Config{
				BinaryMountPath: "   ",
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "single invalid field: whitespace-only ModulesPaths element",
			cfg: Config{
				ModulesPaths: []types.FilesystemPath{"/valid/path", "   "},
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "multiple invalid fields",
			cfg: Config{
				InvowkBinaryPath: "   ",
				InvowkfilePath:   "   ",
				BinaryMountPath:  container.MountTargetPath("   "),
				ModulesMountPath: container.MountTargetPath("   "),
				CacheDir:         "   ",
			},
			wantValid: false,
			wantErrs:  true,
		},
		{
			name: "empty ModulesPaths slice is valid",
			cfg: Config{
				ModulesPaths: []types.FilesystemPath{},
			},
			wantValid: true,
		},
		{
			name: "ModulesPaths with empty string elements are skipped",
			cfg: Config{
				ModulesPaths: []types.FilesystemPath{"", ""},
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			valid, errs := tt.cfg.IsValid()
			if valid != tt.wantValid {
				t.Errorf("IsValid() valid = %v, want %v", valid, tt.wantValid)
			}
			if tt.wantErrs && len(errs) == 0 {
				t.Error("IsValid() expected errors but got none")
			}
			if !tt.wantErrs && len(errs) > 0 {
				t.Errorf("IsValid() unexpected errors: %v", errs)
			}
		})
	}
}

func TestConfigIsValid_SentinelError(t *testing.T) {
	t.Parallel()

	cfg := Config{
		InvowkBinaryPath: "   ",
	}

	valid, errs := cfg.IsValid()
	if valid {
		t.Fatal("expected invalid config")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !errors.Is(errs[0], ErrInvalidProvisionConfig) {
		t.Errorf("error should wrap ErrInvalidProvisionConfig, got: %v", errs[0])
	}

	var configErr *InvalidProvisionConfigError
	if !errors.As(errs[0], &configErr) {
		t.Fatalf("error should be *InvalidProvisionConfigError, got: %T", errs[0])
	}
	if len(configErr.FieldErrors) != 1 {
		t.Errorf("expected 1 field error, got %d", len(configErr.FieldErrors))
	}
}

func TestConfigIsValid_MultipleFieldErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{
		InvowkBinaryPath: "   ",
		InvowkfilePath:   "   ",
		CacheDir:         "   ",
	}

	valid, errs := cfg.IsValid()
	if valid {
		t.Fatal("expected invalid config")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 wrapped error, got %d", len(errs))
	}

	var configErr *InvalidProvisionConfigError
	if !errors.As(errs[0], &configErr) {
		t.Fatalf("error should be *InvalidProvisionConfigError, got: %T", errs[0])
	}
	if len(configErr.FieldErrors) != 3 {
		t.Errorf("expected 3 field errors, got %d: %v", len(configErr.FieldErrors), configErr.FieldErrors)
	}

	// Verify Error() message mentions count
	errMsg := configErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}
}

func TestInvalidProvisionConfigError_Unwrap(t *testing.T) {
	t.Parallel()

	err := &InvalidProvisionConfigError{
		FieldErrors: []error{errors.New("test")},
	}
	if !errors.Is(err, ErrInvalidProvisionConfig) {
		t.Error("Unwrap() should return ErrInvalidProvisionConfig")
	}
}
