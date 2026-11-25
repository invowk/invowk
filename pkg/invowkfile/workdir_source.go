// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
)

const (
	// WorkDirSourceCLIOverride marks a workdir given on the command line (--ivk-workdir).
	WorkDirSourceCLIOverride WorkDirSource = "cli-override"
	// WorkDirSourceImplementation marks a workdir declared by the selected implementation.
	WorkDirSourceImplementation WorkDirSource = "implementation"
	// WorkDirSourceCommand marks a workdir declared by the command.
	WorkDirSourceCommand WorkDirSource = "command"
	// WorkDirSourceRoot marks a workdir declared at the invowkfile root.
	WorkDirSourceRoot WorkDirSource = "root"
	// WorkDirSourceDefault marks the default workdir: the invowkfile directory,
	// or the module directory for a module.
	WorkDirSourceDefault WorkDirSource = "default"
)

// ErrInvalidWorkDirSource is the sentinel error wrapped by InvalidWorkDirSourceError.
var ErrInvalidWorkDirSource = errors.New("invalid workdir source")

type (
	// WorkDirSource names the level of the workdir precedence that supplied an
	// effective workdir.
	WorkDirSource string

	// InvalidWorkDirSourceError is returned when a WorkDirSource is not recognized.
	// It wraps ErrInvalidWorkDirSource for errors.Is() compatibility.
	InvalidWorkDirSourceError struct {
		Value WorkDirSource
	}

	// EffectiveWorkDir is the effective workdir of a command together with the
	// precedence level that supplied it.
	EffectiveWorkDir struct {
		// Path is the resolved workdir, as GetEffectiveWorkDir returns it.
		Path FilesystemPath
		// Source is the precedence level that supplied Path.
		Source WorkDirSource
	}
)

// Error implements the error interface for InvalidWorkDirSourceError.
func (e *InvalidWorkDirSourceError) Error() string {
	return fmt.Sprintf("invalid workdir source %q (valid: cli-override, implementation, command, root, default)", e.Value)
}

// Unwrap returns ErrInvalidWorkDirSource for errors.Is() compatibility.
func (e *InvalidWorkDirSourceError) Unwrap() error { return ErrInvalidWorkDirSource }

// String returns the string representation of the WorkDirSource.
func (s WorkDirSource) String() string { return string(s) }

// Validate returns nil if the WorkDirSource is one of the known levels.
func (s WorkDirSource) Validate() error {
	switch s {
	case WorkDirSourceCLIOverride, WorkDirSourceImplementation, WorkDirSourceCommand,
		WorkDirSourceRoot, WorkDirSourceDefault:
		return nil
	default:
		return &InvalidWorkDirSourceError{Value: s}
	}
}

// DeclaredByInvowkfile reports whether the workdir comes from the invowkfile's
// own CUE: its implementation, command or root workdir. The CLI override and
// the default are not declared by the invowkfile.
//
// The name is bound by the VirtualPathHarness correspondence table
// (formal/alloy/VirtualPathHarness.als); keep it.
func (s WorkDirSource) DeclaredByInvowkfile() bool {
	switch s {
	case WorkDirSourceImplementation, WorkDirSourceCommand, WorkDirSourceRoot:
		return true
	case WorkDirSourceCLIOverride, WorkDirSourceDefault:
		return false
	default:
		return false
	}
}

// Validate returns nil if both the path and the source are valid.
func (w EffectiveWorkDir) Validate() error {
	return errors.Join(w.Path.Validate(), w.Source.Validate())
}

// ResolveEffectiveWorkDir returns the effective workdir, as GetEffectiveWorkDir
// resolves it, together with the precedence level that supplied it.
//
// The name is bound by the VirtualPathHarness correspondence table
// (formal/alloy/VirtualPathHarness.als); keep it.
func (inv *Invowkfile) ResolveEffectiveWorkDir(cmd *Command, impl *Implementation, cliOverride WorkDir) EffectiveWorkDir {
	return EffectiveWorkDir{
		Path:   inv.GetEffectiveWorkDir(cmd, impl, cliOverride),
		Source: inv.effectiveWorkDirSource(cmd, impl, cliOverride),
	}
}

// effectiveWorkDirSource picks the precedence level that GetEffectiveWorkDir
// takes its value from, with the same tests in the same order.
//
// It sits beside GetEffectiveWorkDir on purpose and must not be merged into it:
// GetEffectiveWorkDir is bound by the ModulePathContainment correspondence
// table and by its mutation ledger entries, and changing its body would re-key
// them. TestResolveEffectiveWorkDir fails if the two precedences drift apart.
// The name is bound by the VirtualPathHarness correspondence table; keep it.
func (inv *Invowkfile) effectiveWorkDirSource(cmd *Command, impl *Implementation, cliOverride WorkDir) WorkDirSource {
	switch {
	case cliOverride != "":
		return WorkDirSourceCLIOverride
	case impl != nil && impl.WorkDir != "":
		return WorkDirSourceImplementation
	case cmd != nil && cmd.WorkDir != "":
		return WorkDirSourceCommand
	case inv.WorkDir != "":
		return WorkDirSourceRoot
	default:
		return WorkDirSourceDefault
	}
}
