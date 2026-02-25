// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestLoadOptions_IsValid_AllEmpty(t *testing.T) {
	t.Parallel()
	opts := LoadOptions{}
	valid, errs := opts.IsValid()
	if !valid {
		t.Errorf("empty LoadOptions should be valid, got errors: %v", errs)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestLoadOptions_IsValid_AllValid(t *testing.T) {
	t.Parallel()
	opts := LoadOptions{
		ConfigFilePath: "/tmp/config.cue",
		ConfigDirPath:  "/tmp/config",
		BaseDir:        "/tmp/base",
	}
	valid, errs := opts.IsValid()
	if !valid {
		t.Errorf("LoadOptions with valid paths should be valid, got errors: %v", errs)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestLoadOptions_IsValid_InvalidConfigFilePath(t *testing.T) {
	t.Parallel()
	opts := LoadOptions{
		ConfigFilePath: types.FilesystemPath("   "),
	}
	valid, errs := opts.IsValid()
	if valid {
		t.Fatal("LoadOptions with whitespace-only ConfigFilePath should be invalid")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !errors.Is(errs[0], ErrInvalidLoadOptions) {
		t.Errorf("error should wrap ErrInvalidLoadOptions, got: %v", errs[0])
	}

	var loadErr *InvalidLoadOptionsError
	if !errors.As(errs[0], &loadErr) {
		t.Fatalf("error should be *InvalidLoadOptionsError, got: %T", errs[0])
	}
	if len(loadErr.FieldErrors) != 1 {
		t.Errorf("expected 1 field error, got %d", len(loadErr.FieldErrors))
	}
}

func TestLoadOptions_IsValid_InvalidConfigDirPath(t *testing.T) {
	t.Parallel()
	opts := LoadOptions{
		ConfigDirPath: types.FilesystemPath("\t"),
	}
	valid, errs := opts.IsValid()
	if valid {
		t.Fatal("LoadOptions with whitespace-only ConfigDirPath should be invalid")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestLoadOptions_IsValid_InvalidBaseDir(t *testing.T) {
	t.Parallel()
	opts := LoadOptions{
		BaseDir: types.FilesystemPath("  \t  "),
	}
	valid, errs := opts.IsValid()
	if valid {
		t.Fatal("LoadOptions with whitespace-only BaseDir should be invalid")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestLoadOptions_IsValid_MultipleInvalid(t *testing.T) {
	t.Parallel()
	opts := LoadOptions{
		ConfigFilePath: types.FilesystemPath("   "),
		ConfigDirPath:  types.FilesystemPath("\t"),
		BaseDir:        types.FilesystemPath("  "),
	}
	valid, errs := opts.IsValid()
	if valid {
		t.Fatal("LoadOptions with all invalid paths should be invalid")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 wrapped error, got %d", len(errs))
	}

	var loadErr *InvalidLoadOptionsError
	if !errors.As(errs[0], &loadErr) {
		t.Fatalf("error should be *InvalidLoadOptionsError, got: %T", errs[0])
	}
	if len(loadErr.FieldErrors) != 3 {
		t.Errorf("expected 3 field errors, got %d: %v", len(loadErr.FieldErrors), loadErr.FieldErrors)
	}
}

func TestLoadOptions_IsValid_MixedEmptyAndInvalid(t *testing.T) {
	t.Parallel()
	// Empty fields are valid (zero-value means "use default"),
	// only non-empty invalid fields should be caught.
	opts := LoadOptions{
		ConfigFilePath: "",
		ConfigDirPath:  types.FilesystemPath("   "),
		BaseDir:        "/valid/path",
	}
	valid, errs := opts.IsValid()
	if valid {
		t.Fatal("LoadOptions with one invalid field should be invalid")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 wrapped error, got %d", len(errs))
	}

	var loadErr *InvalidLoadOptionsError
	if !errors.As(errs[0], &loadErr) {
		t.Fatalf("error should be *InvalidLoadOptionsError, got: %T", errs[0])
	}
	if len(loadErr.FieldErrors) != 1 {
		t.Errorf("expected 1 field error (only ConfigDirPath), got %d", len(loadErr.FieldErrors))
	}
}

func TestInvalidLoadOptionsError_Error_Single(t *testing.T) {
	t.Parallel()
	err := &InvalidLoadOptionsError{
		FieldErrors: []error{errors.New("test error")},
	}
	msg := err.Error()
	if msg != "invalid load options: test error" {
		t.Errorf("Error() = %q, want %q", msg, "invalid load options: test error")
	}
}

func TestInvalidLoadOptionsError_Error_Multiple(t *testing.T) {
	t.Parallel()
	err := &InvalidLoadOptionsError{
		FieldErrors: []error{errors.New("err1"), errors.New("err2")},
	}
	msg := err.Error()
	if msg != "invalid load options: 2 field errors" {
		t.Errorf("Error() = %q, want %q", msg, "invalid load options: 2 field errors")
	}
}

func TestInvalidLoadOptionsError_Unwrap(t *testing.T) {
	t.Parallel()
	err := &InvalidLoadOptionsError{
		FieldErrors: []error{errors.New("test")},
	}
	if !errors.Is(err, ErrInvalidLoadOptions) {
		t.Error("Unwrap() should return ErrInvalidLoadOptions")
	}
}
