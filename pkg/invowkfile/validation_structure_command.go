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
	validationErrors = append(validationErrors, v.validateDependsOn(ctx, inv, cmd.DependsOn, path.Copy())...)

	// Validate command-level env configuration
	validationErrors = append(validationErrors, v.validateEnvConfig(ctx, cmd.Env, path.Copy())...)

	// [GO-ONLY] Positive duration semantics require time.ParseDuration; CUE only
	// enforces the syntactic duration shape.
	if cmd.Watch != nil {
		if err := cmd.Watch.Validate(); err != nil {
			validationErrors = append(validationErrors, watchConfigValidationErrors(v.Name(), path.Copy().Field("watch").String(), err)...)
		}
	}

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

	validationErrors = append(validationErrors, v.validateImplementationScript(ctx, inv, impl, path)...)

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
	validationErrors = append(validationErrors, v.validateDependsOn(ctx, inv, impl.DependsOn, path.Copy())...)

	// Validate implementation-level env configuration
	validationErrors = append(validationErrors, v.validateEnvConfig(ctx, impl.Env, path.Copy())...)

	if err := impl.AllowedPaths.ValidateForPlatforms(impl.Platforms); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.Copy().Field("allowed_paths").String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}

	// [GO-ONLY] Positive duration semantics require time.ParseDuration; CUE only
	// enforces the syntactic duration shape.
	if err := impl.Timeout.Validate(); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.Copy().Field("timeout").String(),
			Message:   err.Error(),
			Severity:  SeverityError,
		})
	}

	return validationErrors
}

func (v *StructureValidator) validateImplementationScript(ctx *ValidationContext, inv *Invowkfile, impl *Implementation, path *FieldPath) []ValidationError {
	if err := impl.Script.Validate(); err != nil {
		return []ValidationError{{
			Validator: v.Name(),
			Field:     path.Copy().Field("script").String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		}}
	}

	var validationErrors []ValidationError
	validationErrors = append(validationErrors, v.validateImplementationScriptContent(ctx, impl, path)...)
	validationErrors = append(validationErrors, v.validateImplementationScriptFile(ctx, inv, impl, path)...)
	return validationErrors
}

func (v *StructureValidator) validateImplementationScriptContent(ctx *ValidationContext, impl *Implementation, path *FieldPath) []ValidationError {
	if !impl.Script.IsContent() {
		return nil
	}
	// [CUE-VALIDATED] Script content length also enforced by CUE schema (#ScriptSourceContent.content MaxRunes(10485760)).
	if err := ValidateStringLength(string(impl.Script.Content), "script.content", MaxScriptLength); err != nil {
		return []ValidationError{{
			Validator: v.Name(),
			Field:     path.Copy().Field("script").Field("content").String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		}}
	}
	return nil
}

func (v *StructureValidator) validateImplementationScriptFile(ctx *ValidationContext, inv *Invowkfile, impl *Implementation, path *FieldPath) []ValidationError {
	if !impl.Script.IsFile() {
		return nil
	}

	var validationErrors []ValidationError
	// [CUE-VALIDATED] Script file length also enforced by CUE schema (#ScriptSourceFile.file MaxRunes(4096)).
	if err := ValidateStringLength(string(*impl.Script.File), "script.file", MaxPathLength); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.Copy().Field("script").Field("file").String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}
	if err := validateModuleScriptFileSelection(impl.GetScriptFilePathWithModule(ctx.FilePath, inv.ModulePath), inv.ModulePath); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Validator: v.Name(),
			Field:     path.Copy().Field("script").Field("file").String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
			Cause:     err,
		})
	}
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
		validationErrors = append(validationErrors, v.validateDependsOn(ctx, inv, rt.DependsOn, path.Copy())...)
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

//goplint:ignore -- validation field paths are rendered diagnostic strings.
func watchConfigValidationErrors(validatorName ValidatorName, field string, err error) []ValidationError {
	if invalid, ok := errors.AsType[*InvalidWatchConfigError](err); ok {
		result := make([]ValidationError, 0, len(invalid.FieldErrors))
		for _, fieldErr := range invalid.FieldErrors {
			result = append(result, ValidationError{
				Validator: validatorName,
				Field:     field,
				Message:   fieldErr.Error(),
				Severity:  SeverityError,
			})
		}
		return result
	}
	return []ValidationError{{
		Validator: validatorName,
		Field:     field,
		Message:   err.Error(),
		Severity:  SeverityError,
	}}
}
