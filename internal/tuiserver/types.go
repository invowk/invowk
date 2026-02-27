// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidAuthToken is the sentinel error wrapped by InvalidAuthTokenError.
var ErrInvalidAuthToken = errors.New("invalid auth token")

type (
	// AuthToken represents an authentication token for TUI server communication.
	// A valid token must be non-empty and not whitespace-only.
	AuthToken string

	// InvalidAuthTokenError is returned when an AuthToken value is
	// empty or whitespace-only.
	InvalidAuthTokenError struct {
		Value AuthToken
	}
)

// String returns the string representation of the AuthToken.
func (t AuthToken) String() string { return string(t) }

// Validate returns nil if the AuthToken is valid (non-empty and not whitespace-only),
// or an error wrapping ErrInvalidAuthToken if it is not.
//
//goplint:nonzero
func (t AuthToken) Validate() error {
	if strings.TrimSpace(string(t)) == "" {
		return &InvalidAuthTokenError{Value: t}
	}
	return nil
}

// Error implements the error interface for InvalidAuthTokenError.
func (e *InvalidAuthTokenError) Error() string {
	return fmt.Sprintf("invalid auth token %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidAuthToken for errors.Is() compatibility.
func (e *InvalidAuthTokenError) Unwrap() error { return ErrInvalidAuthToken }
