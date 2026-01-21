// SPDX-License-Identifier: MPL-2.0

// Package invkmod provides comprehensive functionality for working with invowk modules.
//
// A module is a self-contained folder with a ".invkmod" suffix that contains
// an invkfile and optionally script files. Modules enable portable distribution
// of invowk commands with their associated scripts.
//
// This package consolidates all module-related functionality:
//
// # Local Module Operations
//
// Local module management including validation, creation, archiving, and vendoring:
//   - [Validate]: Comprehensive module structure and security validation
//   - [Load]: Validate and load a module into a runtime representation
//   - [Create]: Scaffold new modules with proper structure
//   - [Archive]: Create ZIP archives for module distribution
//   - [Unpack]: Extract modules from ZIP archives (local or remote)
//
// # Git-Based Dependency Resolution
//
// Remote module management from Git repositories:
//   - [Resolver]: Orchestrates dependency resolution, caching, and synchronization
//   - [GitFetcher]: Handles Git operations (clone, fetch, checkout)
//   - [SemverResolver]: Semantic version constraint matching
//   - Lock file management for reproducible builds
//
// # Module Metadata and Parsing
//
// Types and parsing for module configuration:
//   - [Invkmod]: Module metadata from invkmod.cue (identity, version, dependencies)
//   - [ModuleRequirement]: Dependency declaration for Git-based modules
//   - [ParseModule]: Parse a complete module (metadata + commands)
//   - [CommandScope]: Enforce module command visibility rules
//
// # Module Naming
//
// Module naming follows these rules:
//   - Folder name must end with ".invkmod"
//   - Prefix (before .invkmod) must be POSIX-compliant: start with a letter,
//     contain only alphanumeric characters, with optional dot-separated segments
//   - Compatible with RDNS naming conventions (e.g., "com.example.mycommands")
//   - The folder prefix must match the 'module' field in invkmod.cue
//
// # Module Structure
//
//   - Must contain exactly one invkmod.cue at the root (required)
//   - May contain invkfile.cue with command definitions (optional for library-only modules)
//   - May contain script files referenced by implementations
//   - Cannot be nested inside other modules (except in invk_modules/ vendor directory)
package invkmod
