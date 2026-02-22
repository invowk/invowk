// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
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

// ErrInvalidFlagType is returned when a FlagType value is not one of the defined types.
var ErrInvalidFlagType = errors.New("invalid flag type")

type (
	// FlagType represents the data type of a flag
	FlagType string

	// InvalidFlagTypeError is returned when a FlagType value is not recognized.
	// It wraps ErrInvalidFlagType for errors.Is() compatibility.
	InvalidFlagTypeError struct {
		Value FlagType
	}

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

// Error implements the error interface for InvalidFlagTypeError.
func (e *InvalidFlagTypeError) Error() string {
	return fmt.Sprintf("invalid flag type %q (valid: string, bool, int, float)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidFlagTypeError) Unwrap() error {
	return ErrInvalidFlagType
}

// IsValid returns whether the FlagType is one of the defined flag types,
// and a list of validation errors if it is not.
// Note: the zero value ("") is valid — it is treated as "string" by GetType().
func (ft FlagType) IsValid() (bool, []error) {
	switch ft {
	case FlagTypeString, FlagTypeBool, FlagTypeInt, FlagTypeFloat, "":
		return true, nil
	default:
		return false, []error{&InvalidFlagTypeError{Value: ft}}
	}
}

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
	if err := validateValueType(value, f.GetType()); err != nil {
		return fmt.Errorf("flag '%s' value '%s' is invalid: %s", f.Name, value, err.Error())
	}
	if err := validateValueWithRegex("flag '"+f.Name+"'", value, f.Validation); err != nil {
		return err
	}
	return nil
}
