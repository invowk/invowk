// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidTUIServerURL is the sentinel error wrapped by InvalidTUIServerURLError.
	ErrInvalidTUIServerURL = errors.New("invalid TUI server URL")

	// ErrInvalidTUIServerToken is the sentinel error wrapped by InvalidTUIServerTokenError.
	ErrInvalidTUIServerToken = errors.New("invalid TUI server token")
)

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

	// TUIServerToken represents an authentication token for the TUI server.
	// The zero value ("") is valid and means no token is configured.
	// Non-zero values must not be whitespace-only.
	TUIServerToken string

	// InvalidTUIServerTokenError is returned when a TUIServerToken is non-empty
	// but whitespace-only.
	InvalidTUIServerTokenError struct {
		Value TUIServerToken
	}
)

// String returns the string representation of the TUIServerURL.
func (u TUIServerURL) String() string { return string(u) }

// IsValid returns whether the TUIServerURL is valid.
// The zero value ("") is valid. Non-zero values must start with "http://" or "https://".
func (u TUIServerURL) IsValid() (bool, []error) {
	if u == "" {
		return true, nil
	}
	s := string(u)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false, []error{&InvalidTUIServerURLError{Value: u}}
	}
	return true, nil
}

// Error implements the error interface for InvalidTUIServerURLError.
func (e *InvalidTUIServerURLError) Error() string {
	return fmt.Sprintf("invalid TUI server URL %q: must start with http:// or https://", e.Value)
}

// Unwrap returns ErrInvalidTUIServerURL for errors.Is() compatibility.
func (e *InvalidTUIServerURLError) Unwrap() error { return ErrInvalidTUIServerURL }

// String returns the string representation of the TUIServerToken.
func (t TUIServerToken) String() string { return string(t) }

// IsValid returns whether the TUIServerToken is valid.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (t TUIServerToken) IsValid() (bool, []error) {
	if t == "" {
		return true, nil
	}
	if strings.TrimSpace(string(t)) == "" {
		return false, []error{&InvalidTUIServerTokenError{Value: t}}
	}
	return true, nil
}

// Error implements the error interface for InvalidTUIServerTokenError.
func (e *InvalidTUIServerTokenError) Error() string {
	return "invalid TUI server token: non-empty value must not be whitespace-only"
}

// Unwrap returns ErrInvalidTUIServerToken for errors.Is() compatibility.
func (e *InvalidTUIServerTokenError) Unwrap() error { return ErrInvalidTUIServerToken }
