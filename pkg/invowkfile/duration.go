// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"time"

	coretypes "github.com/invowk/invowk/pkg/types"
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

// Validate returns nil if the DurationString is a parseable, positive duration,
// or a validation error if it is not.
// The zero value ("") is valid — it means "no duration configured".
//
//goplint:nonzero
func (d DurationString) Validate() error {
	err := coretypes.OptionalPositiveDurationString(d).Validate()
	if err != nil {
		reason := err.Error()
		if invalid, ok := errors.AsType[*coretypes.InvalidOptionalPositiveDurationStringError](err); ok {
			reason = invalid.Reason
		}
		return &InvalidDurationStringError{Value: d, Reason: reason}
	}
	return nil
}

// String returns the string representation of the DurationString.
func (d DurationString) String() string { return string(d) }

// parseDuration parses an optional Go duration string and rejects zero or
// negative non-empty values.
// Returns (0, nil) when value is empty (caller should apply default).
// The fieldName is used in error messages (e.g., "debounce", "timeout").
func parseDuration(fieldName string, value DurationString) (time.Duration, error) {
	d, err := coretypes.OptionalPositiveDurationString(value).Duration()
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", fieldName, value, err)
	}
	return d, nil
}
