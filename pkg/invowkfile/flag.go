// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
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

var (
	// ErrInvalidFlagType is returned when a FlagType value is not one of the defined types.
	ErrInvalidFlagType = errors.New("invalid flag type")

	// ErrInvalidFlagName is the sentinel error wrapped by InvalidFlagNameError.
	ErrInvalidFlagName = errors.New("invalid flag name")

	// ErrInvalidFlagShorthand is the sentinel error wrapped by InvalidFlagShorthandError.
	ErrInvalidFlagShorthand = errors.New("invalid flag shorthand")

	// flagNamePattern mirrors the CUE schema constraint: ^[a-zA-Z][a-zA-Z0-9_-]*$
	flagNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

	// flagShorthandPattern validates a single ASCII letter.
	flagShorthandPattern = regexp.MustCompile(`^[a-zA-Z]$`)
)

type (
	// FlagType represents the data type of a flag.
	//
	//goplint:enum-cue=#FlagType
	FlagType string

	// InvalidFlagTypeError is returned when a FlagType value is not recognized.
	// It wraps ErrInvalidFlagType for errors.Is() compatibility.
	InvalidFlagTypeError struct {
		Value FlagType
	}

	// FlagName represents a POSIX-compliant flag identifier.
	// Must start with a letter followed by alphanumeric, underscore, or hyphen characters.
	// Maximum 256 runes. Mirrors the CUE schema constraint.
	FlagName string

	// InvalidFlagNameError is returned when a FlagName does not match the
	// required format or exceeds length limits.
	InvalidFlagNameError struct {
		Value  FlagName
		Reason string
	}

	// FlagShorthand represents a single-character flag alias.
	// Must be exactly one ASCII letter when set.
	// The zero value ("") is valid and means "no shorthand".
	FlagShorthand string

	// InvalidFlagShorthandError is returned when a FlagShorthand is not a
	// single ASCII letter.
	InvalidFlagShorthandError struct {
		Value FlagShorthand
	}

	// Flag represents a command-line flag for a command
	Flag struct {
		// Name is the flag name (POSIX-compliant: starts with a letter, alphanumeric/hyphen/underscore)
		Name FlagName `json:"name"`
		// Description provides help text for the flag
		Description DescriptionText `json:"description"`
		// DefaultValue is the default value for the flag (optional)
		DefaultValue string `json:"default_value,omitempty"`
		// Type specifies the data type of the flag (optional, defaults to "string")
		// Supported types: "string", "bool", "int", "float"
		Type FlagType `json:"type,omitempty"`
		// Required indicates whether this flag must be provided (optional, defaults to false)
		Required bool `json:"required,omitempty"`
		// Short is a single-character alias for the flag (optional)
		Short FlagShorthand `json:"short,omitempty"`
		// Validation is a regex pattern to validate the flag value (optional)
		Validation RegexPattern `json:"validation,omitempty"`
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

// String returns the string representation of the FlagType.
func (ft FlagType) String() string { return string(ft) }

// Validate returns nil if the FlagType is one of the defined flag types,
// or a validation error if it is not.
// Note: the zero value ("") is valid — it is treated as "string" by GetType().
func (ft FlagType) Validate() error {
	switch ft {
	case FlagTypeString, FlagTypeBool, FlagTypeInt, FlagTypeFloat, "":
		return nil
	default:
		return &InvalidFlagTypeError{Value: ft}
	}
}

// Error implements the error interface.
func (e *InvalidFlagNameError) Error() string {
	return fmt.Sprintf("invalid flag name %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidFlagName so callers can use errors.Is for programmatic detection.
func (e *InvalidFlagNameError) Unwrap() error { return ErrInvalidFlagName }

// Validate returns nil if the FlagName matches the required POSIX-like format,
// or a validation error if it does not.
//
//goplint:nonzero
func (n FlagName) Validate() error {
	s := string(n)
	if s == "" {
		return &InvalidFlagNameError{Value: n, Reason: "must not be empty"}
	}
	if utf8.RuneCountInString(s) > MaxNameLength {
		return &InvalidFlagNameError{Value: n, Reason: fmt.Sprintf("exceeds maximum length of %d runes", MaxNameLength)}
	}
	if !flagNamePattern.MatchString(s) {
		return &InvalidFlagNameError{Value: n, Reason: "must start with a letter followed by alphanumeric, underscore, or hyphen characters"}
	}
	return nil
}

// String returns the string representation of the FlagName.
func (n FlagName) String() string { return string(n) }

// Error implements the error interface.
func (e *InvalidFlagShorthandError) Error() string {
	return fmt.Sprintf("invalid flag shorthand %q (must be a single ASCII letter)", e.Value)
}

// Unwrap returns ErrInvalidFlagShorthand so callers can use errors.Is for programmatic detection.
func (e *InvalidFlagShorthandError) Unwrap() error { return ErrInvalidFlagShorthand }

// Validate returns nil if the FlagShorthand is a single ASCII letter,
// or a validation error if it is not.
// The zero value ("") is valid — it means "no shorthand".
func (s FlagShorthand) Validate() error {
	if s == "" {
		return nil
	}
	if !flagShorthandPattern.MatchString(string(s)) {
		return &InvalidFlagShorthandError{Value: s}
	}
	return nil
}

// String returns the string representation of the FlagShorthand.
func (s FlagShorthand) String() string { return string(s) }

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
	if err := validateValueWithRegex("flag '"+f.Name.String()+"'", value, string(f.Validation)); err != nil {
		return err
	}
	return nil
}
