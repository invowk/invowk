// SPDX-License-Identifier: MPL-2.0

package invkfile

import "regexp"

var (
	// flagNameRegex validates POSIX-compliant flag names.
	flagNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

	// invkPrefixRegex matches flags starting with the reserved "invk-" prefix.
	// The "invk-", "invowk-", and "i-" prefix namespaces are all reserved for system flags.
	invkPrefixRegex = regexp.MustCompile(`^invk-`)

	// invowkPrefixRegex matches flags starting with the reserved "invowk-" prefix.
	invowkPrefixRegex = regexp.MustCompile(`^invowk-`)

	// iPrefixRegex matches flags starting with the reserved "i-" prefix.
	iPrefixRegex = regexp.MustCompile(`^i-`)

	// reservedFlagNames maps system-reserved flag names to the long flag they belong to.
	// User-defined flags with these names would conflict with flags injected by invowk
	// at CLI construction time (leaf flags, parent persistent flags, and Cobra/fang built-ins).
	// All invowk system flags use the "invk-" prefix except Cobra's "help" and fang's "version".
	reservedFlagNames = map[string]string{
		"invk-env-file":          "invk-env-file",
		"invk-env-var":           "invk-env-var",
		"invk-env-inherit-mode":  "invk-env-inherit-mode",
		"invk-env-inherit-allow": "invk-env-inherit-allow",
		"invk-env-inherit-deny":  "invk-env-inherit-deny",
		"invk-workdir":           "invk-workdir",
		"invk-runtime":           "invk-runtime",
		"invk-from":              "invk-from",
		"invk-force-rebuild":     "invk-force-rebuild",
		"invk-verbose":           "invk-verbose",
		"invk-config":            "invk-config",
		"invk-interactive":       "invk-interactive",
		"help":                   "help",
		"version":                "version",
	}

	// reservedShortAliases maps reserved single-letter short aliases to the long flag they belong to.
	reservedShortAliases = map[string]string{
		"e": "invk-env-file",
		"E": "invk-env-var",
		"w": "invk-workdir",
		"h": "help",
		"r": "invk-runtime",
		"v": "invk-verbose",
		"i": "invk-interactive",
		"c": "invk-config",
		"f": "invk-from",
	}
)

// validateFlags validates all flags for a command and collects all errors.
func (v *StructureValidator) validateFlags(ctx *ValidationContext, cmd *Command) []ValidationError {
	var errors []ValidationError
	seenNames := make(map[string]bool)
	seenShorts := make(map[string]bool)

	for i := range cmd.Flags {
		flag := &cmd.Flags[i]
		errors = append(errors, v.validateFlag(ctx, cmd, flag, i, seenNames, seenShorts)...)
	}

	return errors
}

// validateFlag validates a single flag and collects all errors.
func (v *StructureValidator) validateFlag(ctx *ValidationContext, cmd *Command, flag *Flag, idx int, seenNames, seenShorts map[string]bool) []ValidationError {
	var errors []ValidationError
	path := NewFieldPath().Command(cmd.Name)

	// Validate name is not empty
	if flag.Name == "" {
		path = path.Copy().FlagIndex(idx)
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a name in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
		return errors // Can't validate further without a name
	}

	path = path.Copy().Flag(flag.Name)

	// [CUE-VALIDATED] Flag name length also enforced by CUE schema (#Flag.name MaxRunes(256))
	if err := ValidateStringLength(flag.Name, "flag name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate name is POSIX-compliant
	if !flagNameRegex.MatchString(flag.Name) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate description is not empty (after trimming whitespace)
	if trimSpace(flag.Description) == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a non-empty description in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Flag description length also enforced by CUE schema (#Flag.description MaxRunes(10240))
	if err := ValidateStringLength(flag.Description, "flag description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Check for duplicate flag names
	if seenNames[flag.Name] {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     NewFieldPath().Command(cmd.Name).String(),
			Message:   "has duplicate flag name '" + flag.Name + "' in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}
	seenNames[flag.Name] = true

	// Reject any flag starting with the reserved "invk-", "invowk-", or "i-" prefixes.
	// These namespaces are reserved for system flags to prevent future conflicts.
	if invkPrefixRegex.MatchString(flag.Name) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "the 'invk-' prefix is reserved for system flags and cannot be used for user-defined flags in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	if invowkPrefixRegex.MatchString(flag.Name) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "the 'invowk-' prefix is reserved for system flags and cannot be used for user-defined flags in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	if iPrefixRegex.MatchString(flag.Name) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "the 'i-' prefix is reserved for system flags and cannot be used for user-defined flags in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Check for reserved flag names
	if longFlag, reserved := reservedFlagNames[flag.Name]; reserved {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "'" + longFlag + "' is a reserved system flag and cannot be used in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate type is valid (if specified)
	if flag.Type != "" && flag.Type != FlagTypeString && flag.Type != FlagTypeBool && flag.Type != FlagTypeInt && flag.Type != FlagTypeFloat {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid type '" + string(flag.Type) + "' (must be 'string', 'bool', 'int', or 'float') in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate that required flags don't have default values
	if flag.Required && flag.DefaultValue != "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "cannot be both required and have a default_value in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate short alias format (single letter a-z or A-Z)
	if flag.Short != "" {
		isValidShort := len(flag.Short) == 1 &&
			((flag.Short[0] >= 'a' && flag.Short[0] <= 'z') || (flag.Short[0] >= 'A' && flag.Short[0] <= 'Z'))
		if !isValidShort {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has invalid short alias '" + flag.Short + "' (must be a single letter a-z or A-Z) in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}

		// Check for reserved short aliases
		if longFlag, reserved := reservedShortAliases[flag.Short]; reserved {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "short alias '" + flag.Short + "' is reserved for the system --" + longFlag + " flag in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}

		// Check for duplicate short aliases
		if seenShorts[flag.Short] {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     NewFieldPath().Command(cmd.Name).String(),
				Message:   "has duplicate short alias '" + flag.Short + "' in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
		seenShorts[flag.Short] = true
	}

	// Validate default_value is compatible with type
	if flag.DefaultValue != "" {
		if err := validateFlagValueType(flag.DefaultValue, flag.GetType()); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "default_value '" + flag.DefaultValue + "' is not compatible with type '" + string(flag.GetType()) + "': " + err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	// Validate validation regex is valid and safe
	if flag.Validation != "" {
		if err := ValidateRegexPattern(flag.Validation); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has unsafe validation regex '" + flag.Validation + "': " + err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		} else if flag.DefaultValue != "" {
			// Check if default_value matches validation regex
			if !matchesValidation(flag.DefaultValue, flag.Validation) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "default_value '" + flag.DefaultValue + "' does not match validation pattern '" + flag.Validation + "' in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}
	}

	return errors
}
