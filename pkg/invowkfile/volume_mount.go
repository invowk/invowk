// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidVolumeMountSpec is the sentinel error wrapped by InvalidVolumeMountSpecError.
var ErrInvalidVolumeMountSpec = errors.New("invalid volume mount spec")

type (
	// VolumeMountSpec represents a volume mount specification in "host:container[:options]" format.
	// A valid spec must be non-empty and contain at least one ':' separator.
	VolumeMountSpec string

	// InvalidVolumeMountSpecError is returned when a VolumeMountSpec value is
	// empty or missing the required ':' separator.
	InvalidVolumeMountSpecError struct {
		Value  VolumeMountSpec
		Reason string
	}
)

// String returns the string representation of the VolumeMountSpec.
func (v VolumeMountSpec) String() string { return string(v) }

// Validate returns nil if the VolumeMountSpec is valid, or a validation error if not.
// A valid spec must be non-empty and contain at least one ':' separator.
//
//goplint:nonzero
func (v VolumeMountSpec) Validate() error {
	s := string(v)
	if s == "" {
		return &InvalidVolumeMountSpecError{Value: v, Reason: "must not be empty"}
	}
	if !strings.Contains(s, ":") {
		return &InvalidVolumeMountSpecError{Value: v, Reason: "must contain ':' separator (host:container format)"}
	}
	return nil
}

// Error implements the error interface for InvalidVolumeMountSpecError.
func (e *InvalidVolumeMountSpecError) Error() string {
	return fmt.Sprintf("invalid volume mount spec %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidVolumeMountSpec for errors.Is() compatibility.
func (e *InvalidVolumeMountSpecError) Unwrap() error { return ErrInvalidVolumeMountSpec }
