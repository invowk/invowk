// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"fmt"
	"invowk-cli/internal/platform"
	"path/filepath"
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

// ValidateRegexPattern validates a user-provided regex pattern for safety and complexity.
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

// ValidateContainerImage validates a container image name format.
// Valid formats:
//   - image
//   - image:tag
//   - registry/image
//   - registry/image:tag
//   - registry:port/image:tag
//   - registry/namespace/image:tag
func ValidateContainerImage(image string) error {
	if image == "" {
		return nil // Empty is valid (will use Containerfile)
	}

	// Basic length check
	if len(image) > 512 {
		return fmt.Errorf("container image name too long (%d chars, max 512)", len(image))
	}

	// Check for obvious injection attempts
	if strings.ContainsAny(image, ";&|`$(){}[]<>\\'\"\n\r\t") {
		return fmt.Errorf("container image name contains invalid characters")
	}

	// Basic format validation using a permissive regex
	// Format: [registry[:port]/][namespace/]name[:tag][@digest]
	// Allow: registry:port/image, registry/namespace/image:tag, image@sha256:...
	imageRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._:/-]*[a-zA-Z0-9])?(:[a-zA-Z0-9._-]+)?(@sha256:[a-fA-F0-9]{64})?$`)
	if !imageRegex.MatchString(image) {
		return fmt.Errorf("container image name '%s' has invalid format", image)
	}

	return nil
}

// ValidateVolumeMount validates a container volume mount specification.
// Valid formats:
//   - /host/path:/container/path
//   - /host/path:/container/path:ro
//   - /host/path:/container/path:rw
//   - relative/path:/container/path
//   - named-volume:/container/path
func ValidateVolumeMount(volume string) error {
	if volume == "" {
		return fmt.Errorf("volume mount cannot be empty")
	}

	// Check length
	if len(volume) > 4096 {
		return fmt.Errorf("volume mount specification too long")
	}

	// Check for shell injection characters
	if strings.ContainsAny(volume, ";&|`$(){}[]<>\\'\"\n\r\t") {
		return fmt.Errorf("volume mount contains invalid characters")
	}

	// Split by colon - expect 2 or 3 parts
	parts := strings.Split(volume, ":")

	// Handle Windows paths with drive letters (e.g., C:\path:/container)
	if len(parts) >= 2 && len(parts[0]) == 1 && isWindowsDriveLetter(parts[0][0]) {
		// Reconstruct Windows path
		if len(parts) < 3 {
			return fmt.Errorf("volume mount '%s' has invalid format (expected host:container)", volume)
		}
		// Windows path: C:\path -> parts[0]="C", parts[1]="\path"
		// Rejoin: hostPath = "C:" + parts[1], containerPath = parts[2], options = parts[3:]
		parts = append([]string{parts[0] + ":" + parts[1]}, parts[2:]...)
	}

	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("volume mount '%s' has invalid format (expected host:container[:options])", volume)
	}

	hostPath := parts[0]
	containerPath := parts[1]

	// Validate host path is not empty
	if hostPath == "" {
		return fmt.Errorf("volume mount host path cannot be empty")
	}

	// Validate container path
	if containerPath == "" {
		return fmt.Errorf("volume mount container path cannot be empty")
	}
	if !strings.HasPrefix(containerPath, "/") {
		return fmt.Errorf("volume mount container path must be absolute (start with /)")
	}

	// Validate options if present
	if len(parts) == 3 {
		options := strings.ToLower(parts[2])
		validOptions := map[string]bool{
			"ro": true, "rw": true,
			"z": true, "Z": true, // SELinux labels
			"shared": true, "slave": true, "private": true,
			"rshared": true, "rslave": true, "rprivate": true,
			"nocopy": true, "copy": true,
		}
		for opt := range strings.SplitSeq(options, ",") {
			opt = strings.TrimSpace(opt)
			if opt == "" {
				continue
			}
			if !validOptions[opt] {
				return fmt.Errorf("volume mount has invalid option '%s'", opt)
			}
		}
	}

	// Check for sensitive path patterns (security)
	sensitivePaths := []string{
		"/etc/shadow", "/etc/passwd", "/etc/sudoers",
		"/.ssh", "/root/.ssh", "/home/*/.ssh",
		"/etc/ssl/private", "/var/run/docker.sock",
		"/proc", "/sys", "/dev",
	}
	lowerHost := strings.ToLower(hostPath)
	for _, sensitive := range sensitivePaths {
		if strings.Contains(sensitive, "*") {
			// Simple glob matching for patterns like /home/*/.ssh
			pattern := strings.ReplaceAll(sensitive, "*", "")
			parts := strings.Split(pattern, "/")
			if matchesSensitivePattern(lowerHost, parts) {
				return fmt.Errorf("volume mount attempts to mount sensitive path pattern '%s'", sensitive)
			}
		} else if strings.HasPrefix(lowerHost, sensitive) || lowerHost == sensitive {
			return fmt.Errorf("volume mount attempts to mount sensitive path '%s'", sensitive)
		}
	}

	return nil
}

// isWindowsDriveLetter returns true if c is a valid Windows drive letter.
func isWindowsDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// matchesSensitivePattern checks if a path matches a sensitive pattern.
func matchesSensitivePattern(path string, patternParts []string) bool {
	for _, part := range patternParts {
		if part == "" {
			continue
		}
		if strings.Contains(path, part) {
			return true
		}
	}
	return false
}

// ValidatePortMapping validates a container port mapping specification.
// Valid formats:
//   - containerPort
//   - hostPort:containerPort
//   - hostIP:hostPort:containerPort
//   - hostPort:containerPort/protocol
func ValidatePortMapping(port string) error {
	if port == "" {
		return fmt.Errorf("port mapping cannot be empty")
	}

	// Check for invalid characters
	if strings.ContainsAny(port, ";&|`$(){}[]<>\\'\"\n\r\t ") {
		return fmt.Errorf("port mapping contains invalid characters")
	}

	// Remove protocol suffix if present
	portSpec := port
	if idx := strings.LastIndex(port, "/"); idx != -1 {
		protocol := strings.ToLower(port[idx+1:])
		if protocol != "tcp" && protocol != "udp" && protocol != "sctp" {
			return fmt.Errorf("port mapping has invalid protocol '%s' (expected tcp, udp, or sctp)", protocol)
		}
		portSpec = port[:idx]
	}

	// Split by colon
	parts := strings.Split(portSpec, ":")

	if len(parts) > 3 {
		return fmt.Errorf("port mapping '%s' has invalid format", port)
	}

	// Validate each port number
	for i, part := range parts {
		if part == "" {
			if i == 0 && len(parts) == 3 {
				// Empty host IP is allowed
				continue
			}
			return fmt.Errorf("port mapping has empty port value")
		}

		// Check if it's an IP address (first part of 3-part format)
		if i == 0 && len(parts) == 3 {
			// This should be an IP address
			if !isValidIPAddress(part) {
				return fmt.Errorf("port mapping has invalid host IP '%s'", part)
			}
			continue
		}

		// Parse port number or range
		if strings.Contains(part, "-") {
			// Port range
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return fmt.Errorf("port mapping has invalid port range '%s'", part)
			}
			for _, rp := range rangeParts {
				if err := validatePortNumber(rp); err != nil {
					return err
				}
			}
		} else {
			if err := validatePortNumber(part); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePortNumber checks if a string is a valid port number (1-65535).
func validatePortNumber(s string) error {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fmt.Errorf("port '%s' is not a valid number", s)
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return fmt.Errorf("port %d is out of range (1-65535)", n)
		}
	}
	if n == 0 {
		return fmt.Errorf("port number cannot be 0")
	}
	return nil
}

// isValidIPAddress performs a simple validation of an IP address string.
func isValidIPAddress(s string) bool {
	// Accept IPv4 or IPv6
	if s == "" {
		return false
	}

	// Check for IPv6
	if strings.Contains(s, ":") {
		// Basic IPv6 validation
		return regexp.MustCompile(`^[a-fA-F0-9:]+$`).MatchString(s)
	}

	// IPv4 validation
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		n := 0
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}

// ValidateFilename checks if a filename is valid across platforms.
func ValidateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Check length
	if len(name) > 255 {
		return fmt.Errorf("filename too long (%d chars, max 255)", len(name))
	}

	// Check for invalid characters (common across platforms)
	invalidChars := []byte{'<', '>', ':', '"', '|', '?', '*', '\x00'}
	for _, c := range invalidChars {
		if strings.ContainsRune(name, rune(c)) {
			return fmt.Errorf("filename contains invalid character '%c'", c)
		}
	}

	// Check for control characters
	for _, r := range name {
		if r < 32 {
			return fmt.Errorf("filename contains control character")
		}
	}

	// Check for Windows reserved names
	if platform.IsWindowsReservedName(name) {
		return fmt.Errorf("filename '%s' is reserved on Windows", name)
	}

	// Check for names ending with space or period (invalid on Windows)
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("filename cannot end with space or period")
	}

	return nil
}

// ValidateContainerfilePath validates a containerfile path for security.
// It ensures paths are relative, don't escape the invkfile directory,
// and contain valid filename characters.
func ValidateContainerfilePath(containerfile, baseDir string) error {
	if containerfile == "" {
		return nil
	}

	// Check length limit
	if len(containerfile) > MaxPathLength {
		return fmt.Errorf("containerfile path too long (%d chars, max %d)", len(containerfile), MaxPathLength)
	}

	// Containerfile path must be relative (use cross-platform check)
	if isAbsolutePath(containerfile) {
		return fmt.Errorf("containerfile path must be relative, not absolute")
	}

	// Check for null bytes (security)
	if strings.ContainsRune(containerfile, '\x00') {
		return fmt.Errorf("containerfile path contains null byte")
	}

	// Convert to native path separators and resolve
	nativePath := filepath.FromSlash(containerfile)
	fullPath := filepath.Join(baseDir, nativePath)
	cleanPath := filepath.Clean(fullPath)

	// Verify the resolved path stays within baseDir
	relPath, err := filepath.Rel(baseDir, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("containerfile path '%s' escapes the invkfile directory", containerfile)
	}

	// Validate the filename component
	return ValidateFilename(filepath.Base(containerfile))
}

// ValidateEnvFilePath validates an env file path for security.
// Env file paths support an optional '?' suffix to mark the file as optional.
// It ensures paths are relative and don't contain path traversal sequences.
func ValidateEnvFilePath(filePath string) error {
	// Remove optional '?' suffix
	cleanPath := strings.TrimSuffix(filePath, "?")

	if cleanPath == "" {
		return fmt.Errorf("env file path cannot be empty")
	}

	// Check length limit
	if len(cleanPath) > MaxPathLength {
		return fmt.Errorf("env file path too long (%d chars, max %d)", len(cleanPath), MaxPathLength)
	}

	// Env file path must be relative (use cross-platform check)
	if isAbsolutePath(cleanPath) {
		return fmt.Errorf("env file path must be relative: %s", cleanPath)
	}

	// Check for null bytes (security)
	if strings.ContainsRune(cleanPath, '\x00') {
		return fmt.Errorf("env file path contains null byte")
	}

	// Check for path traversal sequences
	normalized := filepath.Clean(cleanPath)
	if strings.HasPrefix(normalized, "..") || strings.Contains(normalized, string(filepath.Separator)+"..") {
		return fmt.Errorf("env file path cannot contain '..': %s", filePath)
	}

	return nil
}

// ValidateFilepathDependency validates filepath dependency alternatives.
// These paths are checked at runtime, but we validate basic security constraints.
func ValidateFilepathDependency(paths []string) error {
	for i, path := range paths {
		if path == "" {
			return fmt.Errorf("filepath alternative #%d cannot be empty", i+1)
		}

		if len(path) > MaxPathLength {
			return fmt.Errorf("filepath alternative #%d too long (%d chars, max %d)", i+1, len(path), MaxPathLength)
		}

		// Check for null bytes (security)
		if strings.ContainsRune(path, '\x00') {
			return fmt.Errorf("filepath alternative #%d contains null byte", i+1)
		}
	}
	return nil
}

// toolNameRegex validates tool/binary names for security.
// Tool names must start with alphanumeric and can include . _ + -
var toolNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)

// ValidateToolName validates a tool/binary name.
// This is a Go-level backup to the CUE schema constraint.
func ValidateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("tool name too long (%d chars, max %d)", len(name), MaxNameLength)
	}
	if !toolNameRegex.MatchString(name) {
		return fmt.Errorf("tool name '%s' is invalid (must be alphanumeric, can include . _ + -)", name)
	}
	return nil
}

// cmdDependencyNameRegex validates command dependency names.
// Command names must start with a letter, can include letters, digits, underscores, hyphens, and spaces.
var cmdDependencyNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_ -]*$`)

// ValidateCommandDependencyName validates a command dependency name.
func ValidateCommandDependencyName(name string) error {
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("command name too long (%d chars, max %d)", len(name), MaxNameLength)
	}
	if !cmdDependencyNameRegex.MatchString(name) {
		return fmt.Errorf("command name '%s' is invalid (must start with letter, can include alphanumeric, underscores, hyphens, spaces)", name)
	}
	return nil
}

// isAbsolutePath checks if a path is absolute in either Unix or Windows format.
// Unlike filepath.IsAbs(), this function works cross-platform: it detects both
// Unix-style paths (/etc/passwd) and Windows-style paths (C:\Windows or C:/Windows)
// regardless of the host operating system. This is essential for security validation
// of user-provided paths that may originate from different platforms.
func isAbsolutePath(path string) bool {
	if path == "" {
		return false
	}

	// Unix-style absolute path
	if path[0] == '/' {
		return true
	}

	// Windows-style absolute path: drive letter + colon + path separator
	// Examples: "C:\Users" or "C:/Users"
	if len(path) >= 3 && isWindowsDriveLetter(path[0]) && path[1] == ':' {
		sep := path[2]
		return sep == '\\' || sep == '/'
	}

	return false
}
