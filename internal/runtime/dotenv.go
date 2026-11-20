// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	// ErrEnvFileOutsideModule is returned when an env file of an invowkfile
	// loaded from a module does not resolve inside the module directory: an
	// absolute path, a path that leaves the module, a link to outside it, a
	// link with an absolute target, or a link that does not resolve (SC-01,
	// finding F17).
	ErrEnvFileOutsideModule = errors.New("env file resolves outside module")

	// errEnvFileEscapes marks a contained-read error that
	// classifyModuleEnvFileError judged an escape; LoadEnvFile turns it into
	// ErrEnvFileOutsideModule.
	errEnvFileEscapes = errors.New("contained read escapes the module")

	// errRootResolverPanic marks a panic recovered from os.Root (rootCall).
	errRootResolverPanic = errors.New("os.Root path resolution panicked")
)

// LoadEnvFile loads a dotenv file and merges its contents into the provided env map.
// The path is resolved relative to basePath (invowkfile directory or module root).
// Files suffixed with '?' are optional; missing optional files do not cause an error.
// Later calls to LoadEnvFile override earlier values for the same keys.
//
// moduleRoot is the module directory when the invowkfile was loaded from a
// module, and "" otherwise. With a module root the read is physically
// contained (design D3 of module-read-physical-containment): the file is read
// through an os.Root opened on moduleRoot, so every link on the path is
// followed by the filesystem and an escape fails with ErrEnvFileOutsideModule,
// an optional file included. An optional file is skipped only when the
// contained read reports that nothing exists (the module root is missing, or
// Stat and Lstat both report not-exist). Without a module root the path is
// read as before, and links on it are followed wherever they lead.
func LoadEnvFile(env map[string]string, path, basePath, moduleRoot string) error {
	optional := strings.HasSuffix(path, "?")
	if optional {
		path = strings.TrimSuffix(path, "?")
	}

	if moduleRoot != "" {
		// Module context (SC-01): the module names this file, so it must
		// resolve inside the module directory.
		if filepath.IsAbs(path) {
			return envFileOutsideModuleError(path, moduleRoot, "is an absolute path outside", nil)
		}
		rel, relErr := filepath.Rel(moduleRoot, filepath.Join(basePath, filepath.FromSlash(path)))
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return envFileOutsideModuleError(path, moduleRoot, "resolves outside", relErr)
		}
		root, rootErr := os.OpenRoot(moduleRoot)
		if rootErr != nil {
			if optional && errors.Is(rootErr, fs.ErrNotExist) {
				return nil // a missing module directory holds no file and no link
			}
			return fmt.Errorf("failed to read env file '%s': %w", path, rootErr)
		}
		defer func() { _ = root.Close() }() //nolint:errcheck // read-only os.Root; a close error is not actionable

		moduleContent, absent, readErr := readModuleEnvFile(root, rel)
		if readErr != nil {
			if optional && absent {
				return nil // Optional file missing is OK
			}
			if errors.Is(readErr, errEnvFileEscapes) {
				return envFileOutsideModuleError(path, moduleRoot, "resolves outside", readErr)
			}
			return fmt.Errorf("failed to read env file '%s': %w", path, readErr)
		}
		return ParseEnvFile(env, moduleContent, path)
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

// readModuleEnvFile reads rel through root: Stat first, so an unresolvable link
// is told apart from a missing file (Lstat), then ReadFile, whose content read
// is race-free. Both errors go through classifyModuleEnvFileError.
//
//goplint:ignore -- rel is a transient os.Root-relative name and content raw dotenv bytes, as with os.ReadFile.
func readModuleEnvFile(root *os.Root, rel string) (content []byte, absent bool, err error) {
	return readModuleEnvFileWith(root, rel, root.Stat, root.ReadFile)
}

// readModuleEnvFileWith is readModuleEnvFile with the Stat and ReadFile calls
// injected, so a test can drive the recovered-panic branches; Lstat still runs
// on root.
//
//goplint:ignore -- rel is a transient os.Root-relative name and content raw dotenv bytes, as with os.ReadFile.
func readModuleEnvFileWith(root *os.Root, rel string, stat func(string) (fs.FileInfo, error), readFile func(string) ([]byte, error)) (content []byte, absent bool, err error) {
	if _, statErr := rootCall(stat, rel); statErr != nil {
		if errors.Is(statErr, errRootResolverPanic) {
			return nil, false, statErr // a link to its own directory: a read error
		}
		_, lstatErr := root.Lstat(rel)
		absent, err = classifyModuleEnvFileError(statErr, lstatErr)
		return nil, absent, err
	}
	content, err = rootCall(readFile, rel)
	if err != nil {
		if errors.Is(err, errRootResolverPanic) {
			return nil, false, err
		}
		// After a successful Stat, a non-OS error is an escape that appeared
		// since (os.Root's "path escapes from parent"); an OS error stays a
		// read error, or absent when the file vanished.
		absent, err = classifyModuleEnvFileError(err, err)
		return nil, absent, err
	}
	return content, false, nil
}

// rootCall runs an os.Root method on rel and turns a panic into an error
// wrapping errRootResolverPanic. Go 1.27's os.Root on Windows panics (index out
// of range in doInRoot) when the last path component is a link whose target
// resolves to the directory that holds it, such as a/x -> "." or
// a/b/x -> "../b". Such a link names a directory inside the root, which holds
// no env content, so LoadEnvFile reports a read error, as it does for a
// directory on the other platforms.
//
//goplint:ignore -- rel is a transient os.Root-relative name, as with the os.Root methods.
func rootCall[T any](call func(string) (T, error), rel string) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T
			v, err = zero, fmt.Errorf("%w: %v", errRootResolverPanic, r)
		}
	}()
	return call(rel)
}

// classifyModuleEnvFileError classifies an os.Root error on a module env file.
// lstatErr is the result of Lstat on the same path (nil when it succeeded):
//   - a non-OS error (os.Root rejected the path) is an escape;
//   - an OS error while Lstat succeeds is an unresolvable link (dangling, a
//     broken chain or a cycle), also reported as an escape;
//   - not-exist from both is absent: only then may an optional file be skipped;
//   - any other OS error is a read error.
//
// Escapes wrap errEnvFileEscapes and the cause.
func classifyModuleEnvFileError(err, lstatErr error) (absent bool, classified error) {
	switch {
	case !isOSError(err):
		return false, fmt.Errorf("%w: %w", errEnvFileEscapes, err)
	case lstatErr == nil:
		return false, fmt.Errorf("%w: link does not resolve: %w", errEnvFileEscapes, err)
	case errors.Is(err, fs.ErrNotExist) && errors.Is(lstatErr, fs.ErrNotExist):
		return true, err
	default:
		return false, err
	}
}

// isOSError reports whether err carries an operating-system error number.
// os.Root reports a path that leaves the root with a plain (non-errno) error.
func isOSError(err error) bool {
	_, ok := errors.AsType[syscall.Errno](err) //nolint:errcheck // only the type-match verdict matters
	return ok
}

// envFileOutsideModuleError wraps ErrEnvFileOutsideModule with the path, the
// module root, the case and, when known, its cause.
//
//goplint:ignore -- path, moduleRoot and how are display text for the error message.
func envFileOutsideModuleError(path, moduleRoot, how string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: env file '%s' %s module '%s'", ErrEnvFileOutsideModule, path, how, moduleRoot)
	}
	return fmt.Errorf("%w: env file '%s' %s module '%s': %w", ErrEnvFileOutsideModule, path, how, moduleRoot, cause)
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
				return "", errors.New("unterminated double quote")
			}
			// Process escape sequences
			return parseDoubleQuotedValue(value[1 : len(value)-1])
		}
		if value[0] == '\'' {
			// Single-quoted value
			if len(value) < 2 || value[len(value)-1] != '\'' {
				return "", errors.New("unterminated single quote")
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
