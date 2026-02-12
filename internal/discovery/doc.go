// SPDX-License-Identifier: MPL-2.0

// Package discovery handles invkfile and invkmod discovery and command aggregation.
//
// This package intentionally combines two related concerns:
//   - File discovery: locating invkfile.cue and invkmod directories
//   - Command aggregation: building the unified command tree from discovered files
//
// These concerns are tightly coupled because command aggregation depends directly
// on discovery results and ordering. Splitting them would create unnecessary
// indirection without meaningful abstraction benefit.
//
// File organization:
//   - discovery.go: Core types (Discovery, ModuleCollisionError) and loading methods
//   - discovery_files.go: File discovery (DiscoverAll, discoverInDir, etc.)
//   - discovery_commands.go: Command aggregation (DiscoverCommands, CommandInfo, etc.)
//
// Discovery follows a precedence order:
//  1. Current directory invkfile.cue (highest)
//  2. Modules in current directory (*.invkmod)
//  3. Configured includes (module paths from config)
//  4. User commands directory (~/.invowk/cmds — modules only, non-recursive)
//
// For command aggregation, local invkfile commands take highest precedence. Commands from
// sibling modules are included with conflict detection when names collide across sources.
// The DiscoveredCommandSet type provides indexed access for efficient conflict detection
// and grouped listing.
package discovery
