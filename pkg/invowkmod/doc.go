// SPDX-License-Identifier: MPL-2.0

// Package invowkmod provides comprehensive functionality for working with invowk modules.
//
// A module is a self-contained folder with a ".invowkmod" suffix that contains
// an invowkfile and optionally script files. Modules enable portable distribution
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
//   - [Invowkmod]: Module metadata from invowkmod.cue (identity, version, dependencies)
//   - [ModuleRequirement]: Dependency declaration for Git-based modules
//   - [ParseModule]: Parse a complete module (metadata + commands)
//   - [CommandScope]: Enforce module command visibility rules
//
// # Module Naming
//
// Module naming follows these rules:
//   - Folder name must end with ".invowkmod"
//   - Prefix (before .invowkmod) must be POSIX-compliant: start with a letter,
//     contain only alphanumeric characters, with optional dot-separated segments
//   - Compatible with RDNS naming conventions (e.g., "com.example.mycommands")
//   - The folder prefix must match the 'module' field in invowkmod.cue
//
// # Module Structure
//
//   - Must contain exactly one invowkmod.cue at the root (required)
//   - May contain invowkfile.cue with command definitions (optional for library-only modules)
//   - May contain script files referenced by implementations
//   - Cannot be nested inside other modules (except in invowk_modules/ vendor directory)
package invowkmod
