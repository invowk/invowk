// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"regexp"
	"strconv"
)

// validateValueType validates that a value is compatible with the specified type.
// Shared by both flag and argument validation to avoid duplicating type-check logic.
// Argument callers cast via FlagType(arg.GetType()) since ArgumentType values are
// a subset of FlagType values (both are string-based named types).
func validateValueType(value string, typeName FlagType) error {
	switch typeName {
	case FlagTypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
	case FlagTypeInt:
		for i, c := range value {
			if i == 0 && c == '-' {
				continue // Allow negative sign at start
			}
			if c < '0' || c > '9' {
				return fmt.Errorf("must be a valid integer")
			}
		}
		if value == "" || value == "-" {
			return fmt.Errorf("must be a valid integer")
		}
	case FlagTypeFloat:
		if value == "" {
			return fmt.Errorf("must be a valid floating-point number")
		}
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("must be a valid floating-point number")
		}
	case FlagTypeString:
		// Any string is valid
	default:
		// Defense-in-depth: CUE schema enforces valid types at parse time.
		// This catches programmatic misuse where an invalid type bypasses parsing.
		return fmt.Errorf("unknown flag type %q", typeName)
	}
	return nil
}

// validateValueWithRegex validates a value against an optional regex pattern.
// Returns nil if the pattern is empty or matches. name is used for error messages.
func validateValueWithRegex(name, value, pattern string) error {
	if pattern == "" {
		return nil
	}
	validationRegex, err := regexp.Compile(pattern)
	if err != nil {
		// This shouldn't happen as the regex is validated at parse time
		return fmt.Errorf("'%s' has invalid validation pattern: %s", name, err.Error())
	}
	if !validationRegex.MatchString(value) {
		return fmt.Errorf("'%s' value '%s' does not match required pattern '%s'", name, value, pattern)
	}
	return nil
}
