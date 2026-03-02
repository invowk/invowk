// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidTUIServerURL is re-exported from pkg/types for backward compatibility.
	ErrInvalidTUIServerURL = types.ErrInvalidTUIServerURL

	// ErrInvalidTUIServerToken is the sentinel error wrapped by InvalidTUIServerTokenError.
	ErrInvalidTUIServerToken = errors.New("invalid TUI server token")
	// ErrInvalidTUIContext is the sentinel error wrapped by InvalidTUIContextError.
	ErrInvalidTUIContext = errors.New("invalid TUI context")
)

type (
	// TUIServerURL is re-exported from pkg/types for backward compatibility.
	TUIServerURL = types.TUIServerURL

	// InvalidTUIServerURLError is re-exported from pkg/types for backward compatibility.
	InvalidTUIServerURLError = types.InvalidTUIServerURLError

	// TUIServerToken represents an authentication token for the TUI server.
	// The zero value ("") is valid and means no token is configured.
	// Non-zero values must not be whitespace-only.
	TUIServerToken string

	// InvalidTUIServerTokenError is returned when a TUIServerToken is non-empty
	// but whitespace-only.
	InvalidTUIServerTokenError struct {
		Value TUIServerToken
	}

	// InvalidTUIContextError is returned when a TUIContext has invalid fields.
	// It wraps ErrInvalidTUIContext for errors.Is() compatibility and collects
	// field-level validation errors from ServerURL and ServerToken.
	InvalidTUIContextError struct {
		FieldErrors []error
	}
)

// String returns the string representation of the TUIServerToken.
func (t TUIServerToken) String() string { return string(t) }

// Validate returns nil if the TUIServerToken is valid, or a validation error if not.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (t TUIServerToken) Validate() error {
	if t == "" {
		return nil
	}
	if strings.TrimSpace(string(t)) == "" {
		return &InvalidTUIServerTokenError{Value: t}
	}
	return nil
}

// Error implements the error interface for InvalidTUIServerTokenError.
func (e *InvalidTUIServerTokenError) Error() string {
	return "invalid TUI server token: non-empty value must not be whitespace-only"
}

// Unwrap returns ErrInvalidTUIServerToken for errors.Is() compatibility.
func (e *InvalidTUIServerTokenError) Unwrap() error { return ErrInvalidTUIServerToken }

// Error implements the error interface for InvalidTUIContextError.
func (e *InvalidTUIContextError) Error() string {
	return fmt.Sprintf("invalid TUI context: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidTUIContext for errors.Is() compatibility.
func (e *InvalidTUIContextError) Unwrap() error { return ErrInvalidTUIContext }
