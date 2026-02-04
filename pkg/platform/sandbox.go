// SPDX-License-Identifier: MPL-2.0

package platform

import (
	"os"
	"sync"
)

// SandboxType identifies the type of application sandbox, if any.
type SandboxType string

// Sandbox type constants.
const (
	// SandboxNone indicates no sandbox environment detected.
	SandboxNone SandboxType = ""
	// SandboxFlatpak indicates a Flatpak sandbox environment.
	SandboxFlatpak SandboxType = "flatpak"
	// SandboxSnap indicates a Snap sandbox environment.
	SandboxSnap SandboxType = "snap"
)

var (
	detectedSandbox SandboxType
	sandboxOnce     sync.Once
)

// DetectSandbox returns the type of application sandbox the current process is running in.
// The result is cached after the first call for performance.
//
// Detection methods:
//   - Flatpak: Checks for existence of /.flatpak-info
//   - Snap: Checks for SNAP_NAME environment variable
func DetectSandbox() SandboxType {
	sandboxOnce.Do(func() {
		detectedSandbox = detectSandboxInternal()
	})
	return detectedSandbox
}

// IsInSandbox returns true if the current process is running inside a sandbox.
func IsInSandbox() bool {
	return DetectSandbox() != SandboxNone
}

// GetSpawnCommand returns the command used to spawn processes on the host system.
// Returns an empty string if not in a sandbox.
//
// For Flatpak, returns "flatpak-spawn".
// For Snap, returns "snap".
func GetSpawnCommand() string {
	switch DetectSandbox() {
	case SandboxFlatpak:
		return "flatpak-spawn"
	case SandboxSnap:
		return "snap"
	default:
		return ""
	}
}

// GetSpawnArgs returns the arguments needed to execute a command on the host.
// These arguments should be prepended to the actual command.
//
// For Flatpak, returns ["--host"].
// For Snap, returns ["run", "--shell"].
// For no sandbox, returns nil.
func GetSpawnArgs() []string {
	switch DetectSandbox() {
	case SandboxFlatpak:
		return []string{"--host"}
	case SandboxSnap:
		return []string{"run", "--shell"}
	default:
		return nil
	}
}

// detectSandboxInternal performs the actual sandbox detection.
// This is separated from DetectSandbox to allow testing with custom checks.
func detectSandboxInternal() SandboxType {
	// Check for Flatpak sandbox
	// The /.flatpak-info file is always present inside Flatpak sandboxes.
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		return SandboxFlatpak
	}

	// Check for Snap sandbox
	// The SNAP_NAME environment variable is set for all snaps.
	if os.Getenv("SNAP_NAME") != "" {
		return SandboxSnap
	}

	return SandboxNone
}

// resetSandboxDetection resets the sandbox detection cache.
// This is only intended for testing purposes.
func resetSandboxDetection() {
	sandboxOnce = sync.Once{}
	detectedSandbox = SandboxNone
}
