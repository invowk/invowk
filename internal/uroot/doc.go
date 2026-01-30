// SPDX-License-Identifier: MPL-2.0

// Package uroot provides built-in POSIX utilities for Invowk's virtual shell runtime.
//
// This package integrates utilities from the u-root project (github.com/u-root/u-root)
// with custom implementations for commands not available in u-root's pkg/core.
// When enabled, these utilities intercept command execution in the virtual shell,
// allowing scripts to run without requiring external binaries on the host system.
//
// # Supported Commands
//
// The following 15 utilities are provided:
//
// From u-root pkg/core (7 wrappers):
//   - cat: Concatenate and display files
//   - cp: Copy files and directories
//   - ls: List directory contents
//   - mkdir: Create directories
//   - mv: Move/rename files and directories
//   - rm: Remove files and directories
//   - touch: Create files or update timestamps
//
// Custom implementations (8 commands):
//   - cut: Select portions of lines
//   - grep: Search for patterns in files
//   - head: Output first N lines
//   - sort: Sort lines of text
//   - tail: Output last N lines
//   - tr: Translate characters
//   - uniq: Report or omit repeated lines
//   - wc: Count lines, words, and bytes
//
// # Usage
//
// The u-root utilities are enabled via the config option:
//
//	virtual_shell:
//	    enable_uroot_utils: true
//
// When enabled, the VirtualRuntime's exec handler checks the Registry for each
// command before falling back to system binaries.
//
// # Error Format
//
// All errors from u-root utilities are prefixed with "[uroot]" for clear
// identification:
//
//	[uroot] cp: /source/file: no such file or directory
//	[uroot] rm: /protected: permission denied
//
// # Streaming I/O
//
// All file operations use streaming I/O (io.Copy or equivalent) to ensure
// constant memory usage regardless of file size. This prevents OOM conditions
// when processing large files.
//
// # Unsupported Flags
//
// GNU-specific flags not supported by u-root implementations are silently
// ignored. Commands execute with supported flags only, providing better
// compatibility with scripts written for GNU coreutils.
package uroot
