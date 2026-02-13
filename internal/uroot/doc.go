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
// The following 29 utilities are provided:
//
// From u-root pkg/core (14 wrappers):
//   - base64: Encode/decode base64
//   - cat: Concatenate and display files
//   - chmod: Change file mode bits
//   - cp: Copy files and directories
//   - find: Search for files in a directory hierarchy
//   - gzip: Compress or expand files
//   - ls: List directory contents
//   - mkdir: Create directories
//   - mktemp: Create temporary files or directories
//   - mv: Move/rename files and directories
//   - rm: Remove files and directories
//   - shasum: Compute SHA message digests
//   - tar: Archive files
//   - touch: Create files or update timestamps
//
// Custom implementations (15 commands):
//   - basename: Strip directory and suffix from filenames
//   - cut: Select portions of lines
//   - dirname: Strip last component from filenames
//   - grep: Search for patterns in files
//   - head: Output first N lines
//   - ln: Create hard or symbolic links
//   - realpath: Resolve absolute path names
//   - seq: Generate number sequences
//   - sleep: Delay for a specified time
//   - sort: Sort lines of text
//   - tail: Output last N lines
//   - tee: Duplicate standard input to files
//   - tr: Translate characters
//   - uniq: Report or omit repeated lines
//   - wc: Count lines, words, and bytes
//
// # Usage
//
// The u-root utilities are enabled via the config option:
//
//	virtual_shell: { enable_uroot_utils: true }
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
// # POSIX Combined Short Flags
//
// Custom implementations support POSIX-style combined short flags (e.g., "-sf"
// is equivalent to "-s -f"). Registry.Run() preprocesses arguments using
// unixflag.ArgsToGoArgs before dispatching to custom commands, splitting
// combined flags into individual Go-style flags that flag.NewFlagSet can parse.
//
// Upstream u-root wrappers (those embedding baseWrapper) handle this
// preprocessing internally in their RunContext method and implement the
// NativePreprocessor marker interface. Registry.Run() skips preprocessing
// for these commands to avoid double-splitting that would corrupt long flags.
//
// # Unsupported Flags
//
// GNU-specific flags not supported by u-root implementations are silently
// ignored. Commands execute with supported flags only, providing better
// compatibility with scripts written for GNU coreutils.
package uroot
