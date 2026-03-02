// SPDX-License-Identifier: MPL-2.0

// Package fspath provides typed wrappers around path/filepath functions that
// accept and return types.FilesystemPath. Each wrapper centralizes the single
// //goplint:ignore annotation so callers get typed-in/typed-out path operations
// without needing per-site suppression directives.
package fspath
