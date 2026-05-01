// SPDX-License-Identifier: MPL-2.0

// Package invowkmod defines Invowk module metadata, validation, lock-file
// structures, and module-domain policies.
//
// A module is a self-contained folder with a ".invowkmod" suffix that contains
// an invowkfile and optionally script files. Modules enable portable distribution
// of invowk commands with their associated scripts.
//
// This package owns stable value types shared by application services. Workflow
// operations such as archive/import/vendor are coordinated by the application
// layer in internal/app/moduleops so this package stays focused on module
// structure and invariants.
//
// # Module Structure and Validation
//
// Local module structure checks:
//   - [Validate]: Comprehensive module structure and security validation
//   - [Load]: Validate and load a module into a runtime representation
//   - [Create]: Scaffold new modules with proper structure
//
// # Dependency Resolution Model
//
// Types and policies used by the application-layer module resolver:
//   - [ModuleRef]: Dependency declaration for Git-based modules
//   - [ResolvedModule]: Resolved module metadata persisted to lock files
//   - [SemverResolver]: Semantic version constraint matching
//   - [LockFile]: Lock file management for reproducible builds
//   - [DeclaredLockedModuleEntries]: Declaration-to-lock policy helpers
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
