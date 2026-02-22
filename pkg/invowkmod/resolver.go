// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LockFileName is the name of the lock file.
// The lock file pairs naturally with invowkmod.cue (like go.sum pairs with go.mod).
const LockFileName = "invowkmod.lock.cue"

type (
	// ModuleRef represents a module dependency declaration from invowkmod.cue.
	ModuleRef struct {
		// GitURL is the Git repository URL (HTTPS or SSH format).
		// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
		GitURL GitURL

		// Version is the semver constraint for version selection.
		// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
		Version SemVerConstraint

		// Alias overrides the default namespace for imported commands (optional).
		// If not set, the namespace is: <module>@<resolved-version>
		Alias ModuleAlias

		// Path specifies a subdirectory containing the module (optional).
		// Used for monorepos with multiple modules.
		Path SubdirectoryPath
	}

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
		CachePath string

		// Namespace is the computed namespace for this module's commands.
		// Format: "<module>@<version>" or alias if specified.
		Namespace ModuleNamespace

		// ModuleName is the name of the module (from the folder name without .invowkmod).
		ModuleName string

		// ModuleID is the module identifier from the module's invowkmod.cue.
		ModuleID ModuleID

		// TransitiveDeps are dependencies declared by this module (for recursive resolution).
		TransitiveDeps []ModuleRef
	}

	// Resolver handles module operations including resolution, caching, and synchronization.
	Resolver struct {
		// cacheDir is the root directory for module cache.
		cacheDir string

		// workingDir is the directory containing invowkmod.cue (for relative path resolution).
		workingDir string

		// fetcher handles Git operations.
		fetcher *GitFetcher

		// semver handles version constraint resolution.
		semver *SemverResolver

		// mu protects concurrent access to the resolver.
		mu sync.Mutex
	}

	// RemoveResult contains metadata about a removed module for CLI reporting.
	RemoveResult struct {
		// LockKey is the lock file key that was removed.
		LockKey string
		// RemovedEntry is the lock file entry that was removed.
		RemovedEntry LockedModule
	}

	// AmbiguousMatch describes a single ambiguous lock file entry.
	AmbiguousMatch struct {
		// LockKey is the lock file key.
		LockKey string
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

// Key returns a unique key for this requirement based on GitURL and Path.
func (r ModuleRef) Key() string {
	if r.Path != "" {
		return fmt.Sprintf("%s#%s", r.GitURL, string(r.Path))
	}
	return string(r.GitURL)
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

// NewResolver creates a new module resolver.
//
// workingDir is the directory containing invowkmod.cue (typically current working directory).
// cacheDir can be empty to use the default (~/.invowk/modules or $INVOWK_MODULES_PATH).
func NewResolver(workingDir, cacheDir string) (*Resolver, error) {
	if workingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		workingDir = wd
	}

	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}

	if cacheDir == "" {
		cacheDir, err = GetDefaultCacheDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get cache directory: %w", err)
		}
	}

	absCacheDir, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(absCacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Resolver{
		cacheDir:   absCacheDir,
		workingDir: absWorkingDir,
		fetcher:    NewGitFetcher(absCacheDir),
		semver:     NewSemverResolver(),
	}, nil
}

// CacheDir returns the root directory for the module cache.
func (m *Resolver) CacheDir() string { return m.cacheDir }

// WorkingDir returns the directory containing invowkmod.cue.
func (m *Resolver) WorkingDir() string { return m.workingDir }

// Add resolves a new module requirement, caches it, and updates the lock file.
func (m *Resolver) Add(ctx context.Context, req ModuleRef) (*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate the requirement
	if err := m.validateModuleRef(req); err != nil {
		return nil, fmt.Errorf("invalid requirement: %w", err)
	}

	// Resolve the module
	resolved, err := m.resolveOne(ctx, req, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve module: %w", err)
	}

	// Persist to lock file so Add is a complete single-step operation
	lockPath := filepath.Join(m.workingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock file: %w", err)
	}
	lock.AddModule(resolved)
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf("failed to save lock file: %w", err)
	}

	return resolved, nil
}

// Remove removes module(s) matching the identifier from the lock file.
// The identifier can be a git URL, lock file key, namespace, or module name.
func (m *Resolver) Remove(_ context.Context, identifier string) ([]RemoveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(m.workingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock file: %w", err)
	}

	// Resolve identifier to lock file keys
	keys, err := resolveIdentifier(identifier, lock.Modules)
	if err != nil {
		return nil, err
	}

	// Collect results and delete entries
	results := make([]RemoveResult, 0, len(keys))
	for _, key := range keys {
		entry := lock.Modules[key]
		results = append(results, RemoveResult{
			LockKey:      key,
			RemovedEntry: entry,
		})
		delete(lock.Modules, key)
	}

	// Save updated lock file
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf("failed to save lock file: %w", err)
	}

	return results, nil
}

// Update updates one or all modules to their latest matching versions.
// If identifier is empty, all modules are updated. The identifier can be a
// git URL, lock file key, namespace, or module name.
func (m *Resolver) Update(ctx context.Context, identifier string) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(m.workingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock file: %w", err)
	}

	// Determine which keys to update
	var keysToUpdate []string
	if identifier == "" {
		for key := range lock.Modules {
			keysToUpdate = append(keysToUpdate, key)
		}
	} else {
		keysToUpdate, err = resolveIdentifier(identifier, lock.Modules)
		if err != nil {
			return nil, err
		}
	}

	var updated []*ResolvedModule
	visited := make(map[string]bool)

	for _, key := range keysToUpdate {
		entry := lock.Modules[key]

		// Re-resolve to get the latest matching version
		req := ModuleRef{
			GitURL:  entry.GitURL,
			Version: entry.Version,
			Alias:   entry.Alias,
			Path:    entry.Path,
		}

		resolved, err := m.resolveOne(ctx, req, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to update %s: %w", key, err)
		}

		// Update lock entry
		lock.Modules[key] = LockedModule{
			GitURL:          resolved.ModuleRef.GitURL,
			Version:         resolved.ModuleRef.Version,
			ResolvedVersion: resolved.ResolvedVersion,
			GitCommit:       resolved.GitCommit,
			Alias:           resolved.ModuleRef.Alias,
			Path:            resolved.ModuleRef.Path,
			Namespace:       resolved.Namespace,
		}

		updated = append(updated, resolved)
	}

	// Save updated lock file
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf("failed to save lock file: %w", err)
	}

	return updated, nil
}

// Sync resolves all requirements from invowkmod.cue and updates the lock file.
func (m *Resolver) Sync(ctx context.Context, requirements []ModuleRef) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(requirements) == 0 {
		return nil, nil
	}

	// Build dependency graph and resolve all modules
	resolved, err := m.resolveAll(ctx, requirements)
	if err != nil {
		return nil, err
	}

	// Save lock file
	lock := &LockFile{
		Version: "1.0",
		Modules: make(map[string]LockedModule),
	}

	for _, mod := range resolved {
		lock.Modules[mod.ModuleRef.Key()] = LockedModule{
			GitURL:          mod.ModuleRef.GitURL,
			Version:         mod.ModuleRef.Version,
			ResolvedVersion: mod.ResolvedVersion,
			GitCommit:       mod.GitCommit,
			Alias:           mod.ModuleRef.Alias,
			Path:            mod.ModuleRef.Path,
			Namespace:       mod.Namespace,
		}
	}

	lockPath := filepath.Join(m.workingDir, LockFileName)
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf("failed to save lock file: %w", err)
	}

	return resolved, nil
}

// List returns all currently resolved modules from the lock file.
func (m *Resolver) List(ctx context.Context) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lockPath := filepath.Join(m.workingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load lock file: %w", err)
	}

	var modules []*ResolvedModule
	for key, entry := range lock.Modules {
		modules = append(modules, &ResolvedModule{
			ModuleRef: ModuleRef{
				GitURL:  entry.GitURL,
				Version: entry.Version,
				Alias:   entry.Alias,
				Path:    entry.Path,
			},
			ResolvedVersion: entry.ResolvedVersion,
			GitCommit:       entry.GitCommit,
			CachePath:       m.getCachePath(string(entry.GitURL), string(entry.ResolvedVersion), string(entry.Path)),
			Namespace:       entry.Namespace,
			ModuleName:      extractModuleName(key),
		})
	}

	return modules, nil
}

// LoadFromLock loads modules from an existing lock file without re-resolving.
// This is used for command discovery when a lock file already exists.
func (m *Resolver) LoadFromLock(ctx context.Context) ([]*ResolvedModule, error) {
	return m.List(ctx)
}

// isGitURL returns true if s looks like a Git URL.
// Matches the CUE schema regex at invowkmod_schema.cue (https://, git@, ssh://).
func isGitURL(s string) bool {
	return isSupportedGitURLPrefix(s)
}

// resolveIdentifier resolves a user-provided identifier to lock file keys.
// Priority: git URL prefix match → exact lock key → exact namespace → namespace prefix.
// Returns matched keys or an error if no match or ambiguous.
func resolveIdentifier(identifier string, modules map[string]LockedModule) ([]string, error) {
	if isGitURL(identifier) {
		// Git URL mode: prefix-match on lock keys (preserves monorepo #subpath matching)
		var keys []string
		for key := range modules {
			if strings.HasPrefix(key, identifier) {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("no module found matching git URL %q", identifier)
		}
		return keys, nil
	}

	// Exact lock key match
	if _, ok := modules[identifier]; ok {
		return []string{identifier}, nil
	}

	// Namespace match: exact and prefix (bare name without @version)
	var exactMatches, prefixMatches []string
	for key, entry := range modules {
		if string(entry.Namespace) == identifier {
			exactMatches = append(exactMatches, key)
		} else if strings.HasPrefix(string(entry.Namespace), identifier+"@") {
			prefixMatches = append(prefixMatches, key)
		}
	}

	// Prefer exact namespace matches
	if len(exactMatches) == 1 {
		return exactMatches, nil
	}
	if len(exactMatches) > 1 {
		return nil, buildAmbiguousError(identifier, exactMatches, modules)
	}

	// Fall back to prefix matches (bare module name)
	if len(prefixMatches) == 1 {
		return prefixMatches, nil
	}
	if len(prefixMatches) > 1 {
		return nil, buildAmbiguousError(identifier, prefixMatches, modules)
	}

	return nil, fmt.Errorf("no module found matching %q", identifier)
}

// buildAmbiguousError creates an AmbiguousIdentifierError from matched keys.
func buildAmbiguousError(identifier string, keys []string, modules map[string]LockedModule) *AmbiguousIdentifierError {
	matches := make([]AmbiguousMatch, 0, len(keys))
	for _, key := range keys {
		entry := modules[key]
		matches = append(matches, AmbiguousMatch{
			LockKey:   key,
			Namespace: entry.Namespace,
			GitURL:    entry.GitURL,
		})
	}
	return &AmbiguousIdentifierError{
		Identifier: identifier,
		Matches:    matches,
	}
}
