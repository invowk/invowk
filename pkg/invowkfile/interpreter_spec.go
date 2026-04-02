// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	invalidInterpreterSpecErrMsg = "invalid interpreter spec"
	unsafeInterpreterSpecErrMsg  = "unsafe interpreter spec"

	// shellMetachars are characters that must not appear in interpreter specs.
	// Their presence indicates injection attempts or misconfigured values.
	shellMetachars = ";|&`$(){}><\n"
)

var (
	// ErrInvalidInterpreterSpec is the sentinel error wrapped by InvalidInterpreterSpecError.
	ErrInvalidInterpreterSpec = errors.New(invalidInterpreterSpecErrMsg)
	// ErrUnsafeInterpreterSpec is the sentinel error for interpreter specs containing
	// shell metacharacters or unknown interpreter names (SC-08).
	ErrUnsafeInterpreterSpec = errors.New(unsafeInterpreterSpecErrMsg)

	// knownInterpreters is the allowlist of interpreter base names accepted by Validate().
	// This prevents arbitrary binary execution via module interpreter specs (SC-08).
	// Names are matched after stripping directory prefix and .exe suffix.
	knownInterpreters = map[string]bool{
		// POSIX shells
		"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true, "ksh": true, "mksh": true,
		// Python
		"python": true, "python3": true, "python2": true,
		// JavaScript runtimes
		"node": true, "deno": true, "bun": true,
		// Other scripting languages
		"ruby": true, "perl": true, "php": true, "lua": true,
		// Data science / stats
		"Rscript": true,
		// Windows shells
		"pwsh": true, "powershell": true, "cmd": true,
	}
)

type (
	// InterpreterSpec represents a command interpreter specification in runtime config.
	// The zero value ("") is valid and means "auto" (detect from shebang).
	// Non-zero values must not be whitespace-only.
	InterpreterSpec string

	// InvalidInterpreterSpecError is returned when an InterpreterSpec value is
	// non-empty but whitespace-only. It wraps ErrInvalidInterpreterSpec for errors.Is().
	InvalidInterpreterSpecError struct {
		Value InterpreterSpec
	}

	// UnsafeInterpreterSpecError is returned when an InterpreterSpec contains
	// shell metacharacters or references an interpreter not in the allowlist (SC-08).
	// It wraps ErrUnsafeInterpreterSpec for errors.Is().
	UnsafeInterpreterSpecError struct {
		Value  InterpreterSpec
		Reason string
	}
)

// String returns the string representation of the InterpreterSpec.
func (s InterpreterSpec) String() string { return string(s) }

// Validate returns nil if the InterpreterSpec is valid, or a validation error if not.
// The zero value ("") is valid (means "auto"). Non-zero values must not be
// whitespace-only, must not contain shell metacharacters, and must reference
// an interpreter from the known allowlist (SC-08).
func (s InterpreterSpec) Validate() error {
	if s == "" {
		return nil
	}

	trimmed := strings.TrimSpace(string(s))
	if trimmed == "" {
		return &InvalidInterpreterSpecError{Value: s}
	}

	// "auto" is a special keyword meaning "detect from shebang" — not an
	// actual interpreter name. It is handled by ParseInterpreterString.
	if trimmed == "auto" {
		return nil
	}

	// Metachar rejection precedes the allowlist check because injection patterns
	// (e.g., "python3; rm -rf /") would pass the allowlist on the first token.
	if strings.ContainsAny(trimmed, shellMetachars) {
		return &UnsafeInterpreterSpecError{
			Value:  s,
			Reason: "contains shell metacharacters",
		}
	}

	// SC-08: Validate interpreter name against the allowlist.
	parts := strings.Fields(trimmed)
	interpreter := parts[0]
	baseName := filepath.Base(interpreter)
	baseName = strings.TrimSuffix(baseName, ".exe")

	// Bare "env" without a full path enables PATH hijacking — require
	// /usr/bin/env or /bin/env instead.
	if baseName == "env" {
		if interpreter != "/usr/bin/env" && interpreter != "/bin/env" {
			return &UnsafeInterpreterSpecError{
				Value:  s,
				Reason: "bare 'env' requires full path (/usr/bin/env or /bin/env)",
			}
		}
		// For env-prefixed specs, validate the actual interpreter (second word).
		if len(parts) < 2 {
			return &UnsafeInterpreterSpecError{
				Value:  s,
				Reason: "env requires an interpreter argument",
			}
		}
		baseName = filepath.Base(parts[1])
		baseName = strings.TrimSuffix(baseName, ".exe")
	}

	if !knownInterpreters[baseName] {
		return &UnsafeInterpreterSpecError{
			Value:  s,
			Reason: fmt.Sprintf("interpreter %q not in allowlist", baseName),
		}
	}

	return nil
}

// Error implements the error interface for InvalidInterpreterSpecError.
func (e *InvalidInterpreterSpecError) Error() string {
	return fmt.Sprintf("invalid interpreter spec %q: non-empty value must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidInterpreterSpec for errors.Is() compatibility.
func (e *InvalidInterpreterSpecError) Unwrap() error { return ErrInvalidInterpreterSpec }

// Error implements the error interface for UnsafeInterpreterSpecError.
func (e *UnsafeInterpreterSpecError) Error() string {
	return fmt.Sprintf("unsafe interpreter spec %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrUnsafeInterpreterSpec for errors.Is() compatibility.
func (e *UnsafeInterpreterSpecError) Unwrap() error { return ErrUnsafeInterpreterSpec }
