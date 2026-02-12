// invkmod_schema.cue - Schema definitions for module metadata (invkmod.cue)
// This file defines the structure for module identity and dependency declarations.
// This schema is embedded in the invowk binary for validation.
//
// The invkmod.cue file is analogous to Go's go.mod - it contains:
// - Module identity (mandatory)
// - Module version metadata
// - Dependency declarations (requires)
//
// Command definitions remain in invkfile.cue (separate file).

import "strings"

// ModuleRequirement represents a dependency on another module from a Git repository
#ModuleRequirement: close({
	// git_url is the Git repository URL (required)
	// Supports HTTPS and SSH URLs
	// The repository name MUST end with .invkmod suffix
	// Examples: "https://github.com/user/mytools.invkmod.git", "git@github.com:user/utils.invkmod.git"
	git_url: string & =~"^(https://|git@|ssh://)" & strings.MaxRunes(2048)

	// version is the semver constraint for version selection (required)
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0", "1.2.3"
	version: string & =~"^[~^>=<]?[0-9]+" & strings.MaxRunes(64)

	// alias overrides the default namespace for imported commands (optional)
	// If not specified, namespace is: <module>@<resolved-version>
	// Must follow module naming rules
	// Used to disambiguate collisions between modules with same name
	alias?: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$" & strings.MaxRunes(256)

	// path specifies a subdirectory containing the module (optional)
	// Used for monorepos with multiple modules
	// Must be relative and cannot contain path traversal sequences
	path?: string & strings.MaxRunes(4096) & =~"^[^/]" & !~"\\.\\."
})

// Invkmod is the root schema for module metadata (invkmod.cue)
// This file MUST exist in every .invkmod directory
#Invkmod: close({
	// module is a MANDATORY identifier for this module
	// Acts as module identity and command namespace prefix
	// Must start with a letter, contain only alphanumeric characters, with optional
	// dot-separated segments. RDNS format recommended (e.g., "io.invowk.sample", "com.example.mytools")
	// Cannot start or end with a dot, and cannot have consecutive dots
	// IMPORTANT: The module value MUST match the folder name prefix (before .invkmod)
	// Example: folder "io.invowk.sample.invkmod" must have module: "io.invowk.sample"
	module: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$" & strings.MaxRunes(256)

	// version specifies the module version using semantic versioning (mandatory)
	// Format: MAJOR.MINOR.PATCH with optional pre-release label (e.g., "1.0.0", "2.1.0-alpha.1")
	// No "v" prefix, no build metadata, no leading zeros on numeric segments
	version: string & =~"^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(-[0-9a-zA-Z-]+(\\.[0-9a-zA-Z-]+)*)?$" & strings.MaxRunes(64)

	// description provides a summary of this module's purpose (optional)
	// Maximum 10KB to prevent abuse
	description?: string & strings.MaxRunes(10240)

	// requires declares dependencies on other modules from Git repositories (optional)
	// Dependencies are resolved at module level
	// All required modules are loaded and their commands made available
	// IMPORTANT: Commands in this module can ONLY call:
	//   1. Commands from globally installed modules (~/.invowk/modules/)
	//   2. Commands from modules declared directly in THIS requires list
	// Commands CANNOT call transitive dependencies (dependencies of dependencies)
	requires?: [...#ModuleRequirement]
})

// Example invkmod.cue:
//
//   module: "io.invowk.sample"
//   version: "1.0.0"
//   description: "Sample module demonstrating invowk capabilities"
//
//   requires: [
//       {
//           git_url: "https://github.com/example/utils.invkmod.git"
//           version: "^1.0.0"
//       },
//   ]
//
// Example usage with the cue command-line tool:
//   cue vet invkmod.cue invkmod_schema.cue -d '#Invkmod'
