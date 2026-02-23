// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidContainerfilePath is the sentinel error wrapped by InvalidContainerfilePathError.
var ErrInvalidContainerfilePath = errors.New("invalid containerfile path")

type (
	// ContainerfilePath represents a path to a Containerfile/Dockerfile for container runtime.
	// The zero value ("") is valid (means no containerfile specified).
	// Mutually exclusive with ContainerImage in RuntimeConfig.
	// Non-zero values must not be whitespace-only.
	ContainerfilePath string

	// InvalidContainerfilePathError is returned when a ContainerfilePath value is
	// non-empty but whitespace-only. It wraps ErrInvalidContainerfilePath for errors.Is().
	InvalidContainerfilePathError struct {
		Value ContainerfilePath
	}
)

// String returns the string representation of the ContainerfilePath.
func (p ContainerfilePath) String() string { return string(p) }

// IsValid returns whether the ContainerfilePath is valid.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (p ContainerfilePath) IsValid() (bool, []error) {
	if p == "" {
		return true, nil
	}
	if strings.TrimSpace(string(p)) == "" {
		return false, []error{&InvalidContainerfilePathError{Value: p}}
	}
	return true, nil
}

// Error implements the error interface for InvalidContainerfilePathError.
func (e *InvalidContainerfilePathError) Error() string {
	return fmt.Sprintf("invalid containerfile path %q: non-empty value must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidContainerfilePath for errors.Is() compatibility.
func (e *InvalidContainerfilePathError) Unwrap() error { return ErrInvalidContainerfilePath }
