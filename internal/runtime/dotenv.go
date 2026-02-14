// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFile loads a dotenv file and merges its contents into the provided env map.
// The path is resolved relative to basePath (invowkfile directory or module root).
// Files suffixed with '?' are optional; missing optional files do not cause an error.
// Later calls to LoadEnvFile override earlier values for the same keys.
func LoadEnvFile(env map[string]string, path, basePath string) error {
	optional := strings.HasSuffix(path, "?")
	if optional {
		path = strings.TrimSuffix(path, "?")
	}

	// Resolve relative paths against basePath
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		// Convert forward slashes to native path separator for cross-platform compatibility
		nativePath := filepath.FromSlash(path)
		fullPath = filepath.Join(basePath, nativePath)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if optional && os.IsNotExist(err) {
			return nil // Optional file missing is OK
		}
		return fmt.Errorf("failed to read env file '%s': %w", path, err)
	}

	return ParseEnvFile(env, content, path)
}

// LoadEnvFileFromCwd loads a dotenv file relative to the working directory.
// This is used for --ivk-env-file flag files specified at runtime.
// When cwd is empty, os.Getwd() is used as the fallback.
// Files suffixed with '?' are optional; missing optional files do not cause an error.
func LoadEnvFileFromCwd(env map[string]string, path, cwd string) error {
	optional := strings.HasSuffix(path, "?")
	if optional {
		path = strings.TrimSuffix(path, "?")
	}

	// --ivk-env-file paths are relative to CWD (where invowk was invoked)
	// If path is already absolute, use it as-is
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
		}
		fullPath = filepath.Join(cwd, path)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if optional && os.IsNotExist(err) {
			return nil // Optional file missing is OK
		}
		return fmt.Errorf("failed to read env file '%s': %w", path, err)
	}

	return ParseEnvFile(env, content, path)
}

// ParseEnvFile parses dotenv format content and merges into the env map.
// Supported format:
//   - Lines starting with # are comments
//   - Empty lines are ignored
//   - KEY=value (unquoted)
//   - KEY="value" (double-quoted, escape sequences: \n, \r, \t, \\, \")
//   - KEY='value' (single-quoted, literal - no escape processing)
//   - export KEY=value (export prefix is optional and ignored)
//   - KEY= (empty value)
//
// The filename parameter is used for error messages.
func ParseEnvFile(env map[string]string, content []byte, filename string) error {
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		lineNum := i + 1

		// Trim trailing carriage return (for Windows line endings)
		line = strings.TrimSuffix(line, "\r")
		// Trim leading and trailing whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove optional 'export ' prefix
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		// Split on first '='
		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("%s:%d: invalid format (missing '=')", filename, lineNum)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("%s:%d: empty variable name", filename, lineNum)
		}

		// Parse value based on quoting
		parsedValue, err := parseEnvValue(value)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", filename, lineNum, err)
		}

		env[key] = parsedValue
	}

	return nil
}

// parseEnvValue parses a dotenv value, handling quoting and escape sequences.
func parseEnvValue(value string) (string, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return "", nil
	}

	// Check for quoted values
	if len(value) >= 1 {
		if value[0] == '"' {
			// Double-quoted value
			if len(value) < 2 || value[len(value)-1] != '"' {
				return "", fmt.Errorf("unterminated double quote")
			}
			// Process escape sequences
			return parseDoubleQuotedValue(value[1 : len(value)-1])
		}
		if value[0] == '\'' {
			// Single-quoted value
			if len(value) < 2 || value[len(value)-1] != '\'' {
				return "", fmt.Errorf("unterminated single quote")
			}
			// Single-quoted: literal value, no escape processing
			return value[1 : len(value)-1], nil
		}
	}

	// Unquoted: strip inline comments and return
	// Look for # that's not inside the value
	if idx := strings.Index(value, " #"); idx != -1 {
		value = strings.TrimSpace(value[:idx])
	}

	return value, nil
}

// parseDoubleQuotedValue processes escape sequences in a double-quoted value.
func parseDoubleQuotedValue(value string) (string, error) {
	var result strings.Builder
	result.Grow(len(value))

	i := 0
	for i < len(value) {
		if value[i] == '\\' && i+1 < len(value) {
			// Escape sequence
			next := value[i+1]
			switch next {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			case '\\':
				result.WriteByte('\\')
			case '"':
				result.WriteByte('"')
			case '$':
				result.WriteByte('$')
			default:
				// Unknown escape - keep both characters
				result.WriteByte('\\')
				result.WriteByte(next)
			}
			i += 2
		} else {
			result.WriteByte(value[i])
			i++
		}
	}

	return result.String(), nil
}
