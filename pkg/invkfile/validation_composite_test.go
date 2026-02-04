// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"slices"
	"testing"
)

// mockValidator is a test validator that returns configured errors.
type mockValidator struct {
	name   string
	errors []ValidationError
}

func (v *mockValidator) Name() string {
	return v.name
}

func (v *mockValidator) Validate(_ *ValidationContext, _ *Invkfile) []ValidationError {
	return v.errors
}

func TestCompositeValidator_Name(t *testing.T) {
	cv := NewCompositeValidator()
	if cv.Name() != "composite" {
		t.Errorf("Name() = %q, want %q", cv.Name(), "composite")
	}
}

func TestCompositeValidator_EmptyValidate(t *testing.T) {
	cv := NewCompositeValidator()
	ctx := &ValidationContext{}
	inv := &Invkfile{}

	errors := cv.Validate(ctx, inv)
	if len(errors) != 0 {
		t.Errorf("empty composite should return 0 errors, got %d", len(errors))
	}
}

func TestCompositeValidator_SingleValidator(t *testing.T) {
	mock := &mockValidator{
		name: "mock",
		errors: []ValidationError{
			{Validator: "mock", Message: "error 1", Severity: SeverityError},
		},
	}

	cv := NewCompositeValidator(mock)
	ctx := &ValidationContext{}
	inv := &Invkfile{}

	errors := cv.Validate(ctx, inv)
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Message != "error 1" {
		t.Errorf("expected 'error 1', got %q", errors[0].Message)
	}
}

func TestCompositeValidator_MultipleValidators(t *testing.T) {
	mock1 := &mockValidator{
		name: "mock1",
		errors: []ValidationError{
			{Validator: "mock1", Message: "error from mock1", Severity: SeverityError},
		},
	}
	mock2 := &mockValidator{
		name: "mock2",
		errors: []ValidationError{
			{Validator: "mock2", Message: "error from mock2", Severity: SeverityError},
			{Validator: "mock2", Message: "warning from mock2", Severity: SeverityWarning},
		},
	}
	mock3 := &mockValidator{
		name:   "mock3",
		errors: []ValidationError{}, // No errors
	}

	cv := NewCompositeValidator(mock1, mock2, mock3)
	ctx := &ValidationContext{}
	inv := &Invkfile{}

	errors := cv.Validate(ctx, inv)
	if len(errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errors))
	}

	// Verify all validators ran
	validators := make(map[string]bool)
	for _, e := range errors {
		validators[e.Validator] = true
	}
	if !validators["mock1"] || !validators["mock2"] {
		t.Errorf("not all validators ran: %v", validators)
	}
}

func TestCompositeValidator_Add(t *testing.T) {
	cv := NewCompositeValidator()
	if cv.Count() != 0 {
		t.Errorf("initial count should be 0, got %d", cv.Count())
	}

	mock := &mockValidator{
		name: "added",
		errors: []ValidationError{
			{Validator: "added", Message: "added error", Severity: SeverityError},
		},
	}
	cv.Add(mock)

	if cv.Count() != 1 {
		t.Errorf("count after Add should be 1, got %d", cv.Count())
	}

	ctx := &ValidationContext{}
	inv := &Invkfile{}
	errors := cv.Validate(ctx, inv)

	if len(errors) != 1 {
		t.Errorf("expected 1 error after Add, got %d", len(errors))
	}
}

func TestCompositeValidator_Validators(t *testing.T) {
	mock1 := &mockValidator{name: "mock1"}
	mock2 := &mockValidator{name: "mock2"}

	cv := NewCompositeValidator(mock1, mock2)
	validators := cv.Validators()

	if len(validators) != 2 {
		t.Errorf("expected 2 validators, got %d", len(validators))
	}
}

func TestCompositeValidator_CollectsAllErrors(t *testing.T) {
	// This test verifies that the composite collects ALL errors from all validators,
	// not just the first error encountered.

	mock := &mockValidator{
		name: "multi-error",
		errors: []ValidationError{
			{Validator: "multi-error", Field: "field1", Message: "first error", Severity: SeverityError},
			{Validator: "multi-error", Field: "field2", Message: "second error", Severity: SeverityError},
			{Validator: "multi-error", Field: "field3", Message: "third error", Severity: SeverityError},
		},
	}

	cv := NewCompositeValidator(mock)
	ctx := &ValidationContext{}
	inv := &Invkfile{}

	errors := cv.Validate(ctx, inv)
	if len(errors) != 3 {
		t.Errorf("should collect all 3 errors, got %d", len(errors))
	}

	// Verify each error is present
	messages := make([]string, len(errors))
	for i, e := range errors {
		messages[i] = e.Message
	}
	for _, expected := range []string{"first error", "second error", "third error"} {
		if !slices.Contains(messages, expected) {
			t.Errorf("missing expected error: %q", expected)
		}
	}
}
