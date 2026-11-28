// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// WorkspaceMountRootInvowkfile mounts the directory containing the invowkfile/module at /workspace.
	WorkspaceMountRootInvowkfile WorkspaceMountRoot = "invowkfile"
	// WorkspaceMountRootWorkdir mounts the resolved effective workdir at /workspace (subject to containment).
	WorkspaceMountRootWorkdir WorkspaceMountRoot = "workdir"

	// WorkspaceMountSELinuxAuto applies :z when the container engine enforces SELinux (default).
	WorkspaceMountSELinuxAuto WorkspaceMountSELinuxLabel = "auto"
	// WorkspaceMountSELinuxShared always applies the shared SELinux label (:z) to /workspace.
	WorkspaceMountSELinuxShared WorkspaceMountSELinuxLabel = "shared"
	// WorkspaceMountSELinuxPrivate always applies the private SELinux label (:Z) to /workspace.
	WorkspaceMountSELinuxPrivate WorkspaceMountSELinuxLabel = "private"
	// WorkspaceMountSELinuxNone omits automatic :z/:Z labels on /workspace.
	WorkspaceMountSELinuxNone WorkspaceMountSELinuxLabel = "none"
)

var (
	// ErrInvalidWorkspaceMountRoot is returned when WorkspaceMountRoot is not recognized.
	ErrInvalidWorkspaceMountRoot = errors.New("invalid workspace mount root")
	// ErrInvalidWorkspaceMountSELinuxLabel is returned when WorkspaceMountSELinuxLabel is not recognized.
	ErrInvalidWorkspaceMountSELinuxLabel = errors.New("invalid workspace mount SELinux label")
	// ErrInvalidWorkspaceMountConfig is the sentinel error wrapped by InvalidWorkspaceMountConfigError.
	ErrInvalidWorkspaceMountConfig = errors.New("invalid workspace mount config")
)

type (
	// WorkspaceMountRoot selects which host directory is mounted at /workspace.
	// The zero value ("") is valid and defaults to "invowkfile".
	//
	//goplint:enum-cue=#WorkspaceMountRoot
	WorkspaceMountRoot string

	// InvalidWorkspaceMountRootError is returned when a WorkspaceMountRoot value is invalid.
	InvalidWorkspaceMountRootError struct {
		Value WorkspaceMountRoot
	}

	// WorkspaceMountSELinuxLabel controls SELinux relabeling on the /workspace mount.
	// The zero value ("") is valid and defaults to "auto".
	//
	//goplint:enum-cue=#WorkspaceMountSELinuxLabel
	WorkspaceMountSELinuxLabel string

	// InvalidWorkspaceMountSELinuxLabelError is returned when a WorkspaceMountSELinuxLabel value is invalid.
	InvalidWorkspaceMountSELinuxLabelError struct {
		Value WorkspaceMountSELinuxLabel
	}

	// InvalidWorkspaceMountConfigError is returned when a WorkspaceMountConfig has invalid fields.
	// It wraps ErrInvalidWorkspaceMountConfig for errors.Is() compatibility.
	InvalidWorkspaceMountConfigError struct {
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// WorkspaceMountConfig configures the automatic /workspace bind-mount for container runtimes.
	WorkspaceMountConfig struct {
		// Enabled controls whether Invowk mounts a host directory at /workspace.
		// Nil means default (true).
		Enabled *bool `json:"enabled,omitempty"`
		// Root selects the host root mounted at /workspace ("invowkfile" or "workdir").
		Root WorkspaceMountRoot `json:"root,omitempty"`
		// Readonly mounts /workspace read-only (:ro) when true.
		Readonly bool `json:"readonly,omitempty"`
		// SELinuxLabel controls SELinux relabeling on /workspace ("auto", "shared", "private", "none").
		SELinuxLabel WorkspaceMountSELinuxLabel `json:"selinux_label,omitempty"`
	}
)

// Error implements the error interface for InvalidWorkspaceMountRootError.
func (e *InvalidWorkspaceMountRootError) Error() string {
	return fmt.Sprintf("invalid workspace_mount.root %q (valid: invowkfile, workdir)", e.Value)
}

// Unwrap returns ErrInvalidWorkspaceMountRoot for errors.Is() compatibility.
func (e *InvalidWorkspaceMountRootError) Unwrap() error { return ErrInvalidWorkspaceMountRoot }

// String returns the string representation of WorkspaceMountRoot.
func (r WorkspaceMountRoot) String() string { return string(r) }

// Effective returns the configured root or the default ("invowkfile") when empty.
func (r WorkspaceMountRoot) Effective() WorkspaceMountRoot {
	if r == "" {
		return WorkspaceMountRootInvowkfile
	}
	return r
}

// Validate returns nil if WorkspaceMountRoot is empty (default) or a recognized value.
func (r WorkspaceMountRoot) Validate() error {
	switch r {
	case "", WorkspaceMountRootInvowkfile, WorkspaceMountRootWorkdir:
		return nil
	default:
		return &InvalidWorkspaceMountRootError{Value: r}
	}
}

// Error implements the error interface for InvalidWorkspaceMountSELinuxLabelError.
func (e *InvalidWorkspaceMountSELinuxLabelError) Error() string {
	return fmt.Sprintf("invalid workspace_mount.selinux_label %q (valid: auto, shared, private, none)", e.Value)
}

// Unwrap returns ErrInvalidWorkspaceMountSELinuxLabel for errors.Is() compatibility.
func (e *InvalidWorkspaceMountSELinuxLabelError) Unwrap() error {
	return ErrInvalidWorkspaceMountSELinuxLabel
}

// String returns the string representation of WorkspaceMountSELinuxLabel.
func (l WorkspaceMountSELinuxLabel) String() string { return string(l) }

// Effective returns the configured SELinux label mode or the default ("auto") when empty.
func (l WorkspaceMountSELinuxLabel) Effective() WorkspaceMountSELinuxLabel {
	if l == "" {
		return WorkspaceMountSELinuxAuto
	}
	return l
}

// Validate returns nil if WorkspaceMountSELinuxLabel is empty (default) or a recognized value.
func (l WorkspaceMountSELinuxLabel) Validate() error {
	switch l {
	case "", WorkspaceMountSELinuxAuto, WorkspaceMountSELinuxShared,
		WorkspaceMountSELinuxPrivate, WorkspaceMountSELinuxNone:
		return nil
	default:
		return &InvalidWorkspaceMountSELinuxLabelError{Value: l}
	}
}

// Error implements the error interface for InvalidWorkspaceMountConfigError.
func (e *InvalidWorkspaceMountConfigError) Error() string {
	return types.FormatFieldErrors("workspace mount config", e.FieldErrors)
}

// Unwrap returns ErrInvalidWorkspaceMountConfig for errors.Is() compatibility.
func (e *InvalidWorkspaceMountConfigError) Unwrap() error {
	return errors.Join(ErrInvalidWorkspaceMountConfig, errors.Join(e.FieldErrors...))
}

// IsEnabled reports whether /workspace mounting is enabled (default true when nil).
func (w *WorkspaceMountConfig) IsEnabled() bool {
	if w == nil || w.Enabled == nil {
		return true
	}
	return *w.Enabled
}

// EffectiveRoot returns the configured mount root, defaulting to "invowkfile".
func (w *WorkspaceMountConfig) EffectiveRoot() WorkspaceMountRoot {
	if w == nil {
		return WorkspaceMountRootInvowkfile
	}
	return w.Root.Effective()
}

// EffectiveSELinuxLabel returns the configured SELinux label mode, defaulting to "auto".
func (w *WorkspaceMountConfig) EffectiveSELinuxLabel() WorkspaceMountSELinuxLabel {
	if w == nil {
		return WorkspaceMountSELinuxAuto
	}
	return w.SELinuxLabel.Effective()
}

// Validate returns nil if all fields of WorkspaceMountConfig are valid (including nil).
func (w *WorkspaceMountConfig) Validate() error {
	if w == nil {
		return nil
	}
	var errs []error
	appendOptionalValidation(&errs, w.Root, w.Root != "")
	appendOptionalValidation(&errs, w.SELinuxLabel, w.SELinuxLabel != "")
	if len(errs) > 0 {
		return &InvalidWorkspaceMountConfigError{FieldErrors: errs}
	}
	return nil
}
