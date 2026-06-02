// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestValidationTypesMutationErrorPayloads(t *testing.T) {
	t.Parallel()

	severityErr := requireValidationTypesMutationAs[*InvalidValidationSeverityError](
		t,
		ValidationSeverity(7).Validate(),
		ErrInvalidValidationSeverity,
	)
	if severityErr.Value != ValidationSeverity(7) {
		t.Fatalf("InvalidValidationSeverityError.Value = %d, want 7", severityErr.Value)
	}
	if got, want := severityErr.Error(), "invalid validation severity 7 (valid: 0=error, 1=warning)"; got != want {
		t.Fatalf("InvalidValidationSeverityError.Error() = %q, want %q", got, want)
	}

	validatorErr := requireValidationTypesMutationAs[*InvalidValidatorNameError](
		t,
		ValidatorName(" \t").Validate(),
		ErrInvalidValidatorName,
	)
	if validatorErr.Value != ValidatorName(" \t") {
		t.Fatalf("InvalidValidatorNameError.Value = %q, want original whitespace", validatorErr.Value)
	}
	if got, want := validatorErr.Error(), "invalid validator name: \" \\t\""; got != want {
		t.Fatalf("InvalidValidatorNameError.Error() = %q, want %q", got, want)
	}
}

func TestValidationTypesMutationConstructorPreservesFieldsAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("domain cause")
	got, err := NewValidationErrorWithCause("structure", "command 'build'", "missing script", SeverityWarning, cause)
	if err != nil {
		t.Fatalf("NewValidationErrorWithCause() error = %v, want nil", err)
	}
	if got.Validator != "structure" {
		t.Fatalf("ValidationError.Validator = %q, want structure", got.Validator)
	}
	if got.Field != "command 'build'" {
		t.Fatalf("ValidationError.Field = %q, want command field", got.Field)
	}
	if got.Message != "missing script" {
		t.Fatalf("ValidationError.Message = %q, want missing script", got.Message)
	}
	if got.Severity != SeverityWarning {
		t.Fatalf("ValidationError.Severity = %d, want SeverityWarning", got.Severity)
	}
	if !errors.Is(got, cause) {
		t.Fatalf("ValidationError does not unwrap cause: %v", got)
	}

	invalid, err := NewValidationErrorWithCause(" \t", "field", "message", ValidationSeverity(9), cause)
	if err == nil {
		t.Fatal("NewValidationErrorWithCause() error = nil, want joined validation errors")
	}
	if invalid != (ValidationError{}) {
		t.Fatalf("invalid constructor returned %+v, want zero ValidationError", invalid)
	}
	if !errors.Is(err, ErrInvalidValidatorName) || !errors.Is(err, ErrInvalidValidationSeverity) {
		t.Fatalf("NewValidationErrorWithCause() error = %v, want both validation sentinels", err)
	}
}

func TestValidationTypesMutationValidationErrorsExactText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		errs ValidationErrors
		want string
	}{
		{
			name: "empty",
			errs: ValidationErrors{},
			want: "",
		},
		{
			name: "single",
			errs: ValidationErrors{
				{Field: "cmd", Message: "missing script", Severity: SeverityError},
			},
			want: "cmd: missing script",
		},
		{
			name: "two errors",
			errs: ValidationErrors{
				{Field: "first", Message: "bad runtime", Severity: SeverityError},
				{Field: "second", Message: "bad platform", Severity: SeverityError},
			},
			want: "validation failed with 2 errors:\n  - first: bad runtime\n  - second: bad platform",
		},
		{
			name: "two warnings",
			errs: ValidationErrors{
				{Field: "first", Message: "soft issue", Severity: SeverityWarning},
				{Field: "second", Message: "another soft issue", Severity: SeverityWarning},
			},
			want: "validation failed with 2 warnings:\n  - first: soft issue\n  - second: another soft issue",
		},
		{
			name: "error and warning",
			errs: ValidationErrors{
				{Field: "first", Message: "hard issue", Severity: SeverityError},
				{Field: "second", Message: "soft issue", Severity: SeverityWarning},
			},
			want: "validation failed with 1 error and 1 warning:\n  - first: hard issue\n  - second: soft issue",
		},
		{
			name: "plural mixed",
			errs: ValidationErrors{
				{Field: "one", Message: "hard issue", Severity: SeverityError},
				{Field: "two", Message: "another hard issue", Severity: SeverityError},
				{Field: "three", Message: "soft issue", Severity: SeverityWarning},
				{Field: "four", Message: "another soft issue", Severity: SeverityWarning},
			},
			want: "validation failed with 2 errors and 2 warnings:\n  - one: hard issue\n  - two: another hard issue\n  - three: soft issue\n  - four: another soft issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.errs.Error(); got != tt.want {
				t.Fatalf("ValidationErrors.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationTypesMutationUnwrapCopiesEachValidationError(t *testing.T) {
	t.Parallel()

	firstCause := errors.New("first cause")
	secondCause := errors.New("second cause")
	errs := ValidationErrors{
		{Field: "first", Message: "bad", Severity: SeverityError, Cause: firstCause},
		{Field: "second", Message: "warn", Severity: SeverityWarning, Cause: secondCause},
	}

	unwrapped := errs.Unwrap()
	if len(unwrapped) != len(errs) {
		t.Fatalf("Unwrap() length = %d, want %d", len(unwrapped), len(errs))
	}
	if !errors.Is(unwrapped[0], firstCause) {
		t.Fatalf("Unwrap()[0] = %v, want first cause", unwrapped[0])
	}
	if !errors.Is(unwrapped[1], secondCause) {
		t.Fatalf("Unwrap()[1] = %v, want second cause", unwrapped[1])
	}
}

func TestValidationTypesMutationFieldPathIndexedSegments(t *testing.T) {
	t.Parallel()

	got := NewFieldPath().
		Command("build").
		FlagIndex(1).
		ArgIndex(2).
		Filepaths(3).
		String()
	want := "command 'build' flag #2 argument #3 filepaths[4]"
	if got != want {
		t.Fatalf("FieldPath indexed String() = %q, want %q", got, want)
	}
}

func requireValidationTypesMutationAs[T any](t *testing.T, err, sentinel error) T {
	t.Helper()

	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel %v", err, sentinel)
	}
	var typed T
	if !errors.As(err, &typed) {
		t.Fatalf("error type = %T, want requested typed error", err)
	}
	return typed
}
