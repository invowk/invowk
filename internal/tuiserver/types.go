// SPDX-License-Identifier: MPL-2.0

package tuiserver

import "github.com/invowk/invowk/internal/tuiwire"

// ErrInvalidAuthToken is the sentinel error wrapped by InvalidAuthTokenError.
var ErrInvalidAuthToken = tuiwire.ErrInvalidAuthToken

type (
	// AuthToken represents an authentication token for TUI server communication.
	// A valid token must be non-empty and not whitespace-only.
	AuthToken = tuiwire.AuthToken

	// InvalidAuthTokenError is returned when an AuthToken value is
	// empty or whitespace-only.
	InvalidAuthTokenError = tuiwire.InvalidAuthTokenError
)
