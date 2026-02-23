// SPDX-License-Identifier: MPL-2.0

// Package provision handles container image provisioning for invowk.
//
// This package provides functionality to create ephemeral container image layers
// that include invowk resources (binary, modules, etc.) on top of a base image.
// The provisioned images are cached based on content hashes for fast reuse.
//
// The main entry point is the Provisioner interface, implemented by LayerProvisioner:
//
//	provisioner := provision.NewLayerProvisioner(engine, cfg)
//	result, err := provisioner.Provision(ctx, container.ImageTag("debian:stable-slim"))
//	// result.ImageTag contains the provisioned image to use
//
// Provisioned images enable nested invowk commands inside containers by bundling
// the invowk binary and user modules into the container's filesystem.
package provision
