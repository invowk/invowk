// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"regexp"
	"strings"
)

// Validation limits to prevent resource exhaustion
const (
	// MaxRegexPatternLength is the maximum allowed length for user-provided regex patterns
	MaxRegexPatternLength = 1000
	// MaxScriptLength is the maximum allowed length for script content (10 MB)
	MaxScriptLength = 10 * 1024 * 1024
	// MaxDescriptionLength is the maximum allowed length for description fields
	MaxDescriptionLength = 10 * 1024
	// MaxNameLength is the maximum allowed length for command/flag/arg names
	MaxNameLength = 256
	// MaxNestedGroups is the maximum depth of nested groups in regex patterns
	MaxNestedGroups = 10
	// MaxQuantifierRepeats limits how many repetition operators can appear in a pattern
	MaxQuantifierRepeats = 20
	// MaxPathLength is the maximum allowed length for file paths
	MaxPathLength = 4096
	// MaxShellPathLength is the maximum allowed length for shell/interpreter paths
	MaxShellPathLength = 1024
	// MaxEnvVarValueLength is the maximum allowed length for environment variable values
	MaxEnvVarValueLength = 32768 // 32 KB
	// MaxGitURLLength is the maximum allowed length for Git repository URLs
	MaxGitURLLength = 2048
)

var (
	// toolNameRegex validates tool/binary names for security.
	// Tool names must start with alphanumeric and can include . _ + -
	toolNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)

	// cmdDependencyNameRegex validates command dependency names.
	// Command names must start with a letter, can include letters, digits, underscores, hyphens, and spaces.
	cmdDependencyNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_ -]*$`)
)

// ValidateRegexPattern validates a user-provided regex pattern for safety and complexity.
// [GO-ONLY] ReDoS (Regular Expression Denial of Service) prevention MUST be in Go.
// CUE cannot analyze regex complexity or detect catastrophic backtracking patterns.
// This is a security-critical validation that protects against malicious user input.
//
// It checks for:
// - Pattern length limits
// - Dangerous patterns that could cause catastrophic backtracking
// - Excessive nesting depth
// - Excessive quantifier usage
//
// Returns an error if the pattern is considered unsafe.
func ValidateRegexPattern(pattern string) error {
	if pattern == "" {
		return nil
	}

	// Check length limit
	if len(pattern) > MaxRegexPatternLength {
		return fmt.Errorf("regex pattern too long (%d chars, max %d)", len(pattern), MaxRegexPatternLength)
	}

	// Check for dangerous patterns (simplified check)
	if err := checkDangerousPatterns(pattern); err != nil {
		return err
	}

	// Check nesting depth
	if err := checkNestingDepth(pattern); err != nil {
		return err
	}

	// Check quantifier count
	if err := checkQuantifierCount(pattern); err != nil {
		return err
	}

	// Verify the pattern compiles (final validation)
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}

	return nil
}

// checkDangerousPatterns looks for patterns known to cause catastrophic backtracking.
// This is a heuristic check, not exhaustive.
func checkDangerousPatterns(pattern string) error {
	// Check for nested quantifiers: patterns like (x+)+ or (x*)* or (x+)*
	// These are the most common cause of regex DOS

	// Pattern to detect nested quantifiers on groups
	// Look for: group with quantifier inside, followed by another quantifier
	// Examples: (a+)+, (a*)+, (.+)*, (a|b+)+

	// Simple heuristic: look for quantifier immediately after a group that contains a quantifier
	nestedQuantifierPattern := regexp.MustCompile(`\([^)]*[+*][^)]*\)[+*?]|\([^)]*[+*][^)]*\)\{`)
	if nestedQuantifierPattern.MatchString(pattern) {
		return fmt.Errorf("regex pattern contains nested quantifiers which may cause performance issues")
	}

	// Check for repetition on alternation with overlapping patterns
	// Example: (a|a)+ or (aa|a)+
	// This is harder to detect perfectly, so we use a simpler heuristic
	alternationRepeatPattern := regexp.MustCompile(`\([^)]*\|[^)]*\)[+*]\{?\d*,?\d*\}?`)
	if alternationRepeatPattern.MatchString(pattern) {
		// Only flag if both sides of alternation have similar starting patterns
		// This is a conservative check - we allow most alternations
		if hasOverlappingAlternation(pattern) {
			return fmt.Errorf("regex pattern contains alternation with potentially overlapping patterns and quantifiers")
		}
	}

	return nil
}

// hasOverlappingAlternation checks if an alternation has obviously overlapping patterns.
// This is a simplified heuristic check.
func hasOverlappingAlternation(pattern string) bool {
	// Extract alternation groups
	altGroupRegex := regexp.MustCompile(`\(([^)]+)\)[+*]`)
	matches := altGroupRegex.FindAllStringSubmatch(pattern, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		groupContent := match[1]
		parts := strings.Split(groupContent, "|")
		if len(parts) < 2 {
			continue
		}

		// Check if any two parts have the same starting character or one is prefix of another
		for i := range len(parts) {
			for j := i + 1; j < len(parts); j++ {
				p1 := strings.TrimSpace(parts[i])
				p2 := strings.TrimSpace(parts[j])
				if p1 == "" || p2 == "" {
					continue
				}
				// Check if one is prefix of another or they share a prefix
				if strings.HasPrefix(p1, p2) || strings.HasPrefix(p2, p1) {
					return true
				}
				// Check if they start with the same literal character
				if p1 != "" && p2 != "" && p1[0] == p2[0] && isLiteralChar(p1[0]) {
					return true
				}
			}
		}
	}

	return false
}

// isLiteralChar returns true if the character is a literal (not a regex metacharacter)
func isLiteralChar(c byte) bool {
	switch c {
	case '.', '*', '+', '?', '[', ']', '(', ')', '{', '}', '|', '^', '$', '\\':
		return false
	default:
		return true
	}
}

// checkNestingDepth counts the maximum depth of nested groups.
func checkNestingDepth(pattern string) error {
	maxDepth := 0
	currentDepth := 0
	escaped := false

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if escaped {
			escaped = false
			continue
		}
		switch c {
		case '\\':
			escaped = true
			continue
		case '(':
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		case ')':
			if currentDepth > 0 {
				currentDepth--
			}
		}
	}

	if maxDepth > MaxNestedGroups {
		return fmt.Errorf("regex pattern has too many nested groups (%d, max %d)", maxDepth, MaxNestedGroups)
	}

	return nil
}

// checkQuantifierCount counts the number of quantifiers in the pattern.
func checkQuantifierCount(pattern string) error {
	count := 0
	escaped := false
	inCharClass := false

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '[' && !inCharClass {
			inCharClass = true
			continue
		}
		if c == ']' && inCharClass {
			inCharClass = false
			continue
		}
		if inCharClass {
			continue
		}
		// Count quantifiers
		if c == '*' || c == '+' || c == '?' {
			count++
		} else if c == '{' {
			// Check if this is a quantifier like {n} or {n,m}
			for j := i + 1; j < len(pattern); j++ {
				if pattern[j] == '}' {
					count++
					break
				}
				if pattern[j] != ',' && (pattern[j] < '0' || pattern[j] > '9') {
					break
				}
			}
		}
	}

	if count > MaxQuantifierRepeats {
		return fmt.Errorf("regex pattern has too many quantifiers (%d, max %d)", count, MaxQuantifierRepeats)
	}

	return nil
}

// ValidateStringLength checks if a string exceeds the maximum length.
func ValidateStringLength(value, fieldName string, maxLen int) error {
	if len(value) > maxLen {
		return fmt.Errorf("%s too long (%d chars, max %d)", fieldName, len(value), maxLen)
	}
	return nil
}
