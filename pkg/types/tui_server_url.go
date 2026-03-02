// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidTUIServerURL is the sentinel error wrapped by InvalidTUIServerURLError.
var ErrInvalidTUIServerURL = errors.New("invalid TUI server URL")

type (
	// TUIServerURL represents the URL of a TUI server for interactive mode.
	// The zero value ("") is valid and means no TUI server is configured.
	// Non-zero values must start with "http://" or "https://".
	TUIServerURL string

	// InvalidTUIServerURLError is returned when a TUIServerURL is non-empty but
	// does not start with a valid HTTP(S) scheme.
	InvalidTUIServerURLError struct {
		Value TUIServerURL
	}
)

// String returns the string representation of the TUIServerURL.
func (u TUIServerURL) String() string { return string(u) }

// Validate returns nil if the TUIServerURL is valid, or a validation error if not.
// The zero value ("") is valid. Non-zero values must start with "http://" or "https://".
func (u TUIServerURL) Validate() error {
	if u == "" {
		return nil
	}
	s := string(u)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return &InvalidTUIServerURLError{Value: u}
	}
	return nil
}

// Error implements the error interface for InvalidTUIServerURLError.
func (e *InvalidTUIServerURLError) Error() string {
	return fmt.Sprintf("invalid TUI server URL %q: must start with http:// or https://", e.Value)
}

// Unwrap returns ErrInvalidTUIServerURL for errors.Is() compatibility.
func (e *InvalidTUIServerURLError) Unwrap() error { return ErrInvalidTUIServerURL }
