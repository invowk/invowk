// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestArgumentTypeValidateReportsInvalidValue(t *testing.T) {
	t.Parallel()

	const value ArgumentType = "duration"

	err := value.Validate()
	if err == nil {
		t.Fatal("ArgumentType.Validate() returned nil, want invalid type error")
	}
	if !errors.Is(err, ErrInvalidArgumentType) {
		t.Fatalf("error should wrap ErrInvalidArgumentType, got %v", err)
	}
	var invalid *InvalidArgumentTypeError
	if !errors.As(err, &invalid) {
		t.Fatalf("error should be *InvalidArgumentTypeError, got %T", err)
	}
	if invalid.Value != value {
		t.Fatalf("InvalidArgumentTypeError.Value = %q, want %q", invalid.Value, value)
	}
	want := `invalid argument type "duration" (valid: string, int, float)`
	if got := err.Error(); got != want {
		t.Fatalf("InvalidArgumentTypeError.Error() = %q, want %q", got, want)
	}
}

func TestArgumentNameValidateReportsValueAndReason(t *testing.T) {
	t.Parallel()

	tooLong := ArgumentName(strings.Repeat("a", MaxNameLength+1))
	tests := []struct {
		name       string
		value      ArgumentName
		wantReason string
	}{
		{
			name:       "empty",
			value:      "",
			wantReason: invalidReasonMustNotBeEmpty,
		},
		{
			name:       "too long",
			value:      tooLong,
			wantReason: "exceeds maximum length of 256 runes",
		},
		{
			name:       "invalid pattern",
			value:      "1st",
			wantReason: "must start with a letter followed by alphanumeric, underscore, or hyphen characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if err == nil {
				t.Fatal("ArgumentName.Validate() returned nil, want invalid name error")
			}
			if !errors.Is(err, ErrInvalidArgumentName) {
				t.Fatalf("error should wrap ErrInvalidArgumentName, got %v", err)
			}
			var invalid *InvalidArgumentNameError
			if !errors.As(err, &invalid) {
				t.Fatalf("error should be *InvalidArgumentNameError, got %T", err)
			}
			if invalid.Value != tt.value {
				t.Fatalf("InvalidArgumentNameError.Value = %q, want %q", invalid.Value, tt.value)
			}
			if invalid.Reason != tt.wantReason {
				t.Fatalf("InvalidArgumentNameError.Reason = %q, want %q", invalid.Reason, tt.wantReason)
			}
		})
	}
}

func TestArgumentValidateIncludesInvalidNameFieldError(t *testing.T) {
	t.Parallel()

	arg := Argument{
		Name:        "1st",
		Description: "Input path",
	}

	invalid := requireInvalidArgumentError(t, arg.Validate())
	if len(invalid.FieldErrors) != 1 {
		t.Fatalf("FieldErrors length = %d, want 1: %v", len(invalid.FieldErrors), invalid.FieldErrors)
	}
	nameErr := requireArgumentNameFieldError(t, invalid.FieldErrors)
	if nameErr.Value != arg.Name {
		t.Fatalf("InvalidArgumentNameError.Value = %q, want %q", nameErr.Value, arg.Name)
	}
}

func TestArgumentValidateIncludesInvalidValidationFieldError(t *testing.T) {
	t.Parallel()

	arg := Argument{
		Name:        "env",
		Description: "Deployment environment",
		Validation:  "[",
	}

	invalid := requireInvalidArgumentError(t, arg.Validate())
	if len(invalid.FieldErrors) != 1 {
		t.Fatalf("FieldErrors length = %d, want 1: %v", len(invalid.FieldErrors), invalid.FieldErrors)
	}
	regexErr := requireRegexPatternFieldError(t, invalid.FieldErrors)
	if regexErr.Value != arg.Validation {
		t.Fatalf("InvalidRegexPatternError.Value = %q, want %q", regexErr.Value, arg.Validation)
	}
}

func TestArgumentValidateDefaultValuePreservesTypeErrorCause(t *testing.T) {
	t.Parallel()

	arg := Argument{
		Name:         "count",
		Description:  "Number of retries",
		Type:         ArgumentTypeInt,
		DefaultValue: "many",
	}

	invalid := requireInvalidArgumentError(t, arg.Validate())
	if len(invalid.FieldErrors) != 1 {
		t.Fatalf("FieldErrors length = %d, want 1: %v", len(invalid.FieldErrors), invalid.FieldErrors)
	}
	fieldErr := invalid.FieldErrors[0]
	if !strings.Contains(fieldErr.Error(), `default_value "many" is not compatible with type "int"`) {
		t.Fatalf("default value field error = %q, want type compatibility message", fieldErr)
	}
	cause := errors.Unwrap(fieldErr)
	if cause == nil {
		t.Fatal("default value field error does not wrap its type validation cause")
	}
	if !strings.Contains(cause.Error(), "must be a valid integer") {
		t.Fatalf("wrapped cause = %q, want integer validation message", cause)
	}
}

func TestArgumentValidateDefaultValueValidationPatternGuards(t *testing.T) {
	t.Parallel()

	t.Run("matching valid pattern does not add mismatch", func(t *testing.T) {
		t.Parallel()

		arg := Argument{
			Name:         "env",
			Description:  "Deployment environment",
			Validation:   "^prod$",
			DefaultValue: "prod",
		}
		if err := arg.Validate(); err != nil {
			t.Fatalf("Argument.Validate() returned error for matching default value: %v", err)
		}
	})

	t.Run("invalid pattern does not also report default mismatch", func(t *testing.T) {
		t.Parallel()

		arg := Argument{
			Name:         "env",
			Description:  "Deployment environment",
			Validation:   "[",
			DefaultValue: "prod",
		}

		invalid := requireInvalidArgumentError(t, arg.Validate())
		if len(invalid.FieldErrors) != 1 {
			t.Fatalf("FieldErrors length = %d, want 1: %v", len(invalid.FieldErrors), invalid.FieldErrors)
		}
		requireRegexPatternFieldError(t, invalid.FieldErrors)
		if fieldErrorsContain(invalid.FieldErrors, "does not match validation pattern") {
			t.Fatalf("invalid regex should not also report default mismatch: %v", invalid.FieldErrors)
		}
	})
}

func TestArgumentValidateArgumentValueRejectsInvalidArgumentType(t *testing.T) {
	t.Parallel()

	arg := &Argument{
		Name: "count",
		Type: "duration",
	}

	err := arg.ValidateArgumentValue("1")
	if err == nil {
		t.Fatal("ValidateArgumentValue() returned nil, want invalid type error")
	}
	if !errors.Is(err, ErrInvalidArgumentType) {
		t.Fatalf("error should wrap ErrInvalidArgumentType, got %v", err)
	}
	var invalid *InvalidArgumentTypeError
	if !errors.As(err, &invalid) {
		t.Fatalf("error should contain *InvalidArgumentTypeError, got %T", err)
	}
	if invalid.Value != arg.Type {
		t.Fatalf("InvalidArgumentTypeError.Value = %q, want %q", invalid.Value, arg.Type)
	}
}

func requireInvalidArgumentError(t *testing.T, err error) *InvalidArgumentError {
	t.Helper()

	if err == nil {
		t.Fatal("Argument.Validate() returned nil, want invalid argument error")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("error should wrap ErrInvalidArgument, got %v", err)
	}
	var invalid *InvalidArgumentError
	if !errors.As(err, &invalid) {
		t.Fatalf("error should be *InvalidArgumentError, got %T", err)
	}
	return invalid
}

func requireArgumentNameFieldError(t *testing.T, errs []error) *InvalidArgumentNameError {
	t.Helper()

	for _, err := range errs {
		var invalid *InvalidArgumentNameError
		if errors.As(err, &invalid) {
			return invalid
		}
	}
	t.Fatalf("field errors %v do not contain *InvalidArgumentNameError", errs)
	return nil
}

func requireRegexPatternFieldError(t *testing.T, errs []error) *InvalidRegexPatternError {
	t.Helper()

	for _, err := range errs {
		var invalid *InvalidRegexPatternError
		if errors.As(err, &invalid) {
			return invalid
		}
	}
	t.Fatalf("field errors %v do not contain *InvalidRegexPatternError", errs)
	return nil
}
