// SPDX-License-Identifier: MPL-2.0

package container

import "github.com/invowk/invowk/pkg/types"

func validateVolumeMountSpec(volume VolumeMountSpec) error {
	return types.ContainerVolumeMountSpec(volume).Validate()
}

func validatePortMappingSpec(port PortMappingSpec) error {
	return types.ContainerPortMappingSpec(port).Validate()
}
