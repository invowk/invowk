// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateContainerImage validates a container image name format.
// Valid formats:
//   - image
//   - image:tag
//   - registry/image
//   - registry/image:tag
//   - registry:port/image:tag
//   - registry/namespace/image:tag
func ValidateContainerImage(image string) error {
	if image == "" {
		return nil // Empty is valid (will use Containerfile)
	}

	// Basic length check
	if len(image) > 512 {
		return fmt.Errorf("container image name too long (%d chars, max 512)", len(image))
	}

	// Check for obvious injection attempts
	if strings.ContainsAny(image, ";&|`$(){}[]<>\\'\"\n\r\t") {
		return fmt.Errorf("container image name contains invalid characters")
	}

	// Basic format validation using a permissive regex
	// Format: [registry[:port]/][namespace/]name[:tag][@digest]
	// Allow: registry:port/image, registry/namespace/image:tag, image@sha256:...
	imageRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._:/-]*[a-zA-Z0-9])?(:[a-zA-Z0-9._-]+)?(@sha256:[a-fA-F0-9]{64})?$`)
	if !imageRegex.MatchString(image) {
		return fmt.Errorf("container image name '%s' has invalid format", image)
	}

	return nil
}

// ValidateVolumeMount validates a container volume mount specification.
// Valid formats:
//   - /host/path:/container/path
//   - /host/path:/container/path:ro
//   - /host/path:/container/path:rw
//   - relative/path:/container/path
//   - named-volume:/container/path
func ValidateVolumeMount(volume string) error {
	if volume == "" {
		return fmt.Errorf("volume mount cannot be empty")
	}

	// Check length
	if len(volume) > 4096 {
		return fmt.Errorf("volume mount specification too long")
	}

	// Check for shell injection characters
	if strings.ContainsAny(volume, ";&|`$(){}[]<>\\'\"\n\r\t") {
		return fmt.Errorf("volume mount contains invalid characters")
	}

	// Split by colon - expect 2 or 3 parts
	parts := strings.Split(volume, ":")

	// Handle Windows paths with drive letters (e.g., C:\path:/container)
	if len(parts) >= 2 && len(parts[0]) == 1 && isWindowsDriveLetter(parts[0][0]) {
		// Reconstruct Windows path
		if len(parts) < 3 {
			return fmt.Errorf("volume mount '%s' has invalid format (expected host:container)", volume)
		}
		// Windows path: C:\path -> parts[0]="C", parts[1]="\path"
		// Rejoin: hostPath = "C:" + parts[1], containerPath = parts[2], options = parts[3:]
		parts = append([]string{parts[0] + ":" + parts[1]}, parts[2:]...)
	}

	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("volume mount '%s' has invalid format (expected host:container[:options])", volume)
	}

	hostPath := parts[0]
	containerPath := parts[1]

	// Validate host path is not empty
	if hostPath == "" {
		return fmt.Errorf("volume mount host path cannot be empty")
	}

	// Validate container path
	if containerPath == "" {
		return fmt.Errorf("volume mount container path cannot be empty")
	}
	if !strings.HasPrefix(containerPath, "/") {
		return fmt.Errorf("volume mount container path must be absolute (start with /)")
	}

	// Validate options if present
	if len(parts) == 3 {
		options := strings.ToLower(parts[2])
		validOptions := map[string]bool{
			"ro": true, "rw": true,
			"z": true, "Z": true, // SELinux labels
			"shared": true, "slave": true, "private": true,
			"rshared": true, "rslave": true, "rprivate": true,
			"nocopy": true, "copy": true,
		}
		for opt := range strings.SplitSeq(options, ",") {
			opt = strings.TrimSpace(opt)
			if opt == "" {
				continue
			}
			if !validOptions[opt] {
				return fmt.Errorf("volume mount has invalid option '%s'", opt)
			}
		}
	}

	// Check for sensitive path patterns (security)
	sensitivePaths := []string{
		"/etc/shadow", "/etc/passwd", "/etc/sudoers",
		"/.ssh", "/root/.ssh", "/home/*/.ssh",
		"/etc/ssl/private", "/var/run/docker.sock",
		"/proc", "/sys", "/dev",
	}
	lowerHost := strings.ToLower(hostPath)
	for _, sensitive := range sensitivePaths {
		if strings.Contains(sensitive, "*") {
			// Simple glob matching for patterns like /home/*/.ssh
			pattern := strings.ReplaceAll(sensitive, "*", "")
			parts := strings.Split(pattern, "/")
			if matchesSensitivePattern(lowerHost, parts) {
				return fmt.Errorf("volume mount attempts to mount sensitive path pattern '%s'", sensitive)
			}
		} else if strings.HasPrefix(lowerHost, sensitive) || lowerHost == sensitive {
			return fmt.Errorf("volume mount attempts to mount sensitive path '%s'", sensitive)
		}
	}

	return nil
}

// isWindowsDriveLetter returns true if c is a valid Windows drive letter.
func isWindowsDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// matchesSensitivePattern checks if a path matches a sensitive pattern.
func matchesSensitivePattern(path string, patternParts []string) bool {
	for _, part := range patternParts {
		if part == "" {
			continue
		}
		if strings.Contains(path, part) {
			return true
		}
	}
	return false
}

// ValidatePortMapping validates a container port mapping specification.
// Valid formats:
//   - containerPort
//   - hostPort:containerPort
//   - hostIP:hostPort:containerPort
//   - hostPort:containerPort/protocol
func ValidatePortMapping(port string) error {
	if port == "" {
		return fmt.Errorf("port mapping cannot be empty")
	}

	// Check for invalid characters
	if strings.ContainsAny(port, ";&|`$(){}[]<>\\'\"\n\r\t ") {
		return fmt.Errorf("port mapping contains invalid characters")
	}

	// Remove protocol suffix if present
	portSpec := port
	if idx := strings.LastIndex(port, "/"); idx != -1 {
		protocol := strings.ToLower(port[idx+1:])
		if protocol != "tcp" && protocol != "udp" && protocol != "sctp" {
			return fmt.Errorf("port mapping has invalid protocol '%s' (expected tcp, udp, or sctp)", protocol)
		}
		portSpec = port[:idx]
	}

	// Split by colon
	parts := strings.Split(portSpec, ":")

	if len(parts) > 3 {
		return fmt.Errorf("port mapping '%s' has invalid format", port)
	}

	// Validate each port number
	for i, part := range parts {
		if part == "" {
			if i == 0 && len(parts) == 3 {
				// Empty host IP is allowed
				continue
			}
			return fmt.Errorf("port mapping has empty port value")
		}

		// Check if it's an IP address (first part of 3-part format)
		if i == 0 && len(parts) == 3 {
			// This should be an IP address
			if !isValidIPAddress(part) {
				return fmt.Errorf("port mapping has invalid host IP '%s'", part)
			}
			continue
		}

		// Parse port number or range
		if strings.Contains(part, "-") {
			// Port range
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return fmt.Errorf("port mapping has invalid port range '%s'", part)
			}
			for _, rp := range rangeParts {
				if err := validatePortNumber(rp); err != nil {
					return err
				}
			}
		} else {
			if err := validatePortNumber(part); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePortNumber checks if a string is a valid port number (1-65535).
func validatePortNumber(s string) error {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fmt.Errorf("port '%s' is not a valid number", s)
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return fmt.Errorf("port %d is out of range (1-65535)", n)
		}
	}
	if n == 0 {
		return fmt.Errorf("port number cannot be 0")
	}
	return nil
}

// isValidIPAddress performs a simple validation of an IP address string.
func isValidIPAddress(s string) bool {
	// Accept IPv4 or IPv6
	if s == "" {
		return false
	}

	// Check for IPv6
	if strings.Contains(s, ":") {
		// Basic IPv6 validation
		return regexp.MustCompile(`^[a-fA-F0-9:]+$`).MatchString(s)
	}

	// IPv4 validation
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		n := 0
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}
