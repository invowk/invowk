// Invowk Configuration Schema
// This file defines the schema for the invowk configuration file.

package config

import "strings"

// Config is the root configuration structure
#Config: close({
	// container_engine specifies which container runtime to use
	// Valid values: "podman", "docker"
	container_engine?: "podman" | "docker"

	// includes specifies modules to include in command discovery.
	// Each entry points to a *.invkmod directory.
	// Modules may have an optional alias for collision disambiguation.
	includes?: [...#IncludeEntry]

	// default_runtime sets the global default runtime mode
	// Valid values: "native", "virtual", "container"
	default_runtime?: "native" | "virtual" | "container"

	// virtual_shell configures the virtual shell behavior
	virtual_shell?: #VirtualShellConfig

	// ui configures the user interface
	ui?: #UIConfig

	// container configures container runtime behavior
	container?: #ContainerConfig
})

// IncludeEntry specifies a module to include in command discovery.
// The path must end with ".invkmod".
#IncludeEntry: close({
	// path is the absolute filesystem path to a *.invkmod directory.
	path: string & !="" & strings.MaxRunes(4096) & =~"\\.invkmod$"

	// alias optionally overrides the module identifier for collision disambiguation.
	// Must be unique across all includes entries.
	alias?: string & !="" & strings.MaxRunes(256)
})

// ContainerConfig configures container runtime behavior
#ContainerConfig: close({
	// auto_provision controls automatic provisioning of invowk resources
	// into containers. When enabled, invowk binary and modules are automatically
	// added to container images, enabling nested invowk commands.
	auto_provision?: #AutoProvisionConfig
})

// AutoProvisionConfig controls auto-provisioning of invowk resources
#AutoProvisionConfig: close({
	// enabled enables/disables auto-provisioning (default: true)
	enabled?: bool

	// binary_path overrides the path to the invowk binary to provision.
	// If not set, the currently running invowk binary is used.
	binary_path?: string & !="" & strings.MaxRunes(4096)

	// includes specifies modules to provision into containers.
	// Uses the same IncludeEntry format as root-level includes.
	includes?: [...#IncludeEntry]

	// inherit_includes controls whether root-level includes are automatically
	// merged into container provisioning. Default: true.
	inherit_includes?: bool

	// cache_dir specifies where to store cached provisioned images metadata.
	// Default: ~/.cache/invowk/provision
	cache_dir?: string & !="" & strings.MaxRunes(4096)
})

// VirtualShellConfig configures the virtual shell runtime
#VirtualShellConfig: close({
	// enable_uroot_utils enables u-root utilities in virtual shell
	enable_uroot_utils?: bool
})

// UIConfig configures the user interface
#UIConfig: close({
	// color_scheme sets the color scheme
	// Valid values: "auto", "dark", "light"
	color_scheme?: "auto" | "dark" | "light"

	// verbose enables verbose output
	verbose?: bool

	// interactive enables alternate screen buffer mode for command execution
	interactive?: bool
})

// Validate that the configuration conforms to the schema
#Config
