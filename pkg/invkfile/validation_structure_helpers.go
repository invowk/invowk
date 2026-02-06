// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"regexp"
)

// trimSpace removes leading and trailing whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isSpace reports whether the character is a whitespace character.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// containsNullByte reports whether the string contains a null byte.
func containsNullByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}

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
