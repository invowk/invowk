// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidOptionalPositiveDurationString is the sentinel error wrapped by
// InvalidOptionalPositiveDurationStringError.
var ErrInvalidOptionalPositiveDurationString = errors.New("invalid optional positive duration string")

type (
	// OptionalPositiveDurationString is a Go duration string where the zero value
	// means "not configured" and non-empty values must parse to a positive duration.
	OptionalPositiveDurationString string

	// InvalidOptionalPositiveDurationStringError is returned when an
	// OptionalPositiveDurationString cannot be parsed or is not positive.
	InvalidOptionalPositiveDurationStringError struct {
		Value  OptionalPositiveDurationString
		Reason string
	}
)

// String returns the string representation of the OptionalPositiveDurationString.
func (d OptionalPositiveDurationString) String() string { return string(d) }

// Duration parses the optional duration. The zero value returns 0.
func (d OptionalPositiveDurationString) Duration() (time.Duration, error) {
	if d == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(string(d))
	if err != nil {
		return 0, &InvalidOptionalPositiveDurationStringError{Value: d, Reason: err.Error()}
	}
	if parsed <= 0 {
		return 0, &InvalidOptionalPositiveDurationStringError{Value: d, Reason: "must be a positive duration"}
	}
	return parsed, nil
}

// Validate returns nil if the optional duration is empty or a positive Go duration.
func (d OptionalPositiveDurationString) Validate() error {
	_, err := d.Duration()
	return err
}

// Error implements the error interface.
func (e *InvalidOptionalPositiveDurationStringError) Error() string {
	return fmt.Sprintf("invalid optional positive duration string %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidOptionalPositiveDurationString for errors.Is compatibility.
func (e *InvalidOptionalPositiveDurationStringError) Unwrap() error {
	return ErrInvalidOptionalPositiveDurationString
}
