// SPDX-License-Identifier: MPL-2.0

package invowkfile

import "fmt"

const (
	// ArgumentTypeString is the default argument type for string values
	ArgumentTypeString ArgumentType = "string"
	// ArgumentTypeInt is for integer arguments
	ArgumentTypeInt ArgumentType = "int"
	// ArgumentTypeFloat is for floating-point arguments
	ArgumentTypeFloat ArgumentType = "float"
)

type (
	// ArgumentType represents the data type of an argument
	ArgumentType string

	// Argument represents a positional command-line argument for a command
	Argument struct {
		// Name is the argument name (POSIX-compliant: starts with a letter, alphanumeric/hyphen/underscore)
		// Used for documentation and environment variable naming (INVOWK_ARG_<NAME>)
		Name string `json:"name"`
		// Description provides help text for the argument
		Description string `json:"description"`
		// Required indicates whether this argument must be provided (optional, defaults to false)
		Required bool `json:"required,omitempty"`
		// DefaultValue is the default value if the argument is not provided (optional)
		DefaultValue string `json:"default_value,omitempty"`
		// Type specifies the data type of the argument (optional, defaults to "string")
		// Supported types: "string", "int", "float"
		Type ArgumentType `json:"type,omitempty"`
		// Validation is a regex pattern to validate the argument value (optional)
		Validation string `json:"validation,omitempty"`
		// Variadic indicates this argument accepts multiple values (optional, defaults to false)
		// Only the last argument can be variadic
		Variadic bool `json:"variadic,omitempty"`
	}
)

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
	if err := validateValueType(value, string(a.GetType())); err != nil {
		return fmt.Errorf("argument '%s' value '%s' is invalid: %s", a.Name, value, err.Error())
	}
	if err := validateValueWithRegex("argument '"+a.Name+"'", value, a.Validation); err != nil {
		return err
	}
	return nil
}
