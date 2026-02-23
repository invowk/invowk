// SPDX-License-Identifier: MPL-2.0

package invowkfile

import "strings"

// validateDependsOn validates all dependency types in a DependsOn struct.
func (v *StructureValidator) validateDependsOn(ctx *ValidationContext, deps *DependsOn, basePath *FieldPath) []ValidationError {
	if deps == nil {
		return nil
	}

	var errors []ValidationError

	// Validate tool dependencies
	for i, dep := range deps.Tools {
		for j, alt := range dep.Alternatives {
			if err := ValidateToolName(string(alt)); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().Tools(i, j).String(),
					Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
		}
	}

	// Validate command dependencies
	for i, dep := range deps.Commands {
		for j, alt := range dep.Alternatives {
			if err := ValidateCommandDependencyName(string(alt)); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().Commands(i, j).String(),
					Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
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
				Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	// Validate env var dependencies
	for i, dep := range deps.EnvVars {
		for j, alt := range dep.Alternatives {
			name := strings.TrimSpace(string(alt.Name))
			if err := ValidateEnvVarName(name); err != nil {
				errors = append(errors, ValidationError{
					Validator: v.Name(),
					Field:     basePath.Copy().EnvVars(i, j).Field("name").String(),
					Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
					Severity:  SeverityError,
				})
			}
			if alt.Validation != "" {
				if err := ValidateRegexPattern(string(alt.Validation)); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     basePath.Copy().EnvVars(i, j).Field("validation").String(),
						Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}
		}
	}

	// Validate custom check dependencies (security-specific: length limits, ReDoS safety)
	errors = append(errors, v.validateCustomChecks(ctx, deps.CustomChecks, basePath)...)

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
				if err := ValidateStringLength(string(check.Name), "custom_check name", MaxNameLength); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     path.String(),
						Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}

			// [CUE-VALIDATED] Check script length also enforced by CUE schema (#CustomCheck.check_script MaxRunes(10485760))
			if check.CheckScript != "" {
				if err := ValidateStringLength(string(check.CheckScript), "check_script", MaxScriptLength); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     path.String(),
						Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
						Severity:  SeverityError,
					})
				}
			}

			// ReDoS pattern safety - CUE cannot analyze regex complexity
			if check.ExpectedOutput != "" {
				if err := ValidateRegexPattern(string(check.ExpectedOutput)); err != nil {
					errors = append(errors, ValidationError{
						Validator: v.Name(),
						Field:     path.String(),
						Message:   "expected_output: " + err.Error() + " in invowkfile at " + ctx.FilePath,
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
		if err := ValidateEnvFilePath(string(file)); err != nil {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvFile(i).String(),
				Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
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
				Message:   err.Error() + " in invowkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
		// [CUE-VALIDATED] Env var value length also enforced by CUE schema (#EnvConfig.vars MaxRunes(32768))
		if len(value) > MaxEnvVarValueLength {
			errors = append(errors, ValidationError{
				Validator: v.Name(),
				Field:     basePath.Copy().EnvVar(key).String(),
				Message:   "value too long (" + itoa(len(value)) + " chars, max " + itoa(MaxEnvVarValueLength) + ") in invowkfile at " + ctx.FilePath,
				Severity:  SeverityError,
			})
		}
	}

	return errors
}
