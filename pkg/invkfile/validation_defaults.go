// SPDX-License-Identifier: MPL-2.0

package invkfile

// DefaultValidators returns the standard set of validators used for invkfile validation.
// This includes all built-in validators that check structure, dependencies, and security.
func DefaultValidators() []Validator {
	return []Validator{
		NewStructureValidator(),
	}
}

// NewDefaultValidator creates a CompositeValidator with all default validators.
// This is a convenience function for when you need a single validator instance.
func NewDefaultValidator() *CompositeValidator {
	return NewCompositeValidator(DefaultValidators()...)
}

// HasValidationErrors returns true if the ValidationErrors contains any error-level issues.
// This is a convenience function for checking validation results.
func HasValidationErrors(errs ValidationErrors) bool {
	return errs.HasErrors()
}

// FilterValidationErrors returns only the error-level issues from the validation result.
// This is useful when you want to ignore warnings.
func FilterValidationErrors(errs ValidationErrors) ValidationErrors {
	return errs.Errors()
}

// FilterValidationWarnings returns only the warning-level issues from the validation result.
// This is useful when you want to see warnings separately.
func FilterValidationWarnings(errs ValidationErrors) ValidationErrors {
	return errs.Warnings()
}
