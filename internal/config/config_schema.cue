// Invowk Configuration Schema
// This file defines the schema for the invowk configuration file.

package config

// Config is the root configuration structure
#Config: close({
	// container_engine specifies which container runtime to use
	// Valid values: "podman", "docker"
	container_engine?: "podman" | "docker"

	// search_paths contains additional directories to search for invkfiles
	search_paths?: [...string]

	// default_runtime sets the global default runtime mode
	// Valid values: "native", "virtual", "container"
	default_runtime?: "native" | "virtual" | "container"

	// virtual_shell configures the virtual shell behavior
	virtual_shell?: #VirtualShellConfig

	// ui configures the user interface
	ui?: #UIConfig

	// container configures container runtime behavior
	container?: #ContainerConfig

	// pack_aliases maps pack paths to alias names for collision disambiguation
	// When two packs have the same 'pack' identifier, use aliases to differentiate them
	pack_aliases?: [string]: string
})

// ContainerConfig configures container runtime behavior
#ContainerConfig: close({
	// auto_provision controls automatic provisioning of invowk resources
	// into containers. When enabled, invowk binary and packs are automatically
	// added to container images, enabling nested invowk commands.
	auto_provision?: #AutoProvisionConfig
})

// AutoProvisionConfig controls auto-provisioning of invowk resources
#AutoProvisionConfig: close({
	// enabled enables/disables auto-provisioning (default: true)
	enabled?: bool

	// binary_path overrides the path to the invowk binary to provision.
	// If not set, the currently running invowk binary is used.
	binary_path?: string

	// packs_paths specifies additional directories to search for packs.
	// These are added to the default pack search paths.
	packs_paths?: [...string]

	// cache_dir specifies where to store cached provisioned images metadata.
	// Default: ~/.cache/invowk/provision
	cache_dir?: string
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
