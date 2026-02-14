// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

const (
	// homebrewMacARM is the Homebrew prefix on macOS ARM (Apple Silicon).
	homebrewMacARM = "/opt/homebrew/"

	// homebrewMacIntel is the Homebrew Cellar path on macOS Intel.
	homebrewMacIntel = "/usr/local/Cellar/"

	// homebrewLinux is the Linuxbrew prefix.
	homebrewLinux = "/home/linuxbrew/.linuxbrew/"

	// scriptInstallDir is the conventional install location for the shell install script.
	scriptInstallDir = "/.local/bin/"

	// modulePath is the expected Go module path used to confirm go-install origin.
	modulePath = "github.com/invowk/invowk"

	// InstallMethodUnknown indicates the install method could not be determined,
	// typically a manual download or custom installation.
	InstallMethodUnknown InstallMethod = 0

	// InstallMethodScript indicates installation via the shell install script,
	// which places the binary in ~/.local/bin/.
	InstallMethodScript InstallMethod = 1

	// InstallMethodHomebrew indicates installation via Homebrew (brew install).
	// Upgrades should be handled by `brew upgrade invowk`.
	InstallMethodHomebrew InstallMethod = 2

	// InstallMethodGoInstall indicates installation via `go install`.
	// Upgrades should be handled by re-running `go install` with the desired version.
	InstallMethodGoInstall InstallMethod = 3
)

var (
	// installMethodHint is set via -ldflags at build time to override detection.
	// When non-empty, it takes priority over all path heuristics.
	//
	//nolint:gochecknoglobals // Build-time ldflags injection requires a package-level variable.
	installMethodHint string

	// readBuildInfo is a test seam for debug.ReadBuildInfo. Production code uses the
	// real implementation; tests replace it to simulate different build info scenarios.
	//
	//nolint:gochecknoglobals // Test seam requires a package-level variable.
	readBuildInfo = debug.ReadBuildInfo
)

// InstallMethod identifies how invowk was installed on the current system.
// The detection result is used to route upgrade behavior: script installs
// can be self-updated, while Homebrew and go-install should defer to their
// respective package managers.
type InstallMethod int

// String returns a human-readable name for the install method.
func (m InstallMethod) String() string {
	switch m {
	case InstallMethodUnknown:
		return "unknown"
	case InstallMethodScript:
		return "script"
	case InstallMethodHomebrew:
		return "homebrew"
	case InstallMethodGoInstall:
		return "goinstall"
	}
	return "unknown"
}

// DetectInstallMethod determines how invowk was installed based on the executable path
// and build information. Detection priority:
//  1. Build-time ldflags hint (highest priority) -- checked via the installMethodHint package var
//  2. Path heuristics -- Homebrew cellar paths, GOPATH/bin
//  3. debug.ReadBuildInfo() module path confirmation for go-install
//  4. Fallback to Unknown
func DetectInstallMethod(execPath string) InstallMethod {
	// 1. Build-time ldflags hint takes absolute priority.
	if installMethodHint != "" {
		return parseMethodHint(installMethodHint)
	}

	// 2. Homebrew path heuristics -- check all known Homebrew prefixes.
	if strings.Contains(execPath, homebrewMacARM) ||
		strings.Contains(execPath, homebrewMacIntel) ||
		strings.Contains(execPath, homebrewLinux) {
		return InstallMethodHomebrew
	}

	// 3. Go install -- path must be in GOPATH/bin AND build info must confirm
	// the module path. Both conditions are required to avoid false positives
	// from binaries that happen to be placed in GOPATH/bin manually.
	if isInGOPATHBin(execPath) && hasInvowkModulePath() {
		return InstallMethodGoInstall
	}

	// 4. Script install -- the install script places the binary in ~/.local/bin/.
	if strings.Contains(execPath, scriptInstallDir) {
		return InstallMethodScript
	}

	// 5. Fallback -- could not determine install method.
	return InstallMethodUnknown
}

// parseMethodHint converts a build-time ldflags hint string to an InstallMethod.
func parseMethodHint(hint string) InstallMethod {
	switch strings.ToLower(hint) {
	case "homebrew":
		return InstallMethodHomebrew
	case "goinstall":
		return InstallMethodGoInstall
	case "script":
		return InstallMethodScript
	default:
		return InstallMethodUnknown
	}
}

// isInGOPATHBin checks whether the given path is inside $GOPATH/bin.
// It uses the GOPATH environment variable, falling back to ~/go if unset
// (matching the Go toolchain's default behavior).
func isInGOPATHBin(execPath string) bool {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		gopath = filepath.Join(home, "go")
	}

	gopathBin := filepath.Clean(filepath.Join(gopath, "bin"))
	cleanExec := filepath.Clean(execPath)

	// Check if the executable path starts with the GOPATH/bin directory.
	// The trailing separator ensures we match the directory boundary, not
	// a prefix like /home/user/gobin vs /home/user/go/bin.
	return strings.HasPrefix(cleanExec, gopathBin+string(filepath.Separator)) ||
		cleanExec == gopathBin
}

// hasInvowkModulePath checks whether the current binary's build info contains
// the expected invowk module path. This confirms the binary was built via
// `go install github.com/invowk/invowk@...` rather than being a manually-placed
// binary that happens to reside in GOPATH/bin.
func hasInvowkModulePath() bool {
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return false
	}
	return strings.Contains(info.Path, modulePath)
}
