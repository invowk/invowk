// SPDX-License-Identifier: MPL-2.0

package invowkfile

// StructureValidator validates the structural correctness of an invowkfile.
// It checks command structure, implementations, runtimes, flags, args, and dependencies.
// This validator wraps all the existing validation logic and collects ALL errors.
//
// Validation methods are organized across focused files by concern:
//   - validation_structure_command.go: command, implementation, and runtime config validation
//   - validation_structure_flags.go: flag name, type, short alias, and reserved name validation
//   - validation_structure_args.go: argument name, type, ordering, and variadic validation
//   - validation_structure_deps.go: dependency (tools, commands, filepaths, env vars), custom checks, and env config
//   - validation_structure_helpers.go: shared string utilities (trimSpace, containsNullByte, regexpMatch)
type StructureValidator struct{}

// NewStructureValidator creates a new StructureValidator.
func NewStructureValidator() *StructureValidator {
	return &StructureValidator{}
}

// Name returns the validator name.
func (v *StructureValidator) Name() ValidatorName {
	return "structure"
}

// Validate checks the invowkfile structure and collects all validation errors.
func (v *StructureValidator) Validate(ctx *ValidationContext, inv *Invowkfile) []ValidationError {
	var errors []ValidationError

	if len(inv.Commands) == 0 {
		errors = append(errors, ValidationError{
			Validator: v.Name(),
			Field:     "",
			Message:   "invowkfile at " + string(ctx.FilePath) + " has no commands defined (missing required 'cmds' list)",
			Severity:  SeverityError,
		})
		return errors // No point validating further if there are no commands
	}

	// Validate root-level env configuration
	errors = append(errors, v.validateEnvConfig(ctx, inv.Env, NewFieldPath().Root())...)

	// Validate root-level depends_on (all dependency types including custom checks)
	errors = append(errors, v.validateDependsOn(ctx, inv.DependsOn, NewFieldPath().Root())...)

	// Validate each command
	for i := range inv.Commands {
		errors = append(errors, v.validateCommand(ctx, inv, &inv.Commands[i])...)
	}

	return errors
}
