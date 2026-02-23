// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidBinaryName is the sentinel error wrapped by InvalidBinaryNameError.
	ErrInvalidBinaryName = errors.New("invalid binary name")
	// ErrInvalidCheckName is the sentinel error wrapped by InvalidCheckNameError.
	ErrInvalidCheckName = errors.New("invalid check name")
	// ErrInvalidScriptContent is the sentinel error wrapped by InvalidScriptContentError.
	ErrInvalidScriptContent = errors.New("invalid script content")
)

type (
	// BinaryName represents the name of an executable binary expected in PATH.
	// Must be non-empty and must not contain path separators (/ or \).
	BinaryName string

	// InvalidBinaryNameError is returned when a BinaryName value is invalid.
	// It wraps ErrInvalidBinaryName for errors.Is() compatibility.
	InvalidBinaryNameError struct {
		Value  BinaryName
		Reason string
	}

	// CheckName identifies a custom check (used for error reporting).
	// Must be non-empty and not whitespace-only.
	CheckName string

	// InvalidCheckNameError is returned when a CheckName value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidCheckName for errors.Is().
	InvalidCheckNameError struct {
		Value CheckName
	}

	// ScriptContent holds inline script source code or a script file path.
	// The zero value ("") is valid. Non-zero values must not be whitespace-only.
	ScriptContent string

	// InvalidScriptContentError is returned when a ScriptContent value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidScriptContent for errors.Is().
	InvalidScriptContentError struct {
		Value ScriptContent
	}

	// ToolDependency represents a tool/binary that must be available in PATH
	ToolDependency struct {
		// Alternatives is a list of binary names where any match satisfies the dependency
		// If any of the provided tools is found in PATH, the validation succeeds (early return).
		// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
		Alternatives []BinaryName `json:"alternatives"`
	}

	// CustomCheck represents a custom validation script to verify system requirements
	CustomCheck struct {
		// Name is an identifier for this check (used for error reporting)
		Name CheckName `json:"name"`
		// CheckScript is the script to execute for validation
		CheckScript ScriptContent `json:"check_script"`
		// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
		ExpectedCode *types.ExitCode `json:"expected_code,omitempty"`
		// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
		ExpectedOutput RegexPattern `json:"expected_output,omitempty"`
	}

	// CustomCheckDependency represents a custom check dependency that can be either:
	// - A single CustomCheck (direct check with name, check_script, etc.)
	// - An alternatives list of CustomChecks (OR semantics with early return)
	CustomCheckDependency struct {
		// Direct check fields (used when this is a single check)
		// Name is an identifier for this check (used for error reporting)
		Name CheckName `json:"name,omitempty"`
		// CheckScript is the script to execute for validation
		CheckScript ScriptContent `json:"check_script,omitempty"`
		// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
		ExpectedCode *types.ExitCode `json:"expected_code,omitempty"`
		// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
		ExpectedOutput RegexPattern `json:"expected_output,omitempty"`

		// Alternatives is a list of custom checks where any passing check satisfies the dependency
		// If any of the provided checks passes, the validation succeeds (early return).
		// When Alternatives is set, the direct check fields above are ignored.
		Alternatives []CustomCheck `json:"alternatives,omitempty"`
	}

	// CommandDependency represents another invowk command that must be discoverable.
	CommandDependency struct {
		// Alternatives is a list of command names where any match satisfies the dependency.
		// If any of the provided commands is discoverable, the dependency is satisfied (early return).
		// This allows specifying alternative commands (e.g., ["build-debug", "build-release"]).
		Alternatives []CommandName `json:"alternatives"`
	}

	// CapabilityDependency represents a system capability that must be available
	CapabilityDependency struct {
		// Alternatives is a list of capability identifiers where any match satisfies the dependency
		// If any of the provided capabilities is available, the validation succeeds (early return).
		// Available capabilities: "local-area-network", "internet", "containers", "tty"
		Alternatives []CapabilityName `json:"alternatives"`
	}

	// EnvVarCheck represents a single environment variable check
	EnvVarCheck struct {
		// Name is the environment variable name to check (required, non-empty)
		// The check verifies that this env var exists in the user's environment
		Name EnvVarName `json:"name"`
		// Validation is a regex pattern to validate the env var value (optional)
		// If specified, the env var must exist AND its value must match this pattern
		Validation RegexPattern `json:"validation,omitempty"`
	}

	// EnvVarDependency represents an environment variable dependency with alternatives
	EnvVarDependency struct {
		// Alternatives is a list of env var checks where any match satisfies the dependency
		// If any of the provided env vars exists (and passes validation if specified), the dependency is satisfied
		// This allows specifying multiple possible env vars (e.g., ["AWS_ACCESS_KEY_ID", "AWS_PROFILE"])
		Alternatives []EnvVarCheck `json:"alternatives"`
	}

	// FilepathDependency represents a file or directory that must exist
	FilepathDependency struct {
		// Alternatives is a list of file or directory paths where any match satisfies the dependency
		// If any of the provided paths exists and satisfies the permission requirements,
		// the validation succeeds (early return). This allows specifying multiple
		// possible locations for a file (e.g., different paths on different systems).
		Alternatives []FilesystemPath `json:"alternatives"`
		// Readable checks if the path is readable
		Readable bool `json:"readable,omitempty"`
		// Writable checks if the path is writable
		Writable bool `json:"writable,omitempty"`
		// Executable checks if the path is executable
		Executable bool `json:"executable,omitempty"`
	}

	// DependsOn defines the dependencies for a command
	DependsOn struct {
		// Tools lists binaries that must be available in PATH before running
		// Uses OR semantics: if any alternative in the list is found, the dependency is satisfied
		Tools []ToolDependency `json:"tools,omitempty"`
		// Commands lists invowk commands that must be discoverable for this command to run (invowkfile field: 'cmds')
		// Uses OR semantics: if any alternative in the list is discoverable, the dependency is satisfied
		Commands []CommandDependency `json:"cmds,omitempty"`
		// Filepaths lists files or directories that must exist before running
		// Uses OR semantics: if any alternative path exists, the dependency is satisfied
		Filepaths []FilepathDependency `json:"filepaths,omitempty"`
		// Capabilities lists system capabilities that must be available before running
		// Uses OR semantics: if any alternative capability is available, the dependency is satisfied
		Capabilities []CapabilityDependency `json:"capabilities,omitempty"`
		// CustomChecks lists custom validation scripts to verify system requirements
		// Each entry can be a single check or an alternatives list (OR semantics)
		CustomChecks []CustomCheckDependency `json:"custom_checks,omitempty"`
		// EnvVars lists environment variables that must exist before running
		// Uses OR semantics: if any alternative env var exists (and passes validation), the dependency is satisfied
		// IMPORTANT: Validated against the user's environment BEFORE invowk sets command-level env vars
		EnvVars []EnvVarDependency `json:"env_vars,omitempty"`
	}
)

// IsEmpty returns true if this DependsOn has no dependencies of any type.
func (d *DependsOn) IsEmpty() bool {
	return len(d.Tools) == 0 && len(d.Commands) == 0 && len(d.Filepaths) == 0 &&
		len(d.Capabilities) == 0 && len(d.CustomChecks) == 0 && len(d.EnvVars) == 0
}

// IsAlternatives returns true if this dependency uses the alternatives format
func (c *CustomCheckDependency) IsAlternatives() bool {
	return len(c.Alternatives) > 0
}

// GetChecks returns the list of CustomCheck to validate.
// If Alternatives is set, returns those; otherwise returns a single-element list with the direct check.
func (c *CustomCheckDependency) GetChecks() []CustomCheck {
	if c.IsAlternatives() {
		return c.Alternatives
	}
	// Return as a single-element list
	return []CustomCheck{{
		Name:           c.Name,
		CheckScript:    c.CheckScript,
		ExpectedCode:   c.ExpectedCode,
		ExpectedOutput: c.ExpectedOutput,
	}}
}

// MergeDependsOnAll merges root-level, command-level, and implementation-level dependencies.
// Dependencies are combined in order: root -> command -> implementation.
// Returns a new DependsOn struct with combined dependencies.
func MergeDependsOnAll(rootDeps, cmdDeps, implDeps *DependsOn) *DependsOn {
	if rootDeps == nil && cmdDeps == nil && implDeps == nil {
		return nil
	}

	merged := &DependsOn{}

	// Append in declaration order: root → command → implementation.
	merged.appendFrom(rootDeps)
	merged.appendFrom(cmdDeps)
	merged.appendFrom(implDeps)

	// Return nil if no dependencies after merging
	if merged.IsEmpty() {
		return nil
	}

	return merged
}

// Error implements the error interface for InvalidBinaryNameError.
func (e *InvalidBinaryNameError) Error() string {
	return fmt.Sprintf("invalid binary name %q: %s", e.Value, e.Reason)
}

// Unwrap returns ErrInvalidBinaryName for errors.Is() compatibility.
func (e *InvalidBinaryNameError) Unwrap() error { return ErrInvalidBinaryName }

// IsValid returns whether the BinaryName is valid.
// A valid BinaryName must be non-empty and must not contain path separators.
func (b BinaryName) IsValid() (bool, []error) {
	s := string(b)
	if s == "" {
		return false, []error{&InvalidBinaryNameError{Value: b, Reason: "must not be empty"}}
	}
	if strings.ContainsAny(s, "/\\") {
		return false, []error{&InvalidBinaryNameError{Value: b, Reason: "must not contain path separators"}}
	}
	return true, nil
}

// String returns the string representation of the BinaryName.
func (b BinaryName) String() string { return string(b) }

// String returns the string representation of the CheckName.
func (c CheckName) String() string { return string(c) }

// IsValid returns whether the CheckName is valid.
// A valid CheckName is non-empty and not whitespace-only.
func (c CheckName) IsValid() (bool, []error) {
	if strings.TrimSpace(string(c)) == "" {
		return false, []error{&InvalidCheckNameError{Value: c}}
	}
	return true, nil
}

// Error implements the error interface for InvalidCheckNameError.
func (e *InvalidCheckNameError) Error() string {
	return fmt.Sprintf("invalid check name %q: must be non-empty and not whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidCheckName for errors.Is() compatibility.
func (e *InvalidCheckNameError) Unwrap() error { return ErrInvalidCheckName }

// String returns the string representation of the ScriptContent.
func (s ScriptContent) String() string { return string(s) }

// IsValid returns whether the ScriptContent is valid.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (s ScriptContent) IsValid() (bool, []error) {
	if s == "" {
		return true, nil
	}
	if strings.TrimSpace(string(s)) == "" {
		return false, []error{&InvalidScriptContentError{Value: s}}
	}
	return true, nil
}

// Error implements the error interface for InvalidScriptContentError.
func (e *InvalidScriptContentError) Error() string {
	return fmt.Sprintf("invalid script content: non-empty value must not be whitespace-only (got %q)", e.Value)
}

// Unwrap returns ErrInvalidScriptContent for errors.Is() compatibility.
func (e *InvalidScriptContentError) Unwrap() error { return ErrInvalidScriptContent }

// appendFrom appends all dependency slices from src into d. Nil src is a no-op.
func (d *DependsOn) appendFrom(src *DependsOn) {
	if src == nil {
		return
	}
	d.Tools = append(d.Tools, src.Tools...)
	d.Commands = append(d.Commands, src.Commands...)
	d.Filepaths = append(d.Filepaths, src.Filepaths...)
	d.Capabilities = append(d.Capabilities, src.Capabilities...)
	d.CustomChecks = append(d.CustomChecks, src.CustomChecks...)
	d.EnvVars = append(d.EnvVars, src.EnvVars...)
}
