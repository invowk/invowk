// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
)

// ErrInvalidPortMappingSpec is the sentinel error wrapped by InvalidPortMappingSpecError.
var ErrInvalidPortMappingSpec = errors.New("invalid port mapping spec")

type (
	// PortMappingSpec represents a port mapping specification in Docker port syntax.
	PortMappingSpec string

	// InvalidPortMappingSpecError is returned when a PortMappingSpec value is
	// rejected by container port validation.
	InvalidPortMappingSpecError struct {
		Value  PortMappingSpec
		Reason string
	}
)

// String returns the string representation of the PortMappingSpec.
func (p PortMappingSpec) String() string { return string(p) }

// Validate returns nil if the PortMappingSpec is valid, or a validation error if not.
//
//goplint:nonzero
func (p PortMappingSpec) Validate() error {
	if err := ValidatePortMapping(string(p)); err != nil {
		return &InvalidPortMappingSpecError{Value: p, Reason: err.Error()}
	}
	return nil
}

// Error implements the error interface for InvalidPortMappingSpecError.
func (e *InvalidPortMappingSpecError) Error() string {
	return fmt.Sprintf("invalid port mapping spec %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidPortMappingSpec for errors.Is() compatibility.
func (e *InvalidPortMappingSpecError) Unwrap() error { return ErrInvalidPortMappingSpec }
