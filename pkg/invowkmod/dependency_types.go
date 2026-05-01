// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// LockFileName is the name of the lock file.
	// The lock file pairs naturally with invowkmod.cue (like go.sum pairs with go.mod).
	LockFileName = "invowkmod.lock.cue"
)

var moduleSourceIDRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)

type (
	//goplint:validate-all
	//
	// ModuleRef represents a module dependency declaration from invowkmod.cue.
	ModuleRef struct {
		// GitURL is the Git repository URL (HTTPS or SSH format).
		// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
		GitURL GitURL

		// Version is the semver constraint for version selection.
		// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
		Version SemVerConstraint

		// Alias overrides the default command source for imported commands (optional).
		Alias ModuleAlias

		// Path specifies a subdirectory containing the module (optional).
		// Used for monorepos with multiple modules.
		Path SubdirectoryPath
	}

	//goplint:validate-all
	//
	// ResolvedModule represents a fully resolved and cached module.
	ResolvedModule struct {
		// ModuleRef is the original requirement that was resolved.
		ModuleRef ModuleRef

		// ResolvedVersion is the exact version that was selected.
		// This is always a concrete version (e.g., "1.2.3"), not a constraint.
		ResolvedVersion SemVer

		// GitCommit is the Git commit SHA for the resolved version.
		GitCommit GitCommit

		// CachePath is the absolute path to the cached module directory.
		CachePath types.FilesystemPath

		// Namespace is the computed display namespace.
		// Format: "<module>@<version>" or alias if specified.
		Namespace ModuleNamespace

		// CommandSourceID is the command-publishing namespace used by discovery.
		CommandSourceID ModuleSourceID

		// ModuleName is the name of the module (from the folder name without .invowkmod).
		ModuleName ModuleShortName

		// ModuleID is the module identifier from the module's invowkmod.cue.
		ModuleID ModuleID

		// TransitiveDeps are dependencies declared by this module (for validation of
		// explicit-only dependency model — transitive deps must be declared in root invowkmod.cue).
		TransitiveDeps []ModuleRef

		// ContentHash is the SHA-256 content hash of the cached module tree.
		ContentHash ContentHash
	}

	//goplint:validate-all
	//
	// RemoveResult contains metadata about a removed module for CLI reporting.
	RemoveResult struct {
		// LockKey is the lock file key that was removed.
		LockKey ModuleRefKey
		// RemovedEntry is the lock file entry that was removed.
		RemovedEntry LockedModule
	}

	//goplint:validate-all
	//
	// AmbiguousMatch describes a single ambiguous lock file entry.
	AmbiguousMatch struct {
		// LockKey is the lock file key.
		LockKey ModuleRefKey
		// Namespace is the computed namespace.
		Namespace ModuleNamespace
		// GitURL is the Git repository URL.
		GitURL GitURL
	}

	// AmbiguousIdentifierError is returned when a module identifier matches
	// multiple lock file entries and the user must be more specific.
	AmbiguousIdentifierError struct {
		// Identifier is the user-provided identifier that was ambiguous.
		Identifier string
		// Matches contains all matching entries.
		Matches []AmbiguousMatch
	}

	// ModuleSourceID identifies the source namespace used for module commands.
	ModuleSourceID string
)

// Error implements the error interface for AmbiguousIdentifierError.
func (e *AmbiguousIdentifierError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ambiguous identifier %q matches %d modules:\n", e.Identifier, len(e.Matches))
	for _, m := range e.Matches {
		fmt.Fprintf(&sb, "  - %s (namespace: %s, url: %s)\n", m.LockKey, m.Namespace, m.GitURL)
	}
	sb.WriteString("specify a more precise identifier to disambiguate")
	return sb.String()
}

// String returns the string representation of the module source ID.
func (id ModuleSourceID) String() string { return string(id) }

// Validate returns nil if the module source ID is non-empty.
func (id ModuleSourceID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return errors.New("module source ID must not be empty")
	}
	if !moduleSourceIDRegex.MatchString(string(id)) {
		return fmt.Errorf("module source ID %q must start with a letter and contain only letters, digits, dots, underscores, or hyphens", id)
	}
	return nil
}

// Key returns a unique key for this requirement based on GitURL and Path.
func (r ModuleRef) Key() ModuleRefKey {
	if r.Path != "" {
		return ModuleRefKey(fmt.Sprintf("%s#%s", r.GitURL, string(r.Path)))
	}
	return ModuleRefKey(r.GitURL)
}

// MatchesSourceID reports whether this requirement can publish commands under sourceID.
func (r ModuleRef) MatchesSourceID(sourceID ModuleSourceID) bool {
	return r.CommandSourceID() == sourceID
}

// CommandSourceID returns the source namespace used when publishing commands
// from this requirement.
func (r ModuleRef) CommandSourceID() ModuleSourceID {
	if r.Alias != "" {
		return ModuleSourceID(r.Alias)
	}
	return r.DefaultSourceID()
}

// EffectiveCommandSourceID returns the persisted command source ID for this
// lock entry, falling back to the historical alias/default-source derivation.
func (m LockedModule) EffectiveCommandSourceID() ModuleSourceID {
	if m.CommandSourceID != "" {
		return m.CommandSourceID
	}
	return ModuleRef{
		GitURL: m.GitURL,
		Alias:  m.Alias,
		Path:   m.Path,
	}.CommandSourceID()
}

// DefaultSourceID returns the command source namespace implied by this
// requirement when no alias is declared.
func (r ModuleRef) DefaultSourceID() ModuleSourceID {
	if r.Path != "" {
		return CommandSourceIDFromName(ModuleShortName(slashpath.Base(string(r.Path)))) //goplint:ignore -- basename normalized below before validation
	}
	return moduleSourceFromGitURL(r.GitURL)
}

// String returns a human-readable representation of the requirement.
func (r ModuleRef) String() string {
	s := string(r.GitURL)
	if r.Path != "" {
		s += "#" + string(r.Path)
	}
	s += "@" + string(r.Version)
	if r.Alias != "" {
		s += " (alias: " + string(r.Alias) + ")"
	}
	return s
}

func moduleSourceFromGitURL(gitURL GitURL) ModuleSourceID {
	urlPath := string(gitURL)
	if _, after, found := strings.Cut(urlPath, "://"); found {
		urlPath = after
	}
	if before, after, found := strings.Cut(urlPath, ":"); found && !strings.Contains(before, "/") {
		urlPath = after
	}
	return CommandSourceIDFromName(ModuleShortName(slashpath.Base(urlPath))) //goplint:ignore -- repository basename normalized below before validation
}

// CommandSourceIDFromModulePath returns the default command source namespace
// for a module directory path.
func CommandSourceIDFromModulePath(modulePath types.FilesystemPath) ModuleSourceID {
	return CommandSourceIDFromName(ModuleShortName(filepath.Base(string(modulePath)))) //goplint:ignore -- filesystem basename normalized below before validation
}

// CommandSourceIDFromName returns the default command source namespace for a
// module directory name, repository basename, or monorepo subdirectory basename.
func CommandSourceIDFromName(name ModuleShortName) ModuleSourceID {
	sourceName := strings.TrimSuffix(strings.TrimSuffix(string(name), ".git"), ModuleSuffix)
	sourceID := ModuleSourceID(sourceName)
	if err := sourceID.Validate(); err != nil {
		return ""
	}
	return sourceID
}
