// SPDX-License-Identifier: EPL-2.0

// Package invkfile provides interpreter detection and shebang parsing utilities.
package invkfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

// InterpreterAuto is the special value for automatic shebang detection.
// When interpreter is empty or set to "auto", invowk will parse the shebang
// from the script content to determine the interpreter.
const InterpreterAuto = "auto"

// ShebangInfo contains parsed shebang information from a script.
type ShebangInfo struct {
	// Interpreter is the interpreter path or command name (e.g., "/bin/bash", "python3")
	Interpreter string
	// Args contains additional arguments to pass to the interpreter (e.g., ["-u"] for python3 -u)
	Args []string
	// Found indicates whether a valid shebang was detected
	Found bool
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
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx != -1 {
		firstLine = content[:idx]
	}
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

// shellInterpreters maps shell interpreter base names to true.
// These interpreters are compatible with the virtual runtime (mvdan/sh).
var shellInterpreters = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true,
	"ash": true, "ksh": true, "mksh": true,
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

// interpreterExtensions maps interpreter base names to typical file extensions.
// Used when creating temporary script files to ensure proper syntax highlighting
// and interpreter behavior.
var interpreterExtensions = map[string]string{
	"python": ".py", "python3": ".py", "python2": ".py",
	"ruby": ".rb", "perl": ".pl", "node": ".js",
	"bash": ".sh", "sh": ".sh", "zsh": ".zsh",
	"fish": ".fish", "pwsh": ".ps1", "powershell": ".ps1",
	"php": ".php", "lua": ".lua", "Rscript": ".R",
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
func ResolveInterpreter(interpreter string, scriptContent string) ShebangInfo {
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
