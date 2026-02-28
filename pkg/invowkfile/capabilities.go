// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/term"
)

const (
	// CapabilityLocalAreaNetwork checks for Local Area Network presence
	CapabilityLocalAreaNetwork CapabilityName = "local-area-network"
	// CapabilityInternet checks for working Internet connectivity
	CapabilityInternet CapabilityName = "internet"
	// CapabilityContainers checks for available container engine (Docker or Podman)
	CapabilityContainers CapabilityName = "containers"
	// CapabilityTTY checks if invowk is running in an interactive TTY
	CapabilityTTY CapabilityName = "tty"

	// DefaultCapabilityTimeout is the default timeout for capability checks
	DefaultCapabilityTimeout = 5 * time.Second
)

// ErrInvalidCapabilityName is returned when a CapabilityName value is not recognized.
var ErrInvalidCapabilityName = errors.New("invalid capability name")

type (
	// CapabilityName represents a system capability type.
	//
	//goplint:nonzero,enum-cue=#CapabilityName
	CapabilityName string

	// InvalidCapabilityNameError is returned when a CapabilityName value is not recognized.
	// It wraps ErrInvalidCapabilityName for errors.Is() compatibility.
	InvalidCapabilityNameError struct {
		Value CapabilityName
	}

	// CapabilityError represents an error when a capability check fails at runtime.
	// Distinct from InvalidCapabilityNameError which validates the name itself.
	CapabilityError struct {
		Capability CapabilityName
		Message    string
	}
)

// Error implements the error interface for InvalidCapabilityNameError.
func (e *InvalidCapabilityNameError) Error() string {
	return fmt.Sprintf("invalid capability name %q (valid: local-area-network, internet, containers, tty)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidCapabilityNameError) Unwrap() error {
	return ErrInvalidCapabilityName
}

// Error implements the error interface
func (e *CapabilityError) Error() string {
	return fmt.Sprintf("capability %q not available: %s", e.Capability, e.Message)
}

// CheckCapability validates that a system capability is available.
// Returns nil if the capability is available, or an error describing why it's not.
func CheckCapability(capability CapabilityName) error {
	switch capability {
	case CapabilityLocalAreaNetwork:
		return checkLocalAreaNetwork()
	case CapabilityInternet:
		return checkInternet()
	case CapabilityContainers:
		return checkContainers()
	case CapabilityTTY:
		return checkTTY()
	default:
		return &CapabilityError{
			Capability: capability,
			Message:    "unknown capability",
		}
	}
}

// checkLocalAreaNetwork checks for Local Area Network presence.
// It looks for non-loopback network interfaces that are up and have valid IP addresses.
func checkLocalAreaNetwork() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return &CapabilityError{
			Capability: CapabilityLocalAreaNetwork,
			Message:    fmt.Sprintf("failed to list network interfaces: %v", err),
		}
	}

	for _, iface := range interfaces {
		// Skip loopback interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Skip interfaces that are not up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Check if interface has any valid unicast addresses
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			// Parse the IP from the address
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			// Skip loopback and link-local addresses
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			// Found a valid network interface with a routable IP
			return nil
		}
	}

	return &CapabilityError{
		Capability: CapabilityLocalAreaNetwork,
		Message:    "no active network interface with routable IP address found",
	}
}

// checkInternet checks for working Internet connectivity.
// It uses DNS lookup to avoid transferring actual data over the network.
func checkInternet() error {
	// First check if we have LAN - no point checking internet without it
	if err := checkLocalAreaNetwork(); err != nil {
		return &CapabilityError{
			Capability: CapabilityInternet,
			Message:    "no local network available",
		}
	}

	// Use multiple DNS servers for redundancy
	// We try to establish a UDP connection to DNS servers
	// This doesn't actually send data, just checks if we can route to them
	dnsServers := []string{
		"8.8.8.8:53",        // Google DNS
		"1.1.1.1:53",        // Cloudflare DNS
		"208.67.222.222:53", // OpenDNS
	}

	dialer := &net.Dialer{Timeout: DefaultCapabilityTimeout}
	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCapabilityTimeout)
	defer cancel()

	var lastErr error
	for _, server := range dnsServers {
		conn, err := dialer.DialContext(ctx, "udp", server)
		if err != nil {
			lastErr = err
			continue
		}
		_ = conn.Close() // Connectivity check; close error non-critical

		// Additionally, try a DNS lookup to verify DNS resolution works
		// This is a lightweight operation that verifies full connectivity
		_, err = resolver.LookupHost(ctx, "dns.google")
		if err != nil {
			lastErr = err
			continue
		}

		// Successfully connected and resolved DNS
		return nil
	}

	msg := "unable to reach internet DNS servers"
	if lastErr != nil {
		msg = fmt.Sprintf("unable to reach internet: %v", lastErr)
	}

	return &CapabilityError{
		Capability: CapabilityInternet,
		Message:    msg,
	}
}

// checkContainers checks if a container engine (Docker or Podman) is available and ready.
func checkContainers() error {
	type engineCandidate struct {
		name string
		args []string
	}

	candidates := []engineCandidate{
		{name: "podman", args: []string{"version", "--format", "{{.Version}}"}},
		{name: "docker", args: []string{"version", "--format", "{{.Server.Version}}"}},
	}

	foundEngine := false
	var lastErr error
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		foundEngine = true

		ctx, cancel := context.WithTimeout(context.Background(), DefaultCapabilityTimeout)
		cmd := exec.CommandContext(ctx, path, candidate.args...)
		err = cmd.Run()
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err
	}

	if !foundEngine {
		return &CapabilityError{
			Capability: CapabilityContainers,
			Message:    "no container engine (podman or docker) found in PATH",
		}
	}

	msg := "container engine is not ready"
	if lastErr != nil {
		msg = fmt.Sprintf("container engine is not ready: %v", lastErr)
	}

	return &CapabilityError{
		Capability: CapabilityContainers,
		Message:    msg,
	}
}

// checkTTY checks whether invowk is running in an interactive terminal.
func checkTTY() error {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		return nil
	}

	return &CapabilityError{
		Capability: CapabilityTTY,
		Message:    "not running in an interactive TTY (stdin/stdout)",
	}
}

// String returns the string representation of the CapabilityName.
func (c CapabilityName) String() string { return string(c) }

// Validate returns nil if the CapabilityName is one of the defined capability names,
// or a validation error if it is not.
func (c CapabilityName) Validate() error {
	switch c {
	case CapabilityLocalAreaNetwork, CapabilityInternet, CapabilityContainers, CapabilityTTY:
		return nil
	default:
		return &InvalidCapabilityNameError{Value: c}
	}
}

// ValidCapabilityNames returns all valid capability names
func ValidCapabilityNames() []CapabilityName {
	return []CapabilityName{
		CapabilityLocalAreaNetwork,
		CapabilityInternet,
		CapabilityContainers,
		CapabilityTTY,
	}
}
