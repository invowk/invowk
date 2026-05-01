// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		cfg      Config
		wantOK   bool
		wantErrs bool
	}{
		{
			name:   "zero value is valid (all paths empty means use defaults)",
			cfg:    Config{},
			wantOK: true,
		},
		{
			name: "all valid paths",
			cfg: Config{
				InvowkBinaryPath: types.FilesystemPath(filepath.Join(tmpDir, "bin", "invowk")),
				BinaryMountPath:  "/invowk/bin",     // container-internal path
				ModulesMountPath: "/invowk/modules", // container-internal path
				CacheDir:         types.FilesystemPath(filepath.Join(tmpDir, "cache", "provision")),
				ModulesPaths:     []types.FilesystemPath{types.FilesystemPath(filepath.Join(tmpDir, "config", "commands"))},
			},
			wantOK: true,
		},
		{
			name: "booleans and TagSuffix do not affect validity",
			cfg: Config{
				Enabled:          true,
				Strict:           true,
				ForceRebuild:     true,
				TagSuffix:        "test-suffix-123",
				InvowkBinaryPath: types.FilesystemPath(filepath.Join(tmpDir, "bin", "invowk")),
			},
			wantOK: true,
		},
		{
			name: "single invalid field: whitespace-only InvowkBinaryPath",
			cfg: Config{
				InvowkBinaryPath: "   ",
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "single invalid field: whitespace-only BinaryMountPath",
			cfg: Config{
				BinaryMountPath: "   ",
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "single invalid field: whitespace-only ModulesPaths element",
			cfg: Config{
				ModulesPaths: []types.FilesystemPath{types.FilesystemPath(filepath.Join(tmpDir, "valid-path")), "   "},
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "multiple invalid fields",
			cfg: Config{
				InvowkBinaryPath: "   ",
				BinaryMountPath:  container.MountTargetPath("   "),
				ModulesMountPath: container.MountTargetPath("   "),
				CacheDir:         "   ",
			},
			wantOK:   false,
			wantErrs: true,
		},
		{
			name: "empty ModulesPaths slice is valid",
			cfg: Config{
				ModulesPaths: []types.FilesystemPath{},
			},
			wantOK: true,
		},
		{
			name: "ModulesPaths with empty string elements are skipped",
			cfg: Config{
				ModulesPaths: []types.FilesystemPath{"", ""},
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("Validate() error = %v, wantOK %v", err, tt.wantOK)
			}
			if tt.wantErrs && err == nil {
				t.Error("Validate() expected error but got nil")
			}
			if !tt.wantErrs && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestConfigValidate_SentinelError(t *testing.T) {
	t.Parallel()

	cfg := Config{
		InvowkBinaryPath: "   ",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !errors.Is(err, ErrInvalidProvisionConfig) {
		t.Errorf("error should wrap ErrInvalidProvisionConfig, got: %v", err)
	}

	var configErr *InvalidProvisionConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("error should be *InvalidProvisionConfigError, got: %T", err)
	}
	if len(configErr.FieldErrors) != 1 {
		t.Errorf("expected 1 field error, got %d", len(configErr.FieldErrors))
	}
}

func TestConfigValidate_MultipleFieldErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{
		InvowkBinaryPath: "   ",
		CacheDir:         "   ",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	var configErr *InvalidProvisionConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("error should be *InvalidProvisionConfigError, got: %T", err)
	}
	if len(configErr.FieldErrors) != 2 {
		t.Errorf("expected 2 field errors, got %d: %v", len(configErr.FieldErrors), configErr.FieldErrors)
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
