// SPDX-License-Identifier: MPL-2.0

// Package benchmark provides comprehensive benchmarks for PGO profile generation.
// These benchmarks cover all hot paths in the invowk codebase:
//   - CUE parsing and schema validation
//   - Module and command discovery
//   - Native, virtual, and container runtime execution
//   - End-to-end command pipeline
//
// To generate a PGO profile, run:
//
//	make pgo-profile       # Full profile (includes container tests)
//	make pgo-profile-short # Short profile (skips container tests)
package benchmark
