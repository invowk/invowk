// invowkfile_schema.cue - Schema definitions for invowkfile
// This file defines the structure, types, and constraints for invowkfiles.
// This schema is embedded in the invowk binary for validation.

// RuntimeMode defines the available execution runtimes
#RuntimeMode: "native" | "virtual" | "container"

// Platform defines the supported operating systems
#Platform: "linux" | "macos" | "windows"

// Script represents a script with platform and runtime constraints
#Script: {
	// script contains the shell commands to execute OR a path to a script file (required)
	// - Inline: shell commands (single or multi-line using triple quotes)
	// - File: path to script file (e.g., "./scripts/build.sh", "deploy.sh")
	// Recognized script extensions: .sh, .bash, .ps1, .bat, .cmd, .py, .rb, .pl, .zsh, .fish
	script: string & !=""

	// runtimes specifies which runtimes can execute this script (required, at least one)
	// The first element is the default runtime for this platform combination
	runtimes: [...#RuntimeMode] & [_, ...]

	// platforms specifies which operating systems this script is for (optional)
	// If not specified, the script applies to all platforms
	// Valid values: "linux", "macos", "windows"
	platforms?: [...#Platform] & [_, ...]

	// host_ssh enables SSH access from container back to host (optional, container runtime only)
	// When enabled, invowk starts an SSH server and provides connection credentials
	// to the container via environment variables: INVOWK_SSH_HOST, INVOWK_SSH_PORT,
	// INVOWK_SSH_USER, INVOWK_SSH_TOKEN
	// Default: false
	host_ssh?: bool
}

// ToolDependency represents a tool/binary that must be available in PATH
#ToolDependency: {
	// name is the binary name that must be in PATH (required)
	name: string & !=""

	// check_script is a custom script to validate the tool (optional)
	// If provided, this script is executed instead of just checking PATH
	// The script can verify version, configuration, or any other requirement
	check_script?: string

	// expected_code is the expected exit code from check_script (optional, default: 0)
	// Only used when check_script is provided
	expected_code?: int

	// expected_output is a regex pattern to match against check_script output (optional)
	// Only used when check_script is provided
	// Can be used together with expected_code
	expected_output?: string
}

// FilepathDependency represents a file or directory that must exist
#FilepathDependency: {
	// path is the file or directory path (required)
	// Can be absolute or relative to the invowkfile location
	path: string & !=""

	// readable checks if the path is readable (optional, default: false)
	readable?: bool

	// writable checks if the path is writable (optional, default: false)
	writable?: bool

	// executable checks if the path is executable (optional, default: false)
	executable?: bool
}

// CommandDependency represents another invowk command that must run first
#CommandDependency: {
	// name is the command name that must run before this one (required)
	name: string & !=""
}

// DependsOn defines the dependencies for a command
#DependsOn: {
	// tools lists binaries that must be available in PATH before running
	tools?: [...#ToolDependency]
	// commands lists invowk commands that must run before this one
	commands?: [...#CommandDependency]
	// filepaths lists files or directories that must exist before running
	filepaths?: [...#FilepathDependency]
}

// Command represents a single executable command
#Command: {
	// name is the command identifier (required)
	// Can include spaces for subcommand-like behavior (e.g., "test unit")
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_ -]*$"

	// description provides help text for the command (optional)
	description?: string

	// scripts defines the executable scripts with platform/runtime constraints (required, at least one)
	// Each script specifies which platforms and runtimes it supports
	// The first script for a given platform determines the default runtime for that platform
	// There cannot be duplicate combinations of platform+runtime across scripts
	scripts: [...#Script] & [_, ...]

	// env contains environment variables specific to this command (optional)
	env?: [string]: string

	// workdir specifies the working directory for command execution (optional)
	// Can be absolute or relative to the invowkfile location
	workdir?: string

	// depends_on specifies dependencies that must be satisfied before running (optional)
	depends_on?: #DependsOn
}

// ContainerConfig defines container-specific settings for container runtime
#ContainerConfig: {
	// dockerfile specifies the path to Dockerfile relative to invowkfile (optional)
	// Used to build a container image for command execution
	dockerfile?: string

	// image specifies a pre-built container image to use (optional)
	// If both dockerfile and image are specified, image takes precedence
	// Example: "alpine:latest", "ubuntu:22.04", "golang:1.21"
	image?: string

	// volumes specifies volume mounts in "host:container" format (optional)
	// Example: ["./data:/data", "/tmp:/tmp:ro"]
	volumes?: [...string]

	// ports specifies port mappings in "host:container" format (optional)
	// Example: ["8080:80", "3000:3000"]
	ports?: [...string]
}

// EnvMap defines environment variables as key-value string pairs
#EnvMap: [string]: string

// Invowkfile is the root schema for an invowkfile
#Invowkfile: {
	// version specifies the invowkfile schema version (optional but recommended)
	// Current version: "1.0"
	version?: string & =~"^[0-9]+\\.[0-9]+$"

	// description provides a summary of this invowkfile's purpose (optional)
	description?: string

	// default_runtime sets the default runtime for all commands (optional)
	// Defaults to "native" if not specified
	default_runtime?: #RuntimeMode

	// default_shell overrides the default shell for native runtime (optional)
	// Example: "/bin/bash", "pwsh"
	default_shell?: string

	// container holds container-specific configuration (optional)
	// Required if any command uses runtime: "container"
	container?: #ContainerConfig

	// env contains environment variables applied to all commands (optional)
	env?: #EnvMap

	// commands defines the available commands (required, at least one)
	commands: [...#Command] & [_, ...]
}

// Example usage with the cue command-line tool:
//   cue vet invowkfile.cue invowkfile_schema.cue -d '#Invowkfile'

