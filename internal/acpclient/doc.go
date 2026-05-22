// SPDX-License-Identifier: MPL-2.0

// Package acpclient provides a dormant internal Agent Client Protocol client
// foundation for future stateful agent-session features.
//
// This package is intentionally not wired into any current CLI command,
// configuration schema, CUE schema, or documentation surface. Existing LLM
// workflows continue to use their current direct provider and completion paths.
package acpclient
