// invkpack_schema.cue - Schema definitions for pack metadata (invkpack.cue)
// This file defines the structure for pack identity and dependency declarations.
// This schema is embedded in the invowk binary for validation.
//
// The invkpack.cue file is analogous to Go's go.mod - it contains:
// - Pack identity (mandatory)
// - Pack version metadata
// - Dependency declarations (requires)
//
// Command definitions remain in invkfile.cue (separate file).

// PackRequirement represents a dependency on another pack from a Git repository
#PackRequirement: close({
	// git_url is the Git repository URL (required)
	// Supports HTTPS and SSH URLs
	// The repository name MUST end with .invkpack suffix
	// Examples: "https://github.com/user/mytools.invkpack.git", "git@github.com:user/utils.invkpack.git"
	git_url: string & =~"^(https://|git@|ssh://)"

	// version is the semver constraint for version selection (required)
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0", "1.2.3"
	version: string & =~"^[~^>=<]?[0-9]+"

	// alias overrides the default namespace for imported commands (optional)
	// If not specified, namespace is: <pack>@<resolved-version>
	// Must follow pack naming rules
	// Used to disambiguate collisions between packs with same name
	alias?: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$"

	// path specifies a subdirectory containing the pack (optional)
	// Used for monorepos with multiple packs
	path?: string
})

// Invkpack is the root schema for pack metadata (invkpack.cue)
// This file MUST exist in every .invkpack directory
#Invkpack: close({
	// pack is a MANDATORY identifier for this pack
	// Acts as pack identity and command namespace prefix
	// Must start with a letter, contain only alphanumeric characters, with optional
	// dot-separated segments. RDNS format recommended (e.g., "io.invowk.sample", "com.example.mytools")
	// Cannot start or end with a dot, and cannot have consecutive dots
	// IMPORTANT: The pack value MUST match the folder name prefix (before .invkpack)
	// Example: folder "io.invowk.sample.invkpack" must have pack: "io.invowk.sample"
	pack: string & =~"^[a-zA-Z][a-zA-Z0-9]*(\\.[a-zA-Z][a-zA-Z0-9]*)*$"

	// version specifies the pack schema version (optional but recommended)
	// Current version: "1.0"
	version?: string & =~"^[0-9]+\\.[0-9]+$"

	// description provides a summary of this pack's purpose (optional)
	description?: string

	// requires declares dependencies on other packs from Git repositories (optional)
	// Dependencies are resolved at pack level
	// All required packs are loaded and their commands made available
	// IMPORTANT: Commands in this pack can ONLY call:
	//   1. Commands from globally installed packs (~/.invowk/packs/)
	//   2. Commands from packs declared directly in THIS requires list
	// Commands CANNOT call transitive dependencies (dependencies of dependencies)
	requires?: [...#PackRequirement]
})

// Example invkpack.cue:
//
//   pack: "io.invowk.sample"
//   version: "1.0"
//   description: "Sample pack demonstrating invowk capabilities"
//
//   requires: [
//       {
//           git_url: "https://github.com/example/utils.invkpack.git"
//           version: "^1.0.0"
//       },
//   ]
//
// Example usage with the cue command-line tool:
//   cue vet invkpack.cue invkpack_schema.cue -d '#Invkpack'
