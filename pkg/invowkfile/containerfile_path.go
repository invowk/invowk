// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"path/filepath"
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
		Value  ContainerfilePath
		Reason string
	}
)

// String returns the string representation of the ContainerfilePath.
func (p ContainerfilePath) String() string { return string(p) }

// Validate returns nil if the ContainerfilePath is valid, or a validation error if not.
// The zero value ("") is valid. Non-zero values must be relative, non-empty,
// free of NUL bytes, and within the configured path length limit. Base-directory
// traversal checks are context-dependent and remain in the structural validator.
func (p ContainerfilePath) Validate() error {
	if p == "" {
		return nil
	}
	path := string(p)
	if strings.TrimSpace(path) == "" {
		return &InvalidContainerfilePathError{Value: p, Reason: "non-empty value must not be whitespace-only"}
	}
	if len(path) > MaxPathLength {
		return &InvalidContainerfilePathError{Value: p, Reason: fmt.Sprintf("path too long (%d chars, max %d)", len(path), MaxPathLength)}
	}
	if filepath.IsAbs(path) {
		return &InvalidContainerfilePathError{Value: p, Reason: "path must be relative, not absolute"}
	}
	if strings.ContainsRune(path, '\x00') {
		return &InvalidContainerfilePathError{Value: p, Reason: "path contains null byte"}
	}
	return nil
}

// Error implements the error interface for InvalidContainerfilePathError.
func (e *InvalidContainerfilePathError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "non-empty value must not be whitespace-only"
	}
	return fmt.Sprintf("invalid containerfile path %q: %s", e.Value, reason)
}

// Unwrap returns ErrInvalidContainerfilePath for errors.Is() compatibility.
func (e *InvalidContainerfilePathError) Unwrap() error { return ErrInvalidContainerfilePath }
