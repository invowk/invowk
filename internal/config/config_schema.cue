// Invowk Configuration Schema
// This file defines the schema for the invowk configuration file.

package config

// Config is the root configuration structure
#Config: {
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
}

// VirtualShellConfig configures the virtual shell runtime
#VirtualShellConfig: {
	// enable_uroot_utils enables u-root utilities in virtual shell
	enable_uroot_utils?: bool
}

// UIConfig configures the user interface
#UIConfig: {
	// color_scheme sets the color scheme
	// Valid values: "auto", "dark", "light"
	color_scheme?: "auto" | "dark" | "light"

	// verbose enables verbose output
	verbose?: bool
}

// Validate that the configuration conforms to the schema
#Config
