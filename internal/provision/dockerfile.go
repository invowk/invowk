// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"fmt"
	pathpkg "path"
	"strings"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provisionenv"
)

// generateDockerfile creates the Dockerfile content for the provisioned image.
//
//plint:render
func (p *LayerProvisioner) generateDockerfile(baseImage string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "FROM %s\n\n", baseImage)
	sb.WriteString("# Invowk auto-provisioned layer\n")
	sb.WriteString("# This layer adds invowk binary and modules to enable nested invowk commands\n\n")

	// Copy invowk binary
	if p.config.InvowkBinaryPath != "" {
		binaryPath := string(p.config.BinaryMountPath)
		sb.WriteString("# Install invowk binary\n")
		fmt.Fprintf(&sb, "COPY invowk %s/invowk\n", binaryPath)
		fmt.Fprintf(&sb, "RUN chmod +x %s/invowk\n\n", binaryPath)
	}

	// Copy modules
	modulesPath := string(p.config.ModulesMountPath)
	sb.WriteString("# Install modules\n")
	fmt.Fprintf(&sb, "COPY modules/ %s/\n\n", modulesPath)

	globalModulesPath := string(p.globalModulesMountPath())
	sb.WriteString("# Install global user command modules\n")
	fmt.Fprintf(&sb, "COPY global_modules/ %s/\n\n", globalModulesPath)

	// Set environment variables
	sb.WriteString("# Configure environment\n")
	if p.config.InvowkBinaryPath != "" {
		fmt.Fprintf(&sb, "ENV PATH=\"%s:$PATH\"\n", string(p.config.BinaryMountPath))
	}
	modulePathValue := provisionenv.Value(modulesPath)
	if err := modulePathValue.Validate(); err != nil {
		modulePathValue = ""
	}
	globalModulePathValue := provisionenv.Value(globalModulesPath)
	if err := globalModulePathValue.Validate(); err != nil {
		globalModulePathValue = ""
	}
	writeDockerfileEnv(&sb, provisionenv.ModulePathName, modulePathValue)
	writeDockerfileEnv(&sb, provisionenv.ModuleManifestName, p.moduleManifest(false))
	writeDockerfileEnv(&sb, provisionenv.GlobalModulePathName, globalModulePathValue)
	writeDockerfileEnv(&sb, provisionenv.GlobalModuleManifestName, p.moduleManifest(true))

	return sb.String()
}

// buildEnvVars returns environment variables to set in the container.
func (p *LayerProvisioner) buildEnvVars() map[string]string {
	env := make(map[string]string)

	// PATH is set in the Dockerfile, but we also set it here for consistency
	if p.config.InvowkBinaryPath != "" {
		env["PATH"] = string(p.config.BinaryMountPath) + ":/usr/local/bin:/usr/bin:/bin"
	}

	env[provisionenv.ModulePathName.String()] = string(p.config.ModulesMountPath)
	env[provisionenv.ModuleManifestName.String()] = p.moduleManifest(false).String()
	env[provisionenv.GlobalModulePathName.String()] = string(p.globalModulesMountPath())
	env[provisionenv.GlobalModuleManifestName.String()] = p.moduleManifest(true).String()

	return env
}

func (p *LayerProvisioner) globalModulesMountPath() container.MountTargetPath {
	return defaultGlobalModulesMountPath
}

func (p *LayerProvisioner) moduleManifest(global bool) provisionenv.Value {
	mountPath := p.config.ModulesMountPath
	paths := p.config.ModulesPaths
	entries := p.config.ModuleEntries
	if global {
		mountPath = p.globalModulesMountPath()
		paths = p.config.GlobalModulesPaths
		entries = p.config.GlobalModuleEntries
	}
	return moduleManifest(discoverProvisionedModuleCopies(paths, entries), mountPath)
}

func moduleManifest(copies []provisionedModuleCopy, mountPath container.MountTargetPath) provisionenv.Value {
	entries := make(provisionenv.Entries, 0, len(copies))
	for _, module := range copies {
		modulePath := container.MountTargetPath(pathpkg.Join(string(mountPath), string(module.DestinationPath)))
		if err := modulePath.Validate(); err != nil {
			continue
		}
		entries = append(entries, provisionenv.Entry{
			Path:             modulePath,
			CommandNamespace: module.CommandNamespace,
		})
	}
	value, err := provisionenv.MarshalManifest(entries)
	if err != nil {
		return "[]"
	}
	return value
}

func writeDockerfileEnv(sb *strings.Builder, key provisionenv.Name, value provisionenv.Value) {
	fmt.Fprintf(sb, "ENV %s=%q\n", key.String(), value.String())
}
