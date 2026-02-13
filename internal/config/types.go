// SPDX-License-Identifier: MPL-2.0

package config

import "strings"

const (
	// ContainerEnginePodman uses Podman as the container runtime.
	ContainerEnginePodman ContainerEngine = "podman"
	// ContainerEngineDocker uses Docker as the container runtime.
	ContainerEngineDocker ContainerEngine = "docker"

	// moduleSuffix is the filesystem suffix for invkmod directories.
	// Defined locally to avoid coupling config to pkg/invkmod.
	moduleSuffix = ".invkmod"
)

type (
	// ContainerEngine specifies which container runtime to use.
	ContainerEngine string

	// IncludeEntry specifies a module to include in command discovery.
	// Each entry must point to a *.invkmod directory via an absolute filesystem path.
	IncludeEntry struct {
		// Path is the absolute filesystem path to a *.invkmod directory.
		Path string `json:"path" mapstructure:"path"`
		// Alias optionally overrides the module identifier for collision disambiguation.
		Alias string `json:"alias,omitempty" mapstructure:"alias"`
	}

	// Config holds the application configuration.
	Config struct {
		// ContainerEngine specifies whether to use "podman" or "docker"
		ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
		// Includes specifies modules to include in command discovery.
		Includes []IncludeEntry `json:"includes" mapstructure:"includes"`
		// DefaultRuntime sets the global default runtime mode
		DefaultRuntime string `json:"default_runtime" mapstructure:"default_runtime"`
		// VirtualShell configures the virtual shell behavior
		VirtualShell VirtualShellConfig `json:"virtual_shell" mapstructure:"virtual_shell"`
		// UI configures the user interface
		UI UIConfig `json:"ui" mapstructure:"ui"`
		// Container configures container runtime behavior
		Container ContainerConfig `json:"container" mapstructure:"container"`
	}

	// ContainerConfig configures container runtime behavior.
	ContainerConfig struct {
		// AutoProvision configures automatic provisioning of invowk resources
		AutoProvision AutoProvisionConfig `json:"auto_provision" mapstructure:"auto_provision"`
	}

	// AutoProvisionConfig controls auto-provisioning of invowk resources into containers.
	AutoProvisionConfig struct {
		// Enabled enables/disables auto-provisioning (default: true)
		Enabled bool `json:"enabled" mapstructure:"enabled"`
		// Strict makes provisioning failure a hard error instead of falling back
		// to the unprovisioned base image. When false (default), provisioning
		// failure logs a warning and continues with the base image.
		Strict bool `json:"strict" mapstructure:"strict"`
		// BinaryPath overrides the path to the invowk binary to provision
		BinaryPath string `json:"binary_path" mapstructure:"binary_path"`
		// Includes specifies modules to provision into containers.
		Includes []IncludeEntry `json:"includes" mapstructure:"includes"`
		// InheritIncludes controls whether root-level includes are automatically
		// merged into container provisioning (default: true).
		InheritIncludes bool `json:"inherit_includes" mapstructure:"inherit_includes"`
		// CacheDir specifies where to store cached provisioned images metadata
		CacheDir string `json:"cache_dir" mapstructure:"cache_dir"`
	}

	// VirtualShellConfig configures the virtual shell runtime.
	VirtualShellConfig struct {
		// EnableUrootUtils enables u-root utilities in virtual shell
		EnableUrootUtils bool `json:"enable_uroot_utils" mapstructure:"enable_uroot_utils"`
	}

	// UIConfig configures the user interface.
	UIConfig struct {
		// ColorScheme sets the color scheme ("auto", "dark", "light")
		ColorScheme string `json:"color_scheme" mapstructure:"color_scheme"`
		// Verbose enables verbose output
		Verbose bool `json:"verbose" mapstructure:"verbose"`
		// Interactive enables alternate screen buffer mode for command execution
		Interactive bool `json:"interactive" mapstructure:"interactive"`
	}
)

// IsModule reports whether this entry points to a module directory (.invkmod).
func (e IncludeEntry) IsModule() bool {
	return strings.HasSuffix(e.Path, moduleSuffix)
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ContainerEngine: ContainerEnginePodman,
		Includes:        []IncludeEntry{},
		DefaultRuntime:  "native",
		VirtualShell: VirtualShellConfig{
			EnableUrootUtils: true,
		},
		UI: UIConfig{
			ColorScheme: "auto",
			Verbose:     false,
			Interactive: false,
		},
		Container: ContainerConfig{
			AutoProvision: AutoProvisionConfig{
				Enabled:         true,
				Strict:          false,
				BinaryPath:      "", // Will use os.Executable() if empty
				Includes:        []IncludeEntry{},
				InheritIncludes: true,
				CacheDir:        "", // Will use default cache dir if empty
			},
		},
	}
}
