// SPDX-License-Identifier: MPL-2.0

package containerargs

import (
	"errors"
	"fmt"
)

const (
	// MaxContainerNameLength is the maximum portable container name length Invowk accepts.
	MaxContainerNameLength = 128
)

// ErrInvalidContainerName is the sentinel error wrapped by InvalidContainerNameError.
var ErrInvalidContainerName = errors.New("invalid container name")

type (
	// ContainerName is a portable Docker/Podman container name.
	// The zero value is valid for APIs where an omitted name lets the engine assign one.
	ContainerName string

	// InvalidContainerNameError is returned when a container name is invalid.
	InvalidContainerNameError struct {
		Value  ContainerName
		Reason string
	}
)

// String returns the string representation of the ContainerName.
func (n ContainerName) String() string { return string(n) }

// Validate returns nil if the container name is portable across supported
// Docker and Podman targets. Explicit values must be lowercase ASCII and may
// contain digits, dots, underscores, and hyphens.
func (n ContainerName) Validate() error {
	if n == "" {
		return nil
	}

	name := string(n)
	if len(name) > MaxContainerNameLength {
		return &InvalidContainerNameError{
			Value:  n,
			Reason: fmt.Sprintf("must be at most %d characters", MaxContainerNameLength),
		}
	}
	if !isLowerASCIILetterOrDigit(name[0]) {
		return &InvalidContainerNameError{
			Value:  n,
			Reason: "must start with a lowercase ASCII letter or digit",
		}
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		default:
			return &InvalidContainerNameError{
				Value:  n,
				Reason: "must contain only lowercase ASCII letters, digits, '.', '_', or '-'",
			}
		}
	}
	return nil
}

// Error implements the error interface for InvalidContainerNameError.
func (e *InvalidContainerNameError) Error() string {
	return fmt.Sprintf("invalid container name %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidContainerName for errors.Is compatibility.
func (e *InvalidContainerNameError) Unwrap() error { return ErrInvalidContainerName }

//goplint:ignore -- byte parser helper for ASCII container-name grammar.
func isLowerASCIILetterOrDigit(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}
