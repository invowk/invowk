// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// ErrInvalidEnvVarName is the sentinel error wrapped by InvalidEnvVarNameError.
	ErrInvalidEnvVarName = errors.New("invalid environment variable name")

	// envVarNameRegex validates environment variable names
	envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type (
	// EnvVarName represents an environment variable name.
	// A valid env var name starts with a letter or underscore, followed by
	// letters, digits, or underscores (matching POSIX conventions).
	EnvVarName string

	// InvalidEnvVarNameError is returned when an EnvVarName value is empty,
	// whitespace-only, or doesn't match the POSIX env var naming convention.
	InvalidEnvVarNameError struct {
		Value EnvVarName
	}

	// EnvConfig holds environment configuration for a command or implementation
	EnvConfig struct {
		// Files lists dotenv files to load (optional)
		// Files are loaded in order; later files override earlier ones.
		// Paths are relative to the invowkfile location (or module root for modules).
		// Files suffixed with '?' are optional and will not cause an error if missing.
		Files []DotenvFilePath `json:"files,omitempty"`
		// Vars contains environment variables as key-value pairs (optional)
		// These override values loaded from Files.
		Vars map[EnvVarName]string `json:"vars,omitempty"`
	}
)

// Error implements the error interface.
func (e *InvalidEnvVarNameError) Error() string {
	return fmt.Sprintf("invalid environment variable name %q (must match [A-Za-z_][A-Za-z0-9_]*)", e.Value)
}

// Unwrap returns ErrInvalidEnvVarName so callers can use errors.Is for programmatic detection.
func (e *InvalidEnvVarNameError) Unwrap() error { return ErrInvalidEnvVarName }

// Validate returns nil if the EnvVarName is a valid POSIX environment variable name,
// or a validation error if it is not.
//
//goplint:nonzero
func (n EnvVarName) Validate() error {
	s := string(n)
	if strings.TrimSpace(s) == "" || !envVarNameRegex.MatchString(s) {
		return &InvalidEnvVarNameError{Value: n}
	}
	return nil
}

// String returns the string representation of the EnvVarName.
func (n EnvVarName) String() string { return string(n) }

// GetFiles returns the files list, or an empty slice if EnvConfig is nil
func (e *EnvConfig) GetFiles() []DotenvFilePath {
	if e == nil {
		return nil
	}
	return e.Files
}

// GetVars returns the vars as a map[string]string, converting typed keys back
// to raw strings for compatibility with maps.Copy and exec.Cmd.Env consumers.
// Returns nil if EnvConfig is nil or Vars is empty.
func (e *EnvConfig) GetVars() map[string]string {
	if e == nil || e.Vars == nil {
		return nil
	}
	result := make(map[string]string, len(e.Vars))
	for k, v := range e.Vars {
		result[string(k)] = v
	}
	return result
}

// ValidateEnvVarName validates a single environment variable name.
// [CUE-REDUNDANT] For invowkfile parsing, this is also validated in CUE schema:
// env_inherit_allow?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]
// [GO-REQUIRED] This function is also called from CLI code (cmd_execute.go)
// to validate user-provided --ivk-env-var flags, which don't go through CUE.
// Therefore, this Go validation MUST be kept.
func ValidateEnvVarName(name string) error {
	return EnvVarName(name).Validate()
}

// FlagNameToEnvVar converts a flag name to an environment variable name.
// Example: "output-file" -> "INVOWK_FLAG_OUTPUT_FILE"
func FlagNameToEnvVar(name FlagName) string {
	return "INVOWK_FLAG_" + strings.ToUpper(strings.ReplaceAll(string(name), "-", "_"))
}

// ArgNameToEnvVar converts an argument name to an environment variable name.
// Example: "output-file" -> "INVOWK_ARG_OUTPUT_FILE"
func ArgNameToEnvVar(name ArgumentName) string {
	return "INVOWK_ARG_" + strings.ToUpper(strings.ReplaceAll(string(name), "-", "_"))
}
