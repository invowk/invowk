// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strconv"
	"strings"
)

// validateDependsOn validates all dependency types in a DependsOn struct.
func (v *StructureValidator) validateDependsOn(ctx *ValidationContext, inv *Invowkfile, deps *DependsOn, basePath *FieldPath) []ValidationError {
	if deps == nil {
		return nil
	}

	var errs []ValidationError

	// Validate tool dependencies
	for i, dep := range deps.Tools {
		if err := dep.Validate(); err != nil {
			errs = append(errs, dependencyValidationError(v.Name(), basePath.Copy().DependsOn().Field("tools["+strconv.Itoa(i+1)+"]"), ctx, err))
			continue
		}
		for j, alt := range dep.Alternatives {
			if err := ValidateToolName(alt); err != nil {
				errs = append(errs, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().Tools(i, j).String(),
					Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
					Severity:  SeverityError,
				})
			}
		}
	}

	// Validate command dependencies
	for i, dep := range deps.Commands {
		if err := dep.Validate(); err != nil {
			errs = append(errs, dependencyValidationError(v.Name(), basePath.Copy().DependsOn().Field("cmds["+strconv.Itoa(i+1)+"]"), ctx, err))
			continue
		}
		for j, alt := range dep.Alternatives {
			if err := ValidateCommandDependencyRef(alt); err != nil {
				errs = append(errs, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().Commands(i, j).String(),
					Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
					Severity:  SeverityError,
				})
			}
		}
	}

	// Validate filepath dependencies
	for i, dep := range deps.Filepaths {
		if err := dep.Validate(); err != nil {
			errs = append(errs, dependencyValidationError(v.Name(), basePath.Copy().DependsOn().Filepaths(i), ctx, err))
			continue
		}
		if err := ValidateFilepathDependency(dep.Alternatives); err != nil {
			errs = append(errs, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().Filepaths(i).String(),
				Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
	}

	// Validate capability dependencies.
	for i, dep := range deps.Capabilities {
		if err := dep.Validate(); err != nil {
			errs = append(errs, dependencyValidationError(v.Name(), basePath.Copy().DependsOn().Field("capabilities["+strconv.Itoa(i+1)+"]"), ctx, err))
		}
	}

	// Validate env var dependencies
	for i, dep := range deps.EnvVars {
		if err := dep.Validate(); err != nil {
			errs = append(errs, dependencyValidationError(v.Name(), basePath.Copy().DependsOn().Field("env_vars["+strconv.Itoa(i+1)+"]"), ctx, err))
			continue
		}
		for j, alt := range dep.Alternatives {
			name := strings.TrimSpace(string(alt.Name))
			if err := ValidateEnvVarName(name); err != nil {
				errs = append(errs, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().EnvVars(i, j).Field("name").String(),
					Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
					Severity:  SeverityError,
				})
			}
			if alt.Validation != "" {
				if err := ValidateRegexPattern(string(alt.Validation)); err != nil {
					errs = append(errs, ValidationError{
						Validator: v.Name(),
						Field:     basePath.Copy().EnvVars(i, j).Field("validation").String(),
						Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
						Severity:  SeverityError,
						Cause:     err,
					})
				}
			}
		}
	}

	// Validate custom check dependencies (security-specific: length limits, ReDoS safety)
	errs = append(errs, v.validateCustomChecks(ctx, inv, deps.CustomChecks, basePath)...)

	return errs
}

func dependencyValidationError(validator ValidatorName, fieldPath *FieldPath, ctx *ValidationContext, err error) ValidationError {
	return ValidationError{
		Validator: validator,
		Field:     fieldPath.String(),
		Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
		Severity:  SeverityError,
		Cause:     err,
	}
}

// validateCustomChecks validates custom check dependencies for security and correctness.
func (v *StructureValidator) validateCustomChecks(ctx *ValidationContext, inv *Invowkfile, checks []CustomCheckDependency, basePath *FieldPath) []ValidationError {
	var errs []ValidationError

	for i, checkDep := range checks {
		if err := checkDep.Validate(); err != nil {
			errs = append(errs, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().CustomCheck(i, 0).String(),
				Message:   customCheckDependencyValidationMessage(err) + invowkfileAtSuffix + string(ctx.FilePath),
				Severity:  SeverityError,
				Cause:     err,
			})
			continue
		}

		for j, check := range checkDep.GetChecks() {
			path := basePath.Copy().CustomCheck(i, j)
			errs = append(errs, v.validateCustomCheck(ctx, inv, check, path)...)
		}
	}

	return errs
}

func (v *StructureValidator) validateCustomCheck(ctx *ValidationContext, inv *Invowkfile, check CustomCheck, path *FieldPath) []ValidationError {
	var errs []ValidationError
	errs = append(errs, v.validateCustomCheckName(ctx, check, path)...)
	errs = append(errs, v.validateCustomCheckScriptContent(ctx, check, path)...)
	errs = append(errs, v.validateCustomCheckScriptFile(ctx, inv, check, path)...)
	errs = append(errs, v.validateCustomCheckExpectedOutput(ctx, check, path)...)
	return errs
}

func (v *StructureValidator) validateCustomCheckName(ctx *ValidationContext, check CustomCheck, path *FieldPath) []ValidationError {
	if check.Name == "" {
		return nil
	}
	// [CUE-VALIDATED] Custom check name length also enforced by CUE schema (#CustomCheck.name MaxRunes(256)).
	if err := ValidateStringLength(string(check.Name), "custom_check name", MaxNameLength); err != nil {
		return []ValidationError{{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		}}
	}
	return nil
}

func (v *StructureValidator) validateCustomCheckScriptContent(ctx *ValidationContext, check CustomCheck, path *FieldPath) []ValidationError {
	if check.Script.Content == "" {
		return nil
	}
	// [CUE-VALIDATED] Custom check script content length also enforced by CUE schema (#ScriptSourceContent.content MaxRunes(10485760)).
	if err := ValidateStringLength(string(check.Script.Content), "script.content", MaxScriptLength); err != nil {
		return []ValidationError{{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		}}
	}
	return nil
}

func (v *StructureValidator) validateCustomCheckScriptFile(ctx *ValidationContext, inv *Invowkfile, check CustomCheck, path *FieldPath) []ValidationError {
	if check.Script.File == nil {
		return nil
	}

	var errs []ValidationError
	// [CUE-VALIDATED] Custom check script file length also enforced by CUE schema (#ScriptSourceFile.file MaxRunes(4096)).
	if err := ValidateStringLength(string(*check.Script.File), "script.file", MaxPathLength); err != nil {
		errs = append(errs, ValidationError{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
		})
	}
	if err := validateModuleScriptFileSelection(check.Script.GetScriptFilePathWithModule(inv.ModulePath), inv.ModulePath); err != nil {
		errs = append(errs, ValidationError{
			Validator: v.Name(),
			Field:     path.Copy().Field("script").Field("file").String(),
			Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
			Cause:     err,
		})
	}
	return errs
}

func (v *StructureValidator) validateCustomCheckExpectedOutput(ctx *ValidationContext, check CustomCheck, path *FieldPath) []ValidationError {
	if check.ExpectedOutput == "" {
		return nil
	}
	// ReDoS pattern safety - CUE cannot analyze regex complexity.
	if err := ValidateRegexPattern(string(check.ExpectedOutput)); err != nil {
		return []ValidationError{{
			Validator: v.Name(),
			Field:     path.String(),
			Message:   "expected_output: " + err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
			Severity:  SeverityError,
			Cause:     err,
		}}
	}
	return nil
}

//goplint:ignore -- returns human-readable validation detail for ValidationError.Message.
func customCheckDependencyValidationMessage(err error) string {
	var message strings.Builder
	message.WriteString(err.Error())
	if depErr, ok := errors.AsType[*InvalidCustomCheckDependencyError](err); ok {
		for i := range depErr.FieldErrors {
			message.WriteString(": ")
			message.WriteString(customCheckFieldValidationMessage(depErr.FieldErrors[i]))
		}
	}
	return message.String()
}

//goplint:ignore -- returns human-readable validation detail for ValidationError.Message.
func customCheckFieldValidationMessage(err error) string {
	var message strings.Builder
	message.WriteString(err.Error())
	if checkErr, ok := errors.AsType[*InvalidCustomCheckError](err); ok {
		for i := range checkErr.FieldErrors {
			message.WriteString(": ")
			message.WriteString(checkErr.FieldErrors[i].Error())
		}
	}
	if scriptErr, ok := errors.AsType[*InvalidCustomCheckScriptError](err); ok {
		for i := range scriptErr.FieldErrors {
			message.WriteString(": ")
			message.WriteString(scriptErr.FieldErrors[i].Error())
		}
	}
	return message.String()
}

// validateEnvConfig validates environment configuration for security.
func (v *StructureValidator) validateEnvConfig(ctx *ValidationContext, env *EnvConfig, basePath *FieldPath) []ValidationError {
	if env == nil {
		return nil
	}

	var errs []ValidationError

	// Env file path validation - path traversal prevention
	for i, file := range env.Files {
		if err := ValidateEnvFilePath(string(file)); err != nil {
			errs = append(errs, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvFile(i).String(),
				Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
	}

	// Validate env var names and values
	for key, value := range env.Vars {
		keyStr := string(key)
		if err := key.Validate(); err != nil {
			errs = append(errs, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvVar(keyStr).String(),
				Message:   err.Error() + invowkfileAtSuffix + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
		// [CUE-VALIDATED] Env var value length also enforced by CUE schema (#EnvConfig.vars MaxRunes(32768))
		if len(value) > MaxEnvVarValueLength {
			errs = append(errs, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvVar(keyStr).String(),
				Message:   "value too long (" + strconv.Itoa(len(value)) + " chars, max " + strconv.Itoa(MaxEnvVarValueLength) + ") in invowkfile at " + string(ctx.FilePath),
				Severity:  SeverityError,
			})
		}
	}

	return errs
}
