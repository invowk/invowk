// SPDX-License-Identifier: MPL-2.0

// Package containerargs validates raw Docker/Podman argument syntax shared by
// invowkfile schema values and container adapters.
package containerargs

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type (
	// ContainerVolumeMountSpec represents a container volume mount specification
	// in "host:container[:options]" format.
	ContainerVolumeMountSpec string

	// ContainerPortMappingSpec represents a container port mapping specification
	// in Docker/Podman port syntax.
	ContainerPortMappingSpec string
)

// String returns the string representation of the ContainerVolumeMountSpec.
func (s ContainerVolumeMountSpec) String() string { return string(s) }

// Validate returns nil when the volume mount specification is supported.
//
//goplint:nonzero
func (s ContainerVolumeMountSpec) Validate() error {
	return validateContainerVolumeMountSpec(string(s))
}

// String returns the string representation of the ContainerPortMappingSpec.
func (s ContainerPortMappingSpec) String() string { return string(s) }

// Validate returns nil when the port mapping specification is supported.
//
//goplint:nonzero
func (s ContainerPortMappingSpec) Validate() error {
	return validateContainerPortMappingSpec(string(s))
}

//goplint:ignore -- parser helper validates raw Docker/Podman CLI volume syntax.
func validateContainerVolumeMountSpec(volume string) error {
	if volume == "" {
		return errors.New("volume mount cannot be empty")
	}
	if len(volume) > 4096 {
		return errors.New("volume mount specification too long")
	}
	if strings.ContainsAny(volume, ";&|`$(){}[]<>\\'\"\n\r\t") {
		return errors.New("volume mount contains invalid characters")
	}

	parts := strings.Split(volume, ":")
	if len(parts) >= 2 && len(parts[0]) == 1 && isWindowsDriveLetter(parts[0][0]) {
		if len(parts) < 3 {
			return fmt.Errorf("volume mount %q has invalid format (expected host:container)", volume)
		}
		parts = append([]string{parts[0] + ":" + parts[1]}, parts[2:]...)
	}
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("volume mount %q has invalid format (expected host:container[:options])", volume)
	}

	hostPath := parts[0]
	containerPath := parts[1]
	if hostPath == "" {
		return errors.New("volume mount host path cannot be empty")
	}
	if containerPath == "" {
		return errors.New("volume mount container path cannot be empty")
	}
	if !strings.HasPrefix(containerPath, "/") {
		return errors.New("volume mount container path must be absolute (start with /)")
	}
	if len(parts) == 3 {
		if err := validateVolumeMountOptions(parts[2]); err != nil {
			return err
		}
	}
	return validateSensitiveVolumeMountPath(hostPath)
}

//goplint:ignore -- parser helper validates raw Docker/Podman CLI volume options.
func validateVolumeMountOptions(options string) error {
	validOptions := map[string]bool{
		"ro": true, "rw": true,
		"z": true, "Z": true,
		"exec": true, "shared": true, "slave": true, "private": true,
		"rshared": true, "rslave": true, "rprivate": true,
		"nocopy": true, "copy": true,
	}
	for opt := range strings.SplitSeq(strings.ToLower(options), ",") {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		if !validOptions[opt] {
			return fmt.Errorf("volume mount has invalid option %q", opt)
		}
	}
	return nil
}

//goplint:ignore -- parser helper evaluates raw host paths embedded in volume syntax.
func validateSensitiveVolumeMountPath(hostPath string) error {
	sensitivePaths := []string{
		"/etc/shadow", "/etc/passwd", "/etc/sudoers",
		"/.ssh", "/root/.ssh", "/home/*/.ssh",
		"/etc/ssl/private", "/var/run/docker.sock",
		"/proc", "/sys", "/dev",
	}
	lowerHost := strings.ToLower(hostPath)
	for _, sensitive := range sensitivePaths {
		if strings.Contains(sensitive, "*") {
			pattern := strings.ReplaceAll(sensitive, "*", "")
			parts := strings.Split(pattern, "/")
			if matchesSensitivePattern(lowerHost, parts) {
				return fmt.Errorf("volume mount attempts to mount sensitive path pattern %q", sensitive)
			}
			continue
		}
		if isPathBoundaryMatch(lowerHost, sensitive) {
			return fmt.Errorf("volume mount attempts to mount sensitive path %q", sensitive)
		}
	}
	return nil
}

//goplint:ignore -- parser helper compares raw slash-separated host path syntax.
func isPathBoundaryMatch(candidate, base string) bool {
	return candidate == base || strings.HasPrefix(candidate, base+"/")
}

//goplint:ignore -- parser helper validates raw Docker/Podman CLI port syntax.
func validateContainerPortMappingSpec(port string) error {
	if port == "" {
		return errors.New("port mapping cannot be empty")
	}
	if strings.ContainsAny(port, ";&|`$(){}[]<>\\'\"\n\r\t ") {
		return errors.New("port mapping contains invalid characters")
	}

	portSpec := port
	if idx := strings.LastIndex(port, "/"); idx != -1 {
		protocol := strings.ToLower(port[idx+1:])
		if protocol != "tcp" && protocol != "udp" && protocol != "sctp" {
			return fmt.Errorf("port mapping has invalid protocol %q (expected tcp, udp, or sctp)", protocol)
		}
		portSpec = port[:idx]
	}

	parts := strings.Split(portSpec, ":")
	if len(parts) > 3 {
		return fmt.Errorf("port mapping %q has invalid format", port)
	}

	for i, part := range parts {
		if part == "" {
			if i == 0 && len(parts) == 3 {
				continue
			}
			return errors.New("port mapping has empty port value")
		}
		if i == 0 && len(parts) == 3 {
			if !isValidIPAddress(part) {
				return fmt.Errorf("port mapping has invalid host IP %q", part)
			}
			continue
		}
		if err := validatePortPart(part); err != nil {
			return err
		}
	}
	return nil
}

//goplint:ignore -- parser helper validates raw port segments and ranges.
func validatePortPart(part string) error {
	if strings.Contains(part, "-") {
		rangeParts := strings.Split(part, "-")
		if len(rangeParts) != 2 {
			return fmt.Errorf("port mapping has invalid port range %q", part)
		}
		for _, rp := range rangeParts {
			if err := validatePortNumber(rp); err != nil {
				return err
			}
		}
		return nil
	}
	return validatePortNumber(part)
}

//goplint:ignore -- parser helper validates raw port number text.
func validatePortNumber(s string) error {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fmt.Errorf("port %q is not a valid number", s)
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return fmt.Errorf("port %d is out of range (1-65535)", n)
		}
	}
	if n == 0 {
		return errors.New("port number cannot be 0")
	}
	return nil
}

//goplint:ignore -- parser helper inspects raw path syntax.
func isWindowsDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

//goplint:ignore -- parser helper compares raw filesystem pattern fragments.
func matchesSensitivePattern(path string, patternParts []string) bool {
	offset := 0
	matched := false
	for _, part := range patternParts {
		if part == "" {
			continue
		}
		idx := strings.Index(path[offset:], part)
		if idx == -1 {
			return false
		}
		offset += idx + len(part)
		matched = true
	}
	return matched
}

//goplint:ignore -- parser helper validates raw host IP text from port syntax.
func isValidIPAddress(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, ":") {
		return regexp.MustCompile(`^[a-fA-F0-9:]+$`).MatchString(s)
	}

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
