// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/invowk/invowk/internal/app/modulecache"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// moduleFetcher is the source-repository conversation required by Resolver.
	// GitFetcher is the production adapter; tests can inject deterministic fakes
	// without reaching the network or a real Git remote.
	moduleFetcher interface {
		ListVersions(ctx context.Context, gitURL GitURL) ([]SemVer, error)
		Fetch(ctx context.Context, gitURL GitURL, version SemVer) (types.FilesystemPath, GitCommit, error)
	}

	// Resolver handles module operations including resolution, caching, and synchronization.
	Resolver struct {
		// cacheDir is the root directory for module cache.
		cacheDir types.FilesystemPath

		// workingDir is the directory containing invowkmod.cue (for relative path resolution).
		workingDir types.FilesystemPath

		// fetcher handles source repository operations.
		fetcher moduleFetcher

		// semver handles version constraint resolution.
		semver *SemverResolver

		// mu protects concurrent access to the resolver.
		mu sync.Mutex
	}
)

// NewResolver creates a new module resolver.
//
// workingDir is the directory containing invowkmod.cue (typically current working directory).
// cacheDir can be empty to use the default (~/.invowk/modules or $INVOWK_MODULES_PATH).
func NewResolver(workingDir, cacheDir types.FilesystemPath) (*Resolver, error) {
	return newResolver(workingDir, cacheDir, nil, true)
}

func newResolverWithFetcher(workingDir, cacheDir types.FilesystemPath, fetcher moduleFetcher) (*Resolver, error) {
	return newResolver(workingDir, cacheDir, fetcher, true)
}

func newLockOnlyResolver(workingDir types.FilesystemPath) (*Resolver, error) {
	return newResolver(workingDir, "", nil, false)
}

func newResolver(workingDir, cacheDir types.FilesystemPath, fetcher moduleFetcher, ensureCacheDir bool) (*Resolver, error) {
	wd := string(workingDir)
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	absWorkingDir, err := filepath.Abs(wd)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}

	cd := cacheDir
	if cd == "" {
		cd, err = modulecache.DefaultDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get cache directory: %w", err)
		}
	}

	absCacheDir, err := filepath.Abs(string(cd))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	absCachePath := types.FilesystemPath(absCacheDir)
	if err := absCachePath.Validate(); err != nil {
		return nil, fmt.Errorf("cache directory: %w", err)
	}

	if ensureCacheDir {
		if err := os.MkdirAll(absCacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	absWorkingPath := types.FilesystemPath(absWorkingDir)
	if err := absWorkingPath.Validate(); err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}

	resolver := &Resolver{
		cacheDir:   absCachePath,
		workingDir: absWorkingPath,
		fetcher:    NewGitFetcher(absCachePath),
		semver:     invowkmod.NewSemverResolver(),
	}
	if fetcher != nil {
		resolver.fetcher = fetcher
	}
	return resolver, nil
}

// CacheDir returns the root directory for the module cache.
func (m *Resolver) CacheDir() types.FilesystemPath { return m.cacheDir }

// WorkingDir returns the directory containing invowkmod.cue.
func (m *Resolver) WorkingDir() types.FilesystemPath { return m.workingDir }

// Add resolves a new module requirement, caches it, and updates the lock file.
func (m *Resolver) Add(ctx context.Context, req ModuleRef) (*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate the requirement
	if err := m.validateModuleRef(req); err != nil {
		return nil, fmt.Errorf("invalid requirement: %w", err)
	}

	// Load existing lock file hashes for cache tamper detection.
	knownHashes, err := m.loadExistingLockHashes()
	if err != nil {
		return nil, err
	}

	// Resolve the module
	resolved, err := m.resolveOne(ctx, req, knownHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve module: %w", err)
	}

	// Persist to lock file so Add is a complete single-step operation
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, err)
	}
	// Reject v1.0 lock files — require upgrade to v2.0 for tamper detection.
	if v2Err := lock.RequireV2(); v2Err != nil {
		return nil, v2Err
	}
	lock.AddModule(resolved)
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf(errFmtSaveLockFile, err)
	}

	return resolved, nil
}

// Remove removes module(s) matching the identifier from the lock file.
// The identifier can be a git URL, lock file key, namespace, or module name.
func (m *Resolver) Remove(_ context.Context, identifier string) ([]RemoveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, err)
	}
	// Reject v1.0 lock files — require upgrade to v2.0 for tamper detection.
	if v2Err := lock.RequireV2(); v2Err != nil {
		return nil, v2Err
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
		return nil, fmt.Errorf(errFmtSaveLockFile, err)
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
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, err)
	}
	// Reject v1.0 lock files — require upgrade to v2.0 for tamper detection.
	if v2Err := lock.RequireV2(); v2Err != nil {
		return nil, v2Err
	}

	// Determine which keys to update
	var keysToUpdate []ModuleRefKey
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

	for _, key := range keysToUpdate {
		entry := lock.Modules[key]

		// Re-resolve to get the latest matching version
		req := ModuleRef{
			GitURL:  entry.GitURL,
			Version: entry.Version,
			Alias:   entry.Alias,
			Path:    entry.Path,
		}

		resolved, err := m.resolveOne(ctx, req, lock.ContentHashes())
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
			ModuleID:        resolved.ModuleID,
			ContentHash:     resolved.ContentHash,
		}

		updated = append(updated, resolved)
	}

	// Save updated lock file
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf(errFmtSaveLockFile, err)
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

	// Load existing lock file hashes for cache tamper detection.
	// When re-syncing, cached modules are verified against the prior
	// lock file's content hashes to detect tampering of the local cache.
	knownHashes, err := m.loadExistingLockHashes()
	if err != nil {
		return nil, err
	}

	// Resolve only direct dependencies (no transitive recursion).
	resolved, err := m.resolveAll(ctx, requirements, knownHashes)
	if err != nil {
		return nil, err
	}

	// Validate that all transitive deps are explicitly declared in root requirements.
	// If any are missing, return an actionable error suggesting `invowk module tidy`.
	if diags := invowkmod.CheckMissingTransitiveDeps(requirements, resolved); len(diags) > 0 {
		return nil, &MissingTransitiveDepError{Diagnostics: diags}
	}

	// Save lock file
	lock := &LockFile{
		Version: "2.0",
		Modules: make(map[ModuleRefKey]LockedModule),
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
			ModuleID:        mod.ModuleID,
			ContentHash:     mod.ContentHash,
		}
	}

	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf(errFmtSaveLockFile, err)
	}

	return resolved, nil
}

// List returns all currently resolved modules from the lock file.
func (m *Resolver) List(_ context.Context) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lockSnapshot := invowkmod.InspectLockFile(types.FilesystemPath(lockPath))
	if lockSnapshot.StatErr != nil || lockSnapshot.ParseErr != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, errors.Join(lockSnapshot.StatErr, lockSnapshot.ParseErr))
	}
	if !lockSnapshot.Present {
		return nil, nil
	}
	lock := lockSnapshot.LockFile

	var modules []*ResolvedModule
	for key := range lock.Modules {
		entry := lock.Modules[key]
		modules = append(modules, m.resolvedModuleFromLockEntry(key, entry))
	}

	return modules, nil
}

// LoadFromLock loads modules from an existing lock file without re-resolving.
// This is used for command discovery when a lock file already exists.
func (m *Resolver) LoadFromLock(_ context.Context) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, err := m.loadV2LockFile()
	if err != nil {
		return nil, err
	}

	modules := make([]*ResolvedModule, 0, len(lock.Modules))
	for key := range lock.Modules {
		entry := lock.Modules[key]
		modules = append(modules, m.resolvedModuleFromLockEntry(key, entry))
	}
	return modules, nil
}

// LoadDeclaredFromLock loads only the modules declared by requirements from an
// existing lock file. Lock-only entries are ignored so callers whose source of
// truth is invowkmod.cue can prune stale vendored modules instead of
// accidentally preserving them.
func (m *Resolver) LoadDeclaredFromLock(_ context.Context, requirements []ModuleRef) ([]*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, err := m.loadV2LockFile()
	if err != nil {
		return nil, err
	}

	modules := make([]*ResolvedModule, 0, len(requirements))
	for _, req := range requirements {
		key := req.Key()
		entry, ok := lock.Modules[key]
		if !ok {
			return nil, fmt.Errorf("declared module %s missing from lock file", key)
		}
		modules = append(modules, m.resolvedModuleFromLockEntry(key, entry))
	}
	return modules, nil
}

func (m *Resolver) loadV2LockFile() (*LockFile, error) {
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, err)
	}
	if err := lock.RequireV2(); err != nil {
		return nil, err
	}
	return lock, nil
}

func (m *Resolver) resolvedModuleFromLockEntry(key ModuleRefKey, entry LockedModule) *ResolvedModule {
	return &ResolvedModule{
		ModuleRef: ModuleRef{
			GitURL:  entry.GitURL,
			Version: entry.Version,
			Alias:   entry.Alias,
			Path:    entry.Path,
		},
		ResolvedVersion: entry.ResolvedVersion,
		GitCommit:       entry.GitCommit,
		CachePath:       types.FilesystemPath(m.getCachePath(string(entry.GitURL), string(entry.ResolvedVersion), string(entry.Path))),
		Namespace:       entry.Namespace,
		ModuleID:        entry.ModuleID,
		ModuleName:      extractModuleName(key),
		ContentHash:     entry.ContentHash,
	}
}

// loadExistingLockHashes loads content hashes from the existing lock file for
// cache tamper detection. A missing lock file means no prior hashes exist; an
// unreadable or invalid lock file is an integrity failure and must not silently
// downgrade cache verification.
func (m *Resolver) loadExistingLockHashes() (map[ModuleRefKey]ContentHash, error) {
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lockSnapshot := invowkmod.InspectLockFile(types.FilesystemPath(lockPath))
	if lockSnapshot.StatErr != nil || lockSnapshot.ParseErr != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, errors.Join(lockSnapshot.StatErr, lockSnapshot.ParseErr))
	}
	if !lockSnapshot.Present {
		return map[ModuleRefKey]ContentHash{}, nil
	}
	return lockSnapshot.LockFile.ContentHashes(), nil
}

// isGitURL returns true if s looks like a Git URL.
// Matches the CUE schema regex at invowkmod_schema.cue (https://, git@, ssh://).
func isGitURL(s string) bool {
	return isSupportedGitURLPrefix(s)
}

// resolveIdentifier resolves a user-provided identifier to lock file keys.
// Priority: git URL prefix match → exact lock key → exact namespace → namespace prefix.
// Returns matched keys or an error if no match or ambiguous.
func resolveIdentifier(identifier string, modules map[ModuleRefKey]LockedModule) ([]ModuleRefKey, error) {
	if isGitURL(identifier) {
		// Git URL mode: prefix-match on lock keys (preserves monorepo #subpath matching)
		var keys []ModuleRefKey
		for key := range modules {
			if strings.HasPrefix(string(key), identifier) {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("no module found matching git URL %q", identifier)
		}
		return keys, nil
	}

	// Exact lock key match
	if _, ok := modules[ModuleRefKey(identifier)]; ok {
		return []ModuleRefKey{ModuleRefKey(identifier)}, nil
	}

	// Namespace match: exact and prefix (bare name without @version)
	var exactMatches, prefixMatches []ModuleRefKey
	for key := range modules {
		entry := modules[key]
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
func buildAmbiguousError(identifier string, keys []ModuleRefKey, modules map[ModuleRefKey]LockedModule) *AmbiguousIdentifierError {
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
