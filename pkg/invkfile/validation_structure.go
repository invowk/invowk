// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"path/filepath"
	"regexp"
)

var (
	// flagNameRegex validates POSIX-compliant flag names.
	flagNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

	// argNameRegex validates POSIX-compliant argument names.
	argNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

// StructureValidator validates the structural correctness of an invkfile.
// It checks command structure, implementations, runtimes, flags, args, and dependencies.
// This validator wraps all the existing validation logic and collects ALL errors.
type StructureValidator struct{}

// NewStructureValidator creates a new StructureValidator.
func NewStructureValidator() *StructureValidator {
	return &StructureValidator{}
}

// Name returns the validator name.
func (v *StructureValidator) Name() string {
	return "structure"
}

// Validate checks the invkfile structure and collects all validation errors.
func (v *StructureValidator) Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError {
	var errors []ValidationError

	if len(inv.Commands) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     "",
			Message:   "invkfile at " + ctx.FilePath + " has no commands defined (missing required 'cmds' list)",
			Severity:  SeverityError,
		})
		return errors // No point validating further if there are no commands
	}

	// Validate root-level env configuration
	errors = append(errors, v.validateEnvConfig(ctx, inv.Env, NewFieldPath().Root())...)

	// Validate root-level depends_on (all dependency types)
	errors = append(errors, v.validateDependsOn(ctx, inv.DependsOn, NewFieldPath().Root())...)

	// Validate root-level custom_checks dependencies (security-specific checks)
	if inv.DependsOn != nil && len(inv.DependsOn.CustomChecks) > 0 {
		errors = append(errors, v.validateCustomChecks(ctx, inv.DependsOn.CustomChecks, NewFieldPath().Root())...)
	}

	// Validate each command
	for i := range inv.Commands {
		errors = append(errors, v.validateCommand(ctx, inv, &inv.Commands[i])...)
	}

	return errors
}

// validateCommand validates a single command and collects all errors.
func (v *StructureValidator) validateCommand(ctx *ValidationContext, inv *Invkfile, cmd *Command) []ValidationError {
	var errors []ValidationError
	path := NewFieldPath().Command(cmd.Name)

	if cmd.Name == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     "",
			Message:   "command must have a name in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
		return errors // Can't validate further without a name
	}

	// [CUE-VALIDATED] Command name length also enforced by CUE schema (#Command.name MaxRunes(256))
	if err := ValidateStringLength(cmd.Name, "command name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Description length also enforced by CUE schema (#Command.description MaxRunes(10240))
	if err := ValidateStringLength(cmd.Description, "description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate command-level depends_on (all dependency types)
	errors = append(errors, v.validateDependsOn(ctx, cmd.DependsOn, path.Copy())...)

	// Validate command-level custom_checks dependencies (security-specific checks)
	if cmd.DependsOn != nil && len(cmd.DependsOn.CustomChecks) > 0 {
		errors = append(errors, v.validateCustomChecks(ctx, cmd.DependsOn.CustomChecks, path.Copy())...)
	}

	// Validate command-level env configuration
	errors = append(errors, v.validateEnvConfig(ctx, cmd.Env, path.Copy())...)

	if len(cmd.Implementations) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one implementation in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	} else {
		// Validate each implementation
		for i := range cmd.Implementations {
			errors = append(errors, v.validateImplementation(ctx, inv, cmd, i)...)
		}
	}

	// Validate that there are no duplicate platform+runtime combinations
	if err := cmd.ValidateImplementations(); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error(),
			Severity:  SeverityError,
		})
	}

	// Validate flags
	errors = append(errors, v.validateFlags(ctx, cmd)...)

	// Validate args
	errors = append(errors, v.validateArgs(ctx, cmd)...)

	return errors
}

// validateImplementation validates a single implementation and collects all errors.
func (v *StructureValidator) validateImplementation(ctx *ValidationContext, inv *Invkfile, cmd *Command, implIdx int) []ValidationError {
	var errors []ValidationError
	impl := &cmd.Implementations[implIdx]
	path := NewFieldPath().Command(cmd.Name).Implementation(implIdx)

	if impl.Script == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a script in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	} else if !impl.IsScriptFile() {
		// [CUE-VALIDATED] Script length also enforced by CUE schema (#Implementation.script MaxRunes(10485760))
		if err := ValidateStringLength(impl.Script, "script", MaxScriptLength); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	if len(impl.Runtimes) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one runtime in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	} else {
		// Validate each runtime config
		for j := range impl.Runtimes {
			errors = append(errors, v.validateRuntimeConfig(ctx, inv, cmd.Name, implIdx, j)...)
		}
	}

	// Validate implementation-level depends_on (all dependency types)
	errors = append(errors, v.validateDependsOn(ctx, impl.DependsOn, path.Copy())...)

	// Validate implementation-level custom_checks dependencies (security-specific checks)
	if impl.DependsOn != nil && len(impl.DependsOn.CustomChecks) > 0 {
		errors = append(errors, v.validateCustomChecks(ctx, impl.DependsOn.CustomChecks, path.Copy())...)
	}

	// Validate implementation-level env configuration
	errors = append(errors, v.validateEnvConfig(ctx, impl.Env, path.Copy())...)

	return errors
}

// validateRuntimeConfig validates a single runtime configuration and collects all errors.
func (v *StructureValidator) validateRuntimeConfig(ctx *ValidationContext, inv *Invkfile, cmdName string, implIdx, rtIdx int) []ValidationError {
	var errors []ValidationError
	rt := &inv.GetCommand(cmdName).Implementations[implIdx].Runtimes[rtIdx]
	path := NewFieldPath().Command(cmdName).Implementation(implIdx).Runtime(rtIdx)

	// Validate env inherit mode
	if rt.EnvInheritMode != "" && !rt.EnvInheritMode.IsValid() {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "env_inherit_mode must be one of: none, allow, all",
			Severity:  SeverityError,
		})
	}

	// Validate env_inherit_allow names
	for _, name := range rt.EnvInheritAllow {
		if err := ValidateEnvVarName(name); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "env_inherit_allow: " + err.Error(),
				Severity:  SeverityError,
			})
		}
	}

	// Validate env_inherit_deny names
	for _, name := range rt.EnvInheritDeny {
		if err := ValidateEnvVarName(name); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "env_inherit_deny: " + err.Error(),
				Severity:  SeverityError,
			})
		}
	}

	// Container-specific fields validation
	if rt.Name != RuntimeContainer {
		if rt.EnableHostSSH {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "enable_host_ssh is only valid for container runtime",
				Severity:  SeverityError,
			})
		}
		if rt.Containerfile != "" {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "containerfile is only valid for container runtime",
				Severity:  SeverityError,
			})
		}
		if rt.Image != "" {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "image is only valid for container runtime",
				Severity:  SeverityError,
			})
		}
		if len(rt.Volumes) > 0 {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "volumes is only valid for container runtime",
				Severity:  SeverityError,
			})
		}
		if len(rt.Ports) > 0 {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "ports is only valid for container runtime",
				Severity:  SeverityError,
			})
		}
	} else {
		// For container runtime, validate mutual exclusivity
		if rt.Containerfile != "" && rt.Image != "" {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "containerfile and image are mutually exclusive - specify only one",
				Severity:  SeverityError,
			})
		}

		// At least one of containerfile or image must be specified
		if rt.Containerfile == "" && rt.Image == "" {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "container runtime requires either containerfile or image to be specified",
				Severity:  SeverityError,
			})
		}

		// Validate container image name format
		if rt.Image != "" {
			if err := ValidateContainerImage(rt.Image); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "invalid image: " + err.Error(),
					Severity:  SeverityError,
				})
			}
		}

		// Validate containerfile path
		if rt.Containerfile != "" {
			if len(rt.Containerfile) > MaxPathLength {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "containerfile path too long (" + itoa(len(rt.Containerfile)) + " chars, max " + itoa(MaxPathLength) + ")",
					Severity:  SeverityError,
				})
			}
			if filepath.IsAbs(rt.Containerfile) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "containerfile path must be relative, not absolute",
					Severity:  SeverityError,
				})
			}
			if containsNullByte(rt.Containerfile) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "containerfile path contains null byte",
					Severity:  SeverityError,
				})
			}

			// Validate containerfile path traversal
			baseDir := filepath.Dir(ctx.FilePath)
			if err := ValidateContainerfilePath(rt.Containerfile, baseDir); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   err.Error() + " in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}

		// Validate volume mounts
		for i, vol := range rt.Volumes {
			if err := ValidateVolumeMount(vol); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.Copy().Volume(i).String(),
					Message:   err.Error(),
					Severity:  SeverityError,
				})
			}
		}

		// Validate port mappings
		for i, port := range rt.Ports {
			if err := ValidatePortMapping(port); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.Copy().Port(i).String(),
					Message:   err.Error(),
					Severity:  SeverityError,
				})
			}
		}
	}

	return errors
}

// validateFlags validates all flags for a command and collects all errors.
func (v *StructureValidator) validateFlags(ctx *ValidationContext, cmd *Command) []ValidationError {
	var errors []ValidationError
	seenNames := make(map[string]bool)
	seenShorts := make(map[string]bool)

	for i := range cmd.Flags {
		flag := &cmd.Flags[i]
		errors = append(errors, v.validateFlag(ctx, cmd, flag, i, seenNames, seenShorts)...)
	}

	return errors
}

// validateFlag validates a single flag and collects all errors.
func (v *StructureValidator) validateFlag(ctx *ValidationContext, cmd *Command, flag *Flag, idx int, seenNames, seenShorts map[string]bool) []ValidationError {
	var errors []ValidationError
	path := NewFieldPath().Command(cmd.Name)

	// Validate name is not empty
	if flag.Name == "" {
		path = path.Copy().FlagIndex(idx)
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a name in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
		return errors // Can't validate further without a name
	}

	path = path.Copy().Flag(flag.Name)

	// [CUE-VALIDATED] Flag name length also enforced by CUE schema (#Flag.name MaxRunes(256))
	if err := ValidateStringLength(flag.Name, "flag name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate name is POSIX-compliant
	if !flagNameRegex.MatchString(flag.Name) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate description is not empty (after trimming whitespace)
	if trimSpace(flag.Description) == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a non-empty description in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Flag description length also enforced by CUE schema (#Flag.description MaxRunes(10240))
	if err := ValidateStringLength(flag.Description, "flag description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Check for duplicate flag names
	if seenNames[flag.Name] {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     NewFieldPath().Command(cmd.Name).String(),
			Message:   "has duplicate flag name '" + flag.Name + "' in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}
	seenNames[flag.Name] = true

	// Check for reserved flag names
	if flag.Name == "env-file" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "'env-file' is a reserved system flag and cannot be used in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}
	if flag.Name == "env-var" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "'env-var' is a reserved system flag and cannot be used in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate type is valid (if specified)
	if flag.Type != "" && flag.Type != FlagTypeString && flag.Type != FlagTypeBool && flag.Type != FlagTypeInt && flag.Type != FlagTypeFloat {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid type '" + string(flag.Type) + "' (must be 'string', 'bool', 'int', or 'float') in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate that required flags don't have default values
	if flag.Required && flag.DefaultValue != "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "cannot be both required and have a default_value in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate short alias format (single letter a-z or A-Z)
	if flag.Short != "" {
		isValidShort := len(flag.Short) == 1 &&
			((flag.Short[0] >= 'a' && flag.Short[0] <= 'z') || (flag.Short[0] >= 'A' && flag.Short[0] <= 'Z'))
		if !isValidShort {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has invalid short alias '" + flag.Short + "' (must be a single letter a-z or A-Z) in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}

		// Check for reserved short aliases
		if flag.Short == "e" {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "short alias 'e' is reserved for the system --env-file flag in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
		if flag.Short == "E" {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "short alias 'E' is reserved for the system --env-var flag in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}

		// Check for duplicate short aliases
		if seenShorts[flag.Short] {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     NewFieldPath().Command(cmd.Name).String(),
				Message:   "has duplicate short alias '" + flag.Short + "' in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
		seenShorts[flag.Short] = true
	}

	// Validate default_value is compatible with type
	if flag.DefaultValue != "" {
		if err := validateFlagValueType(flag.DefaultValue, flag.GetType()); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "default_value '" + flag.DefaultValue + "' is not compatible with type '" + string(flag.GetType()) + "': " + err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	// Validate validation regex is valid and safe
	if flag.Validation != "" {
		if err := ValidateRegexPattern(flag.Validation); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has unsafe validation regex '" + flag.Validation + "': " + err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		} else if flag.DefaultValue != "" {
			// Check if default_value matches validation regex
			if !matchesValidation(flag.DefaultValue, flag.Validation) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "default_value '" + flag.DefaultValue + "' does not match validation pattern '" + flag.Validation + "' in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}
	}

	return errors
}

// validateArgs validates all args for a command and collects all errors.
func (v *StructureValidator) validateArgs(ctx *ValidationContext, cmd *Command) []ValidationError {
	if len(cmd.Args) == 0 {
		return nil
	}

	var errors []ValidationError
	seenNames := make(map[string]bool)
	foundOptional := false
	foundVariadic := false

	for i := range cmd.Args {
		arg := &cmd.Args[i]
		argErrors, isOptional, isVariadic := v.validateArg(ctx, cmd, arg, i, seenNames, foundOptional, foundVariadic)
		errors = append(errors, argErrors...)
		if isOptional {
			foundOptional = true
		}
		if isVariadic {
			foundVariadic = true
		}
	}

	return errors
}

// validateArg validates a single argument and collects all errors.
// Returns the errors, whether this arg is optional, and whether it's variadic.
func (v *StructureValidator) validateArg(ctx *ValidationContext, cmd *Command, arg *Argument, idx int, seenNames map[string]bool, foundOptional, foundVariadic bool) ([]ValidationError, bool, bool) {
	var errors []ValidationError
	path := NewFieldPath().Command(cmd.Name)
	isOptional := !arg.Required
	isVariadic := arg.Variadic

	// Validate name is not empty
	if arg.Name == "" {
		path = path.Copy().ArgIndex(idx)
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a name in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
		return errors, isOptional, isVariadic
	}

	path = path.Copy().Arg(arg.Name)

	// [CUE-VALIDATED] Argument name length also enforced by CUE schema (#Argument.name MaxRunes(256))
	if err := ValidateStringLength(arg.Name, "argument name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate name is POSIX-compliant
	if !argNameRegex.MatchString(arg.Name) {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate description is not empty (after trimming whitespace)
	if trimSpace(arg.Description) == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a non-empty description in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Argument description length also enforced by CUE schema (#Argument.description MaxRunes(10240))
	if err := ValidateStringLength(arg.Description, "argument description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Check for duplicate argument names
	if seenNames[arg.Name] {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     NewFieldPath().Command(cmd.Name).String(),
			Message:   "has duplicate argument name '" + arg.Name + "' in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}
	seenNames[arg.Name] = true

	// Validate type is valid (if specified) - note: bool is not allowed for args
	if arg.Type != "" && arg.Type != ArgumentTypeString && arg.Type != ArgumentTypeInt && arg.Type != ArgumentTypeFloat {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "has invalid type '" + string(arg.Type) + "' (must be 'string', 'int', or 'float') in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate that required arguments don't have default values
	if arg.Required && arg.DefaultValue != "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "cannot be both required and have a default_value in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Rule: Required arguments must come before optional arguments
	if arg.Required && foundOptional {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "required arguments must come before optional arguments in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Rule: Only the last argument can be variadic
	if foundVariadic {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "only the last argument can be variadic (found after variadic argument) in invkfile at " + ctx.FilePath,
			Severity:  SeverityError,
		})
	}

	// Validate default_value is compatible with type
	if arg.DefaultValue != "" {
		if err := validateArgumentValueType(arg.DefaultValue, arg.GetType()); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "default_value '" + arg.DefaultValue + "' is not compatible with type '" + string(arg.GetType()) + "': " + err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	// Validate validation regex is valid and safe
	if arg.Validation != "" {
		if err := ValidateRegexPattern(arg.Validation); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "has unsafe validation regex '" + arg.Validation + "': " + err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		} else if arg.DefaultValue != "" {
			// Check if default_value matches validation regex
			if !matchesValidation(arg.DefaultValue, arg.Validation) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "default_value '" + arg.DefaultValue + "' does not match validation pattern '" + arg.Validation + "' in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}
	}

	return errors, isOptional, isVariadic
}

// validateDependsOn validates all dependency types in a DependsOn struct.
func (v *StructureValidator) validateDependsOn(ctx *ValidationContext, deps *DependsOn, basePath *FieldPath) []ValidationError {
	if deps == nil {
		return nil
	}

	var errors []ValidationError

	// Validate tool dependencies
	for i, dep := range deps.Tools {
		for j, alt := range dep.Alternatives {
			if err := ValidateToolName(alt); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().Tools(i, j).String(),
					Message:   err.Error() + " in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}
	}

	// Validate command dependencies
	for i, dep := range deps.Commands {
		for j, alt := range dep.Alternatives {
			if err := ValidateCommandDependencyName(alt); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().Commands(i, j).String(),
					Message:   err.Error() + " in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}
	}

	// Validate filepath dependencies
	for i, dep := range deps.Filepaths {
		if err := ValidateFilepathDependency(dep.Alternatives); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().Filepaths(i).String(),
				Message:   err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	// Validate env var dependencies
	for i, dep := range deps.EnvVars {
		for j, alt := range dep.Alternatives {
			name := trimSpace(alt.Name)
			if err := ValidateEnvVarName(name); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().EnvVars(i, j).Field("name").String(),
					Message:   err.Error() + " in invkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
			if alt.Validation != "" {
				if err := ValidateRegexPattern(alt.Validation); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     basePath.Copy().EnvVars(i, j).Field("validation").String(),
						Message:   err.Error() + " in invkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}
		}
	}

	return errors
}

// validateCustomChecks validates custom check dependencies for security and correctness.
func (v *StructureValidator) validateCustomChecks(ctx *ValidationContext, checks []CustomCheckDependency, basePath *FieldPath) []ValidationError {
	var errors []ValidationError

	for i, checkDep := range checks {
		for j, check := range checkDep.GetChecks() {
			path := basePath.Copy().CustomCheck(i, j)

			// [CUE-VALIDATED] Custom check name length also enforced by CUE schema (#CustomCheck.name MaxRunes(256))
			if check.Name != "" {
				if err := ValidateStringLength(check.Name, "custom_check name", MaxNameLength); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     path.String(),
						Message:   err.Error() + " in invkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}

			// [CUE-VALIDATED] Check script length also enforced by CUE schema (#CustomCheck.check_script MaxRunes(10485760))
			if check.CheckScript != "" {
				if err := ValidateStringLength(check.CheckScript, "check_script", MaxScriptLength); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     path.String(),
						Message:   err.Error() + " in invkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}

			// ReDoS pattern safety - CUE cannot analyze regex complexity
			if check.ExpectedOutput != "" {
				if err := ValidateRegexPattern(check.ExpectedOutput); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     path.String(),
						Message:   "expected_output: " + err.Error() + " in invkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}
		}
	}

	return errors
}

// validateEnvConfig validates environment configuration for security.
func (v *StructureValidator) validateEnvConfig(ctx *ValidationContext, env *EnvConfig, basePath *FieldPath) []ValidationError {
	if env == nil {
		return nil
	}

	var errors []ValidationError

	// Env file path validation - path traversal prevention
	for i, file := range env.Files {
		if err := ValidateEnvFilePath(file); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvFile(i).String(),
				Message:   err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	// Validate env var names and values
	for key, value := range env.Vars {
		if err := ValidateEnvVarName(key); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvVar(key).String(),
				Message:   err.Error() + " in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
		// [CUE-VALIDATED] Env var value length also enforced by CUE schema (#EnvConfig.vars MaxRunes(32768))
		if len(value) > MaxEnvVarValueLength {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvVar(key).String(),
				Message:   "value too long (" + itoa(len(value)) + " chars, max " + itoa(MaxEnvVarValueLength) + ") in invkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	return errors
}

// Helper functions to avoid importing strings package for simple operations

// trimSpace removes leading and trailing whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isSpace reports whether the character is a whitespace character.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// containsNullByte reports whether the string contains a null byte.
func containsNullByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}

// matchesValidation checks if a value matches a validation regex pattern.
// Returns false if the pattern is invalid or doesn't match.
func matchesValidation(value, pattern string) bool {
	// Import regexp inline to avoid it in the file-level imports
	// This keeps the helper function self-contained
	matched, err := regexpMatch(pattern, value)
	return err == nil && matched
}

// regexpMatch compiles a pattern and matches it against a string.
// Returns (matched, nil) on success, or (false, error) if the pattern is invalid.
func regexpMatch(pattern, s string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compiling regex %q: %w", pattern, err)
	}
	return re.MatchString(s), nil
}
