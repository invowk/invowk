// SPDX-License-Identifier: EPL-2.0

package invkfile

type (
	// ToolDependency represents a tool/binary that must be available in PATH
	ToolDependency struct {
		// Alternatives is a list of binary names where any match satisfies the dependency
		// If any of the provided tools is found in PATH, the validation succeeds (early return).
		// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
		Alternatives []string `json:"alternatives"`
	}

	// CustomCheck represents a custom validation script to verify system requirements
	CustomCheck struct {
		// Name is an identifier for this check (used for error reporting)
		Name string `json:"name"`
		// CheckScript is the script to execute for validation
		CheckScript string `json:"check_script"`
		// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
		ExpectedCode *int `json:"expected_code,omitempty"`
		// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
		ExpectedOutput string `json:"expected_output,omitempty"`
	}

	// CustomCheckDependency represents a custom check dependency that can be either:
	// - A single CustomCheck (direct check with name, check_script, etc.)
	// - An alternatives list of CustomChecks (OR semantics with early return)
	CustomCheckDependency struct {
		// Direct check fields (used when this is a single check)
		// Name is an identifier for this check (used for error reporting)
		Name string `json:"name,omitempty"`
		// CheckScript is the script to execute for validation
		CheckScript string `json:"check_script,omitempty"`
		// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
		ExpectedCode *int `json:"expected_code,omitempty"`
		// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
		ExpectedOutput string `json:"expected_output,omitempty"`

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
		Alternatives []string `json:"alternatives"`
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
		Name string `json:"name"`
		// Validation is a regex pattern to validate the env var value (optional)
		// If specified, the env var must exist AND its value must match this pattern
		Validation string `json:"validation,omitempty"`
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
		Alternatives []string `json:"alternatives"`
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
		// Commands lists invowk commands that must be discoverable for this command to run (invkfile field: 'cmds')
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

// MergeDependsOn merges command-level and implementation-level dependencies
// Implementation-level dependencies are added to command-level dependencies
// Returns a new DependsOn struct with combined dependencies
func MergeDependsOn(cmdDeps, scriptDeps *DependsOn) *DependsOn {
	return MergeDependsOnAll(nil, cmdDeps, scriptDeps)
}

// MergeDependsOnAll merges root-level, command-level, and implementation-level dependencies
// Dependencies are combined in order: root -> command -> implementation
// Returns a new DependsOn struct with combined dependencies
func MergeDependsOnAll(rootDeps, cmdDeps, implDeps *DependsOn) *DependsOn {
	if rootDeps == nil && cmdDeps == nil && implDeps == nil {
		return nil
	}

	merged := &DependsOn{
		Tools:        make([]ToolDependency, 0),
		Commands:     make([]CommandDependency, 0),
		Filepaths:    make([]FilepathDependency, 0),
		Capabilities: make([]CapabilityDependency, 0),
		CustomChecks: make([]CustomCheckDependency, 0),
		EnvVars:      make([]EnvVarDependency, 0),
	}

	// Add root-level dependencies first (lowest priority)
	if rootDeps != nil {
		merged.Tools = append(merged.Tools, rootDeps.Tools...)
		merged.Commands = append(merged.Commands, rootDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, rootDeps.Filepaths...)
		merged.Capabilities = append(merged.Capabilities, rootDeps.Capabilities...)
		merged.CustomChecks = append(merged.CustomChecks, rootDeps.CustomChecks...)
		merged.EnvVars = append(merged.EnvVars, rootDeps.EnvVars...)
	}

	// Add command-level dependencies
	if cmdDeps != nil {
		merged.Tools = append(merged.Tools, cmdDeps.Tools...)
		merged.Commands = append(merged.Commands, cmdDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, cmdDeps.Filepaths...)
		merged.Capabilities = append(merged.Capabilities, cmdDeps.Capabilities...)
		merged.CustomChecks = append(merged.CustomChecks, cmdDeps.CustomChecks...)
		merged.EnvVars = append(merged.EnvVars, cmdDeps.EnvVars...)
	}

	// Add implementation-level dependencies
	if implDeps != nil {
		merged.Tools = append(merged.Tools, implDeps.Tools...)
		merged.Commands = append(merged.Commands, implDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, implDeps.Filepaths...)
		merged.Capabilities = append(merged.Capabilities, implDeps.Capabilities...)
		merged.CustomChecks = append(merged.CustomChecks, implDeps.CustomChecks...)
		merged.EnvVars = append(merged.EnvVars, implDeps.EnvVars...)
	}

	// Return nil if no dependencies after merging
	if len(merged.Tools) == 0 && len(merged.Commands) == 0 && len(merged.Filepaths) == 0 && len(merged.Capabilities) == 0 && len(merged.CustomChecks) == 0 && len(merged.EnvVars) == 0 {
		return nil
	}

	return merged
}
