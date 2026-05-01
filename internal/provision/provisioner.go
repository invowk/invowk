// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"context"
	"errors"
	"io"

	"github.com/invowk/invowk/internal/container"
)

type (
	// WarningMessage is a user-facing provisioning warning.
	WarningMessage string

	// Warning describes a non-fatal provisioning issue.
	Warning struct {
		Message WarningMessage
	}

	// Provisioner prepares container images with invowk resources.
	// Implementations should cache provisioned images based on content hashes
	// to enable fast reuse when resources haven't changed.
	Provisioner interface {
		// Provision adds invowk resources (binary, modules) to a base image.
		// Returns provisioned image tag and cleanup function.
		// The cleanup function removes temporary build resources (not the cached image).
		Provision(ctx context.Context, req Request) (*Result, error)
	}

	// Request contains execution-scoped provisioning inputs.
	Request struct {
		// BaseImage is the image to layer invowk resources onto.
		BaseImage container.ImageTag
		// ForceRebuild bypasses the provisioned-image cache.
		ForceRebuild bool
		// Stdout receives build output.
		Stdout io.Writer
		// Stderr receives build errors/progress.
		Stderr io.Writer
	}

	//goplint:validate-all
	//
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

		// Warnings are non-fatal provisioning diagnostics for the runtime adapter to render.
		Warnings []Warning
	}
)

// Validate returns nil when request value fields are valid.
func (r Request) Validate() error {
	var errs []error
	if r.BaseImage != "" {
		if err := r.BaseImage.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
