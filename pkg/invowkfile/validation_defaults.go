// SPDX-License-Identifier: MPL-2.0

package invowkfile

// DefaultValidators returns the standard set of validators used for invowkfile validation.
// This includes all built-in validators that check structure, dependencies, and security.
func DefaultValidators() []Validator {
	return []Validator{
		NewStructureValidator(),
	}
}
