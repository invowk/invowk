// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
)

// validateCustomChecks validates custom check dependencies for security and correctness.
// [CUE-VALIDATED] Basic structure and required fields are validated in CUE schema.
// [GO-ONLY] Security validations that require Go:
//   - String length limits (MaxNameLength, MaxScriptLength) for resource exhaustion prevention
//   - ReDoS pattern safety for expected_output regex patterns
func validateCustomChecks(checks []CustomCheckDependency, context, filePath string) error {
	for i, checkDep := range checks {
		// Get all checks (handles both direct and alternatives formats)
		for j, check := range checkDep.GetChecks() {
			// [GO-ONLY] Length limit validation - defense against resource exhaustion
			if check.Name != "" {
				if err := ValidateStringLength(check.Name, "custom_check name", MaxNameLength); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: %w in invowkfile at %s", context, i+1, j+1, err, filePath)
				}
			}

			// [GO-ONLY] Script length limit - defense against resource exhaustion
			if check.CheckScript != "" {
				if err := ValidateStringLength(check.CheckScript, "check_script", MaxScriptLength); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: %w in invowkfile at %s", context, i+1, j+1, err, filePath)
				}
			}

			// [GO-ONLY] ReDoS pattern safety - CUE cannot analyze regex complexity
			if check.ExpectedOutput != "" {
				if err := ValidateRegexPattern(check.ExpectedOutput); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: expected_output: %w in invowkfile at %s", context, i+1, j+1, err, filePath)
				}
			}
		}
	}
	return nil
}
