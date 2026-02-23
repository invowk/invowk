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
	if len(cmd.Args) == 0 {
		return nil
	}

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
	path := NewFieldPath().Command(string(cmd.Name))
	isOptional := !arg.Required
	isVariadic := arg.Variadic

	// Validate name is not empty
	if arg.Name == "" {
		path = path.Copy().ArgIndex(idx)
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a name in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
		return errors, isOptional, isVariadic
	}

	path = path.Copy().Arg(string(arg.Name))

	// [CUE-VALIDATED] Argument name length also enforced by CUE schema (#Argument.name MaxRunes(256))
	if err := ValidateStringLength(string(arg.Name), "argument name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate name is POSIX-compliant
	if !argNameRegex.MatchString(string(arg.Name)) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate description is not empty (after trimming whitespace)
	if strings.TrimSpace(string(arg.Description)) == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a non-empty description in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Argument description length also enforced by CUE schema (#Argument.description MaxRunes(10240))
	if err := ValidateStringLength(string(arg.Description), "argument description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Check for duplicate argument names
	if seenNames[string(arg.Name)] {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     NewFieldPath().Command(string(cmd.Name)).String(),
			Message:   "has duplicate argument name '" + arg.Name.String() + "' in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}
	seenNames[string(arg.Name)] = true

	// Validate type is valid (if specified) - note: bool is not allowed for args
	if arg.Type != "" && arg.Type != ArgumentTypeString && arg.Type != ArgumentTypeInt && arg.Type != ArgumentTypeFloat {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid type '" + string(arg.Type) + "' (must be 'string', 'int', or 'float') in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate that required arguments don't have default values
	if arg.Required && arg.DefaultValue != "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "cannot be both required and have a default_value in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Rule: Required arguments must come before optional arguments
	if arg.Required && foundOptional {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "required arguments must come before optional arguments in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Rule: Only the last argument can be variadic
	if foundVariadic {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "only the last argument can be variadic (found after variadic argument) in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate default_value is compatible with type
	if arg.DefaultValue != "" {
		if err := validateValueType(arg.DefaultValue, FlagType(arg.GetType())); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "default_value '" + arg.DefaultValue + "' is not compatible with type '" + string(arg.GetType()) + "': " + err.Error() + " in invowkfile at " + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
	}

	// Validate validation regex is valid and safe
	if arg.Validation != "" {
		if err := ValidateRegexPattern(string(arg.Validation)); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has unsafe validation regex '" + string(arg.Validation) + "': " + err.Error() + " in invowkfile at " + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		} else if arg.DefaultValue != "" {
			// Check if default_value matches validation regex
			if !matchesValidation(arg.DefaultValue, string(arg.Validation)) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "default_value '" + arg.DefaultValue + "' does not match validation pattern '" + string(arg.Validation) + "' in invowkfile at " + string(ctx.FilePath),
					Severity:  SeverityError,
				})
			}
		}
	}

	return errors, isOptional, isVariadic
}
