// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"fmt"
	"net"
	"time"
)

// DefaultCapabilityTimeout is the default timeout for capability checks
const DefaultCapabilityTimeout = 5 * time.Second

// CapabilityError represents an error when a capability check fails
type CapabilityError struct {
	Capability CapabilityName
	Message    string
}

// Error implements the error interface
func (e *CapabilityError) Error() string {
	return fmt.Sprintf("capability %q not available: %s", e.Capability, e.Message)
}

// CheckCapability validates that a system capability is available.
// Returns nil if the capability is available, or an error describing why it's not.
func CheckCapability(cap CapabilityName) error {
	switch cap {
	case CapabilityLocalAreaNetwork:
		return checkLocalAreaNetwork()
	case CapabilityInternet:
		return checkInternet()
	default:
		return &CapabilityError{
			Capability: cap,
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

	var lastErr error
	for _, server := range dnsServers {
		conn, err := net.DialTimeout("udp", server, DefaultCapabilityTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		conn.Close()

		// Additionally, try a DNS lookup to verify DNS resolution works
		// This is a lightweight operation that verifies full connectivity
		_, err = net.LookupHost("dns.google")
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

// ValidCapabilityNames returns all valid capability names
func ValidCapabilityNames() []CapabilityName {
	return []CapabilityName{
		CapabilityLocalAreaNetwork,
		CapabilityInternet,
	}
}

// IsValidCapabilityName checks if a capability name is valid
func IsValidCapabilityName(name CapabilityName) bool {
	for _, valid := range ValidCapabilityNames() {
		if name == valid {
			return true
		}
	}
	return false
}
