// SPDX-License-Identifier: MPL-2.0

package invkfile

// Validate checks the invkfile for errors.
// Returns all validation errors found (not just the first one).
//
// By default, validation uses the standard set of validators. This can be
// customized using functional options:
//
//	// Default validation
//	errs := inv.Validate()
//
//	// With custom filesystem for testing
//	errs := inv.Validate(WithFS(testFS))
//
//	// With strict mode (warnings become errors)
//	errs := inv.Validate(WithStrictMode(true))
//
//	// With additional custom validators
//	errs := inv.Validate(WithAdditionalValidators(myValidator))
//
//	// Replace default validators entirely
//	errs := inv.Validate(WithValidators(myValidator1, myValidator2))
func (inv *Invkfile) Validate(opts ...ValidateOption) ValidationErrors {
	// Build options with defaults based on invkfile
	options := defaultValidateOptions(inv)
	for _, opt := range opts {
		opt(&options)
	}

	// Create validation context
	ctx := options.buildValidationContext(inv)

	// Get validators to run
	validators := options.getValidators()

	// Run all validators and collect errors
	composite := NewCompositeValidator(validators...)
	errors := composite.Validate(ctx, inv)

	// In strict mode, promote warnings to errors
	if options.strictMode {
		for i := range errors {
			if errors[i].Severity == SeverityWarning {
				errors[i].Severity = SeverityError
			}
		}
	}

	return errors
}

// ValidateWithContext checks the invkfile using a pre-built ValidationContext.
// This is useful when you need more control over the validation context.
func (inv *Invkfile) ValidateWithContext(ctx *ValidationContext, validators ...Validator) ValidationErrors {
	if len(validators) == 0 {
		validators = DefaultValidators()
	}

	composite := NewCompositeValidator(validators...)
	return composite.Validate(ctx, inv)
}
