// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"strings"
)

const (
	missingTransitiveDepsErrMsg = "module dependency check failed: missing transitive dependencies"
)

// ErrMissingTransitiveDeps is returned by Sync() when resolved modules declare
// transitive dependencies that are not explicitly listed in the root invowkmod.cue.
var ErrMissingTransitiveDeps = errors.New(missingTransitiveDepsErrMsg)

type (
	// MissingTransitiveDepDiagnostic describes a single transitive dependency that
	// a resolved module requires but is not declared in the root invowkmod.cue.
	MissingTransitiveDepDiagnostic struct {
		// RequiringModule is the module ID of the resolved module that declares the dep.
		RequiringModule ModuleID
		// RequiringURL is the Git URL of the resolved module that declares the dep.
		RequiringURL GitURL
		// MissingRef is the undeclared transitive dependency reference.
		MissingRef ModuleRef
	}

	// MissingTransitiveDepError is returned by Sync() when one or more transitive
	// dependencies are not declared in the root invowkmod.cue.
	MissingTransitiveDepError struct {
		Diagnostics []MissingTransitiveDepDiagnostic
	}
)

// Validate returns nil if the diagnostic has valid fields, or an error
// collecting all field-level validation failures.
func (d MissingTransitiveDepDiagnostic) Validate() error {
	var errs []error
	if err := d.RequiringModule.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.RequiringURL.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.MissingRef.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid missing transitive dep diagnostic: %d field error(s): %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// CUESnippet produces a ready-to-paste CUE entry for the missing dependency.
// Delegates to formatRequiresEntry() to keep the format synchronized with
// what AddRequirement() writes to invowkmod.cue.
func (d MissingTransitiveDepDiagnostic) CUESnippet() string {
	lines := formatRequiresEntry(d.MissingRef)
	return strings.Join(lines, "\n")
}

// Error implements the error interface with a terse domain summary. Adapters
// render CLI-specific remediation such as CUE snippets and tidy hints.
func (e *MissingTransitiveDepError) Error() string {
	return fmt.Sprintf("%s: %d missing transitive dependency(ies)", missingTransitiveDepsErrMsg, len(e.Diagnostics))
}

// Unwrap returns ErrMissingTransitiveDeps so callers can use errors.Is for detection.
func (e *MissingTransitiveDepError) Unwrap() error { return ErrMissingTransitiveDeps }
