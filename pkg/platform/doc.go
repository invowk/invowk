// SPDX-License-Identifier: MPL-2.0

// Package platform provides cross-platform compatibility utilities.
//
// This package contains utilities for handling platform-specific concerns:
//
//   - OS name constants ([Windows], [Darwin], [Linux]) for runtime.GOOS comparisons
//
//   - Windows reserved filenames that cannot be used as command names
//     or module directory names (see [IsWindowsReservedName])
//
//   - Application sandbox detection for Flatpak and Snap environments
//     (see [DetectSandbox], [IsInSandbox])
//
// # Sandbox Detection
//
// When invowk runs inside a Flatpak or Snap sandbox, container engines like
// Docker/Podman run on the host system with different filesystem namespaces.
// This package provides detection utilities that allow the container engine
// wrapper to route commands through the sandbox's host spawn mechanism
// (e.g., flatpak-spawn --host).
//
// Detection methods:
//   - Flatpak: Checks for existence of /.flatpak-info
//   - Snap: Checks for SNAP_NAME environment variable
package platform
