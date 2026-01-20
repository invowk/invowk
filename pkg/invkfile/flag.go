// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"fmt"
	"regexp"
	"strconv"
)

const (
	// FlagTypeString is the default flag type for string values
	FlagTypeString FlagType = "string"
	// FlagTypeBool is for boolean flags (true/false)
	FlagTypeBool FlagType = "bool"
	// FlagTypeInt is for integer flags
	FlagTypeInt FlagType = "int"
	// FlagTypeFloat is for floating-point flags
	FlagTypeFloat FlagType = "float"
)

type (
	// FlagType represents the data type of a flag
	FlagType string

	// Flag represents a command-line flag for a command
	Flag struct {
		// Name is the flag name (POSIX-compliant: starts with a letter, alphanumeric/hyphen/underscore)
		Name string `json:"name"`
		// Description provides help text for the flag
		Description string `json:"description"`
		// DefaultValue is the default value for the flag (optional)
		DefaultValue string `json:"default_value,omitempty"`
		// Type specifies the data type of the flag (optional, defaults to "string")
		// Supported types: "string", "bool", "int", "float"
		Type FlagType `json:"type,omitempty"`
		// Required indicates whether this flag must be provided (optional, defaults to false)
		Required bool `json:"required,omitempty"`
		// Short is a single-character alias for the flag (optional)
		Short string `json:"short,omitempty"`
		// Validation is a regex pattern to validate the flag value (optional)
		Validation string `json:"validation,omitempty"`
	}
)

// GetType returns the effective type of the flag (defaults to "string" if not specified)
func (f *Flag) GetType() FlagType {
	if f.Type == "" {
		return FlagTypeString
	}
	return f.Type
}

// ValidateFlagValue validates a flag value at runtime against type and validation regex.
// Returns nil if the value is valid, or an error describing the issue.
func (f *Flag) ValidateFlagValue(value string) error {
	// Validate type
	if err := validateFlagValueType(value, f.GetType()); err != nil {
		return fmt.Errorf("flag '%s' value '%s' is invalid: %s", f.Name, value, err.Error())
	}

	// Validate against regex pattern
	if f.Validation != "" {
		validationRegex, err := regexp.Compile(f.Validation)
		if err != nil {
			// This shouldn't happen as the regex is validated at parse time
			return fmt.Errorf("flag '%s' has invalid validation pattern: %s", f.Name, err.Error())
		}
		if !validationRegex.MatchString(value) {
			return fmt.Errorf("flag '%s' value '%s' does not match required pattern '%s'", f.Name, value, f.Validation)
		}
	}

	return nil
}

// validateFlagValueType validates that a value is compatible with the specified flag type
func validateFlagValueType(value string, flagType FlagType) error {
	switch flagType {
	case FlagTypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
	case FlagTypeInt:
		// Check if value is a valid integer
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
		// Check if value is a valid floating-point number
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
		// Default to string (any value is valid)
	}
	return nil
}
