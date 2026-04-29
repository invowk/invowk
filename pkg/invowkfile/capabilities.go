// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"time"
)

const (
	// CapabilityLocalAreaNetwork checks for Local Area Network presence
	CapabilityLocalAreaNetwork CapabilityName = "local-area-network"
	// CapabilityInternet checks for working Internet connectivity
	CapabilityInternet CapabilityName = "internet"
	// CapabilityContainers checks for available container engine (Docker or Podman)
	CapabilityContainers CapabilityName = "containers"
	// CapabilityTTY checks if invowk is running in an interactive TTY
	CapabilityTTY CapabilityName = "tty"

	// DefaultCapabilityTimeout is the default timeout for capability checks
	DefaultCapabilityTimeout = 5 * time.Second
)

// ErrInvalidCapabilityName is returned when a CapabilityName value is not recognized.
var ErrInvalidCapabilityName = errors.New("invalid capability name")

type (
	// CapabilityName represents a system capability type.
	//
	//goplint:nonzero,enum-cue=#CapabilityName
	CapabilityName string

	// InvalidCapabilityNameError is returned when a CapabilityName value is not recognized.
	// It wraps ErrInvalidCapabilityName for errors.Is() compatibility.
	InvalidCapabilityNameError struct {
		Value CapabilityName
	}

	// CapabilityError represents an error when a capability check fails at runtime.
	// Distinct from InvalidCapabilityNameError which validates the name itself.
	CapabilityError struct {
		Capability CapabilityName
		Message    string
	}
)

// Error implements the error interface for InvalidCapabilityNameError.
func (e *InvalidCapabilityNameError) Error() string {
	return fmt.Sprintf("invalid capability name %q (valid: local-area-network, internet, containers, tty)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidCapabilityNameError) Unwrap() error {
	return ErrInvalidCapabilityName
}

// Error implements the error interface
func (e *CapabilityError) Error() string {
	return fmt.Sprintf("capability %q not available: %s", e.Capability, e.Message)
}

// String returns the string representation of the CapabilityName.
func (c CapabilityName) String() string { return string(c) }

// Validate returns nil if the CapabilityName is one of the defined capability names,
// or a validation error if it is not.
func (c CapabilityName) Validate() error {
	switch c {
	case CapabilityLocalAreaNetwork, CapabilityInternet, CapabilityContainers, CapabilityTTY:
		return nil
	default:
		return &InvalidCapabilityNameError{Value: c}
	}
}

// ValidCapabilityNames returns all valid capability names
func ValidCapabilityNames() []CapabilityName {
	return []CapabilityName{
		CapabilityLocalAreaNetwork,
		CapabilityInternet,
		CapabilityContainers,
		CapabilityTTY,
	}
}
