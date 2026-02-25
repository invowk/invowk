// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidDurationString is the sentinel error wrapped by InvalidDurationStringError.
var ErrInvalidDurationString = errors.New("invalid duration string")

type (
	// DurationString represents a Go duration string (e.g., "30s", "5m", "1h30m").
	// The zero value ("") represents "no duration configured" — callers should
	// treat empty as "use default" rather than an error.
	DurationString string

	// InvalidDurationStringError is returned when a DurationString cannot be
	// parsed by time.ParseDuration or represents a zero/negative duration.
	InvalidDurationStringError struct {
		Value  DurationString
		Reason string
	}
)

// Error implements the error interface.
func (e *InvalidDurationStringError) Error() string {
	return fmt.Sprintf("invalid duration string %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidDurationString so callers can use errors.Is for programmatic detection.
func (e *InvalidDurationStringError) Unwrap() error { return ErrInvalidDurationString }

// IsValid returns whether the DurationString is a parseable, positive duration,
// and a list of validation errors if it is not.
// The zero value ("") is valid — it means "no duration configured".
func (d DurationString) IsValid() (bool, []error) {
	if d == "" {
		return true, nil
	}
	dur, err := time.ParseDuration(string(d))
	if err != nil {
		return false, []error{&InvalidDurationStringError{Value: d, Reason: err.Error()}}
	}
	if dur <= 0 {
		return false, []error{&InvalidDurationStringError{Value: d, Reason: "must be a positive duration"}}
	}
	return true, nil
}

// String returns the string representation of the DurationString.
func (d DurationString) String() string { return string(d) }

// parseDuration parses a Go duration string and rejects empty, zero, or negative values.
// Returns (0, nil) when value is empty (caller should apply default).
// The fieldName is used in error messages (e.g., "debounce", "timeout").
func parseDuration(fieldName string, value DurationString) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(string(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", fieldName, value, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be a positive duration", fieldName, value)
	}
	return d, nil
}
