// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// scriptPathTraversalErrMsg backs ErrScriptPathTraversal.
	scriptPathTraversalErrMsg = "script path escapes module boundary"
	// scriptReaderRequiredErrMsg backs ErrScriptReaderRequired.
	scriptReaderRequiredErrMsg = "script file reader required"
	// scriptFileRequiresModuleErrMsg backs ErrScriptFileRequiresModule.
	scriptFileRequiresModuleErrMsg = "script file requires module invowkfile"
	// invalidImplementationScriptErrMsg backs ErrInvalidImplementationScript.
	invalidImplementationScriptErrMsg = "invalid implementation script"
	// missingImplementationScriptSourceErrMsg backs ErrMissingImplementationScriptSource.
	missingImplementationScriptSourceErrMsg = "implementation script must set content or file"
	// mixedImplementationScriptSourceErrMsg backs ErrMixedImplementationScriptSource.
	mixedImplementationScriptSourceErrMsg = "implementation script must not set both content and file"
)

var (
	// ErrInvalidImplementation is the sentinel error wrapped by InvalidImplementationError.
	ErrInvalidImplementation = errors.New("invalid implementation")

	// ErrInvalidImplementationMatch is the sentinel error wrapped by InvalidImplementationMatchError.
	ErrInvalidImplementationMatch = errors.New("invalid implementation match")

	// ErrScriptPathTraversal is returned when a module script path resolves
	// outside the module boundary (e.g., "../../etc/passwd"). See SC-01.
	ErrScriptPathTraversal = errors.New(scriptPathTraversalErrMsg)

	// ErrScriptReaderRequired is returned when resolving a script file without
	// an explicit filesystem reader.
	ErrScriptReaderRequired = errors.New(scriptReaderRequiredErrMsg)

	// ErrScriptFileRequiresModule is returned when script.file is used outside
	// an invowkmod-backed invowkfile.
	ErrScriptFileRequiresModule = errors.New(scriptFileRequiresModuleErrMsg)

	// ErrInvalidImplementationScript is the sentinel error wrapped by InvalidImplementationScriptError.
	ErrInvalidImplementationScript = errors.New(invalidImplementationScriptErrMsg)

	// ErrMissingImplementationScriptSource is returned when an implementation script selects no source.
	ErrMissingImplementationScriptSource = errors.New(missingImplementationScriptSourceErrMsg)

	// ErrMixedImplementationScriptSource is returned when an implementation script selects both sources.
	ErrMixedImplementationScriptSource = errors.New(mixedImplementationScriptSourceErrMsg)
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
	// ImplementationScript selects either inline script content or a script file reference.
	ImplementationScript struct {
		// Content contains inline script text.
		Content ScriptContent `json:"content,omitempty"`
		// File references a script file resolved at execution time.
		File *FilesystemPath `json:"file,omitempty"`
		// Interpreter specifies how to execute the resolved script content.
		Interpreter InterpreterSpec `json:"interpreter,omitempty"`
	}

	// InvalidImplementationScriptError is returned when an ImplementationScript has invalid fields.
	// It wraps ErrInvalidImplementationScript for errors.Is() compatibility.
	InvalidImplementationScriptError struct {
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// Implementation represents an implementation with platform and runtime constraints
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
	Implementation struct {
		// Script selects inline shell content or a script file reference.
		Script ImplementationScript `json:"script"`
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

// Validate returns nil when the script selects exactly one valid source.
func (s ImplementationScript) Validate() error {
	hasContent := s.Content != ""
	hasFile := s.File != nil
	var errs []error
	switch {
	case hasContent && hasFile:
		errs = append(errs, ErrMixedImplementationScriptSource)
	case !hasContent && !hasFile:
		errs = append(errs, ErrMissingImplementationScriptSource)
	}
	if hasContent {
		if err := s.Content.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if hasFile {
		if err := s.File.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	appendOptionalValidation(&errs, s.Interpreter, s.Interpreter != "")
	if len(errs) > 0 {
		return &InvalidImplementationScriptError{FieldErrors: errs}
	}
	return nil
}

// ResolveInterpreterFromScript resolves the interpreter for this script using
// the provided resolved script content.
//
//goplint:ignore -- interpreter resolution consumes already-validated script bytes from inline or file sources.
func (s ImplementationScript) ResolveInterpreterFromScript(scriptContent string) ShebangInfo {
	return ResolveInterpreter(s.Interpreter, scriptContent)
}

// IsContent returns true when this script contains inline script text.
func (s ImplementationScript) IsContent() bool {
	return s.Content != ""
}

// IsFile returns true when this script references a script file.
func (s ImplementationScript) IsFile() bool {
	return s.File != nil
}

// Error implements the error interface for InvalidImplementationScriptError.
func (e *InvalidImplementationScriptError) Error() string {
	return types.FormatFieldErrors("implementation script", e.FieldErrors)
}

// Unwrap returns ErrInvalidImplementationScript and field errors for errors.Is() compatibility.
func (e *InvalidImplementationScriptError) Unwrap() error {
	return errors.Join(ErrInvalidImplementationScript, errors.Join(e.FieldErrors...))
}

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
	return types.FormatFieldErrors("implementation match", e.FieldErrors)
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
	return types.FormatFieldErrors("implementation", e.FieldErrors)
}

// Unwrap returns ErrInvalidImplementation and field errors for errors.Is() compatibility.
func (e *InvalidImplementationError) Unwrap() error {
	return errors.Join(ErrInvalidImplementation, errors.Join(e.FieldErrors...))
}

// MatchesPlatform returns true if the implementation can run on the given platform.
func (s *Implementation) MatchesPlatform(platform Platform) bool {
	for _, p := range s.Platforms {
		if p.Name == platform {
			return true
		}
	}
	return false
}

// GetPlatformConfig returns the PlatformConfig for the given platform, or nil if not found.
func (s *Implementation) GetPlatformConfig(platform Platform) *PlatformConfig {
	for i := range s.Platforms {
		if s.Platforms[i].Name == platform {
			return &s.Platforms[i]
		}
	}
	return nil
}

// VirtualFilesystemForPlatform returns the effective virtual filesystem config
// for the given platform. Missing config means restricted access with no named paths.
func (s *Implementation) VirtualFilesystemForPlatform(platform Platform) VirtualFilesystemConfig {
	platformCfg := s.GetPlatformConfig(platform)
	if platformCfg == nil {
		return VirtualFilesystemConfig{}
	}
	return platformCfg.VirtualFilesystem()
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

// GetCommandDependencies returns the list of command dependency references from this implementation.
// For dependencies with alternatives, returns all alternatives flattened into a single list.
func (s *Implementation) GetCommandDependencies() []CommandDependencyRef {
	if s.DependsOn == nil {
		return nil
	}
	var refs []CommandDependencyRef
	for _, dep := range s.DependsOn.Commands {
		refs = append(refs, dep.Alternatives...)
	}
	return refs
}

// GetScriptFilePath returns the absolute path to the script file, if Implementation is a file reference.
// Returns empty FilesystemPath if Implementation is inline content.
// The invowkfilePath parameter is used to resolve relative paths.
//
// Deprecated: script.file is only valid for module invowkfiles; use
// GetScriptFilePathWithModule with a non-empty module path.
func (s *Implementation) GetScriptFilePath(invowkfilePath FilesystemPath) FilesystemPath {
	return s.GetScriptFilePathWithModule(invowkfilePath, "")
}

// GetScriptFilePathWithModule returns the absolute path to the script file, if Implementation is a file reference.
// Returns empty FilesystemPath if Implementation is inline content.
// The invowkfilePath parameter is retained for API stability but is not used for
// file-backed script resolution because script.file is valid only in modules.
// The modulePath parameter specifies the module root directory for module-relative paths.
// When modulePath is non-empty, script paths are expected to use forward slashes for
// cross-platform compatibility and are resolved relative to the module root.
//
// Security: This method performs path resolution only — it does NOT validate that the
// resolved path stays within the module boundary. Callers performing file I/O in module
// contexts MUST use ResolveScriptWithModule or ResolveScriptWithFSAndModule, which apply
// validateScriptPathContainment (SC-01). Direct use of this method for file reads without
// a subsequent containment check is a security risk.
func (s *Implementation) GetScriptFilePathWithModule(_, modulePath FilesystemPath) FilesystemPath {
	if !s.Script.IsFile() {
		return ""
	}
	if modulePath == "" {
		return ""
	}

	script := strings.TrimSpace(string(*s.Script.File))

	// Unix-style absolute paths (leading '/') are container-absolute and must
	// pass through unchanged on every platform. On Windows, filepath.IsAbs("/foo")
	// returns false because Windows absolute paths require a drive letter or UNC
	// prefix, so this guard must precede IsAbs to avoid silently joining the
	// container path with the module root.
	if strings.HasPrefix(script, "/") {
		return FilesystemPath(script) //goplint:ignore -- container-absolute path identified by leading slash guard
	}

	// If absolute path, return as-is
	if filepath.IsAbs(script) {
		return FilesystemPath(script) //goplint:ignore -- OS-absolute path from filepath.IsAbs guard
	}

	// Convert forward slashes to native path separator for cross-platform compatibility.
	nativePath := filepath.FromSlash(script)
	return fspath.JoinStr(modulePath, nativePath)
}

// ResolveScript returns inline script content.
// File-based scripts require ResolveScriptWithFS so the caller owns filesystem
// access instead of the schema model.
// The invowkfilePath parameter is used to resolve relative paths.
//
// Security: this method delegates to ResolveScriptWithFSAndModule with an empty
// modulePath, which means NO path containment check is applied. It is only safe
// for trusted (user-controlled) inline invowkfiles — not for third-party module
// file scripts. Module contexts must use ResolveScriptWithFSAndModule with a
// non-empty modulePath.
func (s *Implementation) ResolveScript(invowkfilePath FilesystemPath) (string, error) {
	return s.ResolveScriptWithModule(invowkfilePath, "")
}

// ResolveScriptWithModule returns inline script content.
// File-based scripts require ResolveScriptWithFSAndModule so the caller owns
// filesystem access instead of the schema model.
// The invowkfilePath parameter is used to resolve relative paths when not in a module.
// The modulePath parameter specifies the module root directory for module-relative paths.
func (s *Implementation) ResolveScriptWithModule(invowkfilePath, modulePath FilesystemPath) (string, error) {
	return s.ResolveScriptWithFSAndModule(invowkfilePath, modulePath, scriptReaderRequired)
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
	if err := s.Script.Validate(); err != nil {
		return "", err
	}

	if s.Script.IsFile() {
		if modulePath == "" {
			return "", ErrScriptFileRequiresModule
		}
		scriptPath := s.GetScriptFilePathWithModule(invowkfilePath, modulePath)

		// Security: containment check for module contexts (SC-01).
		if err := validateScriptPathContainment(scriptPath, modulePath); err != nil {
			return "", err
		}

		if readFile == nil {
			return "", ErrScriptReaderRequired
		}
		content, err := readFile(string(scriptPath))
		if err != nil {
			return "", scriptFileReadError(*s.Script.File, scriptPath, err)
		}
		resolved, err := validateResolvedScriptContent("script file content", ScriptContent(content)) //goplint:ignore -- validated by helper before use.
		if err != nil {
			return "", err
		}
		return string(resolved), nil
	}

	// Inline script - use directly
	resolved, err := validateResolvedScriptContent("inline script content", s.Script.Content)
	if err != nil {
		return "", err
	}
	return string(resolved), nil
}

//goplint:ignore -- reader adapter matches os.ReadFile-style boundary signature.
func scriptReaderRequired(string) ([]byte, error) {
	return nil, ErrScriptReaderRequired
}

func scriptFileReadError(selectedPath, scriptPath FilesystemPath, err error) error {
	selected := strings.TrimSpace(selectedPath.String())
	if selected == "" || selected == scriptPath.String() {
		return fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
	}
	return fmt.Errorf("failed to read script file '%s' (resolved to '%s'): %w", selected, scriptPath, err)
}

//goplint:ignore -- helper validates transient script bytes from file readers and inline source.
func validateResolvedScriptContent(label string, content ScriptContent) (ScriptContent, error) {
	resolved := content
	if err := resolved.Validate(); err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return resolved, nil
}

// validateScriptPathContainment ensures a resolved script path stays within
// the module boundary. Prevents path traversal attacks where a module's
// invowkfile.cue specifies paths like "../../etc/passwd" (SC-01).
func validateScriptPathContainment(scriptPath, modulePath FilesystemPath) error {
	relPath, err := filepath.Rel(string(modulePath), string(scriptPath))
	if err != nil {
		return fmt.Errorf("%w: failed to resolve path relative to module: %w", ErrScriptPathTraversal, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: '%s' resolves outside module '%s'",
			ErrScriptPathTraversal, scriptPath, modulePath)
	}
	return nil
}

func validateModuleScriptFileSelection(scriptPath, modulePath FilesystemPath) error {
	if modulePath == "" {
		return ErrScriptFileRequiresModule
	}
	return validateScriptPathContainment(scriptPath, modulePath)
}

// ParseTimeout parses the Timeout field into a time.Duration.
// Returns (0, nil) when Timeout is empty (no timeout configured).
// Returns an error for zero or negative durations, which would cause
// context.WithTimeout to create an immediately-expired context.
func (s *Implementation) ParseTimeout() (time.Duration, error) {
	return parseDuration("timeout", s.Timeout)
}
