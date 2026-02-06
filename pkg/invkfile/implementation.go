// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// scriptFileExtensions contains extensions that indicate a script file
var scriptFileExtensions = []string{".sh", ".bash", ".ps1", ".bat", ".cmd", ".py", ".rb", ".pl", ".zsh", ".fish"}

type (
	// Implementation represents an implementation with platform and runtime constraints
	Implementation struct {
		// Script contains the shell commands to execute OR a path to a script file
		Script string `json:"script"`
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
		// Can be absolute or relative to the invkfile location.
		// Forward slashes should be used for cross-platform compatibility.
		WorkDir string `json:"workdir,omitempty"`
		// DependsOn specifies dependencies that must be satisfied before running this implementation
		// These dependencies are validated according to the runtime being used
		DependsOn *DependsOn `json:"depends_on,omitempty"`

		// resolvedScript caches the resolved script content (lazy memoization).
		// Script content is resolved from file path or inline source on first
		// ResolveScript call and reused for subsequent calls.
		resolvedScript string
		// scriptResolved tracks whether resolvedScript has been populated.
		scriptResolved bool
	}

	// PlatformRuntimeKey represents a unique combination of platform and runtime
	PlatformRuntimeKey struct {
		Platform Platform
		Runtime  RuntimeMode
	}

	// ImplementationMatch represents a matched implementation for execution.
	ImplementationMatch struct {
		Implementation       *Implementation
		Platform             Platform
		Runtime              RuntimeMode
		IsDefaultForPlatform bool
	}
)

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
	for i := range s.Runtimes {
		if s.Runtimes[i].Name == runtime {
			return &s.Runtimes[i]
		}
	}
	return nil
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
	if s.DependsOn == nil {
		return false
	}
	return len(s.DependsOn.Tools) > 0 || len(s.DependsOn.Commands) > 0 || len(s.DependsOn.Filepaths) > 0 || len(s.DependsOn.Capabilities) > 0 || len(s.DependsOn.CustomChecks) > 0 || len(s.DependsOn.EnvVars) > 0
}

// GetCommandDependencies returns the list of command dependency names from this implementation.
// For dependencies with alternatives, returns all alternatives flattened into a single list.
func (s *Implementation) GetCommandDependencies() []string {
	if s.DependsOn == nil {
		return nil
	}
	var names []string
	for _, dep := range s.DependsOn.Commands {
		names = append(names, dep.Alternatives...)
	}
	return names
}

// IsScriptFile returns true if the Implementation field appears to be a file path
func (s *Implementation) IsScriptFile() bool {
	script := strings.TrimSpace(s.Script)
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
// Returns empty string if Implementation is inline content.
// The invkfilePath parameter is used to resolve relative paths.
// If modulePath is provided (non-empty), script paths are resolved relative to the module root
// and are expected to use forward slashes for cross-platform compatibility.
func (s *Implementation) GetScriptFilePath(invkfilePath string) string {
	return s.GetScriptFilePathWithModule(invkfilePath, "")
}

// GetScriptFilePathWithModule returns the absolute path to the script file, if Implementation is a file reference.
// Returns empty string if Implementation is inline content.
// The invkfilePath parameter is used to resolve relative paths when not in a module.
// The modulePath parameter specifies the module root directory for module-relative paths.
// When modulePath is non-empty, script paths are expected to use forward slashes for
// cross-platform compatibility and are resolved relative to the module root.
func (s *Implementation) GetScriptFilePathWithModule(invkfilePath, modulePath string) string {
	if !s.IsScriptFile() {
		return ""
	}

	script := strings.TrimSpace(s.Script)

	// If absolute path, return as-is
	if filepath.IsAbs(script) {
		return script
	}

	// If in a module, resolve relative to module root with cross-platform path conversion
	if modulePath != "" {
		// Convert forward slashes to native path separator for cross-platform compatibility
		nativePath := filepath.FromSlash(script)
		return filepath.Join(modulePath, nativePath)
	}

	// Resolve relative to invkfile directory
	invowkDir := filepath.Dir(invkfilePath)
	return filepath.Join(invowkDir, script)
}

// ResolveScript returns the actual script content to execute.
// If Implementation is a file path, it reads the file content.
// If Implementation is inline content (including multi-line), it returns it directly.
// The invkfilePath parameter is used to resolve relative paths.
func (s *Implementation) ResolveScript(invkfilePath string) (string, error) {
	return s.ResolveScriptWithModule(invkfilePath, "")
}

// ResolveScriptWithModule returns the actual script content to execute.
// If Implementation is a file path, it reads the file content.
// If Implementation is inline content (including multi-line), it returns it directly.
// The invkfilePath parameter is used to resolve relative paths when not in a module.
// The modulePath parameter specifies the module root directory for module-relative paths.
func (s *Implementation) ResolveScriptWithModule(invkfilePath, modulePath string) (string, error) {
	if s.scriptResolved {
		return s.resolvedScript, nil
	}

	script := s.Script
	if script == "" {
		return "", fmt.Errorf("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePathWithModule(invkfilePath, modulePath)
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		s.resolvedScript = string(content)
	} else {
		// Inline script - use directly (multi-line strings from CUE are already handled)
		s.resolvedScript = script
	}

	s.scriptResolved = true
	return s.resolvedScript, nil
}

// ResolveScriptWithFS resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
func (s *Implementation) ResolveScriptWithFS(invkfilePath string, readFile func(path string) ([]byte, error)) (string, error) {
	return s.ResolveScriptWithFSAndModule(invkfilePath, "", readFile)
}

// ResolveScriptWithFSAndModule resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
// The modulePath parameter specifies the module root directory for module-relative paths.
func (s *Implementation) ResolveScriptWithFSAndModule(invkfilePath, modulePath string, readFile func(path string) ([]byte, error)) (string, error) {
	script := s.Script
	if script == "" {
		return "", fmt.Errorf("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePathWithModule(invkfilePath, modulePath)
		content, err := readFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		return string(content), nil
	}

	// Inline script - use directly
	return script, nil
}
