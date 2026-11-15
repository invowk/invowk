// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"

	"github.com/invowk/invowk/pkg/types"
)

// containerArgNetworkPrefix prefixes the single --network=<mode> argument.
// One argument (not "--network <mode>") keeps image detection in argument
// transformers correct: a separate "pasta:-T,<port>" value would be taken as
// the image.
const containerArgNetworkPrefix = "--network="

var (
	// ErrInvalidNetworkMode is returned when a NetworkMode value is malformed.
	ErrInvalidNetworkMode = errors.New("invalid container network mode")

	networkModePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(:[A-Za-z0-9,._=/-]+)?$`)
)

type (
	// NetworkMode is a container network mode emitted as --network=<mode> on
	// "run" only (for example "pasta:-T,43123"). Empty means the engine default.
	NetworkMode string

	// InvalidNetworkModeError is returned when a NetworkMode value is malformed.
	InvalidNetworkModeError struct {
		Value NetworkMode
	}
)

// PastaForwardNetworkMode returns the Podman pasta mode that forwards exactly
// one TCP port from the container to host loopback.
func PastaForwardNetworkMode(port types.ListenPort) (NetworkMode, error) {
	mode := NetworkMode(fmt.Sprintf("pasta:-T,%d", int(port)))
	if port <= 0 {
		return "", &InvalidNetworkModeError{Value: mode}
	}
	if err := mode.Validate(); err != nil {
		return "", err
	}
	return mode, nil
}

// String returns the mode.
func (m NetworkMode) String() string { return string(m) }

// Validate accepts the empty default and "<driver>[:<options>]" without
// whitespace or a leading dash.
func (m NetworkMode) Validate() error {
	if m == "" || networkModePattern.MatchString(string(m)) {
		return nil
	}
	return &InvalidNetworkModeError{Value: m}
}

// Error implements error.
func (e *InvalidNetworkModeError) Error() string {
	return fmt.Sprintf("%s %q", ErrInvalidNetworkMode, e.Value)
}

// Unwrap returns ErrInvalidNetworkMode.
func (e *InvalidNetworkModeError) Unwrap() error { return ErrInvalidNetworkMode }

// IsLocalInterfaceAddr reports whether addr is assigned to a local network
// interface, according to interfaceAddrs (net.InterfaceAddrs in production).
func IsLocalInterfaceAddr(addr netip.Addr, interfaceAddrs func() ([]netip.Prefix, error)) bool {
	if !addr.IsValid() || interfaceAddrs == nil {
		return false
	}
	prefixes, err := interfaceAddrs()
	if err != nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Addr().Unmap() == addr.Unmap() {
			return true
		}
	}
	return false
}
