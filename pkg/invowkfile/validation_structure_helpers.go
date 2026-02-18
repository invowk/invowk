// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"regexp"
)

// matchesValidation checks if a value matches a validation regex pattern.
// Returns false if the pattern is invalid or doesn't match.
func matchesValidation(value, pattern string) bool {
	matched, err := regexpMatch(pattern, value)
	return err == nil && matched
}

// regexpMatch compiles a pattern and matches it against a string.
// Returns (matched, nil) on success, or (false, error) if the pattern is invalid.
func regexpMatch(pattern, s string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compiling regex %q: %w", pattern, err)
	}
	return re.MatchString(s), nil
}
