// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// captureUserEnv captures the current environment as a map.
// This should be called at the start of execution to capture the user's
// actual environment before invowk sets any command-level env vars.
func captureUserEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if key, value, found := strings.Cut(e, "="); found {
			env[key] = value
		}
	}
	return env
}

// validateFlagValues validates flag values at runtime.
// It checks that required flags are provided and validates values against type and regex patterns.
func validateFlagValues(cmdName string, flagValues map[invowkfile.FlagName]string, flagDefs []invowkfile.Flag) error {
	if flagDefs == nil {
		return nil
	}

	var validationErrs []string

	for _, flag := range flagDefs {
		value, hasValue := flagValues[flag.Name]

		// Check required flags
		// Note: Cobra handles required flag checking via MarkFlagRequired,
		// but we double-check here for runtime validation (defense-in-depth for direct service calls)
		if flag.Required && (!hasValue || value == "") {
			validationErrs = append(validationErrs, fmt.Sprintf("required flag '--%s' was not provided", flag.Name))
			continue
		}

		// Validate the value if provided (skip empty values for non-required flags)
		if hasValue && value != "" {
			if err := flag.ValidateFlagValue(value); err != nil {
				validationErrs = append(validationErrs, err.Error())
			}
		}
	}

	if len(validationErrs) > 0 {
		return fmt.Errorf("flag validation failed for command '%s':\n  %s", cmdName, strings.Join(validationErrs, "\n  "))
	}

	return nil
}

// validateArguments validates provided arguments against their definitions.
// It returns an *ArgumentValidationError if validation fails.
func validateArguments(cmdName string, providedArgs []string, argDefs []invowkfile.Argument) error {
	if len(argDefs) == 0 {
		return nil // No argument definitions, allow any args (backward compatible)
	}

	// Count required args and check for variadic
	minArgs := 0
	maxArgs := len(argDefs)
	hasVariadic := false

	for _, arg := range argDefs {
		if arg.Required {
			minArgs++
		}
		if arg.Variadic {
			hasVariadic = true
		}
	}

	// Check minimum args
	if len(providedArgs) < minArgs {
		return &ArgumentValidationError{
			Type:         ArgErrMissingRequired,
			CommandName:  invowkfile.CommandName(cmdName),
			ArgDefs:      argDefs,
			ProvidedArgs: providedArgs,
			MinArgs:      minArgs,
			MaxArgs:      maxArgs,
		}
	}

	// Check maximum args (only if not variadic)
	if !hasVariadic && len(providedArgs) > maxArgs {
		return &ArgumentValidationError{
			Type:         ArgErrTooMany,
			CommandName:  invowkfile.CommandName(cmdName),
			ArgDefs:      argDefs,
			ProvidedArgs: providedArgs,
			MinArgs:      minArgs,
			MaxArgs:      maxArgs,
		}
	}

	// Validate each provided argument
	for i, argValue := range providedArgs {
		if i >= len(argDefs) {
			// Extra args go to the last (variadic) argument - already validated to have one
			break
		}

		argDef := argDefs[i]

		// For variadic args, validate all remaining values
		if argDef.Variadic {
			for j := i; j < len(providedArgs); j++ {
				if err := argDef.ValidateArgumentValue(providedArgs[j]); err != nil {
					return &ArgumentValidationError{
						Type:         ArgErrInvalidValue,
						CommandName:  invowkfile.CommandName(cmdName),
						ArgDefs:      argDefs,
						ProvidedArgs: providedArgs,
						InvalidArg:   argDef.Name,
						InvalidValue: providedArgs[j],
						ValueError:   err,
					}
				}
			}
			break
		}

		// Validate non-variadic argument
		if err := argDef.ValidateArgumentValue(argValue); err != nil {
			return &ArgumentValidationError{
				Type:         ArgErrInvalidValue,
				CommandName:  invowkfile.CommandName(cmdName),
				ArgDefs:      argDefs,
				ProvidedArgs: providedArgs,
				InvalidArg:   argDef.Name,
				InvalidValue: argValue,
				ValueError:   err,
			}
		}
	}

	return nil
}
