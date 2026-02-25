// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

const (
	// ArgumentTypeString is the default argument type for string values
	ArgumentTypeString ArgumentType = "string"
	// ArgumentTypeInt is for integer arguments
	ArgumentTypeInt ArgumentType = "int"
	// ArgumentTypeFloat is for floating-point arguments
	ArgumentTypeFloat ArgumentType = "float"
)

var (
	// ErrInvalidArgumentType is returned when an ArgumentType value is not one of the defined types.
	ErrInvalidArgumentType = errors.New("invalid argument type")

	// ErrInvalidArgumentName is the sentinel error wrapped by InvalidArgumentNameError.
	ErrInvalidArgumentName = errors.New("invalid argument name")

	// argumentNamePattern mirrors the CUE schema constraint: ^[a-zA-Z][a-zA-Z0-9_-]*$
	argumentNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

type (
	// ArgumentType represents the data type of an argument
	ArgumentType string

	// InvalidArgumentTypeError is returned when an ArgumentType value is not recognized.
	// It wraps ErrInvalidArgumentType for errors.Is() compatibility.
	InvalidArgumentTypeError struct {
		Value ArgumentType
	}

	// ArgumentName represents a positional argument identifier.
	// Must start with a letter followed by alphanumeric, underscore, or hyphen characters.
	// Maximum 256 runes. Mirrors the CUE schema constraint.
	ArgumentName string

	// InvalidArgumentNameError is returned when an ArgumentName does not match
	// the required format or exceeds length limits.
	InvalidArgumentNameError struct {
		Value  ArgumentName
		Reason string
	}

	// Argument represents a positional command-line argument for a command
	Argument struct {
		// Name is the argument name (POSIX-compliant: starts with a letter, alphanumeric/hyphen/underscore)
		// Used for documentation and environment variable naming (INVOWK_ARG_<NAME>)
		Name ArgumentName `json:"name"`
		// Description provides help text for the argument
		Description DescriptionText `json:"description"`
		// Required indicates whether this argument must be provided (optional, defaults to false)
		Required bool `json:"required,omitempty"`
		// DefaultValue is the default value if the argument is not provided (optional)
		DefaultValue string `json:"default_value,omitempty"`
		// Type specifies the data type of the argument (optional, defaults to "string")
		// Supported types: "string", "int", "float"
		Type ArgumentType `json:"type,omitempty"`
		// Validation is a regex pattern to validate the argument value (optional)
		Validation RegexPattern `json:"validation,omitempty"`
		// Variadic indicates this argument accepts multiple values (optional, defaults to false)
		// Only the last argument can be variadic
		Variadic bool `json:"variadic,omitempty"`
	}
)

// Error implements the error interface for InvalidArgumentTypeError.
func (e *InvalidArgumentTypeError) Error() string {
	return fmt.Sprintf("invalid argument type %q (valid: string, int, float)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidArgumentTypeError) Unwrap() error {
	return ErrInvalidArgumentType
}

// String returns the string representation of the ArgumentType.
func (at ArgumentType) String() string { return string(at) }

// IsValid returns whether the ArgumentType is one of the defined argument types,
// and a list of validation errors if it is not.
// Note: the zero value ("") is valid — it is treated as "string" by GetType().
func (at ArgumentType) IsValid() (bool, []error) {
	switch at {
	case ArgumentTypeString, ArgumentTypeInt, ArgumentTypeFloat, "":
		return true, nil
	default:
		return false, []error{&InvalidArgumentTypeError{Value: at}}
	}
}

// Error implements the error interface.
func (e *InvalidArgumentNameError) Error() string {
	return fmt.Sprintf("invalid argument name %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidArgumentName so callers can use errors.Is for programmatic detection.
func (e *InvalidArgumentNameError) Unwrap() error { return ErrInvalidArgumentName }

// IsValid returns whether the ArgumentName matches the required POSIX-like format,
// and a list of validation errors if it does not.
func (n ArgumentName) IsValid() (bool, []error) {
	s := string(n)
	if s == "" {
		return false, []error{&InvalidArgumentNameError{Value: n, Reason: "must not be empty"}}
	}
	if utf8.RuneCountInString(s) > MaxNameLength {
		return false, []error{&InvalidArgumentNameError{Value: n, Reason: fmt.Sprintf("exceeds maximum length of %d runes", MaxNameLength)}}
	}
	if !argumentNamePattern.MatchString(s) {
		return false, []error{&InvalidArgumentNameError{Value: n, Reason: "must start with a letter followed by alphanumeric, underscore, or hyphen characters"}}
	}
	return true, nil
}

// String returns the string representation of the ArgumentName.
func (n ArgumentName) String() string { return string(n) }

// GetType returns the effective type of the argument (defaults to "string" if not specified)
func (a *Argument) GetType() ArgumentType {
	if a.Type == "" {
		return ArgumentTypeString
	}
	return a.Type
}

// ValidateArgumentValue validates an argument value at runtime against type and validation regex.
// Returns nil if the value is valid, or an error describing the issue.
func (a *Argument) ValidateArgumentValue(value string) error {
	argType := a.GetType()
	// Validate the argument type itself before cross-casting to FlagType.
	// ArgumentType values ("string", "int", "float") are a strict subset of
	// FlagType values, so the cast is safe for all valid ArgumentType values.
	if isValid, errs := argType.IsValid(); !isValid {
		return fmt.Errorf("argument '%s': %w", a.Name, errs[0])
	}
	if err := validateValueType(value, FlagType(argType)); err != nil {
		return fmt.Errorf("argument '%s' value '%s' is invalid: %s", a.Name, value, err.Error())
	}
	if err := validateValueWithRegex("argument '"+a.Name.String()+"'", value, string(a.Validation)); err != nil {
		return err
	}
	return nil
}
