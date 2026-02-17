// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"regexp"
	"strings"
)

// envVarNameRegex validates environment variable names
var envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// EnvConfig holds environment configuration for a command or implementation
type EnvConfig struct {
	// Files lists dotenv files to load (optional)
	// Files are loaded in order; later files override earlier ones.
	// Paths are relative to the invowkfile location (or module root for modules).
	// Files suffixed with '?' are optional and will not cause an error if missing.
	Files []string `json:"files,omitempty"`
	// Vars contains environment variables as key-value pairs (optional)
	// These override values loaded from Files.
	Vars map[string]string `json:"vars,omitempty"`
}

// GetFiles returns the files list, or an empty slice if EnvConfig is nil
func (e *EnvConfig) GetFiles() []string {
	if e == nil {
		return nil
	}
	return e.Files
}

// GetVars returns the vars map, or nil if EnvConfig is nil
func (e *EnvConfig) GetVars() map[string]string {
	if e == nil {
		return nil
	}
	return e.Vars
}

// ValidateEnvVarName validates a single environment variable name.
// [CUE-REDUNDANT] For invowkfile parsing, this is also validated in CUE schema:
// env_inherit_allow?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]
// [GO-REQUIRED] This function is also called from CLI code (cmd_execute.go)
// to validate user-provided --ivk-env-var flags, which don't go through CUE.
// Therefore, this Go validation MUST be kept.
func ValidateEnvVarName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}
	if !envVarNameRegex.MatchString(name) {
		return fmt.Errorf("invalid environment variable name %q", name)
	}
	return nil
}

// FlagNameToEnvVar converts a flag name to an environment variable name.
// Example: "output-file" -> "INVOWK_FLAG_OUTPUT_FILE"
func FlagNameToEnvVar(name string) string {
	return "INVOWK_FLAG_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// ArgNameToEnvVar converts an argument name to an environment variable name.
// Example: "output-file" -> "INVOWK_ARG_OUTPUT_FILE"
func ArgNameToEnvVar(name string) string {
	return "INVOWK_ARG_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}
