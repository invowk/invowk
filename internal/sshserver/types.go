// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidHostAddress is the sentinel error wrapped by InvalidHostAddressError.
	ErrInvalidHostAddress = errors.New("invalid host address")
	// ErrInvalidTokenValue is the sentinel error wrapped by InvalidTokenValueError.
	ErrInvalidTokenValue = errors.New("invalid token value")
	// ErrInvalidListenPort is re-exported from pkg/types for backward compatibility.
	ErrInvalidListenPort = types.ErrInvalidListenPort
	// ErrInvalidSSHConfig is the sentinel error wrapped by InvalidSSHConfigError.
	ErrInvalidSSHConfig = errors.New("invalid SSH server config")
)

type (
	// HostAddress represents a network host address (IP or hostname) for server binding.
	// A valid address must be non-empty and not whitespace-only.
	HostAddress string

	// TokenValue represents an authentication token value for container callbacks.
	// A valid token must be non-empty and not whitespace-only.
	TokenValue string

	// ListenPort is re-exported from pkg/types as a cross-cutting type
	// used by both sshserver and tuiserver.
	ListenPort = types.ListenPort

	// InvalidHostAddressError is returned when a HostAddress value is
	// empty or whitespace-only.
	InvalidHostAddressError struct {
		Value HostAddress
	}

	// InvalidTokenValueError is returned when a TokenValue value is
	// empty or whitespace-only.
	InvalidTokenValueError struct {
		Value TokenValue
	}

	// InvalidListenPortError is re-exported from pkg/types for backward compatibility.
	InvalidListenPortError = types.InvalidListenPortError

	// InvalidSSHConfigError is returned when an SSH server Config has invalid fields.
	// It wraps ErrInvalidSSHConfig for errors.Is() compatibility and collects
	// field-level validation errors from Host, Port, and DefaultShell.
	InvalidSSHConfigError struct {
		FieldErrors []error
	}
)

// String returns the string representation of the HostAddress.
func (h HostAddress) String() string { return string(h) }

// IsValid returns whether the HostAddress is valid.
// A valid address must be non-empty and not whitespace-only.
func (h HostAddress) IsValid() (bool, []error) {
	if strings.TrimSpace(string(h)) == "" {
		return false, []error{&InvalidHostAddressError{Value: h}}
	}
	return true, nil
}

// String returns the string representation of the TokenValue.
func (t TokenValue) String() string { return string(t) }

// IsValid returns whether the TokenValue is valid.
// A valid token must be non-empty and not whitespace-only.
func (t TokenValue) IsValid() (bool, []error) {
	if strings.TrimSpace(string(t)) == "" {
		return false, []error{&InvalidTokenValueError{Value: t}}
	}
	return true, nil
}

// Error implements the error interface for InvalidHostAddressError.
func (e *InvalidHostAddressError) Error() string {
	return fmt.Sprintf("invalid host address %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidHostAddress for errors.Is() compatibility.
func (e *InvalidHostAddressError) Unwrap() error { return ErrInvalidHostAddress }

// Error implements the error interface for InvalidTokenValueError.
func (e *InvalidTokenValueError) Error() string {
	return fmt.Sprintf("invalid token value %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidTokenValue for errors.Is() compatibility.
func (e *InvalidTokenValueError) Unwrap() error { return ErrInvalidTokenValue }

// Error implements the error interface for InvalidSSHConfigError.
func (e *InvalidSSHConfigError) Error() string {
	return fmt.Sprintf("invalid SSH server config: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidSSHConfig for errors.Is() compatibility.
func (e *InvalidSSHConfigError) Unwrap() error { return ErrInvalidSSHConfig }
