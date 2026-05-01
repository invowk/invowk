// SPDX-License-Identifier: MPL-2.0

package container

import "github.com/invowk/invowk/pkg/containerargs"

func validateVolumeMountSpec(volume VolumeMountSpec) error {
	return containerargs.ContainerVolumeMountSpec(volume).Validate()
}

func validatePortMappingSpec(port PortMappingSpec) error {
	return containerargs.ContainerPortMappingSpec(port).Validate()
}
