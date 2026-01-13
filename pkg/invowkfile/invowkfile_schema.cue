// invowkfile_schema.cue - Schema definitions for invowkfile
// This file defines the structure, types, and constraints for invowkfiles.
// This schema is embedded in the invowk binary for validation.

// RuntimeType defines the available execution runtime types
#RuntimeType: "native" | "virtual" | "container"

// PlatformType defines the supported operating system types
#PlatformType: "linux" | "macos" | "windows"

// EnvMap defines environment variables as key-value string pairs
#EnvMap: [string]: string

// RuntimeConfig represents a runtime configuration with type-specific options
#RuntimeConfig: {
	// name specifies the runtime type (required)
	name: #RuntimeType

	// Container-specific fields (only valid when name is "container")
	if name == "container" {
		// enable_host_ssh enables SSH access from container back to host (optional)
		// When enabled, invowk starts an SSH server and provides connection credentials
		// to the container via environment variables: INVOWK_SSH_HOST, INVOWK_SSH_PORT,
		// INVOWK_SSH_USER, INVOWK_SSH_TOKEN
		// Default: false
		enable_host_ssh?: bool

		// containerfile specifies the path to Containerfile/Dockerfile relative to invowkfile (optional)
		// Used to build a container image for command execution
		// Mutually exclusive with 'image'
		containerfile?: string

		// image specifies a pre-built container image to use (optional)
		// Mutually exclusive with 'containerfile'
		// Example: "alpine:latest", "ubuntu:22.04", "golang:1.21"
		image?: string

		// volumes specifies volume mounts in "host:container" format (optional)
		// Example: ["./data:/data", "/tmp:/tmp:ro"]
		volumes?: [...string]

		// ports specifies port mappings in "host:container" format (optional)
		// Example: ["8080:80", "3000:3000"]
		ports?: [...string]
	}
}

// PlatformConfig represents a platform configuration with optional environment variables
#PlatformConfig: {
	// name specifies the platform type (required)
	name: #PlatformType

	// env contains environment variables specific to this platform (optional)
	env?: #EnvMap
}

// Target defines the runtime and platform constraints for an implementation
#Target: {
	// runtimes specifies which runtimes can execute this implementation (required, at least one)
	// The first element is the default runtime for this platform combination
	// Each runtime is a struct with a 'name' field and optional type-specific fields
	runtimes: [...#RuntimeConfig] & [_, ...]

	// platforms specifies which operating systems this implementation is for (optional)
	// If not specified, the implementation applies to all platforms
	// Each platform is a struct with a 'name' field
	platforms?: [...#PlatformConfig] & [_, ...]
}

// Implementation represents an implementation with platform and runtime constraints
#Implementation: {
	// script contains the shell commands to execute OR a path to a script file (required)
	// - Inline: shell commands (single or multi-line using triple quotes)
	// - File: path to script file (e.g., "./scripts/build.sh", "deploy.sh")
	// Recognized script extensions: .sh, .bash, .ps1, .bat, .cmd, .py, .rb, .pl, .zsh, .fish
	script: string & !=""

	// target defines the runtime and platform constraints (required)
	target: #Target

	// depends_on specifies dependencies that must be satisfied before running this implementation (optional)
	// These dependencies are validated according to the runtime:
	// - native: validated against the native standard shell from the host
	// - virtual: validated against invowk's built-in sh interpreter with core utils
	// - container: validated against the container's default shell from within the container
	// Implementation-level depends_on is combined with command-level depends_on
	depends_on?: #DependsOn
}

// ToolDependency represents a tool/binary that must be available in PATH
#ToolDependency: {
	// alternatives is a list of binary names where any match satisfies the dependency (required, at least one)
	// If any of the provided tools is found in PATH, the validation succeeds (early return).
	// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
	alternatives: [...string & !=""] & [_, ...]
}

// CustomCheck represents a custom validation script to verify system requirements
#CustomCheck: {
	// name is an identifier for this check (required)
	// Used for error reporting and identification
	name: string & !=""

	// check_script is the script to execute for validation (required)
	// The script is executed using the runtime's shell
	check_script: string & !=""

	// expected_code is the expected exit code from check_script (optional, default: 0)
	expected_code?: int

	// expected_output is a regex pattern to match against check_script output (optional)
	// Can be used together with expected_code
	expected_output?: string
}

// CustomCheckDependency represents a custom check dependency that can be either:
// - A single CustomCheck (direct check)
// - An alternatives list of CustomChecks (OR semantics with early return)
#CustomCheckDependency: #CustomCheck | #CustomCheckAlternatives

// CustomCheckAlternatives represents a list of alternative custom checks (OR semantics)
#CustomCheckAlternatives: {
	// alternatives is a list of custom checks where any passing check satisfies the dependency (required, at least one)
	// If any of the provided checks passes, the validation succeeds (early return).
	alternatives: [...#CustomCheck] & [_, ...]
}

// FilepathDependency represents a file or directory that must exist
#FilepathDependency: {
	// alternatives is a list of file or directory paths (required, at least one)
	// If any of the provided paths exists and satisfies the permission requirements,
	// the validation succeeds (early return). This allows specifying multiple
	// possible locations for a file (e.g., different paths on different systems).
	// Paths can be absolute or relative to the invowkfile location.
	alternatives: [...string & !=""] & [_, ...]

	// readable checks if the path is readable (optional, default: false)
	readable?: bool

	// writable checks if the path is writable (optional, default: false)
	writable?: bool

	// executable checks if the path is executable (optional, default: false)
	executable?: bool
}

// CommandDependency represents another invowk command that must run first
#CommandDependency: {
	// alternatives is a list of command names where any match satisfies the dependency (required, at least one)
	// If any of the provided commands has already run successfully, the validation succeeds (early return).
	// This allows specifying alternative commands (e.g., ["build-debug", "build-release"]).
	alternatives: [...string & !=""] & [_, ...]
}

// CapabilityName defines the supported system capability types
#CapabilityName: "local-area-network" | "internet"

// CapabilityDependency represents a system capability that must be available
#CapabilityDependency: {
	// alternatives is a list of capability identifiers where any match satisfies the dependency (required, at least one)
	// If any of the provided capabilities is available, the validation succeeds (early return).
	// Available capabilities:
	//   - "local-area-network": checks for Local Area Network presence
	//   - "internet": checks for working Internet connectivity
	alternatives: [...#CapabilityName] & [_, ...]
}

// DependsOn defines the dependencies for a command
#DependsOn: {
	// tools lists binaries that must be available in PATH before running
	// Each tool is checked for existence in PATH using 'command -v' or equivalent
	// Uses OR semantics: if any alternative in the list is found, the dependency is satisfied
	tools?: [...#ToolDependency]
	// commands lists invowk commands that must run before this one
	// Uses OR semantics: if any alternative in the list has run, the dependency is satisfied
	commands?: [...#CommandDependency]
	// filepaths lists files or directories that must exist before running
	// Uses OR semantics: if any alternative path exists, the dependency is satisfied
	filepaths?: [...#FilepathDependency]
	// capabilities lists system capabilities that must be available before running
	// Uses OR semantics: if any alternative capability is available, the dependency is satisfied
	capabilities?: [...#CapabilityDependency]
	// custom_checks lists custom validation scripts to verify system requirements
	// Use this for version checks, configuration validation, or any other custom requirement
	// Each entry can be a single check or an alternatives list (OR semantics)
	custom_checks?: [...#CustomCheckDependency]
}

// Command represents a single executable command
#Command: {
	// name is the command identifier (required)
	// Can include spaces for subcommand-like behavior (e.g., "test unit")
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_ -]*$"

	// description provides help text for the command (optional)
	description?: string

	// implementations defines the executable implementations with platform/runtime constraints (required, at least one)
	// Each implementation specifies which platforms and runtimes it supports
	// The first implementation for a given platform determines the default runtime for that platform
	// There cannot be duplicate combinations of platform+runtime across implementations
	implementations: [...#Implementation] & [_, ...]

	// env contains environment variables specific to this command (optional)
	env?: [string]: string

	// workdir specifies the working directory for command execution (optional)
	// Can be absolute or relative to the invowkfile location
	workdir?: string

	// depends_on specifies dependencies that must be satisfied before running (optional)
	depends_on?: #DependsOn
}

// Invowkfile is the root schema for an invowkfile
#Invowkfile: {
	// group is a mandatory prefix for all command names from this invowkfile
	// Must start with a letter, contain only alphanumeric characters, with optional
	// dot-separated segments (e.g., "mygroup", "my.group", "my.nested.group")
	// Cannot start or end with a dot, and cannot have consecutive dots
	group: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$"

	// version specifies the invowkfile schema version (optional but recommended)
	// Current version: "1.0"
	version?: string & =~"^[0-9]+\\.[0-9]+$"

	// description provides a summary of this invowkfile's purpose (optional)
	description?: string

	// default_shell overrides the default shell for native runtime (optional)
	// Example: "/bin/bash", "pwsh"
	default_shell?: string

	// commands defines the available commands (required, at least one)
	commands: [...#Command] & [_, ...]
}

// Example usage with the cue command-line tool:
//   cue vet invowkfile.cue invowkfile_schema.cue -d '#Invowkfile'

