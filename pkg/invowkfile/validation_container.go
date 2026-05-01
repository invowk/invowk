// SPDX-License-Identifier: MPL-2.0

package invowkfile

import "github.com/invowk/invowk/pkg/types"

// ValidateVolumeMount validates a container volume mount specification.
// Valid formats:
//   - /host/path:/container/path
//   - /host/path:/container/path:ro
//   - /host/path:/container/path:rw
//   - relative/path:/container/path
//   - named-volume:/container/path
func ValidateVolumeMount(volume string) error {
	return types.ContainerVolumeMountSpec(volume).Validate()
}

// ValidatePortMapping validates a container port mapping specification.
// Valid formats:
//   - containerPort
//   - hostPort:containerPort
//   - hostIP:hostPort:containerPort
//   - hostPort:containerPort/protocol
func ValidatePortMapping(port string) error {
	return types.ContainerPortMappingSpec(port).Validate()
}
