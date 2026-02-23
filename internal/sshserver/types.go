// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// ErrInvalidHostAddress is the sentinel error wrapped by InvalidHostAddressError.
	ErrInvalidHostAddress = errors.New("invalid host address")
	// ErrInvalidTokenValue is the sentinel error wrapped by InvalidTokenValueError.
	ErrInvalidTokenValue = errors.New("invalid token value")
	// ErrInvalidListenPort is the sentinel error wrapped by InvalidListenPortError.
	ErrInvalidListenPort = errors.New("invalid listen port")
)

type (
	// HostAddress represents a network host address (IP or hostname) for server binding.
	// A valid address must be non-empty and not whitespace-only.
	HostAddress string

	// TokenValue represents an authentication token value for container callbacks.
	// A valid token must be non-empty and not whitespace-only.
	TokenValue string

	// ListenPort represents a TCP port for server listening.
	// The zero value (0) is valid and means "auto-select an available port".
	// Non-zero values must be in the range 1–65535.
	ListenPort int

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

	// InvalidListenPortError is returned when a ListenPort value is
	// outside the valid range (0 or 1–65535).
	InvalidListenPortError struct {
		Value ListenPort
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

// String returns the decimal string representation of the ListenPort.
func (p ListenPort) String() string { return strconv.Itoa(int(p)) }

// IsValid returns whether the ListenPort is valid.
// The zero value (0) means auto-select and is valid.
// Non-zero values must be in the range 1–65535.
func (p ListenPort) IsValid() (bool, []error) {
	if p < 0 || p > 65535 {
		return false, []error{&InvalidListenPortError{Value: p}}
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

// Error implements the error interface for InvalidListenPortError.
func (e *InvalidListenPortError) Error() string {
	return fmt.Sprintf("invalid listen port %d: must be 0 (auto-select) or 1-65535", e.Value)
}

// Unwrap returns ErrInvalidListenPort for errors.Is() compatibility.
func (e *InvalidListenPortError) Unwrap() error { return ErrInvalidListenPort }
