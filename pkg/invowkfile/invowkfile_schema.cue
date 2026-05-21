// invowkfile_schema.cue - Schema definitions for command definitions (invowkfile.cue)
// This file defines the structure, types, and constraints for command invowkfiles.
// This schema is embedded in the invowk binary for validation.
//
// NOTE: Module metadata (module, version, description, requires) is now defined
// in invowkmod_schema.cue and must be in a separate invowkmod.cue file.

import "strings"

// RuntimeType defines the available execution runtime types
#RuntimeType: "native" | "virtual-sh" | "virtual-lua" | "container"

// BinaryLookupMode defines how virtual runtimes resolve allowed host binaries.
#BinaryLookupMode: "host" | "strict"

// PlatformType defines the supported operating system types
#PlatformType: "linux" | "macos" | "windows"

// EnvInheritMode defines how environment variables are inherited
#EnvInheritMode: "none" | "allow" | "all"

// VirtualFilesystemAccess controls VM-managed filesystem access for virtual runtimes.
#VirtualFilesystemAccess: *"restricted" | "full"

// VirtualFilesystemPathName defines a logical path name exposed as INVOWK_PATH_<NAME>.
#VirtualFilesystemPathName: string & =~"^[A-Z_][A-Z0-9_]*$" & strings.MaxRunes(256)

#VirtualFilesystemPath: #NonWhitespaceString & strings.MaxRunes(4096)

// FlagType defines the valid types for command flags
#FlagType: "string" | "bool" | "int" | "float"

// ArgumentType defines the valid types for command arguments
#ArgumentType: "string" | "int" | "float"

// DurationString constrains a Go-style duration string (e.g., "30s", "5m", "1h30m").
// Shared by #Implementation.timeout and #WatchConfig.debounce.
// [GO-ONLY] Positive-duration semantics are enforced by DurationString.Validate()
// because CUE only validates the syntactic shape here.
#DurationString: string & =~"^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$" & strings.MaxRunes(32)

// NonWhitespaceString is non-empty and contains at least one non-whitespace rune.
#NonWhitespaceString: string & =~"\\S"

// ContainerName is a strict Docker/Podman-compatible portable container name.
#ContainerName: string & strings.MaxRunes(128) & =~"^[a-z0-9][a-z0-9._-]*$"

// EnvConfig defines environment configuration for a command or implementation
#EnvConfig: close({
	// files lists dotenv files to load (optional)
	// Files are loaded in order; later files override earlier ones.
	// Paths are relative to the invowkfile location (or module root for modules).
	// Files suffixed with '?' are optional and will not cause an error if missing.
	files?: [...string & !="" & strings.MaxRunes(4096)]

	// vars contains environment variables as key-value pairs (optional)
	// These override values loaded from files.
	vars?: [string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]: string & strings.MaxRunes(32768)
})

// RuntimeConfig represents a runtime configuration with type-specific options
#RuntimeConfig: #RuntimeConfigNative | #RuntimeConfigVirtualSh | #RuntimeConfigVirtualLua | #RuntimeConfigContainer

#RuntimeConfigBase: {
	// name specifies the runtime type (required)
	name: #RuntimeType

	// env_inherit_mode controls host environment inheritance (optional)
	// Allowed values: "none", "allow", "all"
	env_inherit_mode?: #EnvInheritMode

	// env_inherit_allow lists host env vars to allow.
	// Requires env_inherit_mode: "allow"; Invowk rejects allowlists with omitted or non-allow modes after decode.
	env_inherit_allow?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)]

	// env_inherit_deny lists host env vars to block (applies to any mode)
	env_inherit_deny?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)]
}

#RuntimeConfigNative: close({
	#RuntimeConfigBase
	name: "native"
})

#RuntimeConfigVirtualBase: {
	#RuntimeConfigBase

	// allowed_binaries lists host binaries this virtual runtime may execute.
	// "*" allows any resolved host binary.
	allowed_binaries?: [...#NonWhitespaceString & strings.MaxRunes(4096)]

	// binary_lookup_mode controls how allowed host binaries are resolved.
	// "host" uses the command environment PATH; "strict" uses Invowk's fixed safe directories.
	binary_lookup_mode?: #BinaryLookupMode
}

#RuntimeConfigVirtualSh: close({
	#RuntimeConfigVirtualBase
	name: "virtual-sh"
})

#RuntimeConfigVirtualLua: close({
	#RuntimeConfigVirtualBase
	name: "virtual-lua"

	// cpu_limit sets an optional golua CPU quota. Zero or omission means unlimited.
	cpu_limit?: int & >=0

	// memory_limit sets an optional golua memory quota.
	memory_limit?: string & =~"^[0-9]+([KkMmGg][Bb]?)?$" & strings.MaxRunes(32)
})

#RuntimeConfigContainer: #RuntimeConfigContainerWithImage | #RuntimeConfigContainerWithContainerfile

#RuntimeConfigContainerBase: {
	#RuntimeConfigBase
	name: "container"

	// enable_host_ssh enables SSH access from container back to host (optional)
	// When enabled, invowk starts an SSH server and provides connection credentials
	// to the container via environment variables: INVOWK_SSH_HOST, INVOWK_SSH_PORT,
	// INVOWK_SSH_USER, INVOWK_SSH_TOKEN, INVOWK_SSH_ENABLED
	// Default: false
	enable_host_ssh?: bool

	// volumes specifies volume mounts in "host:container" format (optional)
	// Example: ["./data:/data", "/tmp:/tmp:ro"]
	volumes?: [...string & !="" & strings.MaxRunes(4096)]

	// ports specifies port mappings in "host:container" format (optional)
	// Example: ["8080:80", "3000:3000"]
	ports?: [...string & !="" & strings.MaxRunes(256)]

	// persistent configures an opt-in persistent container target (optional).
	// create_if_missing defaults to false; name is optional and must be portable.
	persistent?: close({
		create_if_missing?: bool
		name?: #ContainerName
	})

	// depends_on specifies dependencies validated inside the container environment (optional).
	// Unlike root/command/implementation-level depends_on (which always check the host),
	// this validates against the container's own environment — useful for verifying that
	// the container image has the required tools, files, and configuration.
	// Only checked when this container runtime is selected at execution time.
	depends_on?: #DependsOn
}

#RuntimeConfigContainerWithImage: close({
	#RuntimeConfigContainerBase

	// image specifies the pre-built container image source.
	// Exactly one of image or containerfile is required; CUE models the parsed
	// user-config shape and Go keeps the invariant for direct RuntimeConfig values.
	// Example: "debian:stable-slim", "golang:1.26", "python:3-slim"
	image: #NonWhitespaceString & strings.MaxRunes(512)

	// containerfile is not valid in the image-source variant.
	containerfile?: _|_
})

#RuntimeConfigContainerWithContainerfile: close({
	#RuntimeConfigContainerBase

	// containerfile specifies the path to Containerfile/Dockerfile relative to invowkfile.
	// Exactly one of containerfile or image is required; CUE models the parsed
	// user-config shape and Go keeps the invariant for direct RuntimeConfig values.
	// CUE validates non-empty and length. Invowk rejects absolute paths,
	// parent-directory segments, and invalid filename components after decode.
	// [GO-ONLY] Cross-platform path security requires Go.
	containerfile: #NonWhitespaceString & strings.MaxRunes(4096)

	// image is not valid in the containerfile-source variant.
	image?: _|_
})

// VirtualFilesystemConfig configures virtual-runtime filesystem access for a platform.
#VirtualFilesystemConfig: close({
	// access controls whether VM-managed filesystem operations are limited to
	// implicit safe roots plus named paths, or may access the full host filesystem.
	access?: #VirtualFilesystemAccess

	// paths maps logical uppercase path names to platform-local path handles.
	paths?: [#VirtualFilesystemPathName]: #VirtualFilesystemPath
})

// PlatformVirtualConfig holds platform-specific settings for the virtual runtime family.
#PlatformVirtualConfig: close({
	filesystem?: #VirtualFilesystemConfig
})

// PlatformConfig represents a platform configuration.
#PlatformConfig: close({
	// name specifies the platform type (required)
	name: #PlatformType

	// virtual contains platform-specific settings for virtual-* runtimes.
	virtual?: #PlatformVirtualConfig
})

// InterpreterSpec specifies how to execute a script source (optional).
// - Omit: defaults to "auto" (detect from shebang)
// - "auto": detect interpreter from shebang (#!) in first line of resolved script content
// - Specific value: use as interpreter (e.g., "python3", "node", "/usr/bin/ruby")
// - Can include arguments: "python3 -u", "/usr/bin/env perl -w"
// If "auto" and no shebang is found, Invowk falls back to the selected runtime/check default shell behavior.
// [GO-ONLY] The interpreter allowlist and shell metacharacter safety checks are enforced by InterpreterSpec.Validate().
#InterpreterSpec: string & =~"^\\s*\\S.*$" & strings.MaxRunes(1024)

// ScriptSourceCommon contains metadata shared by all script source variants.
#ScriptSourceCommon: {
	// interpreter specifies how to execute the resolved script content (optional).
	interpreter?: #InterpreterSpec
}

// ScriptSource selects an executable script source.
// Exactly one of content or file is required.
#ScriptSource: #ScriptSourceContent | #ScriptSourceFile

#ScriptSourceContent: close({
	#ScriptSourceCommon

	// content contains inline script text.
	content: #NonWhitespaceString & strings.MaxRunes(10485760)

	// file is not valid in the inline-content variant.
	file?: _|_
})

#ScriptSourceFile: close({
	#ScriptSourceCommon

	// file references a script file resolved at execution time.
	// File references are allowed only for invowkfiles loaded from an invowkmod,
	// and the resolved target must stay inside that module. CUE validates local
	// string shape and length. [GO-ONLY] Module-context checks, path resolution,
	// containment, file reads, and resolved script-content validation happen in Go/runtime code.
	file: #NonWhitespaceString & strings.MaxRunes(4096)

	// content is not valid in the file-reference variant.
	content?: _|_
})

// ImplementationScript selects the executable script source for an implementation.
#ImplementationScript: #ScriptSource

// CustomCheckScript selects the executable script source for a custom dependency check.
#CustomCheckScript: #ScriptSource

// Implementation represents an implementation with platform and runtime constraints
#Implementation: close({
	// script selects the executable script source (required).
	// Use content for inline shell commands and file for script-file references.
	script: #ImplementationScript

	// runtimes specifies which runtimes can execute this implementation (required, at least one)
	// The first element is the default runtime for this platform combination
	// Each runtime is a struct with a 'name' field and optional type-specific fields
	runtimes: [...#RuntimeConfig] & [_, ...]

	// platforms specifies which operating systems this implementation is for (required, at least one)
	// Each platform is a struct with a 'name' field
	platforms: [...#PlatformConfig] & [_, ...]

	// env contains environment configuration for this implementation (optional)
	// Implementation-level env is merged with command-level env.
	// Implementation files are loaded after command-level files.
	// Implementation vars override command-level vars.
	env?: #EnvConfig

	// workdir specifies this implementation's working directory (optional).
	// Effective precedence is CLI override > implementation > command > root > default.
	// Applies across native, virtual-sh, and container execution. Relative paths resolve
	// from the invowkfile directory or module root; Go/runtime code owns final resolution
	// and execution-time directory validation.
	workdir?: #NonWhitespaceString & strings.MaxRunes(4096)

	// depends_on specifies dependencies validated against the HOST system (optional).
	// Regardless of the selected runtime, these are always checked on the host.
	// To validate dependencies inside the runtime environment (e.g., inside a container),
	// use depends_on inside the runtime block instead.
	// Implementation-level depends_on is combined with root-level and command-level depends_on.
	depends_on?: #DependsOn

	// timeout specifies the maximum execution duration (optional)
	// Must be a valid Go duration string (e.g., "30s", "5m", "1h30m")
	// When exceeded, the command is cancelled and returns a timeout error.
	timeout?: #DurationString
})

// ToolDependency represents a tool/binary that must be available in PATH
#ToolDependency: close({
	// alternatives is a list of binary names where any match satisfies the dependency (required, at least one)
	// If any of the provided tools is found in PATH, the validation succeeds (early return).
	// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
	// Tool names must be valid binary names: alphanumeric, can include . _ + -
	alternatives: [...string & strings.MaxRunes(256) & =~"^[a-zA-Z0-9][a-zA-Z0-9._+-]*$"] & [_, ...]
})

// CustomCheck represents a custom validation script to verify system requirements
#CustomCheck: close({
	// name is an identifier for this check (required)
	// Used for error reporting and identification
	name: #NonWhitespaceString & strings.MaxRunes(256)

	// script selects the custom-check script source (required).
	// Use content for inline checks and file for module-contained script files.
	script: #CustomCheckScript

	// expected_code is the expected exit code from script (optional, default: 0)
	// Must be in valid exit code range (0-255)
	expected_code?: int & >=0 & <=255

	// expected_output is a regex pattern to match against script output (optional)
	// Can be used together with expected_code
	expected_output?: string & !="" & strings.MaxRunes(1000)
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
	// Paths can be absolute or relative to the invowkfile location.
	alternatives: [...string & !="" & strings.MaxRunes(4096)] & [_, ...]

	// readable checks if the path is readable (optional, default: false)
	readable?: bool

	// writable checks if the path is writable (optional, default: false)
	writable?: bool

	// executable checks if the path is executable (optional, default: false)
	executable?: bool
})

// CommandDependencyRef identifies a command dependency.
// Bare refs resolve only in the declaring command's own source.
// Source-qualified refs use "@source command" syntax, where the source ID can
// contain dots and the command part follows the normal command-name grammar.
#CommandDependencyRef: #BareCommandDependencyRef | #SourceQualifiedCommandDependencyRef
#BareCommandDependencyRef: string & strings.MaxRunes(256) & =~"^[a-zA-Z][a-zA-Z0-9_ -]*$"
#SourceQualifiedCommandDependencyRef: string & strings.MaxRunes(514) & =~"^@[a-zA-Z][a-zA-Z0-9._-]* [a-zA-Z][a-zA-Z0-9_ -]*$"

// CommandDependency represents another invowk command that must be discoverable.
#CommandDependency: close({
	// alternatives is a list of command references where any match satisfies the dependency (required, at least one)
	// If any of the provided commands is discoverable, the dependency is satisfied (early return).
	// This allows specifying alternative commands (e.g., ["build-debug", "@tools lint"]).
	alternatives: [...#CommandDependencyRef] & [_, ...]
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
	// name is the environment variable name to check (required)
	// The check verifies that this env var exists in the user's environment
	// Must be a valid POSIX environment variable name: starts with letter or underscore,
	// followed by letters, digits, or underscores
	name: string & =~"^[A-Za-z_][A-Za-z0-9_]*$"

	// validation is a regex pattern to validate the env var value (optional)
	// If specified, the env var must exist AND its value must match this pattern
	validation?: string & !="" & strings.MaxRunes(1000)
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
	// cmds lists invowk commands that must be discoverable for this command to run.
	// Bare refs are local to the declaring command source. Cross-source refs must
	// use the explicit "@source command" syntax.
	// Uses OR semantics: if any alternative in the list is discoverable, the dependency is satisfied
	cmds?: [...#CommandDependency]

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
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$" & !="" & strings.MaxRunes(256)

	// description provides help text for the argument (required)
	description: string & =~"^\\s*\\S.*$" & strings.MaxRunes(10240)

	// required indicates whether this argument must be provided (optional, defaults to false)
	// If true, the command will fail if the argument is not provided
	// An argument cannot be both required and have a default_value
	// Required arguments must come before optional arguments in the args list
	required?: bool

	// default_value is the default value if the argument is not provided (optional)
	// Cannot be specified together with required: true
	// Must be compatible with the specified type (if type is specified)
	default_value?: string & strings.MaxRunes(4096)

	// type specifies the data type of the argument (optional, defaults to "string")
	// Supported types: "string", "int", "float"
	// - "string": any string value (default)
	// - "int": must be a valid integer
	// - "float": must be a valid floating-point number
	// Note: "bool" is not supported for positional arguments (use flags instead)
	type?: #ArgumentType

	// validation is a regex pattern to validate the argument value (optional)
	// The argument value must match this pattern
	// If default_value is specified, it must also match this pattern
	validation?: string & !="" & strings.MaxRunes(1000)

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
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$" & !="" & strings.MaxRunes(256)

	// description provides help text for the flag (required)
	description: string & =~"^\\s*\\S.*$" & strings.MaxRunes(10240)

	// default_value is the default value for the flag (optional)
	// If not specified, the flag has no default value
	// Must be compatible with the specified type (if type is specified)
	default_value?: string & strings.MaxRunes(4096)

	// type specifies the data type of the flag (optional, defaults to "string")
	// Supported types: "string", "bool", "int", "float"
	// - "string": any string value (default)
	// - "bool": must be "true" or "false"
	// - "int": must be a valid integer
	// - "float": must be a valid floating-point number
	type?: #FlagType

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
	validation?: string & !="" & strings.MaxRunes(1000)
})

// WatchConfig defines file-watching behavior for automatic command re-execution
#WatchConfig: close({
	// patterns lists glob patterns for files to watch (required, at least one)
	// Patterns support ** for recursive matching (e.g., "src/**/*.go", "*.ts")
	// Paths are relative to the effective working directory of the command
	patterns: [...string & !="" & strings.MaxRunes(4096)] & [_, ...]

	// debounce specifies the delay before re-executing after a change (optional)
	// Must be a valid Go duration string (e.g., "500ms", "1s", "2s")
	// Default: "500ms"
	debounce?: #DurationString

	// clear_screen clears the terminal before each re-execution (optional)
	// Default: false
	clear_screen?: bool

	// ignore lists glob patterns for files/directories to exclude from watching (optional)
	// Common ignores (.git, node_modules) are applied by default
	ignore?: [...string & !="" & strings.MaxRunes(4096)]
})

// Command represents a single executable command
#Command: close({
	// name is the command identifier (required)
	// Can include spaces for subcommand-like behavior (e.g., "test unit")
	name: string & =~"^[a-zA-Z][a-zA-Z0-9_ -]*$" & strings.MaxRunes(256)

	// description provides help text for the command (optional)
	// When declared, description must be non-empty (cannot be "" or whitespace-only)
	description?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(10240)

	// category groups this command under a heading in 'invowk cmd' output (optional)
	// When declared, category must be non-empty (cannot be "" or whitespace-only)
	// Examples: "build", "test", "deploy", "utilities"
	category?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(256)

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
	// Can be absolute or relative to the invowkfile location.
	// Paths should use forward slashes for cross-platform compatibility.
	workdir?: #NonWhitespaceString & strings.MaxRunes(4096)

	// depends_on specifies dependencies validated against the HOST system (optional).
	// Regardless of the selected runtime, these are always checked on the host.
	// To validate dependencies inside the runtime environment, use depends_on inside the runtime block.
	depends_on?: #DependsOn

	// flags specifies command-line flags for this command (optional)
	// The 'ivk-', 'invowk-', and 'i-' prefixes are reserved for system flags and cannot be used.
	// Reserved built-in flags: help (h), version.
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

	// watch defines file-watching configuration for this command (optional)
	// Two-tier behavior: when watch config is defined here, its patterns are required
	// and control exactly which files are watched. When --ivk-watch is passed without
	// any watch config, the CLI falls back to watching all files (**/*) in the working directory.
	watch?: #WatchConfig
})

// Invowkfile is the root schema for command definitions (invowkfile.cue)
// Module metadata (module, version, description, requires) is now in invowkmod.cue
#Invowkfile: close({
	// default_shell overrides the default shell for native runtime (optional)
	// Example: "/bin/bash", "pwsh"
	default_shell?: string & =~"^\\s*\\S.*$" & strings.MaxRunes(1024)

	// workdir specifies the default working directory for all commands (optional)
	// Can be absolute or relative to the invowkfile location.
	// Paths should use forward slashes for cross-platform compatibility.
	// Individual commands or implementations can override this with their own workdir.
	workdir?: #NonWhitespaceString & strings.MaxRunes(4096)

	// env contains global environment configuration for all commands (optional)
	// Root-level env is applied first (lowest priority from invowkfile).
	// Command-level and implementation-level env override root-level env.
	env?: #EnvConfig

	// depends_on specifies global dependencies validated against the HOST system (optional).
	// Regardless of the selected runtime, these are always checked on the host.
	// Root-level depends_on is combined with command-level and implementation-level depends_on.
	// Root-level dependencies are validated first (lowest priority in the merge order).
	// This is useful for defining shared prerequisites like required tools or capabilities
	// that apply to all commands in this invowkfile.
	// To validate dependencies inside the runtime environment, use depends_on inside the runtime block.
	depends_on?: #DependsOn

	// cmds defines the available commands (required, at least one)
	cmds: [...#Command] & [_, ...]
})

// Example usage with the cue command-line tool:
//   cue vet invowkfile.cue invowkfile_schema.cue -d '#Invowkfile'
