// SPDX-License-Identifier: MPL-2.0

package invowkfile

import "errors"

const fieldValidatorName ValidatorName = "fields"

// FieldValidator validates the Invowkfile typed value graph.
type FieldValidator struct{}

// NewFieldValidator creates a validator for DDD field-level invariants.
func NewFieldValidator() *FieldValidator {
	return &FieldValidator{}
}

// Name returns the validator name.
func (v *FieldValidator) Name() ValidatorName {
	return fieldValidatorName
}

// Validate checks typed field invariants delegated by Invowkfile.ValidateFields.
func (v *FieldValidator) Validate(_ *ValidationContext, inv *Invowkfile) []ValidationError {
	if inv == nil {
		return nil
	}
	return invowkfileFieldValidationErrors(v.Name(), inv.ValidateFields())
}

func invowkfileFieldValidationErrors(name ValidatorName, err error) []ValidationError {
	if err == nil {
		return nil
	}

	if invErr, ok := errors.AsType[*InvalidInvowkfileError](err); ok {
		validationErrors := make([]ValidationError, 0, len(invErr.FieldErrors))
		for _, fieldErr := range invErr.FieldErrors {
			validationErrors = append(validationErrors, newFieldValidationError(name, fieldErr))
		}
		return validationErrors
	}

	return []ValidationError{newFieldValidationError(name, err)}
}

func newFieldValidationError(name ValidatorName, err error) ValidationError {
	validationErr, buildErr := NewValidationErrorWithCause(name, "invowkfile", err.Error(), SeverityError, err)
	if buildErr != nil {
		return ValidationError{
			Validator: name,
			Field:     "invowkfile",
			Message:   err.Error(),
			Severity:  SeverityError,
			Cause:     err,
		}
	}
	return validationErr
}
