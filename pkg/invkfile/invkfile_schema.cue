// invkfile_schema.cue - Schema definitions for invkfile
// This file defines the structure, types, and constraints for invkfiles.
// This schema is embedded in the invowk binary for validation.

// RuntimeType defines the available execution runtime types
#RuntimeType: "native" | "virtual" | "container"

// PlatformType defines the supported operating system types
#PlatformType: "linux" | "macos" | "windows"

// EnvMap defines environment variables as key-value string pairs
#EnvMap: [string]: string

// EnvConfig defines environment configuration for a command or implementation
#EnvConfig: close({
	// files lists dotenv files to load (optional)
	// Files are loaded in order; later files override earlier ones.
	// Paths are relative to the invkfile location (or pack root for packs).
	// Files suffixed with '?' are optional and will not cause an error if missing.
	files?: [...string]

	// vars contains environment variables as key-value pairs (optional)
	// These override values loaded from files.
	vars?: [string]: string
})

// RuntimeConfig represents a runtime configuration with type-specific options
#RuntimeConfig: #RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer

#RuntimeConfigBase: {
	// name specifies the runtime type (required)
	name: #RuntimeType

	// env_inherit_mode controls host environment inheritance (optional)
	// Allowed values: "none", "allow", "all"
	env_inherit_mode?: "none" | "allow" | "all"

	// env_inherit_allow lists host env vars to allow when env_inherit_mode is "allow"
	env_inherit_allow?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]

	// env_inherit_deny lists host env vars to block (applies to any mode)
	env_inherit_deny?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]
}

#RuntimeConfigNative: close({
	#RuntimeConfigBase
	name: "native"

	// interpreter specifies how to execute the script (optional)
	// - Omit: defaults to "auto" (detect from shebang)
	// - "auto": detect interpreter from shebang (#!) in first line of script
	// - Specific value: use as interpreter (e.g., "python3", "node", "/usr/bin/ruby")
	// - Can include arguments: "python3 -u", "/usr/bin/env perl -w"
	// If "auto" and no shebang is found, falls back to default shell behavior
	// Note: When declared, interpreter must be non-empty (cannot be "" or whitespace-only)
	interpreter?: string & =~"^\\s*\\S.*$"
})

#RuntimeConfigVirtual: close({
	#RuntimeConfigBase
	name: "virtual"
})

#RuntimeConfigContainer: close({
	#RuntimeConfigBase
	name: "container"

	// interpreter specifies how to execute the script (optional)
	// - Omit: defaults to "auto" (detect from shebang)
	// - "auto": detect interpreter from shebang (#!) in first line of script
	// - Specific value: use as interpreter (e.g., "python3", "node", "/usr/bin/ruby")
	// - Can include arguments: "python3 -u", "/usr/bin/env perl -w"
	// If "auto" and no shebang is found, falls back to /bin/sh
	// Note: The interpreter must exist inside the container
	// Note: When declared, interpreter must be non-empty (cannot be "" or whitespace-only)
	interpreter?: string & =~"^\\s*\\S.*$"

	// enable_host_ssh enables SSH access from container back to host (optional)
	// When enabled, invowk starts an SSH server and provides connection credentials
	// to the container via environment variables: INVOWK_SSH_HOST, INVOWK_SSH_PORT,
	// INVOWK_SSH_USER, INVOWK_SSH_TOKEN, INVOWK_SSH_ENABLED
	// Default: false
	enable_host_ssh?: bool

	// containerfile specifies the path to Containerfile/Dockerfile relative to invkfile (optional)
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
})

// PlatformConfig represents a platform configuration
#PlatformConfig: close({
	// name specifies the platform type (required)
	name: #PlatformType
})

// Implementation represents an implementation with platform and runtime constraints
#Implementation: close({
	// script contains the shell commands to execute OR a path to a script file (required)
	// - Inline: shell commands (single or multi-line using triple quotes)
	// - File: path to script file (e.g., "./scripts/build.sh", "deploy.sh")
	// Recognized script extensions: .sh, .bash, .ps1, .bat, .cmd, .py, .rb, .pl, .zsh, .fish
	script: string & !=""

	// runtimes specifies which runtimes can execute this implementation (required, at least one)
	// The first element is the default runtime for this platform combination
	// Each runtime is a struct with a 'name' field and optional type-specific fields
	runtimes: [...#RuntimeConfig] & [_, ...]

	// platforms specifies which operating systems this implementation is for (optional)
	// If not specified, the implementation applies to all platforms
	// Each platform is a struct with a 'name' field
	platforms?: [...#PlatformConfig] & [_, ...]

	// env contains environment configuration for this implementation (optional)
	// Implementation-level env is merged with command-level env.
	// Implementation files are loaded after command-level files.
	// Implementation vars override command-level vars.
	env?: #EnvConfig

	// workdir specifies the working directory for this implementation (optional)
	// Overrides both root-level and command-level workdir settings.
	// Can be absolute or relative to the invkfile location.
	// Paths should use forward slashes for cross-platform compatibility.
	workdir?: string

	// depends_on specifies dependencies that must be satisfied before running this implementation (optional)
	// These dependencies are validated according to the runtime:
	// - native: validated against the native standard shell from the host
	// - virtual: validated against invowk's built-in sh interpreter with core utils
	// - container: validated against the container's default shell from within the container
	// Implementation-level depends_on is combined with command-level depends_on
	depends_on?: #DependsOn
})

// ToolDependency represents a tool/binary that must be available in PATH
#ToolDependency: close({
	// alternatives is a list of binary names where any match satisfies the dependency (required, at least one)
	// If any of the provided tools is found in PATH, the validation succeeds (early return).
	// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
	alternatives: [...string & !=""] & [_, ...]
})

// CustomCheck represents a custom validation script to verify system requirements
#CustomCheck: close({
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
})

// CustomCheckDependency represents a custom check dependency that can be either:
// - A single CustomCheck (direct check)
// - An alternatives list of CustomChecks (OR semantics with early return)
#CustomCheckDependency: #CustomCheck | #CustomCheckAlternatives

// CustomCheckAlternatives represents a list of alternative custom checks (OR semantics)
#CustomCheckAlternatives: close({
	// alternatives is a list of custom checks where any passing check satisfies the dependency (required, at least one)
	// If any of the provided checks passes, the validation succeeds (early return).
	alternatives: [...#CustomCheck] & [_, ...]
})

// FilepathDependency represents a file or directory that must exist
#FilepathDependency: close({
	// alternatives is a list of file or directory paths (required, at least one)
	// If any of the provided paths exists and satisfies the permission requirements,
	// the validation succeeds (early return). This allows specifying multiple
	// possible locations for a file (e.g., different paths on different systems).
	// Paths can be absolute or relative to the invkfile location.
	alternatives: [...string & !=""] & [_, ...]

	// readable checks if the path is readable (optional, default: false)
	readable?: bool

	// writable checks if the path is writable (optional, default: false)
	writable?: bool

	// executable checks if the path is executable (optional, default: false)
	executable?: bool
})

// CommandDependency represents another invowk command that must be discoverable
#CommandDependency: close({
	// alternatives is a list of command names where any match satisfies the dependency (required, at least one)
	// If any of the provided commands is discoverable, the dependency is satisfied (early return).
	// This allows specifying alternative commands (e.g., ["build-debug", "build-release"]).
	alternatives: [...string & !=""] & [_, ...]
})

// CapabilityName defines the supported system capability types
#CapabilityName: "local-area-network" | "internet" | "containers" | "tty"

// CapabilityDependency represents a system capability that must be available
#CapabilityDependency: close({
	// alternatives is a list of capability identifiers where any match satisfies the dependency (required, at least one)
	// If any of the provided capabilities is available, the validation succeeds (early return).
	// Available capabilities:
	//   - "local-area-network": checks for Local Area Network presence
	//   - "internet": checks for working Internet connectivity
	//   - "containers": checks for Docker or Podman availability
	//   - "tty": checks for interactive TTY availability
	alternatives: [...#CapabilityName] & [_, ...]
})

// EnvVarCheck represents a single environment variable check
#EnvVarCheck: close({
	// name is the environment variable name to check (required, non-empty after trimming)
	// The check verifies that this env var exists in the user's environment
	name: string & =~"^\\s*\\S+\\s*$"

	// validation is a regex pattern to validate the env var value (optional)
	// If specified, the env var must exist AND its value must match this pattern
	validation?: string
})

// EnvVarDependency represents an environment variable that must exist
#EnvVarDependency: close({
	// alternatives is a list of env var checks where any match satisfies the dependency (required, at least one)
	// If any of the provided env vars exists (and passes validation if specified), the dependency is satisfied
	// This allows specifying multiple possible env vars (e.g., ["AWS_ACCESS_KEY_ID", "AWS_PROFILE"])
	alternatives: [...#EnvVarCheck] & [_, ...]
})

// DependsOn defines the dependencies for a command
#DependsOn: close({
	// tools lists binaries that must be available in PATH before running
	// Each tool is checked for existence in PATH using 'command -v' or equivalent
	// Uses OR semantics: if any alternative in the list is found, the dependency is satisfied
	tools?: [...#ToolDependency]
	// cmds lists invowk commands that must be discoverable for this command to run
	// Uses OR semantics: if any alternative in the list is discoverable, the dependency is satisfied
	cmds?: [...#CommandDependency]

	// commands is not supported (use cmds)
	commands?: _|_
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
	// env_vars lists environment variables that must exist before running
	// Uses OR semantics: if any alternative env var exists (and passes validation), the dependency is satisfied
	// IMPORTANT: Validated against the user's environment BEFORE invowk sets command-level env vars
	env_vars?: [...#EnvVarDependency]
})

// Argument represents a positional command-line argument for a command
#Argument: close({
	// name is the argument identifier (required, POSIX-compliant)
	// Used for documentation, environment variable naming (INVOWK_ARG_<NAME>), and error messages
	// Must start with a letter, contain only alphanumeric characters, hyphens, and underscores
	// Examples: "file", "output-dir", "source_path"
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$" & !=""

	// description provides help text for the argument (required)
	description: string & =~"^\\s*\\S.*$"

	// required indicates whether this argument must be provided (optional, defaults to false)
	// If true, the command will fail if the argument is not provided
	// An argument cannot be both required and have a default_value
	// Required arguments must come before optional arguments in the args list
	required?: bool

	// default_value is the default value if the argument is not provided (optional)
	// Cannot be specified together with required: true
	// Must be compatible with the specified type (if type is specified)
	default_value?: string

	// type specifies the data type of the argument (optional, defaults to "string")
	// Supported types: "string", "int", "float"
	// - "string": any string value (default)
	// - "int": must be a valid integer
	// - "float": must be a valid floating-point number
	// Note: "bool" is not supported for positional arguments (use flags instead)
	type?: "string" | "int" | "float"

	// validation is a regex pattern to validate the argument value (optional)
	// The argument value must match this pattern
	// If default_value is specified, it must also match this pattern
	validation?: string

	// variadic indicates this argument accepts multiple values (optional, defaults to false)
	// Only the last argument in the args list can be variadic
	// Variadic arguments are passed as space-separated values in INVOWK_ARG_<NAME>
	// Individual values are also available as INVOWK_ARG_<NAME>_1, INVOWK_ARG_<NAME>_2, etc.
	// The count is available as INVOWK_ARG_<NAME>_COUNT
	variadic?: bool
})

// Flag represents a command-line flag for a command
#Flag: close({
	// name is the flag name (required, POSIX-compliant)
	// Must start with a letter, contain only alphanumeric characters, hyphens, and underscores
	// Examples: "verbose", "output-file", "num_retries"
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$" & !=""

	// description provides help text for the flag (required)
	description: string & =~"^\\s*\\S.*$"

	// default_value is the default value for the flag (optional)
	// If not specified, the flag has no default value
	// Must be compatible with the specified type (if type is specified)
	default_value?: string

	// type specifies the data type of the flag (optional, defaults to "string")
	// Supported types: "string", "bool", "int", "float"
	// - "string": any string value (default)
	// - "bool": must be "true" or "false"
	// - "int": must be a valid integer
	// - "float": must be a valid floating-point number
	type?: "string" | "bool" | "int" | "float"

	// required indicates whether this flag must be provided (optional, defaults to false)
	// If true, the command will fail if the flag is not provided
	// A flag cannot be both required and have a default_value
	required?: bool

	// short is a single-character alias for the flag (optional)
	// Must be a single letter (a-z or A-Z)
	// Example: short: "v" allows using -v instead of --verbose
	short?: string & =~"^[a-zA-Z]$"

	// validation is a regex pattern to validate the flag value (optional)
	// The flag value must match this pattern
	// If default_value is specified, it must also match this pattern
	validation?: string
})

// Command represents a single executable command
#Command: close({
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

	// env contains environment configuration for this command (optional)
	// Environment from files is loaded first, then vars override.
	// Command-level env is applied before implementation-level env.
	env?: #EnvConfig

	// workdir specifies the working directory for command execution (optional)
	// Overrides root-level workdir but can be overridden by implementation-level workdir.
	// Can be absolute or relative to the invkfile location.
	// Paths should use forward slashes for cross-platform compatibility.
	workdir?: string

	// depends_on specifies dependencies that must be satisfied before running (optional)
	depends_on?: #DependsOn

	// flags specifies command-line flags for this command (optional)
	// Note: 'env-file' (short 'e') and 'env-var' (short 'E') are reserved system flags and cannot be used.
	flags?: [...#Flag]

	// args specifies positional arguments for this command (optional)
	// Arguments are passed to the script as environment variables:
	//   - INVOWK_ARG_<NAME>: the argument value
	//   - For variadic: INVOWK_ARG_<NAME>_COUNT and INVOWK_ARG_<NAME>_1, _2, etc.
	// Rules:
	//   - Required arguments must come before optional arguments
	//   - Only the last argument can be variadic
	//   - Commands with subcommands cannot have args (validated during command registration)
	args?: [...#Argument]
})

// ModuleRequirement represents a dependency on another pack from a Git repository
#ModuleRequirement: close({
	// git_url is the Git repository URL (required)
	// Supports HTTPS and SSH URLs
	// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
	git_url: string & =~"^(https://|git@|ssh://)"

	// version is the semver constraint for version selection (required)
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0", "1.2.3"
	version: string & =~"^[~^>=<]?[0-9]+"

	// alias overrides the default namespace for imported commands (optional)
	// If not specified, namespace is: <pack-group>@<resolved-version>
	// Must follow pack naming rules
	alias?: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$"

	// path specifies a subdirectory containing the pack (optional)
	// Used for monorepos with multiple packs
	path?: string
})

// Invkfile is the root schema for an invkfile
#Invkfile: close({
	// group is a mandatory prefix for all command names from this invkfile
	// Must start with a letter, contain only alphanumeric characters, with optional
	// dot-separated segments (e.g., "mygroup", "my.group", "my.nested.group")
	// Cannot start or end with a dot, and cannot have consecutive dots
	group: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$"

	// version specifies the invkfile schema version (optional but recommended)
	// Current version: "1.0"
	version?: string & =~"^[0-9]+\\.[0-9]+$"

	// description provides a summary of this invkfile's purpose (optional)
	description?: string

	// default_shell overrides the default shell for native runtime (optional)
	// Example: "/bin/bash", "pwsh"
	default_shell?: string

	// workdir specifies the default working directory for all commands (optional)
	// Can be absolute or relative to the invkfile location.
	// Paths should use forward slashes for cross-platform compatibility.
	// Individual commands or implementations can override this with their own workdir.
	workdir?: string

	// env contains global environment configuration for all commands (optional)
	// Root-level env is applied first (lowest priority from invkfile).
	// Command-level and implementation-level env override root-level env.
	env?: #EnvConfig

	// depends_on specifies global dependencies that apply to all commands (optional)
	// Root-level depends_on is combined with command-level and implementation-level depends_on.
	// Root-level dependencies are validated first (lowest priority in the merge order).
	// This is useful for defining shared prerequisites like required tools or capabilities
	// that apply to all commands in this invkfile.
	depends_on?: #DependsOn

	// cmds defines the available commands (required, at least one)
	cmds: [...#Command] & [_, ...]

	// requires declares dependencies on other packs from Git repositories (optional)
	// Dependencies are resolved at root level only (not per-command)
	// All required modules are loaded and their commands made available
	requires?: [...#ModuleRequirement]

	// commands is not supported (use cmds)
	commands?: _|_
})

// Example usage with the cue command-line tool:
//   cue vet invkfile.cue invkfile_schema.cue -d '#Invkfile'
