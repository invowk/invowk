// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestFlagValueTypeMutationContracts(t *testing.T) {
	t.Parallel()

	typeErr := requireFlagMutationErrorAs[*InvalidFlagTypeError](t, FlagType("bogus").Validate())
	if typeErr.Value != "bogus" {
		t.Fatalf("InvalidFlagTypeError.Value = %q, want bogus", typeErr.Value)
	}
	if !errors.Is(typeErr, ErrInvalidFlagType) {
		t.Fatalf("InvalidFlagTypeError does not wrap ErrInvalidFlagType: %v", typeErr)
	}
	if got := typeErr.Error(); got != `invalid flag type "bogus" (valid: string, bool, int, float)` {
		t.Fatalf("InvalidFlagTypeError.Error() = %q, want invalid type diagnostic", got)
	}

	emptyName := requireFlagMutationErrorAs[*InvalidFlagNameError](t, FlagName("").Validate())
	if emptyName.Value != "" || emptyName.Reason != invalidReasonMustNotBeEmpty {
		t.Fatalf("empty flag name error = %+v, want empty/%q", emptyName, invalidReasonMustNotBeEmpty)
	}
	if got := emptyName.Error(); got != `invalid flag name "": must not be empty` {
		t.Fatalf("InvalidFlagNameError.Error() = %q, want empty-name diagnostic", got)
	}
	if emptyName.Error() != `invalid flag name "": must not be empty` {
		t.Fatalf("empty flag name Error() = %q, want formatted value and reason", emptyName.Error())
	}

	longNameValue := FlagName(strings.Repeat("a", MaxNameLength+1))
	longName := requireFlagMutationErrorAs[*InvalidFlagNameError](t, longNameValue.Validate())
	if longName.Value != longNameValue || !strings.Contains(longName.Reason, "exceeds maximum length") {
		t.Fatalf("long flag name error = %+v, want value and max-length reason", longName)
	}

	badPattern := requireFlagMutationErrorAs[*InvalidFlagNameError](t, FlagName("9bad").Validate())
	if badPattern.Value != "9bad" || !strings.Contains(badPattern.Reason, "must start with a letter") {
		t.Fatalf("bad-pattern flag name error = %+v, want value and pattern reason", badPattern)
	}

	shortErr := requireFlagMutationErrorAs[*InvalidFlagShorthandError](t, FlagShorthand("12").Validate())
	if shortErr.Value != "12" {
		t.Fatalf("InvalidFlagShorthandError.Value = %q, want 12", shortErr.Value)
	}
	if got := shortErr.Error(); got != `invalid flag shorthand "12" (must be a single ASCII letter)` {
		t.Fatalf("InvalidFlagShorthandError.Error() = %q, want shorthand diagnostic", got)
	}
}

func TestFlagValidateMutationContracts(t *testing.T) {
	t.Parallel()

	err := Flag{
		Name:        "9bad",
		Description: "Invalid flag name",
		Validation:  "[",
	}.Validate()
	flagErr := requireFlagMutationErrorAs[*InvalidFlagError](t, err)
	if !errors.Is(err, ErrInvalidFlag) {
		t.Fatalf("Validate() = %v, want ErrInvalidFlag wrapper", err)
	}
	if !flagMutationFieldErrorsContainSentinel(flagErr.FieldErrors, ErrInvalidFlagName) {
		t.Fatalf("FieldErrors = %v, want ErrInvalidFlagName", flagErr.FieldErrors)
	}
	if !fieldErrorsContain(flagErr.FieldErrors, "invalid regex") {
		t.Fatalf("FieldErrors = %v, want invalid regex error", flagErr.FieldErrors)
	}
}

func TestFlagDefaultValueMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("valid default has no errors", func(t *testing.T) {
		t.Parallel()
		errs := Flag{
			Name:         "mode",
			Description:  "Execution mode",
			DefaultValue: "debug",
		}.defaultValueValidationErrors()
		if len(errs) != 0 {
			t.Fatalf("defaultValueValidationErrors() = %v, want no errors", errs)
		}
	})

	t.Run("invalid type default keeps wrapped cause", func(t *testing.T) {
		t.Parallel()
		errs := Flag{
			Name:         "count",
			Description:  "Number of runs",
			Type:         FlagTypeInt,
			DefaultValue: "many",
		}.defaultValueValidationErrors()
		if len(errs) != 1 {
			t.Fatalf("defaultValueValidationErrors() count = %d, want 1: %v", len(errs), errs)
		}
		if errors.Unwrap(errs[0]) == nil {
			t.Fatalf("default type error = %v, want wrapped cause", errs[0])
		}
	})

	t.Run("invalid regex does not add default mismatch", func(t *testing.T) {
		t.Parallel()
		errs := Flag{
			Name:         "mode",
			Description:  "Execution mode",
			DefaultValue: "debug",
			Validation:   "[",
		}.defaultValueValidationErrors()
		if len(errs) != 0 {
			t.Fatalf("defaultValueValidationErrors() = %v, want invalid regex to skip default mismatch", errs)
		}
	})

	t.Run("valid regex adds mismatch", func(t *testing.T) {
		t.Parallel()
		errs := Flag{
			Name:         "mode",
			Description:  "Execution mode",
			DefaultValue: "debug",
			Validation:   "^release$",
		}.defaultValueValidationErrors()
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "does not match validation pattern") {
			t.Fatalf("defaultValueValidationErrors() = %v, want regex mismatch", errs)
		}
	})
}

func requireFlagMutationErrorAs[T error](t *testing.T, err error) T {
	t.Helper()

	got, ok := errors.AsType[T](err)
	if !ok {
		var zero T
		t.Fatalf("error = %v, want %T", err, zero)
	}
	return got
}

func flagMutationFieldErrorsContainSentinel(errs []error, sentinel error) bool {
	for _, err := range errs {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}
