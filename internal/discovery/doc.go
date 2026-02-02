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
package discovery
