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
	// ErrInvalidToolDependency is the sentinel error wrapped by InvalidToolDependencyError.
	ErrInvalidToolDependency = errors.New("invalid tool dependency")
	// ErrInvalidCommandDependency is the sentinel error wrapped by InvalidCommandDependencyError.
	ErrInvalidCommandDependency = errors.New("invalid command dependency")
	// ErrInvalidCapabilityDependency is the sentinel error wrapped by InvalidCapabilityDependencyError.
	ErrInvalidCapabilityDependency = errors.New("invalid capability dependency")
	// ErrInvalidEnvVarCheck is the sentinel error wrapped by InvalidEnvVarCheckError.
	ErrInvalidEnvVarCheck = errors.New("invalid env var check")
	// ErrInvalidEnvVarDependency is the sentinel error wrapped by InvalidEnvVarDependencyError.
	ErrInvalidEnvVarDependency = errors.New("invalid env var dependency")
	// ErrInvalidFilepathDependency is the sentinel error wrapped by InvalidFilepathDependencyError.
	ErrInvalidFilepathDependency = errors.New("invalid filepath dependency")
	// ErrInvalidCustomCheck is the sentinel error wrapped by InvalidCustomCheckError.
	ErrInvalidCustomCheck = errors.New("invalid custom check")
	// ErrInvalidCustomCheckDependency is the sentinel error wrapped by InvalidCustomCheckDependencyError.
	ErrInvalidCustomCheckDependency = errors.New("invalid custom check dependency")
	// ErrInvalidDependsOn is the sentinel error wrapped by InvalidDependsOnError.
	ErrInvalidDependsOn = errors.New("invalid depends_on")
	// ErrMissingDependencyAlternatives is returned when an OR dependency has no alternatives.
	ErrMissingDependencyAlternatives = errors.New("dependency alternatives must contain at least one item")
	// ErrMixedCustomCheckDependency is returned when a custom check dependency
	// combines direct check fields with an alternatives list.
	ErrMixedCustomCheckDependency = errors.New("custom check dependency must use either direct fields or alternatives")
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

	// InvalidToolDependencyError is returned when a ToolDependency has invalid fields.
	InvalidToolDependencyError struct{ FieldErrors []error }
	// InvalidCommandDependencyError is returned when a CommandDependency has invalid fields.
	InvalidCommandDependencyError struct{ FieldErrors []error }
	// InvalidCapabilityDependencyError is returned when a CapabilityDependency has invalid fields.
	InvalidCapabilityDependencyError struct{ FieldErrors []error }
	// InvalidEnvVarCheckError is returned when an EnvVarCheck has invalid fields.
	InvalidEnvVarCheckError struct{ FieldErrors []error }
	// InvalidEnvVarDependencyError is returned when an EnvVarDependency has invalid fields.
	InvalidEnvVarDependencyError struct{ FieldErrors []error }
	// InvalidFilepathDependencyError is returned when a FilepathDependency has invalid fields.
	InvalidFilepathDependencyError struct{ FieldErrors []error }
	// InvalidCustomCheckError is returned when a CustomCheck has invalid fields.
	InvalidCustomCheckError struct{ FieldErrors []error }
	// InvalidCustomCheckDependencyError is returned when a CustomCheckDependency has invalid fields.
	InvalidCustomCheckDependencyError struct{ FieldErrors []error }
	// InvalidDependsOnError is returned when a DependsOn has invalid fields.
	InvalidDependsOnError struct{ FieldErrors []error }

	//goplint:validate-all
	//
	// ToolDependency represents a tool/binary that must be available in PATH
	ToolDependency struct {
		// Alternatives is a list of binary names where any match satisfies the dependency
		// If any of the provided tools is found in PATH, the validation succeeds (early return).
		// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
		Alternatives []BinaryName `json:"alternatives"`
	}

	//goplint:validate-all
	//
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

	//goplint:validate-all
	//
	// CustomCheckDependency represents a custom check dependency that can be either:
	// - A single CustomCheck (direct check with name, check_script, etc.)
	// - An alternatives list of CustomChecks (OR semantics with early return)
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
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

	//goplint:validate-all
	//
	// CommandDependency represents another invowk command that must be discoverable.
	CommandDependency struct {
		// Alternatives is a list of command names where any match satisfies the dependency.
		// If any of the provided commands is discoverable, the dependency is satisfied (early return).
		// This allows specifying alternative commands (e.g., ["build-debug", "build-release"]).
		Alternatives []CommandName `json:"alternatives"`
	}

	//goplint:validate-all
	//
	// CapabilityDependency represents a system capability that must be available
	CapabilityDependency struct {
		// Alternatives is a list of capability identifiers where any match satisfies the dependency
		// If any of the provided capabilities is available, the validation succeeds (early return).
		// Available capabilities: "local-area-network", "internet", "containers", "tty"
		Alternatives []CapabilityName `json:"alternatives"`
	}

	//goplint:validate-all
	//
	// EnvVarCheck represents a single environment variable check
	EnvVarCheck struct {
		// Name is the environment variable name to check (required, non-empty)
		// The check verifies that this env var exists in the user's environment
		Name EnvVarName `json:"name"`
		// Validation is a regex pattern to validate the env var value (optional)
		// If specified, the env var must exist AND its value must match this pattern
		Validation RegexPattern `json:"validation,omitempty"`
	}

	//goplint:validate-all
	//
	// EnvVarDependency represents an environment variable dependency with alternatives
	EnvVarDependency struct {
		// Alternatives is a list of env var checks where any match satisfies the dependency
		// If any of the provided env vars exists (and passes validation if specified), the dependency is satisfied
		// This allows specifying multiple possible env vars (e.g., ["AWS_ACCESS_KEY_ID", "AWS_PROFILE"])
		Alternatives []EnvVarCheck `json:"alternatives"`
	}

	//goplint:validate-all
	//
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

	//goplint:validate-all
	//
	// DependsOn defines the dependencies for a command
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
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

// Validate returns nil if the BinaryName is valid, or a validation error if not.
// A valid BinaryName must be non-empty, not whitespace-only, and must not contain path separators.
//
//goplint:nonzero
func (b BinaryName) Validate() error {
	s := string(b)
	if strings.TrimSpace(s) == "" {
		return &InvalidBinaryNameError{Value: b, Reason: "must not be empty or whitespace-only"}
	}
	if strings.ContainsAny(s, "/\\") {
		return &InvalidBinaryNameError{Value: b, Reason: "must not contain path separators"}
	}
	return nil
}

// String returns the string representation of the BinaryName.
func (b BinaryName) String() string { return string(b) }

// String returns the string representation of the CheckName.
func (c CheckName) String() string { return string(c) }

// Validate returns nil if the CheckName is valid, or a validation error if not.
// A valid CheckName is non-empty and not whitespace-only.
//
//goplint:nonzero
func (c CheckName) Validate() error {
	if strings.TrimSpace(string(c)) == "" {
		return &InvalidCheckNameError{Value: c}
	}
	return nil
}

// Error implements the error interface for InvalidCheckNameError.
func (e *InvalidCheckNameError) Error() string {
	return fmt.Sprintf("invalid check name %q: must be non-empty and not whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidCheckName for errors.Is() compatibility.
func (e *InvalidCheckNameError) Unwrap() error { return ErrInvalidCheckName }

// String returns the string representation of the ScriptContent.
func (s ScriptContent) String() string { return string(s) }

// Validate returns nil if the ScriptContent is valid, or a validation error if not.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (s ScriptContent) Validate() error {
	if s == "" {
		return nil
	}
	if strings.TrimSpace(string(s)) == "" {
		return &InvalidScriptContentError{Value: s}
	}
	return nil
}

// Error implements the error interface for InvalidScriptContentError.
func (e *InvalidScriptContentError) Error() string {
	return fmt.Sprintf("invalid script content: non-empty value must not be whitespace-only (got %q)", e.Value)
}

// Unwrap returns ErrInvalidScriptContent for errors.Is() compatibility.
func (e *InvalidScriptContentError) Unwrap() error { return ErrInvalidScriptContent }

// Validate returns nil if the ToolDependency has valid fields.
func (t ToolDependency) Validate() error {
	var errs []error
	if len(t.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, b := range t.Alternatives {
		if err := b.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidToolDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the CommandDependency has valid fields.
func (c CommandDependency) Validate() error {
	var errs []error
	if len(c.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, n := range c.Alternatives {
		if err := n.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCommandDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the CapabilityDependency has valid fields.
func (c CapabilityDependency) Validate() error {
	var errs []error
	if len(c.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, n := range c.Alternatives {
		if err := n.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCapabilityDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the EnvVarCheck has valid fields.
// Delegates to Name.Validate() (nonzero) and Validation.Validate() (zero-valid).
func (e EnvVarCheck) Validate() error {
	var errs []error
	if err := e.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := e.Validation.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidEnvVarCheckError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the EnvVarDependency has valid fields.
func (e EnvVarDependency) Validate() error {
	var errs []error
	if len(e.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, c := range e.Alternatives {
		if err := c.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidEnvVarDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the FilepathDependency has valid fields.
func (f FilepathDependency) Validate() error {
	var errs []error
	if len(f.Alternatives) == 0 {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	for _, p := range f.Alternatives {
		if err := p.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidFilepathDependencyError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the CustomCheck has valid fields.
// Delegates to Name.Validate() (nonzero), CheckScript.Validate() (required),
// ExpectedCode.Validate() (when non-nil), and ExpectedOutput.Validate() (zero-valid).
func (c CustomCheck) Validate() error {
	var errs []error
	if err := c.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.CheckScript == "" {
		errs = append(errs, &InvalidScriptContentError{Value: c.CheckScript})
	} else if err := c.CheckScript.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.ExpectedCode != nil {
		if err := c.ExpectedCode.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.ExpectedOutput.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidCustomCheckError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the CustomCheckDependency has valid fields.
// When Alternatives is set, delegates to each CustomCheck.Validate().
// Otherwise, validates the direct check fields.
func (c CustomCheckDependency) Validate() error {
	var errs []error

	if len(c.Alternatives) > 0 {
		if c.hasDirectFields() {
			errs = append(errs, ErrMixedCustomCheckDependency)
		}
		for i := range c.Alternatives {
			appendFieldError(&errs, c.Alternatives[i].Validate())
		}
		if len(errs) > 0 {
			return &InvalidCustomCheckDependencyError{FieldErrors: errs}
		}
		return nil
	}

	if c.Name == "" && c.CheckScript == "" {
		errs = append(errs, ErrMissingDependencyAlternatives)
	}
	appendFieldError(&errs, c.Name.Validate())
	if c.CheckScript == "" {
		appendFieldError(&errs, &InvalidScriptContentError{Value: c.CheckScript})
	} else {
		appendFieldError(&errs, c.CheckScript.Validate())
	}
	if c.ExpectedCode != nil {
		appendFieldError(&errs, c.ExpectedCode.Validate())
	}
	if c.ExpectedOutput != "" {
		appendFieldError(&errs, c.ExpectedOutput.Validate())
	}
	if len(errs) > 0 {
		return &InvalidCustomCheckDependencyError{FieldErrors: errs}
	}
	return nil
}

func (c CustomCheckDependency) hasDirectFields() bool {
	return c.Name != "" || c.CheckScript != "" || c.ExpectedCode != nil || c.ExpectedOutput != ""
}

// Validate returns nil if the DependsOn has valid fields.
// Delegates to Validate() on all dependency slices.
func (d DependsOn) Validate() error {
	var errs []error
	appendEachValidation(&errs, d.Tools)
	appendEachValidation(&errs, d.Commands)
	appendEachValidation(&errs, d.Filepaths)
	appendEachValidation(&errs, d.Capabilities)
	appendEachValidation(&errs, d.CustomChecks)
	appendEachValidation(&errs, d.EnvVars)
	if len(errs) > 0 {
		return &InvalidDependsOnError{FieldErrors: errs}
	}
	return nil
}

// Error/Unwrap implementations for dependency error types.

func (e *InvalidToolDependencyError) Error() string {
	return types.FormatFieldErrors("tool dependency", e.FieldErrors)
}

func (e *InvalidToolDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidToolDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidCommandDependencyError) Error() string {
	return types.FormatFieldErrors("command dependency", e.FieldErrors)
}

func (e *InvalidCommandDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidCommandDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidCapabilityDependencyError) Error() string {
	return types.FormatFieldErrors("capability dependency", e.FieldErrors)
}

func (e *InvalidCapabilityDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidCapabilityDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidEnvVarCheckError) Error() string {
	return types.FormatFieldErrors("env var check", e.FieldErrors)
}
func (e *InvalidEnvVarCheckError) Unwrap() error { return ErrInvalidEnvVarCheck }

func (e *InvalidEnvVarDependencyError) Error() string {
	return types.FormatFieldErrors("env var dependency", e.FieldErrors)
}

func (e *InvalidEnvVarDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidEnvVarDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidFilepathDependencyError) Error() string {
	return types.FormatFieldErrors("filepath dependency", e.FieldErrors)
}

func (e *InvalidFilepathDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidFilepathDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidCustomCheckError) Error() string {
	return types.FormatFieldErrors("custom check", e.FieldErrors)
}

func (e *InvalidCustomCheckError) Unwrap() error {
	return errors.Join(ErrInvalidCustomCheck, errors.Join(e.FieldErrors...))
}

func (e *InvalidCustomCheckDependencyError) Error() string {
	return types.FormatFieldErrors("custom check dependency", e.FieldErrors)
}

func (e *InvalidCustomCheckDependencyError) Unwrap() error {
	return errors.Join(ErrInvalidCustomCheckDependency, errors.Join(e.FieldErrors...))
}

func (e *InvalidDependsOnError) Error() string {
	return types.FormatFieldErrors("depends_on", e.FieldErrors)
}

func (e *InvalidDependsOnError) Unwrap() error {
	return errors.Join(ErrInvalidDependsOn, errors.Join(e.FieldErrors...))
}

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
