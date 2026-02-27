// SPDX-License-Identifier: MPL-2.0

package platform

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

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
	// detectOnce caches the sandbox detection result for the lifetime of the process.
	// The detection is performed once on first access using real OS lookups.
	//
	// INVARIANT: detectSandboxFrom MUST NOT panic. Unlike sync.Once (where Do
	// treats a panic as "returned" and silently no-ops on subsequent calls),
	// sync.OnceValue propagates the panic on every call, creating a persistent
	// crash condition.
	// Sandbox type is immutable during process lifetime, making process-wide caching safe.
	detectOnce = sync.OnceValue(func() SandboxType {
		return detectSandboxFrom(os.Getenv, statFile)
	})

	// ErrInvalidSandboxType is returned when a SandboxType value is not one of the defined types.
	ErrInvalidSandboxType = errors.New("invalid sandbox type")
)

type (
	// SandboxType identifies the type of application sandbox, if any.
	SandboxType string

	// InvalidSandboxTypeError is returned when a SandboxType value is not recognized.
	// It wraps ErrInvalidSandboxType for errors.Is() compatibility.
	InvalidSandboxTypeError struct {
		Value SandboxType
	}
)

// String returns the string representation of the sandbox type.
// Returns "none" for the zero value (SandboxNone).
func (st SandboxType) String() string {
	if st == SandboxNone {
		return "none"
	}
	return string(st)
}

// Error implements the error interface for InvalidSandboxTypeError.
func (e *InvalidSandboxTypeError) Error() string {
	return fmt.Sprintf("invalid sandbox type %q (valid: \"\"/none, flatpak, snap)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidSandboxTypeError) Unwrap() error {
	return ErrInvalidSandboxType
}

// Validate returns nil if the SandboxType is one of the defined sandbox types,
// or an error wrapping ErrInvalidSandboxType if it is not.
// The zero value ("", SandboxNone) is valid — it means no sandbox detected.
func (st SandboxType) Validate() error {
	switch st {
	case SandboxNone, SandboxFlatpak, SandboxSnap:
		return nil
	default:
		return &InvalidSandboxTypeError{Value: st}
	}
}

// DetectSandbox returns the type of application sandbox the current process is running in.
// The result is cached after the first call for performance.
//
// Detection methods:
//   - Flatpak: Checks for existence of /.flatpak-info
//   - Snap: Checks for SNAP_NAME environment variable
func DetectSandbox() SandboxType {
	return detectOnce()
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
	return SpawnCommandFor(DetectSandbox())
}

// GetSpawnArgs returns the arguments needed to execute a command on the host.
// These arguments should be prepended to the actual command.
//
// For Flatpak, returns ["--host"].
// For Snap, returns ["run", "--shell"].
// For no sandbox, returns nil.
func GetSpawnArgs() []string {
	return SpawnArgsFor(DetectSandbox())
}

// SpawnCommandFor returns the spawn command for a given sandbox type.
// This is a pure function that does not depend on cached detection state,
// making it directly testable without process-wide side effects.
func SpawnCommandFor(st SandboxType) string {
	switch st {
	case SandboxNone:
		return ""
	case SandboxFlatpak:
		return "flatpak-spawn"
	case SandboxSnap:
		return "snap"
	default:
		return ""
	}
}

// SpawnArgsFor returns the spawn arguments for a given sandbox type.
// This is a pure function that does not depend on cached detection state,
// making it directly testable without process-wide side effects.
func SpawnArgsFor(st SandboxType) []string {
	switch st {
	case SandboxNone:
		return nil
	case SandboxFlatpak:
		return []string{"--host"}
	case SandboxSnap:
		return []string{"run", "--shell"}
	default:
		return nil
	}
}

// detectSandboxFrom performs sandbox detection using the provided lookup functions.
// Accepting lookupEnv and statFile as parameters allows tests to inject custom
// behavior without mutating process-wide state.
func detectSandboxFrom(lookupEnv func(string) string, statFile func(string) error) SandboxType {
	// Check for Flatpak sandbox first (takes precedence).
	// The /.flatpak-info file is always present inside Flatpak sandboxes.
	if err := statFile("/.flatpak-info"); err == nil {
		return SandboxFlatpak
	}

	// Check for Snap sandbox.
	// The SNAP_NAME environment variable is set for all snaps.
	if lookupEnv("SNAP_NAME") != "" {
		return SandboxSnap
	}

	return SandboxNone
}

// statFile checks for the existence of a file at the given path.
// This is the production adapter for the statFile parameter of detectSandboxFrom,
// wrapping os.Stat to match the func(string) error signature.
func statFile(path string) error {
	_, err := os.Stat(path)
	return err
}
