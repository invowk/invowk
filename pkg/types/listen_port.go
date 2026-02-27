// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrInvalidListenPort is the sentinel error wrapped by InvalidListenPortError.
var ErrInvalidListenPort = errors.New("invalid listen port")

type (
	// ListenPort represents a TCP port for server listening.
	// The zero value (0) is valid and means "auto-select an available port".
	// Non-zero values must be in the range 1–65535.
	ListenPort int

	// InvalidListenPortError is returned when a ListenPort value is
	// outside the valid range (0 or 1–65535).
	InvalidListenPortError struct {
		Value ListenPort
	}
)

// String returns the decimal string representation of the ListenPort.
func (p ListenPort) String() string { return strconv.Itoa(int(p)) }

// Validate returns an error if the ListenPort is outside the valid range.
// The zero value (0) means auto-select and is valid.
// Non-zero values must be in the range 1-65535.
func (p ListenPort) Validate() error {
	if p < 0 || p > 65535 {
		return &InvalidListenPortError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidListenPortError.
func (e *InvalidListenPortError) Error() string {
	return fmt.Sprintf("invalid listen port %d: must be 0 (auto-select) or 1-65535", e.Value)
}

// Unwrap returns ErrInvalidListenPort for errors.Is() compatibility.
func (e *InvalidListenPortError) Unwrap() error { return ErrInvalidListenPort }
