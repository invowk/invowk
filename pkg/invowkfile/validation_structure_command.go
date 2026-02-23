// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"path/filepath"
	"strings"
)

// validateCommand validates a single command and collects all errors.
func (v *StructureValidator) validateCommand(ctx *ValidationContext, inv *Invowkfile, cmd *Command) []ValidationError {
	var errors []ValidationError
	path := NewFieldPath().Command(string(cmd.Name))

	if cmd.Name == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     "",
			Message:   "command must have a name in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
		return errors // Can't validate further without a name
	}

	// [CUE-VALIDATED] Command name length also enforced by CUE schema (#Command.name MaxRunes(256))
	if err := ValidateStringLength(string(cmd.Name), "command name", MaxNameLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Description length also enforced by CUE schema (#Command.description MaxRunes(10240))
	if err := ValidateStringLength(string(cmd.Description), "description", MaxDescriptionLength); err != nil {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + " in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate command-level depends_on (all dependency types including custom checks)
	errors = append(errors, v.validateDependsOn(ctx, cmd.DependsOn, path.Copy())...)

	// Validate command-level env configuration
	errors = append(errors, v.validateEnvConfig(ctx, cmd.Env, path.Copy())...)

	if len(cmd.Implementations) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one implementation in invowkfile at " + string(ctx.FilePath),
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
func (v *StructureValidator) validateImplementation(ctx *ValidationContext, inv *Invowkfile, cmd *Command, implIdx int) []ValidationError {
	var errors []ValidationError
	impl := &cmd.Implementations[implIdx]
	path := NewFieldPath().Command(string(cmd.Name)).Implementation(implIdx)

	if impl.Script == "" {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a script in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	} else if !impl.IsScriptFile() {
		// [CUE-VALIDATED] Script length also enforced by CUE schema (#Implementation.script MaxRunes(10485760))
		if err := ValidateStringLength(string(impl.Script), "script", MaxScriptLength); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   err.Error() + " in invowkfile at " + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
	}

	if len(impl.Runtimes) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one runtime in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	} else {
		// Validate each runtime config
		for j := range impl.Runtimes {
			errors = append(errors, v.validateRuntimeConfig(ctx, inv, cmd.Name, implIdx, j)...)
		}
	}

	// [CUE-VALIDATED] Platforms list also enforced by CUE schema (#Implementation.platforms [_, ...])
	if len(impl.Platforms) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one platform in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate implementation-level depends_on (all dependency types including custom checks)
	errors = append(errors, v.validateDependsOn(ctx, impl.DependsOn, path.Copy())...)

	// Validate implementation-level env configuration
	errors = append(errors, v.validateEnvConfig(ctx, impl.Env, path.Copy())...)

	return errors
}

// validateRuntimeConfig validates a single runtime configuration and collects all errors.
func (v *StructureValidator) validateRuntimeConfig(ctx *ValidationContext, inv *Invowkfile, cmdName CommandName, implIdx, rtIdx int) []ValidationError {
	var errors []ValidationError
	rt := &inv.GetCommand(cmdName).Implementations[implIdx].Runtimes[rtIdx]
	path := NewFieldPath().Command(string(cmdName)).Implementation(implIdx).Runtime(rtIdx)

	// Validate env inherit mode
	if rt.EnvInheritMode != "" {
		if isValid, _ := rt.EnvInheritMode.IsValid(); !isValid {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "env_inherit_mode must be one of: none, allow, all",
				Severity:  SeverityError,
			})
		}
	}

	// Validate env_inherit_allow names
	for _, name := range rt.EnvInheritAllow {
		if isValid, errs := name.IsValid(); !isValid {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "env_inherit_allow: " + errs[0].Error(),
				Severity:  SeverityError,
			})
		}
	}

	// Validate env_inherit_deny names
	for _, name := range rt.EnvInheritDeny {
		if isValid, errs := name.IsValid(); !isValid {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "env_inherit_deny: " + errs[0].Error(),
				Severity:  SeverityError,
			})
		}
	}

	// Container-specific fields validation
	if rt.Name != RuntimeContainer {
		// depends_on is only valid for container runtime (defense-in-depth; CUE schema is primary enforcement)
		if rt.DependsOn != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   "depends_on is only valid for container runtime",
				Severity:  SeverityError,
			})
		}
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
			if err := ValidateContainerImage(string(rt.Image)); err != nil {
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
			cfStr := string(rt.Containerfile)
			if filepath.IsAbs(cfStr) {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "containerfile path must be relative, not absolute",
					Severity:  SeverityError,
				})
			}
			if strings.ContainsRune(cfStr, '\x00') {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   "containerfile path contains null byte",
					Severity:  SeverityError,
				})
			}

			// Validate containerfile path traversal
			baseDir := filepath.Dir(string(ctx.FilePath))
			if err := ValidateContainerfilePath(cfStr, baseDir); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   err.Error() + " in invowkfile at " + string(ctx.FilePath),
					Severity:  SeverityError,
				})
			}
		}

		// Validate volume mounts
		for i, vol := range rt.Volumes {
			if err := ValidateVolumeMount(string(vol)); err != nil {
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
			if err := ValidatePortMapping(string(port)); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     path.Copy().Port(i).String(),
					Message:   err.Error(),
					Severity:  SeverityError,
				})
			}
		}

		// Container runtime-level depends_on (structural validation including custom checks)
		errors = append(errors, v.validateDependsOn(ctx, rt.DependsOn, path.Copy())...)
	}

	return errors
}
