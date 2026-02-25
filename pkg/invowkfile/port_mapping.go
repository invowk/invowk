// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidPortMappingSpec is the sentinel error wrapped by InvalidPortMappingSpecError.
var ErrInvalidPortMappingSpec = errors.New("invalid port mapping spec")

type (
	// PortMappingSpec represents a port mapping specification in "host:container[/protocol]" format.
	// A valid spec must be non-empty and contain at least one ':' separator.
	PortMappingSpec string

	// InvalidPortMappingSpecError is returned when a PortMappingSpec value is
	// empty or missing the required ':' separator.
	InvalidPortMappingSpecError struct {
		Value  PortMappingSpec
		Reason string
	}
)

// String returns the string representation of the PortMappingSpec.
func (p PortMappingSpec) String() string { return string(p) }

// IsValid returns whether the PortMappingSpec is valid.
// A valid spec must be non-empty and contain at least one ':' separator.
func (p PortMappingSpec) IsValid() (bool, []error) {
	s := string(p)
	if s == "" {
		return false, []error{&InvalidPortMappingSpecError{Value: p, Reason: "must not be empty"}}
	}
	if !strings.Contains(s, ":") {
		return false, []error{&InvalidPortMappingSpecError{Value: p, Reason: "must contain ':' separator (host:container format)"}}
	}
	return true, nil
}

// Error implements the error interface for InvalidPortMappingSpecError.
func (e *InvalidPortMappingSpecError) Error() string {
	return fmt.Sprintf("invalid port mapping spec %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidPortMappingSpec for errors.Is() compatibility.
func (e *InvalidPortMappingSpecError) Unwrap() error { return ErrInvalidPortMappingSpec }
