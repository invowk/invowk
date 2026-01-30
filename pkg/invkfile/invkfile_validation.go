// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// flagNameRegex validates POSIX-compliant flag names
	flagNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

	// argNameRegex validates POSIX-compliant argument names
	argNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

// validate checks the invkfile for errors and applies defaults
func (inv *Invkfile) validate() error {
	if len(inv.Commands) == 0 {
		return fmt.Errorf("invkfile at %s has no commands defined (missing required 'cmds' list)", inv.FilePath)
	}

	// Validate root-level env configuration
	if err := validateEnvConfig(inv.Env, "root", inv.FilePath); err != nil {
		return err
	}

	// Validate root-level depends_on (all dependency types)
	if err := validateDependsOn(inv.DependsOn, "root", inv.FilePath); err != nil {
		return err
	}
	// Validate root-level custom_checks dependencies (security-specific checks)
	if inv.DependsOn != nil && len(inv.DependsOn.CustomChecks) > 0 {
		if err := validateCustomChecks(inv.DependsOn.CustomChecks, "root", inv.FilePath); err != nil {
			return err
		}
	}

	// Validate each command
	for i := range inv.Commands {
		if err := inv.validateCommand(&inv.Commands[i]); err != nil {
			return err
		}
	}

	return nil
}

// validateRuntimeConfig checks that a runtime configuration is valid.
// Note: Format validation (non-empty interpreter, valid env var names) is handled by the CUE schema.
// This function focuses on Go-only validations: cross-field logic, filesystem access, and security checks.
func validateRuntimeConfig(rt *RuntimeConfig, cmdName string, implIndex int) error {
	// [CUE-VALIDATED] Interpreter format validation is in CUE schema:
	// interpreter?: string & =~"^\\s*\\S.*$" (requires at least one non-whitespace char)

	// Validate env inherit mode and env var names
	if rt.EnvInheritMode != "" && !rt.EnvInheritMode.IsValid() {
		return fmt.Errorf("command '%s' implementation #%d: env_inherit_mode must be one of: none, allow, all", cmdName, implIndex)
	}
	for _, name := range rt.EnvInheritAllow {
		if err := ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_allow: %w", cmdName, implIndex, err)
		}
	}
	for _, name := range rt.EnvInheritDeny {
		if err := ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_deny: %w", cmdName, implIndex, err)
		}
	}

	// [GO-ONLY] Cross-field validation: Container-specific fields are only valid for container runtime.
	// CUE uses discriminated unions (#RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer)
	// which handle field presence at the type level. This Go validation provides clearer error messages
	// and catches any edge cases where the CUE type system might be bypassed.
	if rt.Name != RuntimeContainer {
		if rt.EnableHostSSH {
			return fmt.Errorf("command '%s' implementation #%d: enable_host_ssh is only valid for container runtime", cmdName, implIndex)
		}
		if rt.Containerfile != "" {
			return fmt.Errorf("command '%s' implementation #%d: containerfile is only valid for container runtime", cmdName, implIndex)
		}
		if rt.Image != "" {
			return fmt.Errorf("command '%s' implementation #%d: image is only valid for container runtime", cmdName, implIndex)
		}
		if len(rt.Volumes) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: volumes is only valid for container runtime", cmdName, implIndex)
		}
		if len(rt.Ports) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: ports is only valid for container runtime", cmdName, implIndex)
		}
	} else {
		// For container runtime, validate mutual exclusivity of containerfile and image
		if rt.Containerfile != "" && rt.Image != "" {
			return fmt.Errorf("command '%s' implementation #%d: containerfile and image are mutually exclusive - specify only one", cmdName, implIndex)
		}
		// At least one of containerfile or image must be specified for container runtime
		if rt.Containerfile == "" && rt.Image == "" {
			return fmt.Errorf("command '%s' implementation #%d: container runtime requires either containerfile or image to be specified", cmdName, implIndex)
		}
		// Validate container image name format
		if rt.Image != "" {
			if err := ValidateContainerImage(rt.Image); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: invalid image: %w", cmdName, implIndex, err)
			}
		}
		// Validate containerfile path for security (path traversal prevention)
		// Note: baseDir validation is done at parse time when FilePath is available
		if rt.Containerfile != "" {
			if len(rt.Containerfile) > MaxPathLength {
				return fmt.Errorf("command '%s' implementation #%d: containerfile path too long (%d chars, max %d)", cmdName, implIndex, len(rt.Containerfile), MaxPathLength)
			}
			if filepath.IsAbs(rt.Containerfile) {
				return fmt.Errorf("command '%s' implementation #%d: containerfile path must be relative, not absolute", cmdName, implIndex)
			}
			if strings.ContainsRune(rt.Containerfile, '\x00') {
				return fmt.Errorf("command '%s' implementation #%d: containerfile path contains null byte", cmdName, implIndex)
			}
		}
		// Validate volume mounts
		for i, vol := range rt.Volumes {
			if err := ValidateVolumeMount(vol); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: volume #%d: %w", cmdName, implIndex, i+1, err)
			}
		}
		// Validate port mappings
		for i, port := range rt.Ports {
			if err := ValidatePortMapping(port); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: port #%d: %w", cmdName, implIndex, i+1, err)
			}
		}
	}
	return nil
}

// validateCommand validates a single command
func (inv *Invkfile) validateCommand(cmd *Command) error {
	if cmd.Name == "" {
		return fmt.Errorf("command must have a name in invkfile at %s", inv.FilePath)
	}

	// Validate command name length
	if err := ValidateStringLength(cmd.Name, "command name", MaxNameLength); err != nil {
		return fmt.Errorf("command '%s': %w in invkfile at %s", cmd.Name, err, inv.FilePath)
	}

	// Validate description length
	if err := ValidateStringLength(cmd.Description, "description", MaxDescriptionLength); err != nil {
		return fmt.Errorf("command '%s': %w in invkfile at %s", cmd.Name, err, inv.FilePath)
	}

	// Validate command-level depends_on (all dependency types)
	if err := validateDependsOn(cmd.DependsOn, fmt.Sprintf("command '%s'", cmd.Name), inv.FilePath); err != nil {
		return err
	}
	// Validate command-level custom_checks dependencies (security-specific checks)
	if cmd.DependsOn != nil && len(cmd.DependsOn.CustomChecks) > 0 {
		if err := validateCustomChecks(cmd.DependsOn.CustomChecks, fmt.Sprintf("command '%s'", cmd.Name), inv.FilePath); err != nil {
			return err
		}
	}

	// Validate command-level env configuration
	if err := validateEnvConfig(cmd.Env, fmt.Sprintf("command '%s'", cmd.Name), inv.FilePath); err != nil {
		return err
	}

	if len(cmd.Implementations) == 0 {
		return fmt.Errorf("command '%s' must have at least one implementation in invkfile at %s", cmd.Name, inv.FilePath)
	}

	// Validate each implementation
	for i, impl := range cmd.Implementations {
		if impl.Script == "" {
			return fmt.Errorf("command '%s' implementation #%d must have a script in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate script length (only for inline scripts, not file paths)
		if !impl.IsScriptFile() {
			if err := ValidateStringLength(impl.Script, "script", MaxScriptLength); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: %w in invkfile at %s", cmd.Name, i+1, err, inv.FilePath)
			}
		}

		if len(impl.Runtimes) == 0 {
			return fmt.Errorf("command '%s' implementation #%d must have at least one runtime in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate each runtime config
		for j := range impl.Runtimes {
			if err := validateRuntimeConfig(&impl.Runtimes[j], cmd.Name, i+1); err != nil {
				return err
			}
			// Validate containerfile path traversal (requires base directory)
			rt := &impl.Runtimes[j]
			if rt.Name == RuntimeContainer && rt.Containerfile != "" {
				baseDir := filepath.Dir(inv.FilePath)
				if err := ValidateContainerfilePath(rt.Containerfile, baseDir); err != nil {
					return fmt.Errorf("command '%s' implementation #%d runtime #%d: %w in invkfile at %s", cmd.Name, i+1, j+1, err, inv.FilePath)
				}
			}
		}

		// Validate implementation-level depends_on (all dependency types)
		if err := validateDependsOn(impl.DependsOn, fmt.Sprintf("command '%s' implementation #%d", cmd.Name, i+1), inv.FilePath); err != nil {
			return err
		}
		// Validate implementation-level custom_checks dependencies (security-specific checks)
		if impl.DependsOn != nil && len(impl.DependsOn.CustomChecks) > 0 {
			if err := validateCustomChecks(impl.DependsOn.CustomChecks, fmt.Sprintf("command '%s' implementation #%d", cmd.Name, i+1), inv.FilePath); err != nil {
				return err
			}
		}

		// Validate implementation-level env configuration
		if err := validateEnvConfig(impl.Env, fmt.Sprintf("command '%s' implementation #%d", cmd.Name, i+1), inv.FilePath); err != nil {
			return err
		}
	}

	// Validate that there are no duplicate platform+runtime combinations
	if err := cmd.ValidateScripts(); err != nil {
		return err
	}

	// Validate flags
	if err := inv.validateFlags(cmd); err != nil {
		return err
	}

	// Validate args
	if err := inv.validateArgs(cmd); err != nil {
		return err
	}

	return nil
}

// validateFlags validates the flags for a command
func (inv *Invkfile) validateFlags(cmd *Command) error {
	seenNames := make(map[string]bool)
	seenShorts := make(map[string]bool)

	for i, flag := range cmd.Flags {
		// Validate name is not empty
		if flag.Name == "" {
			return fmt.Errorf("command '%s' flag #%d must have a name in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate flag name length
		if err := ValidateStringLength(flag.Name, "flag name", MaxNameLength); err != nil {
			return fmt.Errorf("command '%s' flag '%s': %w in invkfile at %s", cmd.Name, flag.Name, err, inv.FilePath)
		}

		// Validate name is POSIX-compliant
		if !flagNameRegex.MatchString(flag.Name) {
			return fmt.Errorf("command '%s' flag '%s' has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at %s", cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate description is not empty (after trimming whitespace)
		if strings.TrimSpace(flag.Description) == "" {
			return fmt.Errorf("command '%s' flag '%s' must have a non-empty description in invkfile at %s", cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate description length
		if err := ValidateStringLength(flag.Description, "flag description", MaxDescriptionLength); err != nil {
			return fmt.Errorf("command '%s' flag '%s': %w in invkfile at %s", cmd.Name, flag.Name, err, inv.FilePath)
		}

		// Check for duplicate flag names
		if seenNames[flag.Name] {
			return fmt.Errorf("command '%s' has duplicate flag name '%s' in invkfile at %s", cmd.Name, flag.Name, inv.FilePath)
		}
		seenNames[flag.Name] = true

		// Check for reserved flag names
		if flag.Name == "env-file" {
			return fmt.Errorf("command '%s' flag '%s': 'env-file' is a reserved system flag and cannot be used in invkfile at %s",
				cmd.Name, flag.Name, inv.FilePath)
		}
		if flag.Name == "env-var" {
			return fmt.Errorf("command '%s' flag '%s': 'env-var' is a reserved system flag and cannot be used in invkfile at %s",
				cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate type is valid (if specified)
		if flag.Type != "" && flag.Type != FlagTypeString && flag.Type != FlagTypeBool && flag.Type != FlagTypeInt && flag.Type != FlagTypeFloat {
			return fmt.Errorf("command '%s' flag '%s' has invalid type '%s' (must be 'string', 'bool', 'int', or 'float') in invkfile at %s",
				cmd.Name, flag.Name, flag.Type, inv.FilePath)
		}

		// Validate that required flags don't have default values
		if flag.Required && flag.DefaultValue != "" {
			return fmt.Errorf("command '%s' flag '%s' cannot be both required and have a default_value in invkfile at %s",
				cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate short alias format (single letter a-z or A-Z)
		if flag.Short != "" {
			isValidShort := len(flag.Short) == 1 &&
				((flag.Short[0] >= 'a' && flag.Short[0] <= 'z') || (flag.Short[0] >= 'A' && flag.Short[0] <= 'Z'))
			if !isValidShort {
				return fmt.Errorf("command '%s' flag '%s' has invalid short alias '%s' (must be a single letter a-z or A-Z) in invkfile at %s",
					cmd.Name, flag.Name, flag.Short, inv.FilePath)
			}
			// Check for reserved short aliases
			if flag.Short == "e" {
				return fmt.Errorf("command '%s' flag '%s': short alias 'e' is reserved for the system --env-file flag in invkfile at %s",
					cmd.Name, flag.Name, inv.FilePath)
			}
			if flag.Short == "E" {
				return fmt.Errorf("command '%s' flag '%s': short alias 'E' is reserved for the system --env-var flag in invkfile at %s",
					cmd.Name, flag.Name, inv.FilePath)
			}
			// Check for duplicate short aliases
			if seenShorts[flag.Short] {
				return fmt.Errorf("command '%s' has duplicate short alias '%s' in invkfile at %s",
					cmd.Name, flag.Short, inv.FilePath)
			}
			seenShorts[flag.Short] = true
		}

		// Validate default_value is compatible with type
		if flag.DefaultValue != "" {
			if err := validateFlagValueType(flag.DefaultValue, flag.GetType()); err != nil {
				return fmt.Errorf("command '%s' flag '%s' default_value '%s' is not compatible with type '%s': %s in invkfile at %s",
					cmd.Name, flag.Name, flag.DefaultValue, flag.GetType(), err.Error(), inv.FilePath)
			}
		}

		// Validate validation regex is valid and safe
		if flag.Validation != "" {
			// Check for regex complexity/safety issues first
			if err := ValidateRegexPattern(flag.Validation); err != nil {
				return fmt.Errorf("command '%s' flag '%s' has unsafe validation regex '%s': %s in invkfile at %s",
					cmd.Name, flag.Name, flag.Validation, err.Error(), inv.FilePath)
			}

			validationRegex, err := regexp.Compile(flag.Validation)
			if err != nil {
				return fmt.Errorf("command '%s' flag '%s' has invalid validation regex '%s': %s in invkfile at %s",
					cmd.Name, flag.Name, flag.Validation, err.Error(), inv.FilePath)
			}

			// Validate default_value matches validation regex (if both specified)
			if flag.DefaultValue != "" {
				if !validationRegex.MatchString(flag.DefaultValue) {
					return fmt.Errorf("command '%s' flag '%s' default_value '%s' does not match validation pattern '%s' in invkfile at %s",
						cmd.Name, flag.Name, flag.DefaultValue, flag.Validation, inv.FilePath)
				}
			}
		}
	}

	return nil
}

// validateArgs validates the args for a command
func (inv *Invkfile) validateArgs(cmd *Command) error {
	if len(cmd.Args) == 0 {
		return nil
	}

	seenNames := make(map[string]bool)
	foundOptional := false
	foundVariadic := false

	for i, arg := range cmd.Args {
		// Validate name is not empty
		if arg.Name == "" {
			return fmt.Errorf("command '%s' argument #%d must have a name in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate argument name length
		if err := ValidateStringLength(arg.Name, "argument name", MaxNameLength); err != nil {
			return fmt.Errorf("command '%s' argument '%s': %w in invkfile at %s", cmd.Name, arg.Name, err, inv.FilePath)
		}

		// Validate name is POSIX-compliant
		if !argNameRegex.MatchString(arg.Name) {
			return fmt.Errorf("command '%s' argument '%s' has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at %s", cmd.Name, arg.Name, inv.FilePath)
		}

		// Validate description is not empty (after trimming whitespace)
		if strings.TrimSpace(arg.Description) == "" {
			return fmt.Errorf("command '%s' argument '%s' must have a non-empty description in invkfile at %s", cmd.Name, arg.Name, inv.FilePath)
		}

		// Validate description length
		if err := ValidateStringLength(arg.Description, "argument description", MaxDescriptionLength); err != nil {
			return fmt.Errorf("command '%s' argument '%s': %w in invkfile at %s", cmd.Name, arg.Name, err, inv.FilePath)
		}

		// Check for duplicate argument names
		if seenNames[arg.Name] {
			return fmt.Errorf("command '%s' has duplicate argument name '%s' in invkfile at %s", cmd.Name, arg.Name, inv.FilePath)
		}
		seenNames[arg.Name] = true

		// Validate type is valid (if specified) - note: bool is not allowed for args
		if arg.Type != "" && arg.Type != ArgumentTypeString && arg.Type != ArgumentTypeInt && arg.Type != ArgumentTypeFloat {
			return fmt.Errorf("command '%s' argument '%s' has invalid type '%s' (must be 'string', 'int', or 'float') in invkfile at %s",
				cmd.Name, arg.Name, arg.Type, inv.FilePath)
		}

		// Validate that required arguments don't have default values
		if arg.Required && arg.DefaultValue != "" {
			return fmt.Errorf("command '%s' argument '%s' cannot be both required and have a default_value in invkfile at %s",
				cmd.Name, arg.Name, inv.FilePath)
		}

		// Rule: Required arguments must come before optional arguments
		isOptional := !arg.Required
		if arg.Required && foundOptional {
			return fmt.Errorf("command '%s' argument '%s': required arguments must come before optional arguments in invkfile at %s",
				cmd.Name, arg.Name, inv.FilePath)
		}
		if isOptional {
			foundOptional = true
		}

		// Rule: Only the last argument can be variadic
		if foundVariadic {
			return fmt.Errorf("command '%s' argument '%s': only the last argument can be variadic (found after variadic argument) in invkfile at %s",
				cmd.Name, arg.Name, inv.FilePath)
		}
		if arg.Variadic {
			foundVariadic = true
		}

		// Validate default_value is compatible with type
		if arg.DefaultValue != "" {
			if err := validateArgumentValueType(arg.DefaultValue, arg.GetType()); err != nil {
				return fmt.Errorf("command '%s' argument '%s' default_value '%s' is not compatible with type '%s': %s in invkfile at %s",
					cmd.Name, arg.Name, arg.DefaultValue, arg.GetType(), err.Error(), inv.FilePath)
			}
		}

		// Validate validation regex is valid and safe
		if arg.Validation != "" {
			// Check for regex complexity/safety issues first
			if err := ValidateRegexPattern(arg.Validation); err != nil {
				return fmt.Errorf("command '%s' argument '%s' has unsafe validation regex '%s': %s in invkfile at %s",
					cmd.Name, arg.Name, arg.Validation, err.Error(), inv.FilePath)
			}

			validationRegex, err := regexp.Compile(arg.Validation)
			if err != nil {
				return fmt.Errorf("command '%s' argument '%s' has invalid validation regex '%s': %s in invkfile at %s",
					cmd.Name, arg.Name, arg.Validation, err.Error(), inv.FilePath)
			}

			// Validate default_value matches validation regex (if both specified)
			if arg.DefaultValue != "" {
				if !validationRegex.MatchString(arg.DefaultValue) {
					return fmt.Errorf("command '%s' argument '%s' default_value '%s' does not match validation pattern '%s' in invkfile at %s",
						cmd.Name, arg.Name, arg.DefaultValue, arg.Validation, inv.FilePath)
				}
			}
		}
	}

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
