// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"regexp"
	"strings"
)

// argNameRegex validates POSIX-compliant argument names.
var argNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// validateArgs validates all args for a command and collects all errors.
func (v *StructureValidator) validateArgs(ctx *ValidationContext, cmd *Command) []ValidationError {
	var errors []ValidationError
	seenNames := make(map[string]bool)
	foundOptional := false
	foundVariadic := false

	for i := range cmd.Args {
		arg := &cmd.Args[i]
		argErrors, isOptional, isVariadic := v.validateArg(ctx, cmd, arg, i, seenNames, foundOptional, foundVariadic)
		errors = append(errors, argErrors...)
		if isOptional {
			foundOptional = true
		}
		if isVariadic {
			foundVariadic = true
		}
	}

	return errors
}

// validateArg validates a single argument and collects all errors.
// Returns the errors, whether this arg is optional, and whether it's variadic.
func (v *StructureValidator) validateArg(ctx *ValidationContext, cmd *Command, arg *Argument, idx int, seenNames map[string]bool, foundOptional, foundVariadic bool) ([]ValidationError, bool, bool) {
	var errors []ValidationError
	path := NewFieldPath().Command(cmd.Name)
	isOptional := !arg.Required
	isVariadic := arg.Variadic

	// Validate name is not empty
	if arg.Name == "" {
		path = path.Copy().ArgIndex(idx)
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a name in invowkfile at " + string(ctx.FilePath),
		})
		return errors, isOptional, isVariadic
	}

	path = path.Copy().Arg(arg.Name)

	// [CUE-VALIDATED] Argument name length also enforced by CUE schema (#Argument.name MaxRunes(256))
	if err := ValidateStringLength(string(arg.Name), "argument name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
		})
	}

	// Validate name is POSIX-compliant
	if !argNameRegex.MatchString(string(arg.Name)) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invowkfile at " + string(ctx.FilePath),
		})
	}

	// Validate description is not empty (after trimming whitespace)
	if strings.TrimSpace(string(arg.Description)) == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a non-empty description in invowkfile at " + string(ctx.FilePath),
		})
	}

	// [CUE-VALIDATED] Argument description length also enforced by CUE schema (#Argument.description MaxRunes(10240))
	if err := ValidateStringLength(string(arg.Description), "argument description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
		})
	}

	// Check for duplicate argument names
	if seenNames[string(arg.Name)] {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     NewFieldPath().Command(cmd.Name).String(),
			Message:   "has duplicate argument name '" + arg.Name.String() + quotedInvowkfileAtSuffix + string(ctx.FilePath),
		})
	}
	seenNames[string(arg.Name)] = true

	// Validate type is valid (if specified) - note: bool is not allowed for args
	if arg.Type != "" && arg.Type != ArgumentTypeString && arg.Type != ArgumentTypeInt && arg.Type != ArgumentTypeFloat {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid type '" + string(arg.Type) + "' (must be 'string', 'int', or 'float') in invowkfile at " + string(ctx.FilePath),
		})
	}

	// Rule: Required arguments must come before optional arguments
	if arg.Required && foundOptional {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "required arguments must come before optional arguments in invowkfile at " + string(ctx.FilePath),
		})
	}

	// Rule: Only the last argument can be variadic
	if foundVariadic {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "only the last argument can be variadic (found after variadic argument) in invowkfile at " + string(ctx.FilePath),
		})
	}

	// Validate validation regex is valid and safe
	if arg.Validation != "" {
		if err := ValidateRegexPattern(string(arg.Validation)); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has unsafe validation regex '" + string(arg.Validation) + "': " + err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
				Cause:     err,
			})
		}
	}

	for _, err := range arg.defaultValueValidationErrors() {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
		})
	}

	return errors, isOptional, isVariadic
}
