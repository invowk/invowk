// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
)

// validateRuntimeConfig checks that a runtime configuration is valid.
// Note: Format validation (non-empty interpreter, valid env var names) is handled by the CUE schema.
// This function focuses on Go-only validations: cross-field logic, filesystem access, and security checks.
func validateRuntimeConfig(rt *RuntimeConfig, cmdName string, implIndex int) error {
	if err := rt.Validate(); err != nil {
		if invalid, ok := errors.AsType[*InvalidRuntimeConfigError](err); ok && len(invalid.FieldErrors) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: %w", cmdName, implIndex, invalid.FieldErrors[0])
		}
		return fmt.Errorf("command '%s' implementation #%d: %w", cmdName, implIndex, err)
	}
	return nil
}
