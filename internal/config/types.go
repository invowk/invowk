// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
)

const (
	// ContainerEnginePodman uses Podman as the container runtime.
	ContainerEnginePodman ContainerEngine = "podman"
	// ContainerEngineDocker uses Docker as the container runtime.
	ContainerEngineDocker ContainerEngine = "docker"

	// RuntimeNative runs commands in the host system shell.
	// Defined locally to avoid coupling config to pkg/invowkfile.
	RuntimeNative RuntimeMode = "native"
	// RuntimeVirtual runs commands in the embedded mvdan/sh interpreter.
	RuntimeVirtual RuntimeMode = "virtual"
	// RuntimeContainer runs commands inside a container (Docker/Podman).
	RuntimeContainer RuntimeMode = "container"

	// ColorSchemeAuto detects the terminal color scheme automatically.
	ColorSchemeAuto ColorScheme = "auto"
	// ColorSchemeDark forces dark color scheme.
	ColorSchemeDark ColorScheme = "dark"
	// ColorSchemeLight forces light color scheme.
	ColorSchemeLight ColorScheme = "light"

	// moduleSuffix is the filesystem suffix for invowkmod directories.
	// Defined locally to avoid coupling config to pkg/invowkmod.
	moduleSuffix = ".invowkmod"
)

var (
	// ErrInvalidContainerEngine is returned when a ContainerEngine value is not recognized.
	ErrInvalidContainerEngine = errors.New("invalid container engine")
	// ErrInvalidConfigRuntimeMode is returned when a config RuntimeMode value is not recognized.
	ErrInvalidConfigRuntimeMode = errors.New("invalid runtime mode")
	// ErrInvalidColorScheme is returned when a ColorScheme value is not recognized.
	ErrInvalidColorScheme = errors.New("invalid color scheme")
	// ErrInvalidModuleIncludePath is the sentinel error wrapped by InvalidModuleIncludePathError.
	ErrInvalidModuleIncludePath = errors.New("invalid module include path")
	// ErrInvalidBinaryFilePath is returned when a BinaryFilePath value is whitespace-only.
	ErrInvalidBinaryFilePath = errors.New("invalid binary file path")
	// ErrInvalidCacheDirPath is returned when a CacheDirPath value is whitespace-only.
	ErrInvalidCacheDirPath = errors.New("invalid cache dir path")
	// ErrInvalidIncludeEntry is the sentinel error wrapped by InvalidIncludeEntryError.
	ErrInvalidIncludeEntry = errors.New("invalid include entry")
	// ErrInvalidUIConfig is the sentinel error wrapped by InvalidUIConfigError.
	ErrInvalidUIConfig = errors.New("invalid UI config")
	// ErrInvalidAutoProvisionConfig is the sentinel error wrapped by InvalidAutoProvisionConfigError.
	ErrInvalidAutoProvisionConfig = errors.New("invalid auto-provision config")
	// ErrInvalidContainerConfig is the sentinel error wrapped by InvalidContainerConfigError.
	ErrInvalidContainerConfig = errors.New("invalid container config")
	// ErrInvalidConfig is the sentinel error wrapped by InvalidConfigError.
	ErrInvalidConfig = errors.New("invalid config")
)

type (
	// ContainerEngine specifies which container runtime to use.
	ContainerEngine string

	// InvalidContainerEngineError is returned when a ContainerEngine value is not recognized.
	// It wraps ErrInvalidContainerEngine for errors.Is() compatibility.
	InvalidContainerEngineError struct {
		Value ContainerEngine
	}

	// RuntimeMode specifies the execution runtime for commands.
	// Defined locally to avoid coupling config to pkg/invowkfile;
	// the orchestrator casts to invowkfile.RuntimeMode at the boundary.
	RuntimeMode string

	// InvalidConfigRuntimeModeError is returned when a config RuntimeMode value is not recognized.
	// It wraps ErrInvalidConfigRuntimeMode for errors.Is() compatibility.
	InvalidConfigRuntimeModeError struct {
		Value RuntimeMode
	}

	// ColorScheme specifies the terminal color scheme preference.
	ColorScheme string

	// InvalidColorSchemeError is returned when a ColorScheme value is not recognized.
	// It wraps ErrInvalidColorScheme for errors.Is() compatibility.
	InvalidColorSchemeError struct {
		Value ColorScheme
	}

	// ModuleIncludePath represents an absolute filesystem path to a *.invowkmod directory.
	// A valid path must be non-empty and not whitespace-only.
	ModuleIncludePath string

	// InvalidModuleIncludePathError is returned when a ModuleIncludePath value is
	// empty or whitespace-only. It wraps ErrInvalidModuleIncludePath for errors.Is().
	InvalidModuleIncludePathError struct {
		Value ModuleIncludePath
	}

	// BinaryFilePath represents a filesystem path to a binary executable.
	// A valid path must be non-empty and not whitespace-only.
	// The zero value ("") is valid and means "use auto-detected binary".
	BinaryFilePath string

	// InvalidBinaryFilePathError is returned when a BinaryFilePath value is
	// non-empty but whitespace-only.
	InvalidBinaryFilePathError struct {
		Value BinaryFilePath
	}

	// CacheDirPath represents a filesystem path to a cache directory.
	// The zero value ("") is valid and means "use default cache directory".
	// Non-zero values must not be whitespace-only.
	CacheDirPath string

	// InvalidCacheDirPathError is returned when a CacheDirPath value is
	// non-empty but whitespace-only.
	InvalidCacheDirPathError struct {
		Value CacheDirPath
	}

	// InvalidIncludeEntryError is returned when an IncludeEntry has invalid fields.
	// It wraps ErrInvalidIncludeEntry for errors.Is() compatibility and collects
	// field-level validation errors from Path and Alias.
	InvalidIncludeEntryError struct {
		FieldErrors []error
	}

	// InvalidConfigError is returned when a Config has invalid fields.
	// It wraps ErrInvalidConfig for errors.Is() compatibility and collects
	// field-level validation errors from all sub-components.
	InvalidConfigError struct {
		FieldErrors []error
	}

	// InvalidUIConfigError is returned when a UIConfig has invalid fields.
	// It wraps ErrInvalidUIConfig for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidUIConfigError struct {
		FieldErrors []error
	}

	// InvalidAutoProvisionConfigError is returned when an AutoProvisionConfig has invalid fields.
	// It wraps ErrInvalidAutoProvisionConfig for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidAutoProvisionConfigError struct {
		FieldErrors []error
	}

	// InvalidContainerConfigError is returned when a ContainerConfig has invalid fields.
	// It wraps ErrInvalidContainerConfig for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidContainerConfigError struct {
		FieldErrors []error
	}

	// IncludeEntry specifies a module to include in command discovery.
	// Each entry must point to a *.invowkmod directory via an absolute filesystem path.
	IncludeEntry struct {
		// Path is the absolute filesystem path to a *.invowkmod directory.
		Path ModuleIncludePath `json:"path" mapstructure:"path"`
		// Alias optionally overrides the module identifier for collision disambiguation.
		Alias invowkmod.ModuleAlias `json:"alias,omitempty" mapstructure:"alias"`
	}

	// Config holds the application configuration.
	Config struct {
		// ContainerEngine specifies whether to use "podman" or "docker"
		ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
		// Includes specifies modules to include in command discovery.
		Includes []IncludeEntry `json:"includes" mapstructure:"includes"`
		// DefaultRuntime sets the global default runtime mode
		DefaultRuntime RuntimeMode `json:"default_runtime" mapstructure:"default_runtime"`
		// VirtualShell configures the virtual shell behavior
		VirtualShell VirtualShellConfig `json:"virtual_shell" mapstructure:"virtual_shell"`
		// UI configures the user interface
		UI UIConfig `json:"ui" mapstructure:"ui"`
		// Container configures container runtime behavior
		Container ContainerConfig `json:"container" mapstructure:"container"`
	}

	// ContainerConfig configures container runtime behavior.
	ContainerConfig struct {
		// AutoProvision configures automatic provisioning of invowk resources
		AutoProvision AutoProvisionConfig `json:"auto_provision" mapstructure:"auto_provision"`
	}

	// AutoProvisionConfig controls auto-provisioning of invowk resources into containers.
	AutoProvisionConfig struct {
		// Enabled enables/disables auto-provisioning (default: true)
		Enabled bool `json:"enabled" mapstructure:"enabled"`
		// Strict makes provisioning failure a hard error instead of falling back
		// to the unprovisioned base image. When false (default), provisioning
		// failure logs a warning and continues with the base image.
		Strict bool `json:"strict" mapstructure:"strict"`
		// BinaryPath overrides the path to the invowk binary to provision
		BinaryPath BinaryFilePath `json:"binary_path" mapstructure:"binary_path"`
		// Includes specifies modules to provision into containers.
		Includes []IncludeEntry `json:"includes" mapstructure:"includes"`
		// InheritIncludes controls whether root-level includes are automatically
		// merged into container provisioning (default: true).
		InheritIncludes bool `json:"inherit_includes" mapstructure:"inherit_includes"`
		// CacheDir specifies where to store cached provisioned images metadata
		CacheDir CacheDirPath `json:"cache_dir" mapstructure:"cache_dir"`
	}

	// VirtualShellConfig configures the virtual shell runtime.
	VirtualShellConfig struct {
		// EnableUrootUtils enables u-root utilities in virtual shell
		EnableUrootUtils bool `json:"enable_uroot_utils" mapstructure:"enable_uroot_utils"`
	}

	// UIConfig configures the user interface.
	UIConfig struct {
		// ColorScheme sets the color scheme
		ColorScheme ColorScheme `json:"color_scheme" mapstructure:"color_scheme"`
		// Verbose enables verbose output
		Verbose bool `json:"verbose" mapstructure:"verbose"`
		// Interactive enables alternate screen buffer mode for command execution
		Interactive bool `json:"interactive" mapstructure:"interactive"`
	}
)

// IsModule reports whether this entry points to a module directory (.invowkmod).
func (e IncludeEntry) IsModule() bool {
	return strings.HasSuffix(string(e.Path), moduleSuffix)
}

// Validate returns an error if the IncludeEntry has invalid fields.
// It delegates to Path.Validate() unconditionally and to Alias.Validate()
// only when non-empty (the zero-value alias is always valid).
func (e IncludeEntry) Validate() error {
	var errs []error
	if err := e.Path.Validate(); err != nil {
		errs = append(errs, err)
	}
	if e.Alias != "" {
		if err := e.Alias.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidIncludeEntryError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidIncludeEntryError.
func (e *InvalidIncludeEntryError) Error() string {
	return fmt.Sprintf("invalid include entry: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidIncludeEntry for errors.Is() compatibility.
func (e *InvalidIncludeEntryError) Unwrap() error { return ErrInvalidIncludeEntry }

// Validate returns an error if the UIConfig has invalid fields.
// It delegates to ColorScheme.Validate(); bool fields need no validation.
func (c UIConfig) Validate() error {
	var errs []error
	if err := c.ColorScheme.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidUIConfigError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidUIConfigError.
func (e *InvalidUIConfigError) Error() string {
	return fmt.Sprintf("invalid UI config: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidUIConfig for errors.Is() compatibility.
func (e *InvalidUIConfigError) Unwrap() error { return ErrInvalidUIConfig }

// Validate returns an error if the AutoProvisionConfig has invalid fields.
// It delegates to BinaryPath.Validate(), each Includes entry's Validate(),
// and CacheDir.Validate(). Bool fields (Enabled, Strict, InheritIncludes)
// need no validation.
func (c AutoProvisionConfig) Validate() error {
	var errs []error
	if err := c.BinaryPath.Validate(); err != nil {
		errs = append(errs, err)
	}
	for _, entry := range c.Includes {
		if err := entry.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.CacheDir.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidAutoProvisionConfigError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidAutoProvisionConfigError.
func (e *InvalidAutoProvisionConfigError) Error() string {
	return fmt.Sprintf("invalid auto-provision config: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidAutoProvisionConfig for errors.Is() compatibility.
func (e *InvalidAutoProvisionConfigError) Unwrap() error { return ErrInvalidAutoProvisionConfig }

// Validate returns an error if the ContainerConfig has invalid fields.
// It delegates to AutoProvision.Validate().
func (c ContainerConfig) Validate() error {
	var errs []error
	if err := c.AutoProvision.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidContainerConfigError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidContainerConfigError.
func (e *InvalidContainerConfigError) Error() string {
	return fmt.Sprintf("invalid container config: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidContainerConfig for errors.Is() compatibility.
func (e *InvalidContainerConfigError) Unwrap() error { return ErrInvalidContainerConfig }

// Validate returns an error if the Config has invalid fields.
// It delegates to ContainerEngine.Validate(), DefaultRuntime.Validate(),
// each Includes entry's Validate(), UI.Validate(), and Container.Validate().
// VirtualShell has only bool fields and needs no validation.
func (c Config) Validate() error {
	var errs []error
	if err := c.ContainerEngine.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.DefaultRuntime.Validate(); err != nil {
		errs = append(errs, err)
	}
	for _, entry := range c.Includes {
		if err := entry.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.UI.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Container.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidConfigError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidConfigError.
func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("invalid config: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidConfig for errors.Is() compatibility.
func (e *InvalidConfigError) Unwrap() error { return ErrInvalidConfig }

// String returns the string representation of the ModuleIncludePath.
func (p ModuleIncludePath) String() string { return string(p) }

// Validate returns an error if the ModuleIncludePath is invalid.
// A valid path must be non-empty and not whitespace-only.
func (p ModuleIncludePath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidModuleIncludePathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidModuleIncludePathError.
func (e *InvalidModuleIncludePathError) Error() string {
	return fmt.Sprintf("invalid module include path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidModuleIncludePath for errors.Is() compatibility.
func (e *InvalidModuleIncludePathError) Unwrap() error { return ErrInvalidModuleIncludePath }

// String returns the string representation of the BinaryFilePath.
func (p BinaryFilePath) String() string { return string(p) }

// Validate returns an error if the BinaryFilePath is invalid.
// The zero value ("") is valid (means "use auto-detected binary").
// Non-zero values must not be whitespace-only.
func (p BinaryFilePath) Validate() error {
	if p == "" {
		return nil
	}
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidBinaryFilePathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidBinaryFilePathError.
func (e *InvalidBinaryFilePathError) Error() string {
	return fmt.Sprintf("invalid binary file path %q: non-empty value must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidBinaryFilePath for errors.Is() compatibility.
func (e *InvalidBinaryFilePathError) Unwrap() error { return ErrInvalidBinaryFilePath }

// String returns the string representation of the CacheDirPath.
func (p CacheDirPath) String() string { return string(p) }

// Validate returns an error if the CacheDirPath is invalid.
// The zero value ("") is valid (means "use default cache directory").
// Non-zero values must not be whitespace-only.
func (p CacheDirPath) Validate() error {
	if p == "" {
		return nil
	}
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidCacheDirPathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidCacheDirPathError.
func (e *InvalidCacheDirPathError) Error() string {
	return fmt.Sprintf("invalid cache dir path %q: non-empty value must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidCacheDirPath for errors.Is() compatibility.
func (e *InvalidCacheDirPathError) Unwrap() error { return ErrInvalidCacheDirPath }

// Error implements the error interface for InvalidContainerEngineError.
func (e *InvalidContainerEngineError) Error() string {
	return fmt.Sprintf("invalid container engine %q (valid: podman, docker)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidContainerEngineError) Unwrap() error {
	return ErrInvalidContainerEngine
}

// String returns the string representation of the ContainerEngine.
func (ce ContainerEngine) String() string { return string(ce) }

// Validate returns an error if the ContainerEngine is not one of the defined engine types.
func (ce ContainerEngine) Validate() error {
	switch ce {
	case ContainerEnginePodman, ContainerEngineDocker:
		return nil
	default:
		return &InvalidContainerEngineError{Value: ce}
	}
}

// Error implements the error interface for InvalidConfigRuntimeModeError.
func (e *InvalidConfigRuntimeModeError) Error() string {
	return fmt.Sprintf("invalid runtime mode %q (valid: native, virtual, container)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidConfigRuntimeModeError) Unwrap() error {
	return ErrInvalidConfigRuntimeMode
}

// String returns the string representation of the config RuntimeMode.
func (m RuntimeMode) String() string { return string(m) }

// Validate returns an error if the config RuntimeMode is not one of the defined runtime modes.
func (m RuntimeMode) Validate() error {
	switch m {
	case RuntimeNative, RuntimeVirtual, RuntimeContainer:
		return nil
	default:
		return &InvalidConfigRuntimeModeError{Value: m}
	}
}

// Error implements the error interface for InvalidColorSchemeError.
func (e *InvalidColorSchemeError) Error() string {
	return fmt.Sprintf("invalid color scheme %q (valid: auto, dark, light)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidColorSchemeError) Unwrap() error {
	return ErrInvalidColorScheme
}

// String returns the string representation of the ColorScheme.
func (cs ColorScheme) String() string { return string(cs) }

// Validate returns an error if the ColorScheme is not one of the defined color schemes.
func (cs ColorScheme) Validate() error {
	switch cs {
	case ColorSchemeAuto, ColorSchemeDark, ColorSchemeLight:
		return nil
	default:
		return &InvalidColorSchemeError{Value: cs}
	}
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ContainerEngine: ContainerEnginePodman,
		Includes:        []IncludeEntry{},
		DefaultRuntime:  RuntimeNative,
		VirtualShell: VirtualShellConfig{
			EnableUrootUtils: true,
		},
		UI: UIConfig{
			ColorScheme: ColorSchemeAuto,
			Verbose:     false,
			Interactive: false,
		},
		Container: ContainerConfig{
			AutoProvision: AutoProvisionConfig{
				Enabled:         true,
				Strict:          false,
				BinaryPath:      "", // Will use os.Executable() if empty
				Includes:        []IncludeEntry{},
				InheritIncludes: true,
				CacheDir:        "", // Will use default cache dir if empty
			},
		},
	}
}
