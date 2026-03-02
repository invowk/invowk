// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestArgument_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value Argument has empty Name which is nonzero-required — should fail.
	a := Argument{}
	if err := a.Validate(); err == nil {
		t.Fatal("Argument{}.Validate() should fail (empty Name is nonzero-required)")
	}
}

func TestArgument_Validate_Valid(t *testing.T) {
	t.Parallel()
	a := Argument{
		Name:        "output",
		Description: "The output path",
		Type:        ArgumentTypeString,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("valid Argument.Validate() returned error: %v", err)
	}
}

func TestArgument_Validate_ValidMinimal(t *testing.T) {
	t.Parallel()
	// Only Name is required; everything else is zero-value-valid.
	a := Argument{Name: "file"}
	if err := a.Validate(); err != nil {
		t.Fatalf("minimal Argument.Validate() returned error: %v", err)
	}
}

func TestArgument_Validate_InvalidName(t *testing.T) {
	t.Parallel()
	a := Argument{Name: ""}
	err := a.Validate()
	if err == nil {
		t.Fatal("Argument with empty name should fail validation")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("error should wrap ErrInvalidArgument, got: %v", err)
	}
	var argErr *InvalidArgumentError
	if !errors.As(err, &argErr) {
		t.Errorf("error should be *InvalidArgumentError, got: %T", err)
	} else if len(argErr.FieldErrors) == 0 {
		t.Error("InvalidArgumentError.FieldErrors should not be empty")
	}
}

func TestArgument_Validate_InvalidType(t *testing.T) {
	t.Parallel()
	a := Argument{
		Name: "file",
		Type: "bogus",
	}
	err := a.Validate()
	if err == nil {
		t.Fatal("Argument with invalid type should fail validation")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("error should wrap ErrInvalidArgument, got: %v", err)
	}
}

func TestArgument_Validate_InvalidDescription(t *testing.T) {
	t.Parallel()
	a := Argument{
		Name:        "file",
		Description: "   ", // whitespace-only
	}
	err := a.Validate()
	if err == nil {
		t.Fatal("Argument with whitespace-only description should fail validation")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("error should wrap ErrInvalidArgument, got: %v", err)
	}
}

func TestArgument_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()
	a := Argument{
		Name:       "",      // invalid
		Type:       "bogus", // invalid
		Validation: "[",     // invalid regex
	}
	err := a.Validate()
	if err == nil {
		t.Fatal("Argument with multiple invalid fields should fail validation")
	}
	var argErr *InvalidArgumentError
	if !errors.As(err, &argErr) {
		t.Fatalf("error should be *InvalidArgumentError, got: %T", err)
	}
	if len(argErr.FieldErrors) < 2 {
		t.Errorf("expected at least 2 field errors, got %d", len(argErr.FieldErrors))
	}
}

func TestInvalidArgumentError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidArgumentError{FieldErrors: []error{errors.New("a"), errors.New("b")}}
	got := e.Error()
	want := "invalid argument: 2 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidArgumentError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidArgumentError{}
	if !errors.Is(e, ErrInvalidArgument) {
		t.Error("Unwrap() should return ErrInvalidArgument")
	}
}
