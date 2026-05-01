// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidHostServiceAddress is returned when a host service address is empty.
var ErrInvalidHostServiceAddress = errors.New("invalid host service address")

type (
	// HostServiceAddress is the hostname runtime children use to reach host services.
	HostServiceAddress string

	// InvalidHostServiceAddressError is returned when a HostServiceAddress is invalid.
	InvalidHostServiceAddressError struct {
		Value HostServiceAddress
	}
)

// String returns the raw hostname for environment variables and URLs.
func (h HostServiceAddress) String() string { return string(h) }

// Validate returns nil when the host service address is non-empty.
func (h HostServiceAddress) Validate() error {
	if strings.TrimSpace(string(h)) == "" {
		return &InvalidHostServiceAddressError{Value: h}
	}
	return nil
}

// Error implements the error interface for InvalidHostServiceAddressError.
func (e *InvalidHostServiceAddressError) Error() string {
	return fmt.Sprintf("invalid host service address %q: must not be empty", e.Value)
}

// Unwrap returns ErrInvalidHostServiceAddress for errors.Is compatibility.
func (e *InvalidHostServiceAddressError) Unwrap() error { return ErrInvalidHostServiceAddress }
