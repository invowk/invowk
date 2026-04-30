// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ValidateFlagValues validates flag values at runtime.
// It checks that required flags are provided and validates values against type and regex patterns.
func ValidateFlagValues(cmdName string, flagValues map[invowkfile.FlagName]string, flagDefs []invowkfile.Flag) error {
	if flagDefs == nil {
		return nil
	}

	var validationErrs []DependencyMessage

	for _, flag := range flagDefs {
		value, hasValue := flagValues[flag.Name]

		// Check required flags
		// Note: Cobra handles required flag checking via MarkFlagRequired,
		// but we double-check here for runtime validation (defense-in-depth for direct service calls)
		if flag.Required && (!hasValue || value == "") {
			validationErrs = append(validationErrs, dependencyMessageFromDetail("required flag '--"+string(flag.Name)+"' was not provided"))
			continue
		}

		// Validate the value if provided (skip empty values for non-required flags)
		if hasValue && value != "" {
			if err := flag.ValidateFlagValue(value); err != nil {
				validationErrs = append(validationErrs, dependencyMessageFromDetail(err.Error()))
			}
		}
	}

	if len(validationErrs) > 0 {
		return &FlagValidationError{
			CommandName: invowkfile.CommandName(cmdName), //goplint:ignore -- display value in validation error type
			Failures:    validationErrs,
		}
	}

	return nil
}

// ValidateArguments validates provided arguments against their definitions.
// It returns an *ArgumentValidationError if validation fails.
func ValidateArguments(cmdName string, providedArgs []string, argDefs []invowkfile.Argument) error {
	if len(argDefs) == 0 {
		return nil // No argument definitions, allow any args (backward compatible)
	}

	minArgs, maxArgs, hasVariadic := summarizeArgDefs(argDefs)
	if err := validateArgumentCount(cmdName, providedArgs, argDefs, minArgs, maxArgs, hasVariadic); err != nil {
		return err
	}
	return validateArgumentValues(cmdName, providedArgs, argDefs)
}

//goplint:ignore -- argument-validation helpers intentionally operate on raw argv counts and slices.
func summarizeArgDefs(argDefs []invowkfile.Argument) (minArgs, maxArgs int, hasVariadic bool) {
	maxArgs = len(argDefs)
	for _, arg := range argDefs {
		if arg.Required {
			minArgs++
		}
		if arg.Variadic {
			hasVariadic = true
		}
	}
	return minArgs, maxArgs, hasVariadic
}

//goplint:ignore -- argument-validation helpers intentionally operate on raw argv counts and slices.
func validateArgumentCount(cmdName string, providedArgs []string, argDefs []invowkfile.Argument, minArgs, maxArgs int, hasVariadic bool) error {
	if len(providedArgs) < minArgs {
		return newArgumentCountError(ArgErrMissingRequired, cmdName, providedArgs, argDefs, minArgs, maxArgs)
	}
	if !hasVariadic && len(providedArgs) > maxArgs {
		return newArgumentCountError(ArgErrTooMany, cmdName, providedArgs, argDefs, minArgs, maxArgs)
	}
	return nil
}

//goplint:ignore -- argument-validation helpers intentionally operate on raw argv slices.
func validateArgumentValues(cmdName string, providedArgs []string, argDefs []invowkfile.Argument) error {
	for i, argValue := range providedArgs {
		if i >= len(argDefs) {
			break
		}

		argDef := argDefs[i]
		if argDef.Variadic {
			return validateVariadicArgumentValues(cmdName, providedArgs, argDefs, i, argDef)
		}
		if err := argDef.ValidateArgumentValue(argValue); err != nil {
			return newArgumentValueError(cmdName, providedArgs, argDefs, argDef.Name, argValue, err)
		}
	}
	return nil
}

//goplint:ignore -- argument-validation helpers intentionally operate on raw argv slices and indices.
func validateVariadicArgumentValues(cmdName string, providedArgs []string, argDefs []invowkfile.Argument, start int, argDef invowkfile.Argument) error {
	for i := start; i < len(providedArgs); i++ {
		if err := argDef.ValidateArgumentValue(providedArgs[i]); err != nil {
			return newArgumentValueError(cmdName, providedArgs, argDefs, argDef.Name, providedArgs[i], err)
		}
	}
	return nil
}

//goplint:ignore -- argument-validation helpers intentionally operate on raw argv counts and slices.
func newArgumentCountError(kind ArgErrType, cmdName string, providedArgs []string, argDefs []invowkfile.Argument, minArgs, maxArgs int) error {
	return &ArgumentValidationError{
		Type:         kind,
		CommandName:  invowkfile.CommandName(cmdName), //goplint:ignore -- display value in error type
		ArgDefs:      argDefs,
		ProvidedArgs: providedArgs,
		MinArgs:      minArgs,
		MaxArgs:      maxArgs,
	}
}

//goplint:ignore -- argument-validation helpers intentionally operate on raw argv values.
func newArgumentValueError(cmdName string, providedArgs []string, argDefs []invowkfile.Argument, argName invowkfile.ArgumentName, value string, valueErr error) error {
	return &ArgumentValidationError{
		Type:         ArgErrInvalidValue,
		CommandName:  invowkfile.CommandName(cmdName), //goplint:ignore -- display value in error type
		ArgDefs:      argDefs,
		ProvidedArgs: providedArgs,
		InvalidArg:   argName,
		InvalidValue: value,
		ValueError:   valueErr,
	}
}
