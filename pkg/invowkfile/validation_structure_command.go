// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"path/filepath"
)

// validateCommand validates a single command and collects all errors.
func (v *StructureValidator) validateCommand(ctx *ValidationContext, inv *Invowkfile, cmd *Command) []ValidationError {
	var validationErrors []ValidationError
	path := NewFieldPath().Command(cmd.Name)

	if cmd.Name == "" {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     "",
			Message:   "command must have a name in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
		return validationErrors // Can't validate further without a name
	}

	// [CUE-VALIDATED] Command name length also enforced by CUE schema (#Command.name MaxRunes(256))
	if err := ValidateStringLength(string(cmd.Name), "command name", MaxNameLength); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// [CUE-VALIDATED] Description length also enforced by CUE schema (#Command.description MaxRunes(10240))
	if err := ValidateStringLength(string(cmd.Description), "description", MaxDescriptionLength); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate command-level depends_on (all dependency types including custom checks)
	validationErrors = append(validationErrors, v.validateDependsOn(ctx, cmd.DependsOn, path.Copy())...)

	// Validate command-level env configuration
	validationErrors = append(validationErrors, v.validateEnvConfig(ctx, cmd.Env, path.Copy())...)

	if len(cmd.Implementations) == 0 {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one implementation in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	} else {
		// Validate each implementation
		for i := range cmd.Implementations {
			validationErrors = append(validationErrors, v.validateImplementation(ctx, inv, cmd, i)...)
		}
	}

	// Validate that there are no duplicate platform+runtime combinations
	if err := cmd.ValidateImplementations(); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error(),
			Severity:  SeverityError,
		})
	}

	// Validate flags
	validationErrors = append(validationErrors, v.validateFlags(ctx, cmd)...)

	// Validate args
	validationErrors = append(validationErrors, v.validateArgs(ctx, cmd)...)

	return validationErrors
}

// validateImplementation validates a single implementation and collects all errors.
func (v *StructureValidator) validateImplementation(ctx *ValidationContext, inv *Invowkfile, cmd *Command, implIdx int) []ValidationError {
	var validationErrors []ValidationError
	impl := &cmd.Implementations[implIdx]
	path := NewFieldPath().Command(cmd.Name).Implementation(implIdx)

	if impl.Script == "" {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have a script in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	} else if !impl.IsScriptFile() {
		// [CUE-VALIDATED] Script length also enforced by CUE schema (#Implementation.script MaxRunes(10485760))
		if err := ValidateStringLength(string(impl.Script), "script", MaxScriptLength); err != nil {
			validationErrors = append(validationErrors, ValidationError{
				Validator: v.Name(),
				Field:     path.String(),
				Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
	}

	if len(impl.Runtimes) == 0 {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one runtime in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	} else {
		// Validate each runtime config
		for j := range impl.Runtimes {
			validationErrors = append(validationErrors, v.validateRuntimeConfig(ctx, inv, cmd.Name, implIdx, j)...)
		}
	}

	// [CUE-VALIDATED] Platforms list also enforced by CUE schema (#Implementation.platforms [_, ...])
	if len(impl.Platforms) == 0 {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "must have at least one platform in invowkfile at " + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// Validate implementation-level depends_on (all dependency types including custom checks)
	validationErrors = append(validationErrors, v.validateDependsOn(ctx, impl.DependsOn, path.Copy())...)

	// Validate implementation-level env configuration
	validationErrors = append(validationErrors, v.validateEnvConfig(ctx, impl.Env, path.Copy())...)

	return validationErrors
}

// validateRuntimeConfig validates a single runtime configuration and collects all errors.
func (v *StructureValidator) validateRuntimeConfig(ctx *ValidationContext, inv *Invowkfile, cmdName CommandName, implIdx, rtIdx int) []ValidationError {
	var validationErrors []ValidationError
	rt := &inv.GetCommand(cmdName).Implementations[implIdx].Runtimes[rtIdx]
	path := NewFieldPath().Command(cmdName).Implementation(implIdx).Runtime(rtIdx)

	if err := rt.Validate(); err != nil {
		validationErrors = append(validationErrors, runtimeConfigValidationErrors(v.Name(), path.String(), err)...)
	}

	if rt.Name == RuntimeContainer {
		if rt.Containerfile != "" {
			baseDir := filepath.Dir(string(ctx.FilePath))
			if err := ValidateContainerfilePath(string(rt.Containerfile), baseDir); err != nil {
				validationErrors = append(validationErrors, ValidationError{
					Validator: v.Name(),
					Field:     path.String(),
					Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
					Severity:  SeverityError,
				})
			}
		}
		validationErrors = append(validationErrors, v.validateDependsOn(ctx, rt.DependsOn, path.Copy())...)
	}

	return validationErrors
}

//goplint:ignore -- validation field paths are rendered diagnostic strings.
func runtimeConfigValidationErrors(validatorName ValidatorName, field string, err error) []ValidationError {
	if invalid, ok := errors.AsType[*InvalidRuntimeConfigError](err); ok {
		result := make([]ValidationError, 0, len(invalid.FieldErrors))
		for _, fieldErr := range invalid.FieldErrors {
			result = append(result, runtimeConfigValidationError(validatorName, field, fieldErr))
		}
		return result
	}
	return []ValidationError{runtimeConfigValidationError(validatorName, field, err)}
}

//goplint:ignore -- validation field paths are rendered diagnostic strings.
func runtimeConfigValidationError(validatorName ValidatorName, field string, err error) ValidationError {
	return ValidationError{
		Validator: validatorName,
		Field:     field,
		Message:   err.Error(),
		Severity:  SeverityError,
	}
}
