// Invowk Configuration Schema
// This file defines the schema for the invowk configuration file.

package config

import "strings"

// ContainerEngineType defines valid container engine types
#ContainerEngineType: "podman" | "docker"

// ConfigRuntimeType defines valid default runtime types
#ConfigRuntimeType: "native" | "virtual-sh" | "virtual-lua" | "container"

// ColorSchemeType defines valid color scheme types
#ColorSchemeType: "auto" | "dark" | "light"

// LLMProviderType defines valid LLM provider harnesses
#LLMProviderType: "auto" | "claude" | "codex" | "gemini" | "ollama"

// LLMTimeoutDurationString constrains a Go-style duration string for llm.timeout
// (e.g., "30s", "5m", "1h30m").
// [GO-ONLY] Positive-duration semantics are enforced by pkg/types because CUE
// only validates the syntactic shape here.
#LLMTimeoutDurationString: string & =~"^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$" & strings.MaxRunes(64)

// Config is the root effective configuration structure.
#Config: close({
	// container_engine specifies which container runtime to use
	// Valid values: "podman", "docker"
	container_engine: *"podman" | #ContainerEngineType

	// includes specifies modules to include in command discovery.
	// Each entry points to a *.invowkmod directory.
	// Modules may have an optional alias for collision disambiguation.
	includes: *([]) | [...#IncludeEntry]

	// default_runtime sets the global default runtime mode
	// Valid values: "native", "virtual-sh", "virtual-lua", "container"
	default_runtime: *"native" | #ConfigRuntimeType

	// virtual configures the virtual runtime family.
	virtual: *#VirtualConfig | #VirtualConfig

	// ui configures the user interface
	ui: *#UIConfig | #UIConfig

	// container configures container runtime behavior
	container: *#ContainerConfig | #ContainerConfig

	// llm configures common LLM defaults and the default backend for LLM-aware commands
	llm: *#LLMDefaultsConfig | #LLMConfig
})

// IncludeEntry specifies a module to include in command discovery.
// The path must be a non-empty filesystem path string ending with ".invowkmod";
// Go validation enforces OS-native absoluteness.
#IncludeEntry: close({
	// path is a filesystem path to a *.invowkmod directory.
	path: string & !="" & strings.MaxRunes(4096) & =~"\\.invowkmod$"

	// alias optionally overrides the module identifier for collision disambiguation.
	// Must be unique across all includes entries.
	alias?: string & =~"^[a-zA-Z][a-zA-Z0-9._-]*$" & strings.MaxRunes(256)
})

// ContainerConfig configures container runtime behavior
#ContainerConfig: close({
	// auto_provision controls automatic provisioning of invowk resources
	// into containers. When enabled, invowk binary and modules are automatically
	// added to container images, enabling nested invowk commands.
	auto_provision: *#AutoProvisionConfig | #AutoProvisionConfig
})

// AutoProvisionConfig controls auto-provisioning of invowk resources
#AutoProvisionConfig: close({
	// enabled enables/disables auto-provisioning (default: true)
	enabled: *true | bool

	// strict makes provisioning failure a hard error instead of falling back
	// to the unprovisioned base image. When false (default), provisioning
	// failure logs a warning and continues with the base image.
	strict: *false | bool

	// binary_path overrides the path to the invowk binary to provision.
	// If not set, the currently running invowk binary is used.
	binary_path: *"" | (string & !="" & strings.MaxRunes(4096))

	// includes specifies modules to provision into containers.
	// Uses the same IncludeEntry format as root-level includes.
	includes: *([]) | [...#IncludeEntry]

	// inherit_includes controls whether root-level includes are automatically
	// merged into container provisioning. Default: true.
	inherit_includes: *true | bool

	// cache_dir specifies where to place provision build contexts and cached image metadata.
	// Default: ~/.cache/invowk/provision
	cache_dir: *"" | (string & !="" & strings.MaxRunes(4096))
})

// VirtualConfig configures the virtual runtime family.
#VirtualConfig: close({
	// utilities configures virtual runtime utility helpers.
	utilities: *#VirtualUtilitiesConfig | #VirtualUtilitiesConfig
})

// VirtualUtilitiesConfig configures virtual runtime utility helpers.
#VirtualUtilitiesConfig: close({
	// enabled enables built-in utilities for virtual runtimes.
	enabled: *true | bool
})

// UIConfig configures the user interface
#UIConfig: close({
	// color_scheme sets the color scheme
	// Valid values: "auto", "dark", "light"
	color_scheme: *"auto" | #ColorSchemeType

	// verbose enables verbose output
	verbose: *false | bool

	// interactive enables alternate screen buffer mode for command execution
	interactive: *false | bool
})

// LLMConfig configures common LLM defaults and the default LLM backend.
// Use provider for supported local harnesses, or api for OpenAI-compatible endpoints.
#LLMConfig: #LLMDefaultsConfig | #LLMProviderConfig | #LLMAPIBackendConfig

#LLMCommonConfig: {
	// model sets the common default model. CLI harnesses use their current default when omitted.
	// For API backends, api.model overrides this value when both are set.
	model?: string & !="" & strings.MaxRunes(256)

	// timeout sets the per-request timeout as a Go duration string, e.g. "2m".
	timeout?: #LLMTimeoutDurationString

	// concurrency limits concurrent LLM requests. Zero means use the built-in default.
	concurrency?: int & >=0
}

#LLMDefaultsConfig: close({
	#LLMCommonConfig
})

#LLMProviderConfig: close({
	#LLMCommonConfig

	// provider selects a supported LLM harness/provider.
	provider: #LLMProviderType
})

#LLMAPIBackendConfig: close({
	#LLMCommonConfig

	// api configures an OpenAI-compatible endpoint. Do not store raw API keys here;
	// use api_key_env to name an environment variable that contains the secret.
	api: #LLMAPIConfig
})

// LLMAPIConfig configures an OpenAI-compatible LLM API endpoint.
#LLMAPIConfig: close({
	// base_url is the OpenAI-compatible API base URL.
	base_url?: string & !="" & strings.MaxRunes(2048)

	// model is the model name sent to the API endpoint.
	model?: string & !="" & strings.MaxRunes(256)

	// api_key_env names the environment variable that contains the API key.
	api_key_env?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)
})

// Validate that the configuration conforms to the schema
#Config
