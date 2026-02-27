// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
)

// ErrInvalidRegexPattern is the sentinel error wrapped by InvalidRegexPatternError.
var ErrInvalidRegexPattern = errors.New("invalid regex pattern")

type (
	// RegexPattern represents a Go regular expression pattern string.
	// The zero value ("") is valid and means "no validation pattern".
	// Non-empty values are validated against dangerous patterns, nesting depth,
	// quantifier count, and compilation via ValidateRegexPattern.
	RegexPattern string

	// InvalidRegexPatternError is returned when a RegexPattern cannot be
	// validated by ValidateRegexPattern.
	InvalidRegexPatternError struct {
		Value  RegexPattern
		Reason string
	}
)

// Error implements the error interface.
func (e *InvalidRegexPatternError) Error() string {
	return fmt.Sprintf("invalid regex pattern %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidRegexPattern so callers can use errors.Is for programmatic detection.
func (e *InvalidRegexPatternError) Unwrap() error { return ErrInvalidRegexPattern }

// Validate returns nil if the RegexPattern is a safe, compilable regex,
// or a validation error if it is not.
// The zero value ("") is valid — it means "no validation pattern".
// Delegates to ValidateRegexPattern for dangerous pattern detection,
// nesting depth checks, quantifier count, and compilation.
func (r RegexPattern) Validate() error {
	if r == "" {
		return nil
	}
	if err := ValidateRegexPattern(string(r)); err != nil {
		return &InvalidRegexPatternError{Value: r, Reason: err.Error()}
	}
	return nil
}

// String returns the string representation of the RegexPattern.
func (r RegexPattern) String() string { return string(r) }
