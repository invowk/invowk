// SPDX-License-Identifier: MPL-2.0

package invkfile

// CompositeValidator combines multiple validators into one.
// It runs all validators and collects all errors from each.
type CompositeValidator struct {
	validators []Validator
}

// NewCompositeValidator creates a new CompositeValidator with the given validators.
func NewCompositeValidator(validators ...Validator) *CompositeValidator {
	return &CompositeValidator{
		validators: validators,
	}
}

// Name returns the name of this composite validator.
func (c *CompositeValidator) Name() string {
	return "composite"
}

// Add appends a validator to the composite.
// This allows dynamic composition of validators.
func (c *CompositeValidator) Add(v Validator) {
	c.validators = append(c.validators, v)
}

// Validate runs all contained validators and collects all errors.
// Each validator is run regardless of whether previous validators found errors.
func (c *CompositeValidator) Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError {
	var allErrors []ValidationError

	for _, v := range c.validators {
		errors := v.Validate(ctx, inv)
		allErrors = append(allErrors, errors...)
	}

	return allErrors
}

// Validators returns the list of validators in this composite.
func (c *CompositeValidator) Validators() []Validator {
	return c.validators
}

// Count returns the number of validators in this composite.
func (c *CompositeValidator) Count() int {
	return len(c.validators)
}
