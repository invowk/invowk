// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/fspath"
)

var (
	// scriptFileExtensions contains extensions that indicate a script file
	scriptFileExtensions = []string{".sh", ".bash", ".ps1", ".bat", ".cmd", ".py", ".rb", ".pl", ".zsh", ".fish"}

	// ErrInvalidImplementation is the sentinel error wrapped by InvalidImplementationError.
	ErrInvalidImplementation = errors.New("invalid implementation")

	// ErrInvalidImplementationMatch is the sentinel error wrapped by InvalidImplementationMatchError.
	ErrInvalidImplementationMatch = errors.New("invalid implementation match")
)

type (
	// InvalidImplementationError is returned when an Implementation has invalid fields.
	// It wraps ErrInvalidImplementation for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidImplementationError struct {
		FieldErrors []error
	}

	// InvalidImplementationMatchError is returned when an ImplementationMatch has invalid fields.
	// It wraps ErrInvalidImplementationMatch for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidImplementationMatchError struct {
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// Implementation represents an implementation with platform and runtime constraints
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
	Implementation struct {
		// Script contains the shell commands to execute OR a path to a script file
		Script ScriptContent `json:"script"`
		// Runtimes specifies which runtimes can execute this implementation (required, at least one)
		// The first element is the default runtime for this platform combination
		// Each runtime is a struct with a Name field and optional type-specific fields
		Runtimes []RuntimeConfig `json:"runtimes"`
		// Platforms specifies which operating systems this implementation is for (required, at least one)
		// Each platform is a struct with a Name field
		Platforms []PlatformConfig `json:"platforms"`
		// Env contains environment configuration for this implementation (optional)
		// Implementation-level env is merged with command-level env.
		// Implementation files are loaded after command-level files.
		// Implementation vars override command-level vars.
		Env *EnvConfig `json:"env,omitempty"`
		// WorkDir specifies the working directory for this implementation (optional)
		// Overrides both root-level and command-level workdir settings.
		// Can be absolute or relative to the invowkfile location.
		// Forward slashes should be used for cross-platform compatibility.
		WorkDir WorkDir `json:"workdir,omitempty"`
		// DependsOn specifies dependencies validated against the HOST system.
		// Regardless of the selected runtime, these are always checked on the host.
		// To validate dependencies inside the runtime environment (e.g., inside a container),
		// use DependsOn inside the RuntimeConfig instead.
		DependsOn *DependsOn `json:"depends_on,omitempty"`
		// Timeout specifies the maximum execution duration (optional).
		// Must be a valid Go duration string (e.g., "30s", "5m", "1h30m").
		// When exceeded, the command is cancelled and returns a timeout error.
		Timeout DurationString `json:"timeout,omitempty"`

		// resolvedScript caches the resolved script content (lazy memoization).
		// Script content is resolved from file path or inline source on first
		// ResolveScript call and reused for subsequent calls.
		resolvedScript ScriptContent
		// scriptResolved tracks whether resolvedScript has been populated.
		scriptResolved bool
	}

	// PlatformRuntimeKey represents a unique combination of platform and runtime
	PlatformRuntimeKey struct {
		Platform Platform
		Runtime  RuntimeMode
	}

	//goplint:validate-all
	//
	// ImplementationMatch represents a matched implementation for execution.
	ImplementationMatch struct {
		Implementation       *Implementation
		Platform             Platform
		Runtime              RuntimeMode
		IsDefaultForPlatform bool
	}
)

// Validate returns nil if both Platform and Runtime in the key are valid,
// or a combined error from both fields.
func (k PlatformRuntimeKey) Validate() error {
	var errs []error
	if err := k.Platform.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := k.Runtime.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// String returns "platform/runtime" representation (e.g., "linux/native").
func (k PlatformRuntimeKey) String() string {
	return string(k.Platform) + "/" + string(k.Runtime)
}

// Validate returns nil if the ImplementationMatch has valid fields,
// or an error collecting all field-level validation failures.
// Delegates to Platform.Validate() (nonzero) and Runtime.Validate() (nonzero).
// Implementation is a pointer — not validated here (caller validates separately).
func (m ImplementationMatch) Validate() error {
	var errs []error
	if err := m.Platform.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.Runtime.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidImplementationMatchError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidImplementationMatchError.
func (e *InvalidImplementationMatchError) Error() string {
	return fmt.Sprintf("invalid implementation match: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidImplementationMatch for errors.Is() compatibility.
func (e *InvalidImplementationMatchError) Unwrap() error { return ErrInvalidImplementationMatch }

// Validate returns nil if the Implementation has valid fields,
// or an error collecting all field-level validation failures.
// Delegates to Script.Validate() (zero-valid), RuntimeConfig.Validate() for each runtime,
// PlatformConfig.Validate() for each platform, and validates optional fields when non-empty/non-nil.
//
//goplint:ignore -- helper-based delegation keeps field-order stability while reducing Sonar complexity.
//goplint:ignore -- Sonar refactor keeps optional-field validation local without changing behavior.
func (s Implementation) Validate() error {
	var errs []error
	appendFieldError(&errs, s.Script.Validate())
	appendEachValidation(&errs, s.Runtimes)
	appendEachValidation(&errs, s.Platforms)
	appendOptionalValidation(&errs, s.Env, s.Env != nil)
	appendOptionalValidation(&errs, s.WorkDir, s.WorkDir != "")
	appendOptionalValidation(&errs, s.DependsOn, s.DependsOn != nil)
	appendFieldError(&errs, s.Timeout.Validate())
	if len(errs) > 0 {
		return &InvalidImplementationError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidImplementationError.
func (e *InvalidImplementationError) Error() string {
	return fmt.Sprintf("invalid implementation: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidImplementation for errors.Is() compatibility.
func (e *InvalidImplementationError) Unwrap() error { return ErrInvalidImplementation }

// MatchesPlatform returns true if the implementation can run on the given platform.
func (s *Implementation) MatchesPlatform(platform Platform) bool {
	for _, p := range s.Platforms {
		if p.Name == platform {
			return true
		}
	}
	return false
}

// HasRuntime returns true if the implementation supports the given runtime.
func (s *Implementation) HasRuntime(runtime RuntimeMode) bool {
	for i := range s.Runtimes {
		if s.Runtimes[i].Name == runtime {
			return true
		}
	}
	return false
}

// GetRuntimeConfig returns the RuntimeConfig for the given runtime type, or nil if not found.
func (s *Implementation) GetRuntimeConfig(runtime RuntimeMode) *RuntimeConfig {
	return FindRuntimeConfig(s.Runtimes, runtime)
}

// GetDefaultRuntime returns the default runtime type for this implementation (first runtime in the list).
func (s *Implementation) GetDefaultRuntime() RuntimeMode {
	if len(s.Runtimes) == 0 {
		return RuntimeNative
	}
	return s.Runtimes[0].Name
}

// GetDefaultRuntimeConfig returns the default RuntimeConfig for this implementation (first in the list).
func (s *Implementation) GetDefaultRuntimeConfig() *RuntimeConfig {
	if len(s.Runtimes) == 0 {
		return nil
	}
	return &s.Runtimes[0]
}

// HasHostSSH returns true if any runtime in this implementation has enable_host_ssh enabled.
func (s *Implementation) HasHostSSH() bool {
	for i := range s.Runtimes {
		if s.Runtimes[i].Name == RuntimeContainer && s.Runtimes[i].EnableHostSSH {
			return true
		}
	}
	return false
}

// GetHostSSHForRuntime returns whether enable_host_ssh is enabled for the given runtime
func (s *Implementation) GetHostSSHForRuntime(runtime RuntimeMode) bool {
	if runtime != RuntimeContainer {
		return false // enable_host_ssh is only valid for container runtime
	}
	rc := s.GetRuntimeConfig(runtime)
	if rc == nil {
		return false
	}
	return rc.EnableHostSSH
}

// HasDependencies returns true if the implementation has any dependencies.
func (s *Implementation) HasDependencies() bool {
	return s.DependsOn != nil && !s.DependsOn.IsEmpty()
}

// GetCommandDependencies returns the list of command dependency names from this implementation.
// For dependencies with alternatives, returns all alternatives flattened into a single list.
func (s *Implementation) GetCommandDependencies() []CommandName {
	if s.DependsOn == nil {
		return nil
	}
	var names []CommandName
	for _, dep := range s.DependsOn.Commands {
		names = append(names, dep.Alternatives...)
	}
	return names
}

// IsScriptFile returns true if the Implementation field appears to be a file path
// rather than inline script content. It uses the following heuristics:
//   - Path prefix: starts with "./", "../", or "/" (Unix-style absolute/relative paths)
//   - Drive letter: second character is ':' (Windows-style paths like "C:\script.ps1")
//   - Known extension: ends with a recognized script file extension
//     (.sh, .bash, .ps1, .bat, .cmd, .py, .rb, .pl, .zsh, .fish)
func (s *Implementation) IsScriptFile() bool {
	script := strings.TrimSpace(string(s.Script))
	if script == "" {
		return false
	}

	// Check for explicit path indicators
	if strings.HasPrefix(script, "./") || strings.HasPrefix(script, "../") || strings.HasPrefix(script, "/") {
		return true
	}

	// On Windows, check for drive letter paths
	if len(script) >= 2 && script[1] == ':' {
		return true
	}

	// Check for known script file extensions
	lower := strings.ToLower(script)
	for _, ext := range scriptFileExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	return false
}

// GetScriptFilePath returns the absolute path to the script file, if Implementation is a file reference.
// Returns empty FilesystemPath if Implementation is inline content.
// The invowkfilePath parameter is used to resolve relative paths.
// If modulePath is provided (non-empty), script paths are resolved relative to the module root
// and are expected to use forward slashes for cross-platform compatibility.
func (s *Implementation) GetScriptFilePath(invowkfilePath FilesystemPath) FilesystemPath {
	return s.GetScriptFilePathWithModule(invowkfilePath, "")
}

// GetScriptFilePathWithModule returns the absolute path to the script file, if Implementation is a file reference.
// Returns empty FilesystemPath if Implementation is inline content.
// The invowkfilePath parameter is used to resolve relative paths when not in a module.
// The modulePath parameter specifies the module root directory for module-relative paths.
// When modulePath is non-empty, script paths are expected to use forward slashes for
// cross-platform compatibility and are resolved relative to the module root.
func (s *Implementation) GetScriptFilePathWithModule(invowkfilePath, modulePath FilesystemPath) FilesystemPath {
	if !s.IsScriptFile() {
		return ""
	}

	script := strings.TrimSpace(string(s.Script))

	// If absolute path, return as-is
	if filepath.IsAbs(script) {
		return FilesystemPath(script) //goplint:ignore -- OS-absolute path from filepath.IsAbs guard
	}

	// If in a module, resolve relative to module root with cross-platform path conversion
	if modulePath != "" {
		// Convert forward slashes to native path separator for cross-platform compatibility
		nativePath := filepath.FromSlash(script)
		return fspath.JoinStr(modulePath, nativePath)
	}

	// Resolve relative to invowkfile directory
	return fspath.JoinStr(fspath.Dir(invowkfilePath), script)
}

// ResolveScript returns the actual script content to execute.
// If Implementation is a file path, it reads the file content.
// If Implementation is inline content (including multi-line), it returns it directly.
// The invowkfilePath parameter is used to resolve relative paths.
func (s *Implementation) ResolveScript(invowkfilePath FilesystemPath) (string, error) {
	return s.ResolveScriptWithModule(invowkfilePath, "")
}

// ResolveScriptWithModule returns the actual script content to execute.
// If Implementation is a file path, it reads the file content.
// If Implementation is inline content (including multi-line), it returns it directly.
// The invowkfilePath parameter is used to resolve relative paths when not in a module.
// The modulePath parameter specifies the module root directory for module-relative paths.
func (s *Implementation) ResolveScriptWithModule(invowkfilePath, modulePath FilesystemPath) (string, error) {
	if s.scriptResolved {
		return string(s.resolvedScript), nil
	}

	script := string(s.Script)
	if script == "" {
		return "", errors.New("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePathWithModule(invowkfilePath, modulePath)
		content, err := os.ReadFile(string(scriptPath))
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		resolved := ScriptContent(content)
		if err := resolved.Validate(); err != nil {
			return "", fmt.Errorf("script file content: %w", err)
		}
		s.resolvedScript = resolved
	} else {
		// Inline script - use directly (multi-line strings from CUE are already handled)
		resolved := ScriptContent(script)
		if err := resolved.Validate(); err != nil {
			return "", fmt.Errorf("inline script content: %w", err)
		}
		s.resolvedScript = resolved
	}

	s.scriptResolved = true
	return string(s.resolvedScript), nil
}

// ResolveScriptWithFS resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
func (s *Implementation) ResolveScriptWithFS(invowkfilePath FilesystemPath, readFile func(path string) ([]byte, error)) (string, error) {
	return s.ResolveScriptWithFSAndModule(invowkfilePath, "", readFile)
}

// ResolveScriptWithFSAndModule resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
// The modulePath parameter specifies the module root directory for module-relative paths.
func (s *Implementation) ResolveScriptWithFSAndModule(invowkfilePath, modulePath FilesystemPath, readFile func(path string) ([]byte, error)) (string, error) {
	script := string(s.Script)
	if script == "" {
		return "", errors.New("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePathWithModule(invowkfilePath, modulePath)
		content, err := readFile(string(scriptPath))
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		return string(content), nil
	}

	// Inline script - use directly
	return script, nil
}

// ParseTimeout parses the Timeout field into a time.Duration.
// Returns (0, nil) when Timeout is empty (no timeout configured).
// Returns an error for zero or negative durations, which would cause
// context.WithTimeout to create an immediately-expired context.
func (s *Implementation) ParseTimeout() (time.Duration, error) {
	return parseDuration("timeout", s.Timeout)
}
