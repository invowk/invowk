// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// PortProtocolTCP is the TCP transport protocol for port mappings.
	PortProtocolTCP PortProtocol = "tcp"
	// PortProtocolUDP is the UDP transport protocol for port mappings.
	PortProtocolUDP PortProtocol = "udp"

	// SELinuxLabelNone means no SELinux label is applied to volume mounts.
	SELinuxLabelNone SELinuxLabel = ""
	// SELinuxLabelShared allows sharing the volume between containers.
	SELinuxLabelShared SELinuxLabel = "z"
	// SELinuxLabelPrivate restricts the volume to a single container.
	SELinuxLabelPrivate SELinuxLabel = "Z"

	// VolumeMountSELinuxAuto applies :z automatically when the engine enforces SELinux.
	VolumeMountSELinuxAuto VolumeMountSELinuxMode = ""
	// VolumeMountSELinuxShared always applies :z to the mount (unless disabled by operator config).
	VolumeMountSELinuxShared VolumeMountSELinuxMode = "shared"
	// VolumeMountSELinuxPrivate always applies :Z to the mount (unless disabled by operator config).
	VolumeMountSELinuxPrivate VolumeMountSELinuxMode = "private"
	// VolumeMountSELinuxNone omits automatic :z/:Z labels on the mount.
	VolumeMountSELinuxNone VolumeMountSELinuxMode = "none"
)

var (
	// ErrInvalidPortProtocol is the sentinel error wrapped by InvalidPortProtocolError.
	ErrInvalidPortProtocol = errors.New("invalid port protocol")

	// ErrInvalidSELinuxLabel is the sentinel error wrapped by InvalidSELinuxLabelError.
	ErrInvalidSELinuxLabel = errors.New("invalid SELinux label")
	// ErrInvalidVolumeMountSELinuxMode is returned when VolumeMountSELinuxMode is unrecognized.
	ErrInvalidVolumeMountSELinuxMode = errors.New("invalid volume mount SELinux mode")

	// ErrInvalidNetworkPort is the sentinel error wrapped by InvalidNetworkPortError.
	ErrInvalidNetworkPort = errors.New("invalid network port")

	// ErrInvalidHostFilesystemPath is the sentinel error wrapped by InvalidHostFilesystemPathError.
	ErrInvalidHostFilesystemPath = errors.New("invalid host filesystem path")

	// ErrInvalidMountTargetPath is the sentinel error wrapped by InvalidMountTargetPathError.
	ErrInvalidMountTargetPath = errors.New("invalid container filesystem path")

	// ErrInvalidVolumeMount is the sentinel error wrapped by InvalidVolumeMountError.
	ErrInvalidVolumeMount = errors.New("invalid volume mount")

	// ErrInvalidPortMapping is the sentinel error wrapped by InvalidPortMappingError.
	ErrInvalidPortMapping = errors.New("invalid port mapping")
)

type (
	// PortProtocol represents a network transport protocol for port mappings.
	// The zero value ("") is valid and means "default to tcp".
	PortProtocol string

	// InvalidPortProtocolError is returned when a PortProtocol is not a recognized protocol.
	InvalidPortProtocolError struct {
		Value PortProtocol
	}

	// SELinuxLabel represents an SELinux volume labeling option.
	// The zero value ("") means no SELinux label is applied.
	SELinuxLabel string

	// InvalidSELinuxLabelError is returned when an SELinuxLabel is not a recognized label.
	InvalidSELinuxLabelError struct {
		Value SELinuxLabel
	}

	// VolumeMountSELinuxMode controls per-mount SELinux labeling behavior.
	// The zero value ("") means automatic labeling when SELinux is active.
	VolumeMountSELinuxMode string

	// InvalidVolumeMountSELinuxModeError is returned when VolumeMountSELinuxMode is invalid.
	InvalidVolumeMountSELinuxModeError struct {
		Value VolumeMountSELinuxMode
	}

	// VolumeSELinuxModes maps individual VolumeMountSpec entries to per-mount SELinux modes.
	VolumeSELinuxModes map[VolumeMountSpec]VolumeMountSELinuxMode

	// NetworkPort represents a TCP/UDP port number for container port mappings.
	// A valid port must be greater than zero.
	NetworkPort uint16

	// InvalidNetworkPortError is returned when a NetworkPort value is zero.
	InvalidNetworkPortError struct {
		Value NetworkPort
	}

	// HostFilesystemPath represents a filesystem path on the host for volume mounts.
	// A valid path must be non-empty and not whitespace-only.
	HostFilesystemPath string

	// VolumeMountSpec is the raw volume mount argument accepted by Docker/Podman.
	VolumeMountSpec string

	// InvalidHostFilesystemPathError is returned when a HostFilesystemPath is empty or whitespace-only.
	InvalidHostFilesystemPathError struct {
		Value HostFilesystemPath
	}

	// MountTargetPath represents a filesystem path inside a container for volume mounts.
	// A valid path must be non-empty and not whitespace-only.
	//goplint:path-domain=container
	MountTargetPath string

	// PortMappingSpec is the raw port mapping argument accepted by Docker/Podman.
	PortMappingSpec string

	// InvalidMountTargetPathError is returned when a MountTargetPath is empty or whitespace-only.
	InvalidMountTargetPathError struct {
		Value MountTargetPath
	}

	//goplint:validate-all
	//
	// VolumeMount represents a volume mount specification.
	VolumeMount struct {
		HostPath      HostFilesystemPath
		ContainerPath MountTargetPath
		ReadOnly      bool
		SELinux       SELinuxLabel
	}

	//goplint:validate-all
	//
	// PortMapping represents a port mapping specification.
	PortMapping struct {
		HostPort      NetworkPort
		ContainerPort NetworkPort
		Protocol      PortProtocol
	}

	// InvalidVolumeMountError is returned when a VolumeMount has one or more invalid fields.
	// It wraps the individual field validation errors for inspection.
	InvalidVolumeMountError struct {
		Value     VolumeMount
		FieldErrs []error
	}

	// InvalidPortMappingError is returned when a PortMapping has one or more invalid fields.
	// It wraps the individual field validation errors for inspection.
	InvalidPortMappingError struct {
		Value     PortMapping
		FieldErrs []error
	}
)

// Error implements the error interface.
func (e *InvalidPortProtocolError) Error() string {
	return fmt.Sprintf("invalid port protocol %q (valid: tcp, udp)", e.Value)
}

// Unwrap returns ErrInvalidPortProtocol so callers can use errors.Is for programmatic detection.
func (e *InvalidPortProtocolError) Unwrap() error { return ErrInvalidPortProtocol }

// Validate returns an error if the PortProtocol is not one of the defined protocols.
// The zero value ("") is valid — it is treated as "tcp" by FormatPortMapping.
func (p PortProtocol) Validate() error {
	switch p {
	case PortProtocolTCP, PortProtocolUDP, "":
		return nil
	default:
		return &InvalidPortProtocolError{Value: p}
	}
}

// String returns the string representation of the PortProtocol.
func (p PortProtocol) String() string { return string(p) }

// Error implements the error interface.
func (e *InvalidSELinuxLabelError) Error() string {
	return fmt.Sprintf("invalid SELinux label %q (valid: empty, z, Z)", e.Value)
}

// Unwrap returns ErrInvalidSELinuxLabel so callers can use errors.Is for programmatic detection.
func (e *InvalidSELinuxLabelError) Unwrap() error { return ErrInvalidSELinuxLabel }

// Validate returns an error if the SELinuxLabel is not one of the defined labels.
// The zero value ("") is valid — it means no SELinux label.
func (s SELinuxLabel) Validate() error {
	switch s {
	case SELinuxLabelNone, SELinuxLabelShared, SELinuxLabelPrivate:
		return nil
	default:
		return &InvalidSELinuxLabelError{Value: s}
	}
}

// String returns the string representation of the SELinuxLabel.
func (s SELinuxLabel) String() string { return string(s) }

// Error implements the error interface for InvalidVolumeMountSELinuxModeError.
func (e *InvalidVolumeMountSELinuxModeError) Error() string {
	return fmt.Sprintf("invalid volume SELinux mode %q (valid: empty, shared, private, none)", e.Value)
}

// Unwrap returns ErrInvalidVolumeMountSELinuxMode for errors.Is() compatibility.
func (e *InvalidVolumeMountSELinuxModeError) Unwrap() error {
	return ErrInvalidVolumeMountSELinuxMode
}

// String returns the string representation of VolumeMountSELinuxMode.
func (m VolumeMountSELinuxMode) String() string { return string(m) }

// Validate returns nil if VolumeMountSELinuxMode is empty (auto) or a recognized mode.
func (m VolumeMountSELinuxMode) Validate() error {
	switch m {
	case VolumeMountSELinuxAuto, VolumeMountSELinuxShared, VolumeMountSELinuxPrivate, VolumeMountSELinuxNone:
		return nil
	default:
		return &InvalidVolumeMountSELinuxModeError{Value: m}
	}
}

// Validate returns nil if all volume SELinux mode overrides in the map are valid.
func (m VolumeSELinuxModes) Validate() error {
	var errs []error
	for spec, mode := range m {
		if err := spec.Validate(); err != nil {
			errs = append(errs, err)
		}
		if err := mode.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// String returns the string representation of the NetworkPort.
func (p NetworkPort) String() string { return fmt.Sprintf("%d", p) }

// Validate returns an error if the NetworkPort is invalid.
// A valid port must be greater than zero.
func (p NetworkPort) Validate() error {
	if p == 0 {
		return &InvalidNetworkPortError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidNetworkPortError.
func (e *InvalidNetworkPortError) Error() string {
	return fmt.Sprintf("invalid network port %d: must be greater than zero", e.Value)
}

// Unwrap returns ErrInvalidNetworkPort for errors.Is() compatibility.
func (e *InvalidNetworkPortError) Unwrap() error { return ErrInvalidNetworkPort }

// String returns the string representation of the HostFilesystemPath.
func (p HostFilesystemPath) String() string { return string(p) }

// Validate returns an error if the HostFilesystemPath is invalid.
// A valid path must be non-empty and not whitespace-only.
//
//goplint:nonzero
func (p HostFilesystemPath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidHostFilesystemPathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidHostFilesystemPathError.
func (e *InvalidHostFilesystemPathError) Error() string {
	return fmt.Sprintf("invalid host filesystem path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidHostFilesystemPath for errors.Is() compatibility.
func (e *InvalidHostFilesystemPathError) Unwrap() error { return ErrInvalidHostFilesystemPath }

// String returns the string representation of the VolumeMountSpec.
func (s VolumeMountSpec) String() string { return string(s) }

// Validate returns an error if the VolumeMountSpec is not a supported
// Docker/Podman volume argument.
//
//goplint:nonzero
func (s VolumeMountSpec) Validate() error {
	if err := validateVolumeMountSpec(s); err != nil {
		return &InvalidVolumeMountError{
			Value:     VolumeMount{HostPath: HostFilesystemPath(s)},
			FieldErrs: []error{err},
		}
	}
	return nil
}

// String returns the string representation of the MountTargetPath.
func (p MountTargetPath) String() string { return string(p) }

// Validate returns an error if the MountTargetPath is invalid.
// A valid path must be non-empty and not whitespace-only.
//
//goplint:nonzero
func (p MountTargetPath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidMountTargetPathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidMountTargetPathError.
func (e *InvalidMountTargetPathError) Error() string {
	return fmt.Sprintf("invalid container filesystem path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidMountTargetPath for errors.Is() compatibility.
func (e *InvalidMountTargetPathError) Unwrap() error {
	return ErrInvalidMountTargetPath
}

// String returns the string representation of the PortMappingSpec.
func (s PortMappingSpec) String() string { return string(s) }

// Validate returns an error if the PortMappingSpec is not a supported
// Docker/Podman port argument.
//
//goplint:nonzero
func (s PortMappingSpec) Validate() error {
	if err := validatePortMappingSpec(s); err != nil {
		return &InvalidPortMappingError{
			Value:     PortMapping{},
			FieldErrs: []error{err},
		}
	}
	return nil
}

// Error implements the error interface for InvalidVolumeMountError.
func (e *InvalidVolumeMountError) Error() string {
	return types.FormatFieldErrors(fmt.Sprintf("volume mount %s:%s", e.Value.HostPath, e.Value.ContainerPath), e.FieldErrs)
}

// Unwrap returns ErrInvalidVolumeMount for errors.Is() compatibility.
func (e *InvalidVolumeMountError) Unwrap() error { return ErrInvalidVolumeMount }

// Validate returns an error if any typed field of the VolumeMount is invalid.
// Validates HostPath, ContainerPath, and SELinux.
// ReadOnly is a bool and requires no validation.
func (v VolumeMount) Validate() error {
	var errs []error
	if err := v.HostPath.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := v.ContainerPath.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := v.SELinux.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// String returns the volume mount in "host:container[:selinux][:ro]" format.
func (v VolumeMount) String() string {
	s := filepath.ToSlash(string(v.HostPath)) + ":" + string(v.ContainerPath)
	if v.SELinux != "" {
		s += ":" + string(v.SELinux)
	}
	if v.ReadOnly {
		s += ":ro"
	}
	return s
}

// Error implements the error interface for InvalidPortMappingError.
func (e *InvalidPortMappingError) Error() string {
	return types.FormatFieldErrors(fmt.Sprintf("port mapping %d:%d/%s", e.Value.HostPort, e.Value.ContainerPort, e.Value.Protocol), e.FieldErrs)
}

// Unwrap returns ErrInvalidPortMapping for errors.Is() compatibility.
func (e *InvalidPortMappingError) Unwrap() error { return ErrInvalidPortMapping }

// Validate returns an error if any typed field of the PortMapping is invalid.
// Validates HostPort, ContainerPort, and Protocol.
func (p PortMapping) Validate() error {
	var errs []error
	if err := p.HostPort.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := p.ContainerPort.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := p.Protocol.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// String returns the port mapping in "host:container/protocol" format.
// Defaults to "tcp" when the protocol is empty.
func (p PortMapping) String() string {
	proto := p.Protocol
	if proto == "" {
		proto = PortProtocolTCP
	}
	return fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, proto)
}
