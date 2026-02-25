// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"context"

	"github.com/invowk/invowk/internal/container"
)

type (
	// Provisioner prepares container images with invowk resources.
	// Implementations should cache provisioned images based on content hashes
	// to enable fast reuse when resources haven't changed.
	Provisioner interface {
		// Provision adds invowk resources (binary, modules) to a base image.
		// Returns provisioned image tag and cleanup function.
		// The cleanup function removes temporary build resources (not the cached image).
		Provision(ctx context.Context, baseImage container.ImageTag) (*Result, error)
	}

	// Result contains the output of a provisioning operation.
	Result struct {
		// ImageTag is the tag of the provisioned image (e.g., "invowk-provisioned:abc123")
		ImageTag container.ImageTag

		// Cleanup is called to clean up temporary resources after the container exits.
		// This may remove temporary build contexts but typically does NOT remove
		// the cached image (for reuse). May be nil if no cleanup is needed.
		Cleanup func()

		// EnvVars are environment variables to set in the container.
		// These configure the invowk binary and module paths inside the container.
		EnvVars map[string]string
	}
)
