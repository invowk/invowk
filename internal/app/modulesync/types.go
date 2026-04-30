// SPDX-License-Identifier: MPL-2.0

package modulesync

import "github.com/invowk/invowk/pkg/invowkmod"

const (
	// LockFileName is the name of the module dependency lock file.
	LockFileName = invowkmod.LockFileName

	errFmtCreateModuleResolver = "failed to create module resolver: %w"
	errFmtLoadLockFile         = "failed to load lock file: %w"
	errFmtSaveLockFile         = "failed to save lock file: %w"
)

var (
	// ErrGitURLRequired is returned when a module ref is missing git_url.
	ErrGitURLRequired = invowkmod.ErrGitURLRequired
	// ErrUnsupportedGitURLScheme is returned when git_url uses an unsupported scheme.
	ErrUnsupportedGitURLScheme = invowkmod.ErrUnsupportedGitURLScheme
	// ErrVersionRequired is returned when a module ref is missing version.
	ErrVersionRequired = invowkmod.ErrVersionRequired
	// ErrSSHKeyNotFound is returned when an SSH URL is used but no SSH key is configured.
	ErrSSHKeyNotFound = invowkmod.ErrSSHKeyNotFound
	// ErrTagNotFound is returned when a Git tag does not exist in a repository.
	ErrTagNotFound = invowkmod.ErrTagNotFound
	// ErrCloneFailed is returned when a Git clone operation fails for all attempted tag variants.
	ErrCloneFailed = invowkmod.ErrCloneFailed
)

type (
	// ModuleRef represents a module dependency declaration from invowkmod.cue.
	ModuleRef = invowkmod.ModuleRef
	// ResolvedModule represents a fully resolved and cached module.
	ResolvedModule = invowkmod.ResolvedModule
	// RemoveResult contains metadata about a removed module for CLI reporting.
	RemoveResult = invowkmod.RemoveResult
	// AmbiguousMatch describes a single ambiguous lock file entry.
	AmbiguousMatch = invowkmod.AmbiguousMatch
	// AmbiguousIdentifierError is returned when a module identifier matches multiple lock entries.
	AmbiguousIdentifierError = invowkmod.AmbiguousIdentifierError
	// MissingTransitiveDepDiagnostic describes a missing explicit transitive dependency.
	MissingTransitiveDepDiagnostic = invowkmod.MissingTransitiveDepDiagnostic
	// MissingTransitiveDepError reports missing explicit transitive dependencies.
	MissingTransitiveDepError = invowkmod.MissingTransitiveDepError

	// ModuleRefKey identifies a module requirement in the lock file.
	ModuleRefKey = invowkmod.ModuleRefKey
	// LockedModule is a module entry persisted in the lock file.
	LockedModule = invowkmod.LockedModule
	// LockFile is the parsed module dependency lock file.
	LockFile = invowkmod.LockFile
	// ContentHash is a SHA-256 module tree hash.
	ContentHash = invowkmod.ContentHash
	// GitURL represents a Git repository URL.
	GitURL = invowkmod.GitURL
	// GitCommit represents a Git commit SHA.
	GitCommit = invowkmod.GitCommit
	// SemVer is a concrete semantic version.
	SemVer = invowkmod.SemVer
	// SemVerConstraint is a semantic version constraint.
	SemVerConstraint = invowkmod.SemVerConstraint
	// SemverResolver resolves semantic version constraints.
	SemverResolver = invowkmod.SemverResolver
	// ModuleAlias overrides a module namespace.
	ModuleAlias = invowkmod.ModuleAlias
	// SubdirectoryPath identifies a module subdirectory inside a repository.
	SubdirectoryPath = invowkmod.SubdirectoryPath
	// ModuleNamespace identifies imported module command namespace.
	ModuleNamespace = invowkmod.ModuleNamespace
	// ModuleShortName is the local short module name.
	ModuleShortName = invowkmod.ModuleShortName
	// ModuleID is a module identifier.
	ModuleID = invowkmod.ModuleID
	// ModuleRequirement is a dependency declaration parsed from invowkmod.cue.
	ModuleRequirement = invowkmod.ModuleRequirement
)

func isValidVersionString(s string) bool {
	return invowkmod.SemVer(s).Validate() == nil //goplint:ignore -- raw git tag filtered before typed construction
}
