// SPDX-License-Identifier: MPL-2.0

// Package discovery handles invowkfile and invowkmod discovery and command aggregation.
//
// This package intentionally combines two related concerns:
//   - File discovery: locating invowkfile.cue and invowkmod directories
//   - Command aggregation: building the unified command tree from discovered files
//
// These concerns are tightly coupled because command aggregation depends directly
// on discovery results and ordering. Splitting them would create unnecessary
// indirection without meaningful abstraction benefit.
//
// File organization:
//   - discovery.go: Core types (Discovery, ModuleCollisionError) and loading methods
//   - discovery_files.go: File discovery (DiscoverAll, discoverInDir, vendored scanning, etc.)
//   - discovery_commands.go: Command aggregation (DiscoverCommands, CommandInfo, etc.)
//
// Discovery follows a precedence order:
//  1. Current directory invowkfile.cue (highest)
//  2. Modules in current directory (*.invowkmod) and their vendored dependencies
//  3. Configured includes (module paths from config) and their vendored dependencies
//  4. User commands directory (~/.invowk/cmds — modules only, non-recursive) and their vendored dependencies
//
// Vendored modules live in invowk_modules/ inside a parent module. Only one level
// of vendoring is scanned (no recursive nesting). Vendored modules are tagged with
// a ParentModule reference on DiscoveredFile for ownership tracking and collision
// diagnostics.
//
// For command aggregation, local invowkfile commands take highest precedence. Commands from
// sibling modules are included with conflict detection when names collide across sources.
// The DiscoveredCommandSet type provides indexed access for efficient conflict detection
// and grouped listing.
package discovery
