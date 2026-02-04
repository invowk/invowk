// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"fmt"
	"strings"
)

// generateDockerfile creates the Dockerfile content for the provisioned image.
func (p *LayerProvisioner) generateDockerfile(baseImage string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "FROM %s\n\n", baseImage)
	sb.WriteString("# Invowk auto-provisioned layer\n")
	sb.WriteString("# This layer adds invowk binary and modules to enable nested invowk commands\n\n")

	// Copy invowk binary
	if p.config.InvowkBinaryPath != "" {
		binaryPath := p.config.BinaryMountPath
		sb.WriteString("# Install invowk binary\n")
		fmt.Fprintf(&sb, "COPY invowk %s/invowk\n", binaryPath)
		fmt.Fprintf(&sb, "RUN chmod +x %s/invowk\n\n", binaryPath)
	}

	// Copy modules
	modulesPath := p.config.ModulesMountPath
	sb.WriteString("# Install modules\n")
	fmt.Fprintf(&sb, "COPY modules/ %s/\n\n", modulesPath)

	// Set environment variables
	sb.WriteString("# Configure environment\n")
	if p.config.InvowkBinaryPath != "" {
		fmt.Fprintf(&sb, "ENV PATH=\"%s:$PATH\"\n", p.config.BinaryMountPath)
	}
	fmt.Fprintf(&sb, "ENV INVOWK_MODULE_PATH=\"%s\"\n", modulesPath)

	return sb.String()
}

// buildEnvVars returns environment variables to set in the container.
func (p *LayerProvisioner) buildEnvVars() map[string]string {
	env := make(map[string]string)

	// PATH is set in the Dockerfile, but we also set it here for consistency
	if p.config.InvowkBinaryPath != "" {
		env["PATH"] = p.config.BinaryMountPath + ":/usr/local/bin:/usr/bin:/bin"
	}

	env["INVOWK_MODULE_PATH"] = p.config.ModulesMountPath

	return env
}
