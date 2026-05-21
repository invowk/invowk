// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const virtualFilesystemPathValueMaxRunes = 4096

var (
	// ErrInvalidVirtualFilesystemPathName is returned when a path handle cannot be exposed as a safe env suffix.
	ErrInvalidVirtualFilesystemPathName = errors.New("invalid virtual filesystem path name")
	// ErrInvalidVirtualFilesystemPathConfig is returned when a path handle value is malformed.
	ErrInvalidVirtualFilesystemPathConfig = errors.New("invalid virtual filesystem path config")

	virtualFilesystemPathNameRegex = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
)

type (
	// VirtualFilesystemPathName is a logical path key exposed as INVOWK_PATH_<NAME>.
	VirtualFilesystemPathName string

	// VirtualFilesystemPath is a platform-local path exposed as a named bridge handle.
	VirtualFilesystemPath string

	// VirtualFilesystemPaths maps logical names to platform-local path handles.
	VirtualFilesystemPaths map[VirtualFilesystemPathName]VirtualFilesystemPath

	// InvalidVirtualFilesystemPathNameError reports an invalid virtual filesystem path key.
	InvalidVirtualFilesystemPathNameError struct {
		Value VirtualFilesystemPathName
	}

	// InvalidVirtualFilesystemPathConfigError reports a malformed virtual filesystem path value.
	InvalidVirtualFilesystemPathConfigError struct {
		Name   VirtualFilesystemPathName
		Reason string
	}

	//goplint:validate-all
	//
	// PlatformVirtualConfig contains platform-specific settings for the virtual runtime family.
	PlatformVirtualConfig struct {
		// Filesystem configures virtual-runtime filesystem access for this platform.
		Filesystem *VirtualFilesystemConfig `json:"filesystem,omitempty"`
	}

	//goplint:validate-all
	//
	// VirtualFilesystemConfig configures VM-managed filesystem access for virtual runtimes.
	VirtualFilesystemConfig struct {
		// Access controls whether file operations are restricted or full.
		Access VirtualFilesystemAccess `json:"access,omitempty"`
		// Paths exposes named platform-local path handles to virtual runtimes.
		Paths VirtualFilesystemPaths `json:"paths,omitempty"`
	}
)

func (n VirtualFilesystemPathName) String() string { return string(n) }

// Validate returns nil when the path handle name is a safe environment suffix.
func (n VirtualFilesystemPathName) Validate() error {
	if !virtualFilesystemPathNameRegex.MatchString(string(n)) {
		return &InvalidVirtualFilesystemPathNameError{Value: n}
	}
	return nil
}

func (e *InvalidVirtualFilesystemPathNameError) Error() string {
	return fmt.Sprintf("invalid virtual.filesystem.paths key %q (must match [A-Z_][A-Z0-9_]*)", e.Value)
}

func (e *InvalidVirtualFilesystemPathNameError) Unwrap() error {
	return ErrInvalidVirtualFilesystemPathName
}

func (e *InvalidVirtualFilesystemPathConfigError) Error() string {
	if e.Name == "" {
		return "invalid virtual.filesystem.paths config: " + e.Reason
	}
	return fmt.Sprintf("invalid virtual.filesystem.paths[%q]: %s", e.Name, e.Reason)
}

func (e *InvalidVirtualFilesystemPathConfigError) Unwrap() error {
	return ErrInvalidVirtualFilesystemPathConfig
}

// Validate returns nil when the path value is non-empty and within the schema length limit.
func (p VirtualFilesystemPath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidVirtualFilesystemPathConfigError{Reason: "path must not be empty or whitespace-only"}
	}
	if len([]rune(p)) > virtualFilesystemPathValueMaxRunes {
		return &InvalidVirtualFilesystemPathConfigError{Reason: fmt.Sprintf("path must be at most %d runes", virtualFilesystemPathValueMaxRunes)}
	}
	return nil
}

func (p VirtualFilesystemPath) String() string { return string(p) }

// Validate returns nil when all virtual filesystem path names and values are valid.
func (p VirtualFilesystemPaths) Validate() error {
	if len(p) == 0 {
		return nil
	}
	var errs []error
	for name, path := range p {
		if err := name.Validate(); err != nil {
			errs = append(errs, err)
		}
		if err := path.Validate(); err != nil {
			errs = append(errs, namedVirtualFilesystemPathError(name, err))
		}
	}
	return errors.Join(errs...)
}

// StringMap returns the paths keyed by plain string names.
func (p VirtualFilesystemPaths) StringMap() map[string]string {
	if len(p) == 0 {
		return nil
	}
	values := make(map[string]string, len(p))
	for name, path := range p {
		values[name.String()] = path.String()
	}
	return values
}

func namedVirtualFilesystemPathError(name VirtualFilesystemPathName, err error) error {
	if typed, ok := errors.AsType[*InvalidVirtualFilesystemPathConfigError](err); ok {
		typed.Name = name
		return typed
	}
	return &InvalidVirtualFilesystemPathConfigError{Name: name, Reason: err.Error()}
}

// Validate returns nil when the virtual namespace config is valid.
func (c PlatformVirtualConfig) Validate() error {
	var errs []error
	appendOptionalValidation(&errs, c.Filesystem, c.Filesystem != nil)
	return errors.Join(errs...)
}

// Validate returns nil when the virtual filesystem config is valid.
func (c VirtualFilesystemConfig) Validate() error {
	var errs []error
	appendFieldError(&errs, c.Access.Validate())
	appendFieldError(&errs, c.Paths.Validate())
	return errors.Join(errs...)
}

// EffectiveAccess returns the configured access mode, defaulting to restricted.
func (c VirtualFilesystemConfig) EffectiveAccess() VirtualFilesystemAccess {
	return c.Access.Effective()
}

// HasFilesystemConfig returns true when this config has explicit filesystem settings.
func (c VirtualFilesystemConfig) HasFilesystemConfig() bool {
	return c.Access != "" || len(c.Paths) > 0
}

// HasConfig returns true when the virtual namespace has any configured fields.
func (c PlatformVirtualConfig) HasConfig() bool {
	if c.Filesystem == nil {
		return false
	}
	return c.Filesystem.HasFilesystemConfig()
}
