// SPDX-License-Identifier: EPL-2.0

// Package invkpack provides comprehensive functionality for working with invowk packs.
//
// A pack is a self-contained folder with a ".invkpack" suffix that contains
// an invkfile and optionally script files. Packs enable portable distribution
// of invowk commands with their associated scripts.
//
// This package consolidates all pack-related functionality:
//
// # Local Pack Operations
//
// Local pack management including validation, creation, archiving, and vendoring:
//   - [Validate]: Comprehensive pack structure and security validation
//   - [Load]: Validate and load a pack into a runtime representation
//   - [Create]: Scaffold new packs with proper structure
//   - [Archive]: Create ZIP archives for pack distribution
//   - [Unpack]: Extract packs from ZIP archives (local or remote)
//
// # Git-Based Dependency Resolution
//
// Remote pack management from Git repositories:
//   - [Resolver]: Orchestrates dependency resolution, caching, and synchronization
//   - [GitFetcher]: Handles Git operations (clone, fetch, checkout)
//   - [SemverResolver]: Semantic version constraint matching
//   - Lock file management for reproducible builds
//
// # Pack Metadata and Parsing
//
// Types and parsing for pack configuration:
//   - [Invkpack]: Pack metadata from invkpack.cue (identity, version, dependencies)
//   - [PackRequirement]: Dependency declaration for Git-based packs
//   - [ParsePack]: Parse a complete pack (metadata + commands)
//   - [CommandScope]: Enforce pack command visibility rules
//
// # Pack Naming
//
// Pack naming follows these rules:
//   - Folder name must end with ".invkpack"
//   - Prefix (before .invkpack) must be POSIX-compliant: start with a letter,
//     contain only alphanumeric characters, with optional dot-separated segments
//   - Compatible with RDNS naming conventions (e.g., "com.example.mycommands")
//   - The folder prefix must match the 'pack' field in invkpack.cue
//
// # Pack Structure
//
//   - Must contain exactly one invkpack.cue at the root (required)
//   - May contain invkfile.cue with command definitions (optional for library-only packs)
//   - May contain script files referenced by implementations
//   - Cannot be nested inside other packs (except in invk_packs/ vendor directory)
package invkpack
