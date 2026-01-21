// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// RuntimeNative executes commands using the system's default shell
	RuntimeNative RuntimeMode = "native"
	// RuntimeVirtual executes commands using mvdan/sh with u-root utilities
	RuntimeVirtual RuntimeMode = "virtual"
	// RuntimeContainer executes commands inside a disposable container
	RuntimeContainer RuntimeMode = "container"

	// EnvInheritNone disables host environment inheritance
	EnvInheritNone EnvInheritMode = "none"
	// EnvInheritAllow inherits only allowlisted host environment variables
	EnvInheritAllow EnvInheritMode = "allow"
	// EnvInheritAll inherits all host environment variables (filtered for invowk vars)
	EnvInheritAll EnvInheritMode = "all"

	// PlatformLinux represents Linux operating system
	PlatformLinux PlatformType = "linux"
	// PlatformMac represents macOS operating system
	PlatformMac PlatformType = "macos"
	// PlatformWindows represents Windows operating system
	PlatformWindows PlatformType = "windows"

	// HostLinux represents Linux operating system
	HostLinux = PlatformLinux
	// HostMac represents macOS operating system
	HostMac = PlatformMac
	// HostWindows represents Windows operating system
	HostWindows = PlatformWindows

	// InterpreterAuto is the special value for automatic shebang detection.
	// When interpreter is empty or set to "auto", invowk will parse the shebang
	// from the script content to determine the interpreter.
	InterpreterAuto = "auto"
)

var (
	// shellInterpreters maps shell interpreter base names to true.
	// These interpreters are compatible with the virtual runtime (mvdan/sh).
	shellInterpreters = map[string]bool{
		"sh": true, "bash": true, "zsh": true, "dash": true,
		"ash": true, "ksh": true, "mksh": true,
	}

	// interpreterExtensions maps interpreter base names to typical file extensions.
	// Used when creating temporary script files to ensure proper syntax highlighting
	// and interpreter behavior.
	interpreterExtensions = map[string]string{
		"python": ".py", "python3": ".py", "python2": ".py",
		"ruby": ".rb", "perl": ".pl", "node": ".js",
		"bash": ".sh", "sh": ".sh", "zsh": ".zsh",
		"fish": ".fish", "pwsh": ".ps1", "powershell": ".ps1",
		"php": ".php", "lua": ".lua", "Rscript": ".R",
	}
)

type (
	// RuntimeMode represents the execution runtime type
	RuntimeMode string

	// EnvInheritMode defines how host environment variables are inherited
	EnvInheritMode string

	// PlatformType represents a target platform type
	PlatformType string

	// HostOS represents a supported operating system (deprecated, use PlatformType)
	HostOS = PlatformType

	// RuntimeConfig represents a runtime configuration with type-specific options
	RuntimeConfig struct {
		// Name specifies the runtime type (required)
		Name RuntimeMode `json:"name"`
		// Interpreter specifies how to execute the script (native and container only)
		// - Omit field: defaults to "auto" (detect from shebang)
		// - "auto": detect interpreter from shebang (#!) in first line of script
		// - Specific value: use as interpreter (e.g., "python3", "node")
		// When declared, interpreter must be non-empty (cannot be "" or whitespace-only)
		// Not allowed for virtual runtime (CUE schema enforces this, Go validates as fallback)
		Interpreter string `json:"interpreter,omitempty"`
		// EnvInheritMode controls host environment inheritance (optional)
		// Allowed values: "none", "allow", "all"
		EnvInheritMode EnvInheritMode `json:"env_inherit_mode,omitempty"`
		// EnvInheritAllow lists host env vars to allow when EnvInheritMode is "allow"
		EnvInheritAllow []string `json:"env_inherit_allow,omitempty"`
		// EnvInheritDeny lists host env vars to block (applies to any mode)
		EnvInheritDeny []string `json:"env_inherit_deny,omitempty"`
		// EnableHostSSH enables SSH access from container back to host (container only)
		// Only valid when Name is "container". Default: false
		EnableHostSSH bool `json:"enable_host_ssh,omitempty"`
		// Containerfile specifies the path to Containerfile/Dockerfile (container only)
		// Mutually exclusive with Image
		Containerfile string `json:"containerfile,omitempty"`
		// Image specifies a pre-built container image to use (container only)
		// Mutually exclusive with Containerfile
		Image string `json:"image,omitempty"`
		// Volumes specifies volume mounts in "host:container" format (container only)
		Volumes []string `json:"volumes,omitempty"`
		// Ports specifies port mappings in "host:container" format (container only)
		Ports []string `json:"ports,omitempty"`
	}

	// PlatformConfig represents a platform configuration
	PlatformConfig struct {
		// Name specifies the platform type (required)
		Name PlatformType `json:"name"`
	}

	// ShebangInfo contains parsed shebang information from a script.
	ShebangInfo struct {
		// Interpreter is the interpreter path or command name (e.g., "/bin/bash", "python3")
		Interpreter string
		// Args contains additional arguments to pass to the interpreter (e.g., ["-u"] for python3 -u)
		Args []string
		// Found indicates whether a valid shebang was detected
		Found bool
	}
)

// IsValid returns true if the EnvInheritMode is a valid value
func (m EnvInheritMode) IsValid() bool {
	switch m {
	case EnvInheritNone, EnvInheritAllow, EnvInheritAll:
		return true
	default:
		return false
	}
}

// GetEffectiveInterpreter returns the effective interpreter value for a RuntimeConfig.
// If the Interpreter field is empty, returns "auto" (the default).
func (rc *RuntimeConfig) GetEffectiveInterpreter() string {
	if rc.Interpreter == "" {
		return InterpreterAuto
	}
	return rc.Interpreter
}

// ResolveInterpreterFromScript resolves the interpreter for this runtime config
// using the provided script content. This is a convenience method that combines
// GetEffectiveInterpreter with shebang parsing.
//
// Returns the parsed ShebangInfo. If Found is false, the caller should use
// the default shell-based execution.
func (rc *RuntimeConfig) ResolveInterpreterFromScript(scriptContent string) ShebangInfo {
	return ResolveInterpreter(rc.Interpreter, scriptContent)
}

// ValidateInterpreterForRuntime checks if the interpreter configuration is valid
// for the runtime type. Returns an error if interpreter is set for virtual runtime.
func (rc *RuntimeConfig) ValidateInterpreterForRuntime() error {
	if rc.Name == RuntimeVirtual && rc.Interpreter != "" {
		return fmt.Errorf("interpreter field is not allowed for virtual runtime (got %q); virtual runtime uses mvdan/sh and cannot execute custom interpreters", rc.Interpreter)
	}
	return nil
}

// ParseShebang extracts interpreter information from script content.
// It parses the first line looking for a shebang (#!) pattern.
//
// Supported formats:
//   - #!/bin/bash           -> Interpreter: "/bin/bash", Args: []
//   - #!/usr/bin/env python3 -> Interpreter: "python3", Args: []
//   - #!/usr/bin/env -S python3 -u -> Interpreter: "python3", Args: ["-u"]
//   - #!/usr/bin/perl -w    -> Interpreter: "/usr/bin/perl", Args: ["-w"]
//   - #! /bin/sh            -> Interpreter: "/bin/sh", Args: [] (space after #! allowed)
//
// If no valid shebang is found, returns ShebangInfo{Found: false}.
func ParseShebang(content string) ShebangInfo {
	// Get first line
	firstLine, _, _ := strings.Cut(content, "\n")
	// Also handle Windows-style line endings
	firstLine = strings.TrimSuffix(firstLine, "\r")
	firstLine = strings.TrimSpace(firstLine)

	// Check for shebang prefix
	if !strings.HasPrefix(firstLine, "#!") {
		return ShebangInfo{Found: false}
	}

	// Extract the part after #!
	shebang := strings.TrimSpace(strings.TrimPrefix(firstLine, "#!"))
	if shebang == "" {
		return ShebangInfo{Found: false}
	}

	// Split into parts
	parts := strings.Fields(shebang)
	if len(parts) == 0 {
		return ShebangInfo{Found: false}
	}

	interpreter := parts[0]
	args := parts[1:]

	// Handle /usr/bin/env specially (finds interpreter in PATH)
	if interpreter == "/usr/bin/env" || interpreter == "/bin/env" {
		return parseEnvShebang(args)
	}

	return ShebangInfo{
		Interpreter: interpreter,
		Args:        args,
		Found:       true,
	}
}

// parseEnvShebang handles the special case of #!/usr/bin/env
// which is used to find the interpreter in PATH.
func parseEnvShebang(args []string) ShebangInfo {
	if len(args) == 0 {
		return ShebangInfo{Found: false}
	}

	// Handle -S flag (split string mode, common on BSD/macOS)
	// Example: #!/usr/bin/env -S python3 -u
	if args[0] == "-S" {
		if len(args) < 2 {
			return ShebangInfo{Found: false}
		}
		return ShebangInfo{
			Interpreter: args[1],
			Args:        args[2:],
			Found:       true,
		}
	}

	// Skip other env flags (rare, but handle gracefully)
	// Look for the first non-flag argument as the interpreter
	interpreterIdx := 0
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			interpreterIdx = i
			break
		}
		// If all args are flags, we can't find an interpreter
		if i == len(args)-1 {
			return ShebangInfo{Found: false}
		}
	}

	return ShebangInfo{
		Interpreter: args[interpreterIdx],
		Args:        args[interpreterIdx+1:],
		Found:       true,
	}
}

// ParseInterpreterString parses an interpreter specification string.
// The string may contain the interpreter and arguments, e.g., "python3 -u".
//
// This is used when the interpreter is explicitly specified (not "auto").
// Returns ShebangInfo{Found: false} if the spec is empty or "auto".
func ParseInterpreterString(spec string) ShebangInfo {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == InterpreterAuto {
		return ShebangInfo{Found: false}
	}

	parts := strings.Fields(spec)
	if len(parts) == 0 {
		return ShebangInfo{Found: false}
	}

	// Handle env-based specifications (e.g., "/usr/bin/env python3")
	if parts[0] == "/usr/bin/env" || parts[0] == "/bin/env" || parts[0] == "env" {
		return parseEnvShebang(parts[1:])
	}

	return ShebangInfo{
		Interpreter: parts[0],
		Args:        parts[1:],
		Found:       true,
	}
}

// IsShellInterpreter returns true if the interpreter is a POSIX-compatible shell
// that can potentially be handled by the virtual runtime (mvdan/sh).
// Note: Even for shell interpreters, the virtual runtime uses mvdan/sh,
// so shell-specific features may not be fully supported.
func IsShellInterpreter(interpreter string) bool {
	base := filepath.Base(interpreter)
	// Handle Windows executable extensions
	base = strings.TrimSuffix(base, ".exe")
	return shellInterpreters[base]
}

// GetExtensionForInterpreter returns the typical file extension for an interpreter.
// Returns empty string if the interpreter is not recognized.
func GetExtensionForInterpreter(interpreter string) string {
	base := filepath.Base(interpreter)
	// Handle Windows executable extensions
	base = strings.TrimSuffix(base, ".exe")
	if ext, ok := interpreterExtensions[base]; ok {
		return ext
	}
	return ""
}

// ResolveInterpreter resolves the effective interpreter for a RuntimeConfig.
// If the interpreter field is empty, it defaults to "auto".
// If "auto" (or empty), it parses the shebang from the script content.
// Otherwise, it parses the explicit interpreter string.
//
// Parameters:
//   - interpreter: the RuntimeConfig.Interpreter value (may be empty, "auto", or explicit)
//   - scriptContent: the resolved script content (needed for shebang parsing)
//
// Returns the parsed ShebangInfo. If Found is false, the caller should use
// the default shell-based execution.
func ResolveInterpreter(interpreter, scriptContent string) ShebangInfo {
	// Default to "auto" if empty
	effectiveInterpreter := interpreter
	if effectiveInterpreter == "" {
		effectiveInterpreter = InterpreterAuto
	}

	// Auto-detect from shebang
	if effectiveInterpreter == InterpreterAuto {
		return ParseShebang(scriptContent)
	}

	// Parse explicit interpreter string
	return ParseInterpreterString(effectiveInterpreter)
}
