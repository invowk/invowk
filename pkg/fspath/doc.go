// SPDX-License-Identifier: MPL-2.0

// Package fspath provides typed wrappers around path/filepath functions and
// filesystem utilities. Path wrappers accept and return types.FilesystemPath,
// centralizing //goplint:ignore annotations so callers get typed-in/typed-out
// operations without per-site suppression directives. AtomicWriteFile provides
// crash-safe file writes via temp file + rename with unpredictable temp names.
package fspath
