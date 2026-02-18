// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"regexp"
	"strconv"
)

// validateValueType validates that a value is compatible with the specified type name.
// Shared by both flag and argument validation to avoid duplicating type-check logic.
// Uses typed constants (FlagTypeBool, etc.) for case matching rather than raw string literals.
func validateValueType(value, typeName string) error {
	switch typeName {
	case string(FlagTypeBool):
		if value != "true" && value != "false" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
	case string(FlagTypeInt):
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
	case string(FlagTypeFloat):
		if value == "" {
			return fmt.Errorf("must be a valid floating-point number")
		}
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("must be a valid floating-point number")
		}
	case string(FlagTypeString):
		// Any string is valid
	default:
		// Default to string (any value is valid)
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
