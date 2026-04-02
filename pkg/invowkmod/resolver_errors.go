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
	// dependencies are not declared in the root invowkmod.cue. It contains actionable
	// diagnostics with ready-to-paste CUE snippets.
	MissingTransitiveDepError struct {
		Diagnostics []MissingTransitiveDepDiagnostic
	}
)

// CUESnippet produces a ready-to-paste CUE entry for the missing dependency.
// Delegates to formatRequiresEntry() to keep the format synchronized with
// what AddRequirement() writes to invowkmod.cue.
func (d MissingTransitiveDepDiagnostic) CUESnippet() string {
	lines := formatRequiresEntry(d.MissingRef)
	return strings.Join(lines, "\n")
}

// Error implements the error interface. It produces a multi-line message listing
// all missing transitive dependencies with actionable CUE snippets and a hint
// to run `invowk module tidy`.
func (e *MissingTransitiveDepError) Error() string {
	var sb strings.Builder

	count := len(e.Diagnostics)
	fmt.Fprintf(&sb, "%s: %d missing transitive dependency(ies)\n", missingTransitiveDepsErrMsg, count)

	for _, diag := range e.Diagnostics {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "Module %q (%s) requires\n", diag.RequiringModule, diag.RequiringURL)
		fmt.Fprintf(&sb, "%q but it is not declared in your invowkmod.cue.\n", diag.MissingRef.GitURL)
		sb.WriteString("\nAdd it to your requires list:\n")
		sb.WriteString(diag.CUESnippet())
		sb.WriteString("\n")
	}

	sb.WriteString("\nOr run: invowk module tidy")
	return sb.String()
}

// Unwrap returns ErrMissingTransitiveDeps so callers can use errors.Is for detection.
func (e *MissingTransitiveDepError) Unwrap() error { return ErrMissingTransitiveDeps }
