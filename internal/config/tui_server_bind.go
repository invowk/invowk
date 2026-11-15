// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
)

const (
	// TUIServerBindRestricted keeps the TUI server on loopback plus, at most,
	// the one address a verified container engine mode routes to the host.
	TUIServerBindRestricted TUIServerBind = "restricted"
	// TUIServerBindAllInterfaces lets container modes without a narrow route
	// (and Docker Desktop) bind 0.0.0.0, with a warning at every such run.
	TUIServerBindAllInterfaces TUIServerBind = "all-interfaces"
)

// ErrInvalidTUIServerBind is returned when a TUIServerBind value is not recognized.
var ErrInvalidTUIServerBind = errors.New("invalid TUI server bind policy")

type (
	// TUIServerBind is the container.tui_server_bind policy. It is read only
	// from the config sources of config-source-resolution, never from an
	// invowkfile or module.
	//
	//goplint:enum-cue=#TUIServerBindType
	TUIServerBind string

	// InvalidTUIServerBindError is returned when a TUIServerBind value is not
	// recognized. It wraps ErrInvalidTUIServerBind for errors.Is() compatibility.
	InvalidTUIServerBindError struct {
		Value TUIServerBind
	}
)

// Error implements the error interface for InvalidTUIServerBindError.
func (e *InvalidTUIServerBindError) Error() string {
	return fmt.Sprintf("invalid container.tui_server_bind %q (valid: restricted, all-interfaces)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidTUIServerBindError) Unwrap() error {
	return ErrInvalidTUIServerBind
}

// String returns the string representation of the TUIServerBind.
func (b TUIServerBind) String() string { return string(b) }

// Validate returns an error if the TUIServerBind is not a defined policy.
// The zero value means the default, "restricted".
func (b TUIServerBind) Validate() error {
	switch b {
	case "", TUIServerBindRestricted, TUIServerBindAllInterfaces:
		return nil
	default:
		return &InvalidTUIServerBindError{Value: b}
	}
}

// AllowsAllInterfaces reports whether the user opted in to the wide bind.
func (b TUIServerBind) AllowsAllInterfaces() bool {
	return b == TUIServerBindAllInterfaces
}
