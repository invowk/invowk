// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestFlag_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value Flag has empty Name which is nonzero-required — should fail.
	f := Flag{}
	if err := f.Validate(); err == nil {
		t.Fatal("Flag{}.Validate() should fail (empty Name is nonzero-required)")
	}
}

func TestFlag_Validate_Valid(t *testing.T) {
	t.Parallel()
	f := Flag{
		Name:        "output",
		Description: "The output path",
		Type:        FlagTypeString,
		Short:       "o",
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("valid Flag.Validate() returned error: %v", err)
	}
}

func TestFlag_Validate_ValidMinimal(t *testing.T) {
	t.Parallel()
	// Only Name is required; everything else is zero-value-valid.
	f := Flag{Name: "verbose"}
	if err := f.Validate(); err != nil {
		t.Fatalf("minimal Flag.Validate() returned error: %v", err)
	}
}

func TestFlag_Validate_InvalidName(t *testing.T) {
	t.Parallel()
	f := Flag{Name: ""}
	err := f.Validate()
	if err == nil {
		t.Fatal("Flag with empty name should fail validation")
	}
	if !errors.Is(err, ErrInvalidFlag) {
		t.Errorf("error should wrap ErrInvalidFlag, got: %v", err)
	}
	var flagErr *InvalidFlagError
	if !errors.As(err, &flagErr) {
		t.Errorf("error should be *InvalidFlagError, got: %T", err)
	} else if len(flagErr.FieldErrors) == 0 {
		t.Error("InvalidFlagError.FieldErrors should not be empty")
	}
}

func TestFlag_Validate_InvalidType(t *testing.T) {
	t.Parallel()
	f := Flag{
		Name: "output",
		Type: "bogus",
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("Flag with invalid type should fail validation")
	}
	if !errors.Is(err, ErrInvalidFlag) {
		t.Errorf("error should wrap ErrInvalidFlag, got: %v", err)
	}
}

func TestFlag_Validate_InvalidShorthand(t *testing.T) {
	t.Parallel()
	f := Flag{
		Name:  "output",
		Short: "xx", // not a single letter
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("Flag with invalid shorthand should fail validation")
	}
	if !errors.Is(err, ErrInvalidFlag) {
		t.Errorf("error should wrap ErrInvalidFlag, got: %v", err)
	}
}

func TestFlag_Validate_InvalidDescription(t *testing.T) {
	t.Parallel()
	f := Flag{
		Name:        "output",
		Description: "   ", // whitespace-only
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("Flag with whitespace-only description should fail validation")
	}
	if !errors.Is(err, ErrInvalidFlag) {
		t.Errorf("error should wrap ErrInvalidFlag, got: %v", err)
	}
}

func TestFlag_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()
	f := Flag{
		Name:       "",      // invalid
		Type:       "bogus", // invalid
		Short:      "xx",    // invalid
		Validation: "[",     // invalid regex
	}
	err := f.Validate()
	if err == nil {
		t.Fatal("Flag with multiple invalid fields should fail validation")
	}
	var flagErr *InvalidFlagError
	if !errors.As(err, &flagErr) {
		t.Fatalf("error should be *InvalidFlagError, got: %T", err)
	}
	if len(flagErr.FieldErrors) < 3 {
		t.Errorf("expected at least 3 field errors, got %d", len(flagErr.FieldErrors))
	}
}

func TestInvalidFlagError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidFlagError{FieldErrors: []error{errors.New("a"), errors.New("b")}}
	got := e.Error()
	want := "invalid flag: 2 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidFlagError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidFlagError{}
	if !errors.Is(e, ErrInvalidFlag) {
		t.Error("Unwrap() should return ErrInvalidFlag")
	}
}
