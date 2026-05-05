// SPDX-License-Identifier: MPL-2.0

package container

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// --- Dockerfile Resolution ---

// ResolveDockerfilePath resolves a Dockerfile path relative to the build context.
// If the path is absolute, it is returned as-is.
// If the path is relative, it is resolved against the context path.
// Returns the resolved path or error if path traversal is detected.
func ResolveDockerfilePath(contextPath, dockerfilePath string) (string, error) {
	if dockerfilePath == "" {
		return "", nil
	}

	if filepath.IsAbs(dockerfilePath) {
		return dockerfilePath, nil
	}

	resolved := filepath.Join(contextPath, dockerfilePath)

	// Check for path traversal: the resolved path should be within the context
	resolvedClean := filepath.Clean(resolved)
	contextClean := filepath.Clean(contextPath)

	// Ensure resolved path starts with context path
	if !strings.HasPrefix(resolvedClean, contextClean) {
		return "", fmt.Errorf("dockerfile path %q escapes context directory %q", dockerfilePath, contextPath)
	}

	return resolved, nil
}

// --- Volume Mount Formatting ---

// FormatVolumeMount formats a volume mount as a string for -v flag.
//
//plint:render
func FormatVolumeMount(mount VolumeMount) string {
	var result strings.Builder
	result.WriteString(filepath.ToSlash(string(mount.HostPath)))
	result.WriteString(":")
	result.WriteString(string(mount.ContainerPath))

	var options []string
	if mount.ReadOnly {
		options = append(options, "ro")
	}
	if mount.SELinux != "" {
		options = append(options, string(mount.SELinux))
	}

	if len(options) > 0 {
		result.WriteString(":")
		result.WriteString(strings.Join(options, ","))
	}

	return result.String()
}

// ParseVolumeMount parses a volume string into a VolumeMount struct.
// Volume format: host_path:container_path[:options]
// Options can include: ro, rw, z, Z, and others.
// After parsing, the result is validated via VolumeMount.Validate().
func ParseVolumeMount(volume string) (VolumeMount, error) {
	mount := VolumeMount{}

	parts := strings.Split(volume, ":")

	if len(parts) >= 1 {
		mount.HostPath = HostFilesystemPath(parts[0]) //goplint:ignore -- validated by mount.Validate() below
	}
	if len(parts) >= 2 {
		mount.ContainerPath = MountTargetPath(parts[1]) //goplint:ignore -- validated by mount.Validate() below
	}
	if len(parts) >= 3 {
		options := parts[2]
		for opt := range strings.SplitSeq(options, ",") {
			switch opt {
			case "ro":
				mount.ReadOnly = true
			case "z", "Z":
				mount.SELinux = SELinuxLabel(opt) //goplint:ignore -- validated by mount.Validate() below
			}
		}
	}

	if err := mount.Validate(); err != nil {
		return mount, err
	}
	return mount, nil
}

// --- Port Mapping Formatting ---

// FormatPortMapping formats a port mapping as a string for -p flag.
//
//plint:render
func FormatPortMapping(mapping PortMapping) string {
	result := fmt.Sprintf("%d:%d", mapping.HostPort, mapping.ContainerPort)
	if mapping.Protocol != "" && mapping.Protocol != PortProtocolTCP {
		result += "/" + string(mapping.Protocol)
	}
	return result
}

// ParsePortMapping parses a port mapping string in "hostPort:containerPort[/protocol]" format
// into a PortMapping struct. After parsing, the result is validated via PortMapping.Validate().
func ParsePortMapping(portStr string) (PortMapping, error) {
	mapping := PortMapping{}

	parts := strings.SplitN(portStr, ":", 2)
	if len(parts) != 2 {
		return mapping, fmt.Errorf("invalid port mapping format %q: must contain ':' separator", portStr)
	}

	hostPort, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return mapping, fmt.Errorf("invalid host port %q: %w", parts[0], err)
	}
	mapping.HostPort = NetworkPort(hostPort) //goplint:ignore -- validated by mapping.Validate() below

	// Split container part on "/" to get port number and optional protocol
	containerParts := strings.SplitN(parts[1], "/", 2)
	containerPort, err := strconv.ParseUint(containerParts[0], 10, 16)
	if err != nil {
		return mapping, fmt.Errorf("invalid container port %q: %w", containerParts[0], err)
	}
	mapping.ContainerPort = NetworkPort(containerPort) //goplint:ignore -- validated by mapping.Validate() below

	if len(containerParts) == 2 {
		mapping.Protocol = PortProtocol(containerParts[1]) //goplint:ignore -- validated by mapping.Validate() below
	}

	if err := mapping.Validate(); err != nil {
		return mapping, err
	}
	return mapping, nil
}

// --- Engine Operation Error Helpers ---

// buildContainerError creates a typed error for container build failures.
func buildContainerError(engine string, opts BuildOptions, cause error) error {
	resource := ""
	switch {
	case opts.Dockerfile != "":
		resource = string(opts.Dockerfile)
	case opts.ContextDir != "":
		resource = string(opts.ContextDir) + "/Dockerfile"
	case opts.Tag != "":
		resource = string(opts.Tag)
	}
	return &OperationError{
		Engine:    engine,
		Operation: "build container image",
		Resource:  resource,
		Err:       cause,
	}
}

// runContainerError creates a typed error for container run failures.
func runContainerError(engine string, opts RunOptions, cause error) error {
	return &OperationError{
		Engine:    engine,
		Operation: "run container",
		Resource:  string(opts.Image),
		Err:       cause,
	}
}
