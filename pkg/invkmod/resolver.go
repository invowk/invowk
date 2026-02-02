// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LockFileName is the name of the lock file.
// The lock file pairs naturally with invkmod.cue (like go.sum pairs with go.mod).
const LockFileName = "invkmod.lock.cue"

type (
	// ModuleRef represents a module dependency declaration from invkmod.cue.
	ModuleRef struct {
		// GitURL is the Git repository URL (HTTPS or SSH format).
		// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
		GitURL string

		// Version is the semver constraint for version selection.
		// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
		Version string

		// Alias overrides the default namespace for imported commands (optional).
		// If not set, the namespace is: <module>@<resolved-version>
		Alias string

		// Path specifies a subdirectory containing the module (optional).
		// Used for monorepos with multiple modules.
		Path string
	}

	// ResolvedModule represents a fully resolved and cached module.
	ResolvedModule struct {
		// ModuleRef is the original requirement that was resolved.
		ModuleRef ModuleRef

		// ResolvedVersion is the exact version that was selected.
		// This is always a concrete version (e.g., "1.2.3"), not a constraint.
		ResolvedVersion string

		// GitCommit is the Git commit SHA for the resolved version.
		GitCommit string

		// CachePath is the absolute path to the cached module directory.
		CachePath string

		// Namespace is the computed namespace for this module's commands.
		// Format: "<module>@<version>" or alias if specified.
		Namespace string

		// ModuleName is the name of the module (from the folder name without .invkmod).
		ModuleName string

		// ModuleID is the module identifier from the module's invkmod.cue.
		ModuleID string

		// TransitiveDeps are dependencies declared by this module (for recursive resolution).
		TransitiveDeps []ModuleRef
	}

	// Resolver handles module operations including resolution, caching, and synchronization.
	Resolver struct {
		// CacheDir is the root directory for module cache.
		CacheDir string

		// WorkingDir is the directory containing invkmod.cue (for relative path resolution).
		WorkingDir string

		// fetcher handles Git operations.
		fetcher *GitFetcher

		// semver handles version constraint resolution.
		semver *SemverResolver

		// mu protects concurrent access to the resolver.
		mu sync.Mutex
	}
)

// Key returns a unique key for this requirement based on GitURL and Path.
func (r ModuleRef) Key() string {
	if r.Path != "" {
		return fmt.Sprintf("%s#%s", r.GitURL, r.Path)
	}
	return r.GitURL
}

// String returns a human-readable representation of the requirement.
func (r ModuleRef) String() string {
	s := r.GitURL
	if r.Path != "" {
		s += "#" + r.Path
	}
	s += "@" + r.Version
	if r.Alias != "" {
		s += " (alias: " + r.Alias + ")"
	}
	return s
}

// NewResolver creates a new module resolver.
//
// workingDir is the directory containing invkmod.cue (typically current working directory).
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
		CacheDir:   absCacheDir,
		WorkingDir: absWorkingDir,
		fetcher:    NewGitFetcher(absCacheDir),
		semver:     NewSemverResolver(),
	}, nil
}

// Add resolves a new module requirement and returns the resolved metadata.
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

	return resolved, nil
}

// Remove removes a module requirement from the lock file.
func (m *Resolver) Remove(ctx context.Context, gitURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(m.WorkingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("failed to load lock file: %w", err)
	}

	// Find and remove the module
	found := false
	for key := range lock.Modules {
		if strings.HasPrefix(key, gitURL) {
			delete(lock.Modules, key)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("module not found: %s", gitURL)
	}

	// Save updated lock file
	if err := lock.Save(lockPath); err != nil {
		return fmt.Errorf("failed to save lock file: %w", err)
	}

	return nil
}

// Update updates one or all modules to their latest matching versions.
// If gitURL is empty, all modules are updated.
func (m *Resolver) Update(ctx context.Context, gitURL string) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(m.WorkingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock file: %w", err)
	}

	var updated []*ResolvedModule
	visited := make(map[string]bool)

	for key, entry := range lock.Modules {
		if gitURL != "" && !strings.HasPrefix(key, gitURL) {
			continue
		}

		// Re-resolve with force flag to bypass cache
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

// Sync resolves all requirements from invkmod.cue and updates the lock file.
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

	lockPath := filepath.Join(m.WorkingDir, LockFileName)
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf("failed to save lock file: %w", err)
	}

	return resolved, nil
}

// List returns all currently resolved modules from the lock file.
func (m *Resolver) List(ctx context.Context) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lockPath := filepath.Join(m.WorkingDir, LockFileName)
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
			CachePath:       m.getCachePath(entry.GitURL, entry.ResolvedVersion, entry.Path),
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
