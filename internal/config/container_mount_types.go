// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// SELinuxLabelingEnabled applies automatic SELinux volume labels (:z/:Z) when active (default).
	SELinuxLabelingEnabled SELinuxLabeling = "enabled"
	// SELinuxLabelingDisabled passes --security-opt label=disable and omits automatic :z labels.
	SELinuxLabelingDisabled SELinuxLabeling = "disabled"

	maxIncludeWorkspaceRootRunes = 4096
)

var (
	// ErrInvalidSELinuxLabeling is returned when a SELinuxLabeling value is not recognized.
	ErrInvalidSELinuxLabeling = errors.New("invalid SELinux labeling policy")
	// ErrInvalidIncludeWorkspaceRootPath is returned when an IncludeWorkspaceRootPath value is invalid.
	ErrInvalidIncludeWorkspaceRootPath = errors.New("invalid include workspace root path")
)

type (
	// SELinuxLabeling is the operator-only container.selinux_labeling policy.
	// It is read only from config sources, never from an invowkfile or module.
	//
	//goplint:enum-cue=#SELinuxLabelingType
	SELinuxLabeling string

	// InvalidSELinuxLabelingError is returned when a SELinuxLabeling value is not recognized.
	InvalidSELinuxLabelingError struct {
		Value SELinuxLabeling
	}

	// IncludeWorkspaceRootPath is an optional absolute host path bounding container
	// workspace mounts and relative volumes for an included module.
	IncludeWorkspaceRootPath string

	// InvalidIncludeWorkspaceRootPathError is returned when IncludeWorkspaceRootPath is invalid.
	InvalidIncludeWorkspaceRootPathError struct {
		Value  IncludeWorkspaceRootPath
		Reason string
	}
)

// Error implements the error interface for InvalidSELinuxLabelingError.
func (e *InvalidSELinuxLabelingError) Error() string {
	return fmt.Sprintf("invalid container.selinux_labeling %q (valid: enabled, disabled)", e.Value)
}

// Unwrap returns ErrInvalidSELinuxLabeling for errors.Is() compatibility.
func (e *InvalidSELinuxLabelingError) Unwrap() error {
	return ErrInvalidSELinuxLabeling
}

// String returns the string representation of SELinuxLabeling.
func (s SELinuxLabeling) String() string { return string(s) }

// Effective returns the configured policy, defaulting the zero value to "enabled".
func (s SELinuxLabeling) Effective() SELinuxLabeling {
	if s == "" {
		return SELinuxLabelingEnabled
	}
	return s
}

// IsDisabled reports whether the operator disabled SELinux labeling for containers.
func (s SELinuxLabeling) IsDisabled() bool {
	return s == SELinuxLabelingDisabled
}

// Validate returns an error if SELinuxLabeling is not a recognized policy.
// The zero value means the default, "enabled".
func (s SELinuxLabeling) Validate() error {
	switch s {
	case "", SELinuxLabelingEnabled, SELinuxLabelingDisabled:
		return nil
	default:
		return &InvalidSELinuxLabelingError{Value: s}
	}
}

// Error implements the error interface for InvalidIncludeWorkspaceRootPathError.
func (e *InvalidIncludeWorkspaceRootPathError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid include workspace_root %q: %s", e.Value, e.Reason)
	}
	return fmt.Sprintf("invalid include workspace_root %q: must be an absolute path", e.Value)
}

// Unwrap returns ErrInvalidIncludeWorkspaceRootPath for errors.Is() compatibility.
func (e *InvalidIncludeWorkspaceRootPathError) Unwrap() error {
	return ErrInvalidIncludeWorkspaceRootPath
}

// String returns the string representation of IncludeWorkspaceRootPath.
func (p IncludeWorkspaceRootPath) String() string { return string(p) }

// Validate returns nil if IncludeWorkspaceRootPath is empty (omitted) or an
// OS-native absolute path within the rune limit.
//
// [GO-ONLY] OS-native absoluteness requires Go's filepath.IsAbs semantics.
func (p IncludeWorkspaceRootPath) Validate() error {
	if p == "" {
		return nil
	}
	pathStr := string(p)
	if strings.TrimSpace(pathStr) == "" {
		return &InvalidIncludeWorkspaceRootPathError{Value: p, Reason: "must not be whitespace-only"}
	}
	if utf8.RuneCountInString(pathStr) > maxIncludeWorkspaceRootRunes {
		return &InvalidIncludeWorkspaceRootPathError{Value: p, Reason: fmt.Sprintf("exceeds %d runes", maxIncludeWorkspaceRootRunes)}
	}
	if !filepath.IsAbs(pathStr) {
		return &InvalidIncludeWorkspaceRootPathError{Value: p, Reason: "must be an absolute path"}
	}
	return nil
}

// Validate returns an error if the IncludeEntry has invalid fields.
// It delegates to Path.Validate() unconditionally, and to Alias.Validate()
// and WorkspaceRoot.Validate() when non-empty.
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
	if e.WorkspaceRoot != "" {
		if err := e.WorkspaceRoot.Validate(); err != nil {
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
	return types.FormatFieldErrors("include entry", e.FieldErrors)
}

// Unwrap returns ErrInvalidIncludeEntry for errors.Is() compatibility.
func (e *InvalidIncludeEntryError) Unwrap() error { return ErrInvalidIncludeEntry }

// Validate returns an error if the ContainerConfig has invalid fields.
// It delegates to AutoProvision.Validate(), TUIServerBind.Validate(), and
// SELinuxLabeling.Validate().
func (c ContainerConfig) Validate() error {
	var errs []error
	if err := c.AutoProvision.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.TUIServerBind.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.SELinuxLabeling.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidContainerConfigError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidContainerConfigError.
func (e *InvalidContainerConfigError) Error() string {
	return types.FormatFieldErrors("container config", e.FieldErrors)
}

// Unwrap returns ErrInvalidContainerConfig and field errors for errors.Is/errors.As compatibility.
func (e *InvalidContainerConfigError) Unwrap() error {
	if e == nil {
		return ErrInvalidContainerConfig
	}
	return errors.Join(ErrInvalidContainerConfig, errors.Join(e.FieldErrors...))
}
