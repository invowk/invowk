// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"strings"
)

// validateDependsOn validates all dependency types in a DependsOn struct.
func validateDependsOn(deps *DependsOn, context, filePath string) error {
	if deps == nil {
		return nil
	}

	// Validate tool dependencies
	if err := validateToolDependencies(deps.Tools, context, filePath); err != nil {
		return err
	}

	// Validate command dependencies
	if err := validateCommandDependencies(deps.Commands, context, filePath); err != nil {
		return err
	}

	// Validate filepath dependencies
	if err := validateFilepathDependencies(deps.Filepaths, context, filePath); err != nil {
		return err
	}

	// Validate env var dependencies
	if err := validateEnvVarDependencies(deps.EnvVars, context, filePath); err != nil {
		return err
	}

	// custom_checks are validated separately (already integrated)
	return nil
}

// validateCustomChecks validates custom check dependencies for security and correctness.
// [CUE-VALIDATED] Basic structure and required fields are validated in CUE schema.
// [GO-ONLY] Security validations that require Go:
//   - String length limits (MaxNameLength, MaxScriptLength) for resource exhaustion prevention
//   - ReDoS pattern safety for expected_output regex patterns
func validateCustomChecks(checks []CustomCheckDependency, context, filePath string) error {
	for i, checkDep := range checks {
		// Get all checks (handles both direct and alternatives formats)
		for j, check := range checkDep.GetChecks() {
			// [GO-ONLY] Length limit validation - defense against resource exhaustion
			if check.Name != "" {
				if err := ValidateStringLength(check.Name, "custom_check name", MaxNameLength); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: %w in invkfile at %s", context, i+1, j+1, err, filePath)
				}
			}

			// [GO-ONLY] Script length limit - defense against resource exhaustion
			if check.CheckScript != "" {
				if err := ValidateStringLength(check.CheckScript, "check_script", MaxScriptLength); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: %w in invkfile at %s", context, i+1, j+1, err, filePath)
				}
			}

			// [GO-ONLY] ReDoS pattern safety - CUE cannot analyze regex complexity
			if check.ExpectedOutput != "" {
				if err := ValidateRegexPattern(check.ExpectedOutput); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: expected_output: %w in invkfile at %s", context, i+1, j+1, err, filePath)
				}
			}
		}
	}
	return nil
}

// validateEnvConfig validates environment configuration for security.
// It checks env file paths for traversal and env var names/values for validity.
// [GO-ONLY] Path traversal prevention for env.files requires filesystem operations.
// [GO-ONLY] Env var value length limits prevent resource exhaustion (not in CUE schema).
// [CUE-VALIDATED] Env var name format is validated in CUE but kept here as defense-in-depth.
func validateEnvConfig(env *EnvConfig, context, filePath string) error {
	if env == nil {
		return nil
	}

	// [GO-ONLY] Env file path validation - path traversal prevention requires filesystem operations
	for i, file := range env.Files {
		if err := ValidateEnvFilePath(file); err != nil {
			return fmt.Errorf("%s env.files[%d]: %w in invkfile at %s", context, i+1, err, filePath)
		}
	}

	// Validate env var names and values
	for key, value := range env.Vars {
		// Env var name validation is redundant with CUE but kept as defense-in-depth
		if err := ValidateEnvVarName(key); err != nil {
			return fmt.Errorf("%s env.vars key '%s': %w in invkfile at %s", context, key, err, filePath)
		}
		// [GO-ONLY] Value length limit - prevents resource exhaustion
		if len(value) > MaxEnvVarValueLength {
			return fmt.Errorf("%s env.vars['%s'] value too long (%d chars, max %d) in invkfile at %s",
				context, key, len(value), MaxEnvVarValueLength, filePath)
		}
	}

	return nil
}

// validateEnvVarDependencies validates env var dependencies for security.
// It checks env var names and validation regex patterns.
// [CUE-VALIDATED] Env var name format is in CUE: name: string & =~"^[A-Za-z_][A-Za-z0-9_]*$"
// [GO-ONLY] ReDoS pattern safety validation must be in Go (CUE can't analyze regex complexity).
func validateEnvVarDependencies(deps []EnvVarDependency, context, filePath string) error {
	for i, dep := range deps {
		for j, alt := range dep.Alternatives {
			// Env var name validation is redundant with CUE but kept as defense-in-depth
			name := strings.TrimSpace(alt.Name)
			if err := ValidateEnvVarName(name); err != nil {
				return fmt.Errorf("%s env_vars[%d].alternatives[%d].name: %w in invkfile at %s",
					context, i+1, j+1, err, filePath)
			}
			// [GO-ONLY] ReDoS pattern safety validation - CUE cannot analyze regex complexity
			if alt.Validation != "" {
				if err := ValidateRegexPattern(alt.Validation); err != nil {
					return fmt.Errorf("%s env_vars[%d].alternatives[%d].validation: %w in invkfile at %s",
						context, i+1, j+1, err, filePath)
				}
			}
		}
	}
	return nil
}

// validateToolDependencies validates tool dependency names.
// [CUE-VALIDATED] Tool name format is in CUE: alternatives: [...string & =~"^[a-zA-Z0-9][a-zA-Z0-9._+-]*$"]
// [GO-ONLY] Length limit (MaxNameLength) is Go-only for defense-in-depth.
func validateToolDependencies(deps []ToolDependency, context, filePath string) error {
	for i, dep := range deps {
		for j, alt := range dep.Alternatives {
			if err := ValidateToolName(alt); err != nil {
				return fmt.Errorf("%s tools[%d].alternatives[%d]: %w in invkfile at %s",
					context, i+1, j+1, err, filePath)
			}
		}
	}
	return nil
}

// validateCommandDependencies validates command dependency names.
// [CUE-VALIDATED] Command name format is in CUE: alternatives: [...string & =~"^[a-zA-Z][a-zA-Z0-9_ -]*$"]
// [GO-ONLY] Length limit (MaxNameLength) is Go-only for defense-in-depth.
func validateCommandDependencies(deps []CommandDependency, context, filePath string) error {
	for i, dep := range deps {
		for j, alt := range dep.Alternatives {
			if err := ValidateCommandDependencyName(alt); err != nil {
				return fmt.Errorf("%s cmds[%d].alternatives[%d]: %w in invkfile at %s",
					context, i+1, j+1, err, filePath)
			}
		}
	}
	return nil
}

// validateFilepathDependencies validates filepath dependency paths.
// [GO-ONLY] Filepath validation requires filesystem operations and cross-platform path handling.
// CUE validates basic format constraints but cannot check path security or existence.
func validateFilepathDependencies(deps []FilepathDependency, context, filePath string) error {
	for i, dep := range deps {
		if err := ValidateFilepathDependency(dep.Alternatives); err != nil {
			return fmt.Errorf("%s filepaths[%d]: %w in invkfile at %s",
				context, i+1, err, filePath)
		}
	}
	return nil
}
