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

		// restoreWriter and restoreLinker are the rollback's restore seams,
		// carried into each snapshot; nil means fspath.AtomicWriteFile and
		// os.Symlink (design D8).
		restoreWriter restoreFileWriter
		restoreLinker restoreLinkCreator

		// locks takes the project and source locks; nil when the resolver
		// was built without WithEditLocks, which the locking methods refuse
		// (ErrEditLockerRequired).
		locks *editLocks

		// mu protects concurrent access to the resolver.
		mu sync.Mutex
	}
)

// NewResolver creates a new module resolver.
//
// workingDir is the directory containing invowkmod.cue (typically current working directory).
// cacheDir can be empty to use the default (~/.invowk/modules or $INVOWK_MODULES_PATH),
// or the CacheDir of the edit locks given with WithEditLocks.
// With edit locks, the cache root is created by LockProject, under the
// project lock's error handling, instead of here.
func NewResolver(workingDir, cacheDir types.FilesystemPath, opts ...ResolverOption) (*Resolver, error) {
	return newResolver(workingDir, cacheDir, nil, true, opts...)
}

func newResolverWithFetcher(workingDir, cacheDir types.FilesystemPath, fetcher moduleFetcher, opts ...ResolverOption) (*Resolver, error) {
	return newResolver(workingDir, cacheDir, fetcher, true, opts...)
}

func newLockOnlyResolver(workingDir types.FilesystemPath, opts ...ResolverOption) (*Resolver, error) {
	return newResolver(workingDir, "", nil, false, opts...)
}

func newResolver(workingDir, cacheDir types.FilesystemPath, fetcher moduleFetcher, ensureCacheDir bool, opts ...ResolverOption) (*Resolver, error) {
	var cfg resolverConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.editLocks != nil {
		if cacheDir == "" {
			cacheDir = cfg.editLocks.CacheDir
		}
		// LockProject creates the cache root with the lock directory's
		// actionable error (design D5).
		ensureCacheDir = false
	}

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
			return nil, fmt.Errorf("cannot determine the module cache directory: %w; set INVOWK_MODULES_PATH", err)
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
	if cfg.editLocks != nil {
		resolver.locks = &editLocks{locks: *cfg.editLocks, cacheDir: absCachePath}
		if err := resolver.locks.Validate(); err != nil {
			return nil, err
		}
	}
	return resolver, nil
}

// CacheDir returns the root directory for the module cache.
func (m *Resolver) CacheDir() types.FilesystemPath { return m.cacheDir }

// WorkingDir returns the directory containing invowkmod.cue.
func (m *Resolver) WorkingDir() types.FilesystemPath { return m.workingDir }

// Add resolves a new module requirement, caches it, and updates the lock file.
//
// lockWritten reports whether the lock file was written (false when the
// equal-content skip left it byte-identical, and on every error).
func (m *Resolver) Add(ctx context.Context, req ModuleRef) (added *ResolvedModule, lockWritten bool, addErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate the requirement
	if err := m.validateModuleRef(req); err != nil {
		return nil, false, fmt.Errorf("invalid requirement: %w", err)
	}

	// Load the existing lock entries for commit and content tamper detection.
	locked, err := m.loadLockedModules()
	if err != nil {
		return nil, false, err
	}

	// Resolve the module
	resolved, err := m.resolveOne(ctx, req, locked)
	if err != nil {
		return nil, false, fmt.Errorf("failed to resolve module: %w", err)
	}

	// Persist to lock file so Add is a complete single-step operation
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, false, fmt.Errorf(errFmtLoadLockFile, err)
	}
	// Reject v1.0 lock files — require upgrade to v2.0 for tamper detection.
	if v2Err := lock.RequireV2(); v2Err != nil {
		return nil, false, v2Err
	}
	lock.AddModule(resolved)
	lockWritten, saveErr := lock.SaveIfChanged(lockPath)
	if saveErr != nil {
		return nil, false, fmt.Errorf(errFmtSaveLockFile, saveErr)
	}

	return resolved, lockWritten, nil
}

// Remove removes module(s) matching the identifier from the lock file.
// The identifier can be a git URL, lock file key, namespace, or module name.
//
// lockWritten reports whether the lock file was written.
func (m *Resolver) Remove(_ context.Context, identifier string) (removed []RemoveResult, lockWritten bool, removeErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, false, fmt.Errorf(errFmtLoadLockFile, err)
	}
	// Reject v1.0 lock files — require upgrade to v2.0 for tamper detection.
	if v2Err := lock.RequireV2(); v2Err != nil {
		return nil, false, v2Err
	}

	// Resolve identifier to lock file keys
	keys, err := resolveIdentifier(identifier, lock.Modules)
	if err != nil {
		return nil, false, err
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
	lockWritten, saveErr := lock.SaveIfChanged(lockPath)
	if saveErr != nil {
		return nil, false, fmt.Errorf(errFmtSaveLockFile, saveErr)
	}

	return results, lockWritten, nil
}

// Update updates one or all modules to their latest matching versions.
// If identifier is empty, all modules are updated. The identifier can be a
// git URL, lock file key, namespace, or module name.
//
// lockWritten reports whether the lock file was written.
func (m *Resolver) Update(ctx context.Context, identifier string) (updatedModules []*ResolvedModule, lockWritten bool, updateErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return nil, false, fmt.Errorf(errFmtLoadLockFile, err)
	}
	// Reject v1.0 lock files — require upgrade to v2.0 for tamper detection.
	if v2Err := lock.RequireV2(); v2Err != nil {
		return nil, false, v2Err
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
			return nil, false, err
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

		// Each key is visited once and AddModule rewrites only that key, so
		// passing the live map never exposes an already-updated entry.
		resolved, err := m.resolveOne(ctx, req, lock.Modules)
		if err != nil {
			return nil, false, fmt.Errorf("failed to update %s: %w", key, err)
		}

		lock.AddModule(resolved)

		updated = append(updated, resolved)
	}

	// Save updated lock file
	lockWritten, saveErr := lock.SaveIfChanged(lockPath)
	if saveErr != nil {
		return nil, false, fmt.Errorf(errFmtSaveLockFile, saveErr)
	}

	return updated, lockWritten, nil
}

// Sync resolves all requirements from invowkmod.cue and updates the lock file.
//
// lockWritten reports whether the lock file was written; it is false when
// there are no requirements, since nothing is saved then.
func (m *Resolver) Sync(ctx context.Context, requirements []ModuleRef) (synced []*ResolvedModule, lockWritten bool, syncErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(requirements) == 0 {
		return nil, false, nil
	}

	// Load the existing lock entries for tamper detection. When re-syncing a
	// locked version, the fetched commit and module content are verified
	// against the prior lock entry, whether or not the module is cached locally.
	locked, err := m.loadLockedModules()
	if err != nil {
		return nil, false, err
	}

	// Resolve only direct dependencies (no transitive recursion).
	resolved, err := m.resolveAll(ctx, requirements, locked)
	if err != nil {
		return nil, false, err
	}

	// Validate that all transitive deps are explicitly declared in root requirements.
	// If any are missing, return an actionable error suggesting `invowk module tidy`.
	if diags := invowkmod.CheckMissingTransitiveDeps(requirements, resolved); len(diags) > 0 {
		return nil, false, &MissingTransitiveDepError{Diagnostics: diags}
	}

	lock := invowkmod.NewLockFile()

	for _, mod := range resolved {
		lock.AddModule(mod)
	}

	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lockWritten, saveErr := lock.SaveIfChanged(lockPath)
	if saveErr != nil {
		return nil, false, fmt.Errorf(errFmtSaveLockFile, saveErr)
	}

	return resolved, lockWritten, nil
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
		resolved, err := m.resolvedModuleFromLockEntry(key, entry)
		if err != nil {
			return nil, err
		}
		modules = append(modules, resolved)
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
		resolved, err := m.resolvedModuleFromLockEntry(key, entry)
		if err != nil {
			return nil, err
		}
		modules = append(modules, resolved)
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
		resolved, err := m.resolvedModuleFromLockEntry(key, entry)
		if err != nil {
			return nil, err
		}
		modules = append(modules, resolved)
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

func (m *Resolver) resolvedModuleFromLockEntry(_ ModuleRefKey, entry LockedModule) (*ResolvedModule, error) {
	if entry.ModuleID == "" {
		return nil, errors.New("lock entry missing module_id; run 'invowk module sync' to refresh canonical module metadata")
	}
	cachePath, err := m.getCachePath(string(entry.GitURL), string(entry.ResolvedVersion), string(entry.Path), entry.ModuleID)
	if err != nil {
		return nil, err
	}
	return &ResolvedModule{
		ModuleRef: ModuleRef{
			GitURL:  entry.GitURL,
			Version: entry.Version,
			Alias:   entry.Alias,
			Path:    entry.Path,
		},
		ResolvedVersion: entry.ResolvedVersion,
		GitCommit:       entry.GitCommit,
		CachePath:       types.FilesystemPath(cachePath),
		Namespace:       entry.Namespace,
		CommandSourceID: entry.EffectiveCommandSourceID(),
		ModuleID:        entry.ModuleID,
		ModuleName:      ModuleShortName(entry.ModuleID), //goplint:ignore -- legacy short-name field mirrors validated module ID for compatibility.
		ContentHash:     entry.ContentHash,
	}, nil
}

// loadLockedModules loads the existing lock file's entries for tamper
// detection. A missing lock file means no prior entries exist; an unreadable or
// invalid lock file is an integrity failure and must not silently downgrade
// verification.
func (m *Resolver) loadLockedModules() (map[ModuleRefKey]LockedModule, error) {
	lockPath := filepath.Join(string(m.workingDir), LockFileName)
	lockSnapshot := invowkmod.InspectLockFile(types.FilesystemPath(lockPath))
	if lockSnapshot.StatErr != nil || lockSnapshot.ParseErr != nil {
		return nil, fmt.Errorf(errFmtLoadLockFile, errors.Join(lockSnapshot.StatErr, lockSnapshot.ParseErr))
	}
	if !lockSnapshot.Present {
		return map[ModuleRefKey]LockedModule{}, nil
	}
	return lockSnapshot.LockFile.Modules, nil
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
