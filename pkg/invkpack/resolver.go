// SPDX-License-Identifier: EPL-2.0

package invkpack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PackCachePathEnv is the environment variable for overriding the default pack cache path.
const PackCachePathEnv = "INVOWK_PACKS_PATH"

// DefaultPacksDir is the default subdirectory within ~/.invowk for pack cache.
const DefaultPacksDir = "packs"

// LockFileName is the name of the lock file.
// The lock file pairs naturally with invkpack.cue (like go.sum pairs with go.mod).
const LockFileName = "invkpack.lock.cue"

// PackRef represents a pack dependency declaration from invkpack.cue.
type PackRef struct {
	// GitURL is the Git repository URL (HTTPS or SSH format).
	// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
	GitURL string

	// Version is the semver constraint for version selection.
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
	Version string

	// Alias overrides the default namespace for imported commands (optional).
	// If not set, the namespace is: <pack>@<resolved-version>
	Alias string

	// Path specifies a subdirectory containing the pack (optional).
	// Used for monorepos with multiple packs.
	Path string
}

// Key returns a unique key for this requirement based on GitURL and Path.
func (r PackRef) Key() string {
	if r.Path != "" {
		return fmt.Sprintf("%s#%s", r.GitURL, r.Path)
	}
	return r.GitURL
}

// String returns a human-readable representation of the requirement.
func (r PackRef) String() string {
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

// ResolvedPack represents a fully resolved and cached pack.
type ResolvedPack struct {
	// PackRef is the original requirement that was resolved.
	PackRef PackRef

	// ResolvedVersion is the exact version that was selected.
	// This is always a concrete version (e.g., "1.2.3"), not a constraint.
	ResolvedVersion string

	// GitCommit is the Git commit SHA for the resolved version.
	GitCommit string

	// CachePath is the absolute path to the cached pack directory.
	CachePath string

	// Namespace is the computed namespace for this pack's commands.
	// Format: "<pack>@<version>" or alias if specified.
	Namespace string

	// PackName is the name of the pack (from the folder name without .invkpack).
	PackName string

	// PackID is the pack identifier from the pack's invkpack.cue.
	PackID string

	// TransitiveDeps are dependencies declared by this pack (for recursive resolution).
	TransitiveDeps []PackRef
}

// Resolver handles pack operations including resolution, caching, and synchronization.
type Resolver struct {
	// CacheDir is the root directory for pack cache.
	CacheDir string

	// WorkingDir is the directory containing invkpack.cue (for relative path resolution).
	WorkingDir string

	// fetcher handles Git operations.
	fetcher *GitFetcher

	// semver handles version constraint resolution.
	semver *SemverResolver

	// mu protects concurrent access to the resolver.
	mu sync.Mutex
}

// NewResolver creates a new pack resolver.
//
// workingDir is the directory containing invkpack.cue (typically current working directory).
// cacheDir can be empty to use the default (~/.invowk/packs or $INVOWK_PACKS_PATH).
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
	if err := os.MkdirAll(absCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Resolver{
		CacheDir:   absCacheDir,
		WorkingDir: absWorkingDir,
		fetcher:    NewGitFetcher(absCacheDir),
		semver:     NewSemverResolver(),
	}, nil
}

// GetDefaultCacheDir returns the default pack cache directory.
// It checks INVOWK_PACKS_PATH environment variable first, then falls back to ~/.invowk/packs.
func GetDefaultCacheDir() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv(PackCachePathEnv); envPath != "" {
		return envPath, nil
	}

	// Fall back to ~/.invowk/packs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".invowk", DefaultPacksDir), nil
}

// Add resolves a new pack requirement and returns the resolved metadata.
func (m *Resolver) Add(ctx context.Context, req PackRef) (*ResolvedPack, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate the requirement
	if err := m.validatePackRef(req); err != nil {
		return nil, fmt.Errorf("invalid requirement: %w", err)
	}

	// Resolve the pack
	resolved, err := m.resolveOne(ctx, req, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve pack: %w", err)
	}

	return resolved, nil
}

// Remove removes a pack requirement from the lock file.
func (m *Resolver) Remove(ctx context.Context, gitURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(m.WorkingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("failed to load lock file: %w", err)
	}

	// Find and remove the pack
	found := false
	for key := range lock.Packs {
		if strings.HasPrefix(key, gitURL) {
			delete(lock.Packs, key)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("pack not found: %s", gitURL)
	}

	// Save updated lock file
	if err := lock.Save(lockPath); err != nil {
		return fmt.Errorf("failed to save lock file: %w", err)
	}

	return nil
}

// Update updates one or all packs to their latest matching versions.
// If gitURL is empty, all packs are updated.
func (m *Resolver) Update(ctx context.Context, gitURL string) ([]*ResolvedPack, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current lock file
	lockPath := filepath.Join(m.WorkingDir, LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock file: %w", err)
	}

	var updated []*ResolvedPack
	visited := make(map[string]bool)

	for key, entry := range lock.Packs {
		if gitURL != "" && !strings.HasPrefix(key, gitURL) {
			continue
		}

		// Re-resolve with force flag to bypass cache
		req := PackRef{
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
		lock.Packs[key] = LockedPack{
			GitURL:          resolved.PackRef.GitURL,
			Version:         resolved.PackRef.Version,
			ResolvedVersion: resolved.ResolvedVersion,
			GitCommit:       resolved.GitCommit,
			Alias:           resolved.PackRef.Alias,
			Path:            resolved.PackRef.Path,
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

// Sync resolves all requirements from invkpack.cue and updates the lock file.
func (m *Resolver) Sync(ctx context.Context, requirements []PackRef) ([]*ResolvedPack, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(requirements) == 0 {
		return nil, nil
	}

	// Build dependency graph and resolve all packs
	resolved, err := m.resolveAll(ctx, requirements)
	if err != nil {
		return nil, err
	}

	// Save lock file
	lock := &LockFile{
		Version: "1.0",
		Packs:   make(map[string]LockedPack),
	}

	for _, mod := range resolved {
		lock.Packs[mod.PackRef.Key()] = LockedPack{
			GitURL:          mod.PackRef.GitURL,
			Version:         mod.PackRef.Version,
			ResolvedVersion: mod.ResolvedVersion,
			GitCommit:       mod.GitCommit,
			Alias:           mod.PackRef.Alias,
			Path:            mod.PackRef.Path,
			Namespace:       mod.Namespace,
		}
	}

	lockPath := filepath.Join(m.WorkingDir, LockFileName)
	if err := lock.Save(lockPath); err != nil {
		return nil, fmt.Errorf("failed to save lock file: %w", err)
	}

	return resolved, nil
}

// List returns all currently resolved packs from the lock file.
func (m *Resolver) List(ctx context.Context) ([]*ResolvedPack, error) {
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

	var modules []*ResolvedPack
	for key, entry := range lock.Packs {
		modules = append(modules, &ResolvedPack{
			PackRef: PackRef{
				GitURL:  entry.GitURL,
				Version: entry.Version,
				Alias:   entry.Alias,
				Path:    entry.Path,
			},
			ResolvedVersion: entry.ResolvedVersion,
			GitCommit:       entry.GitCommit,
			CachePath:       m.getCachePath(entry.GitURL, entry.ResolvedVersion, entry.Path),
			Namespace:       entry.Namespace,
			PackName:        extractPackName(key),
		})
	}

	return modules, nil
}

// LoadFromLock loads packs from an existing lock file without re-resolving.
// This is used for command discovery when a lock file already exists.
func (m *Resolver) LoadFromLock(ctx context.Context) ([]*ResolvedPack, error) {
	return m.List(ctx)
}

// validatePackRef validates a pack requirement.
func (m *Resolver) validatePackRef(req PackRef) error {
	if req.GitURL == "" {
		return fmt.Errorf("git_url is required")
	}

	if !strings.HasPrefix(req.GitURL, "https://") && !strings.HasPrefix(req.GitURL, "git@") {
		return fmt.Errorf("git_url must start with https:// or git@")
	}

	if req.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate version constraint format
	if _, err := m.semver.ParseConstraint(req.Version); err != nil {
		return fmt.Errorf("invalid version constraint: %w", err)
	}

	// Validate path to prevent directory traversal attacks
	if req.Path != "" {
		cleanPath := filepath.Clean(req.Path)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			return fmt.Errorf("invalid path: path traversal or absolute paths not allowed")
		}
	}

	return nil
}

// resolveAll resolves all requirements including transitive dependencies.
func (m *Resolver) resolveAll(ctx context.Context, requirements []PackRef) ([]*ResolvedPack, error) {
	var resolved []*ResolvedPack
	visited := make(map[string]bool)
	inProgress := make(map[string]bool) // For cycle detection

	var resolve func(req PackRef) error
	resolve = func(req PackRef) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		key := req.Key()

		// Check for circular dependency
		if inProgress[key] {
			return fmt.Errorf("circular dependency detected: %s", key)
		}

		// Skip if already resolved
		if visited[key] {
			return nil
		}

		inProgress[key] = true
		defer func() { delete(inProgress, key) }()

		// Resolve this module
		mod, err := m.resolveOne(ctx, req, visited)
		if err != nil {
			return err
		}

		visited[key] = true
		resolved = append(resolved, mod)

		// Resolve transitive dependencies
		for _, dep := range mod.TransitiveDeps {
			if err := resolve(dep); err != nil {
				return fmt.Errorf("transitive dependency %s: %w", dep.Key(), err)
			}
		}

		return nil
	}

	for _, req := range requirements {
		if err := resolve(req); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

// resolveOne resolves a single pack requirement.
func (m *Resolver) resolveOne(ctx context.Context, req PackRef, _ map[string]bool) (*ResolvedPack, error) {
	// Get available versions from Git
	versions, err := m.fetcher.ListVersions(ctx, req.GitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions for %s: %w", req.GitURL, err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no version tags found for %s", req.GitURL)
	}

	// Resolve version constraint
	resolvedVersion, err := m.semver.Resolve(req.Version, versions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve version for %s: %w", req.GitURL, err)
	}

	// Clone/fetch the repository at the resolved version
	repoPath, commit, err := m.fetcher.Fetch(ctx, req.GitURL, resolvedVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s@%s: %w", req.GitURL, resolvedVersion, err)
	}

	// Determine pack path within the repository
	packPath := repoPath
	if req.Path != "" {
		packPath = filepath.Join(repoPath, req.Path)
	}

	// Find .invkpack directory
	packDir, packName, err := findPackInDir(packPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find pack in %s: %w", packPath, err)
	}

	// Compute namespace
	namespace := computeNamespace(packName, resolvedVersion, req.Alias)

	// Cache the pack in the versioned directory
	cachePath := m.getCachePath(req.GitURL, resolvedVersion, req.Path)
	if err = m.cacheModule(packDir, cachePath); err != nil {
		return nil, fmt.Errorf("failed to cache pack: %w", err)
	}

	// Load transitive dependencies from the pack's invkpack.cue
	transitiveDeps, packGroup, err := m.loadTransitiveDeps(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load transitive dependencies: %w", err)
	}

	return &ResolvedPack{
		PackRef:         req,
		ResolvedVersion: resolvedVersion,
		GitCommit:       commit,
		CachePath:       cachePath,
		Namespace:       namespace,
		PackName:        packName,
		PackID:          packGroup,
		TransitiveDeps:  transitiveDeps,
	}, nil
}

// getCachePath returns the cache path for a pack.
func (m *Resolver) getCachePath(gitURL, version, subPath string) string {
	// Convert git URL to path-safe format
	// e.g., "https://github.com/user/repo.git" -> "github.com/user/repo"
	urlPath := strings.TrimPrefix(gitURL, "https://")
	urlPath = strings.TrimPrefix(urlPath, "git@")
	urlPath = strings.TrimSuffix(urlPath, ".git")
	urlPath = strings.ReplaceAll(urlPath, ":", "/")

	parts := []string{m.CacheDir, urlPath, version}
	if subPath != "" {
		parts = append(parts, subPath)
	}

	return filepath.Join(parts...)
}

// cacheModule copies a pack to the cache directory.
func (m *Resolver) cacheModule(srcDir, dstDir string) error {
	// Check if already cached
	if _, err := os.Stat(dstDir); err == nil {
		return nil // Already cached
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dstDir), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Copy the pack directory
	return copyDir(srcDir, dstDir)
}

// loadTransitiveDeps loads transitive dependencies from a cached pack.
// Dependencies are declared in invkpack.cue (not invkfile.cue).
func (m *Resolver) loadTransitiveDeps(cachePath string) ([]PackRef, string, error) {
	// Find invkpack.cue in the pack (contains pack metadata and requires)
	invkpackPath := filepath.Join(cachePath, "invkpack.cue")
	if _, err := os.Stat(invkpackPath); err != nil {
		// Try finding .invkpack directory
		entries, err := os.ReadDir(cachePath)
		if err != nil {
			return nil, "", nil // No invkpack, no dependencies
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invkpack") {
				invkpackPath = filepath.Join(cachePath, entry.Name(), "invkpack.cue")
				break
			}
		}
	}

	// Parse invkpack to extract pack name and requires
	// This is a simplified implementation - in practice, we'd use the invkfile package
	data, err := os.ReadFile(invkpackPath)
	if err != nil {
		return nil, "", nil // No invkpack, no dependencies
	}

	// Extract pack and requires from invkpack content
	// This is a basic parser - full implementation uses CUE
	packName := extractPackFromInvkfile(string(data)) // Same format as before
	reqs := extractRequiresFromInvkfile(string(data))

	return reqs, packName, nil
}

// computeNamespace generates the namespace for a pack.
func computeNamespace(packName, version, alias string) string {
	if alias != "" {
		return alias
	}
	return fmt.Sprintf("%s@%s", packName, version)
}

// findPackInDir finds a .invkpack directory or invkpack.cue in the given directory.
// A Git repo is considered a pack if:
//   - Repo name ends with .invkpack suffix, OR
//   - Contains an invkpack.cue file at the root
func findPackInDir(dir string) (packDir, packName string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read directory: %w", err)
	}

	// First, look for .invkpack directories
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invkpack") {
			packName = strings.TrimSuffix(entry.Name(), ".invkpack")
			return filepath.Join(dir, entry.Name()), packName, nil
		}
	}

	// Check if this directory IS a pack (has invkpack.cue at root)
	// This supports Git repos with .invkpack suffix in their name
	invkpackPath := filepath.Join(dir, "invkpack.cue")
	if _, err := os.Stat(invkpackPath); err == nil {
		// Extract pack name from directory (for .invkpack repos)
		dirName := filepath.Base(dir)
		if strings.HasSuffix(dirName, ".invkpack") {
			packName = strings.TrimSuffix(dirName, ".invkpack")
		} else {
			// Fall back to parsing invkpack.cue to get the pack name
			packName = dirName
		}
		return dir, packName, nil
	}

	return "", "", fmt.Errorf("no pack found in %s (expected .invkpack directory or invkpack.cue)", dir)
}

// extractPackName extracts the pack name from a pack key.
func extractPackName(key string) string {
	// key format: "github.com/user/repo" or "github.com/user/repo#subpath"
	parts := strings.Split(key, "#")
	url := parts[0]

	// Extract repo name
	urlParts := strings.Split(url, "/")
	if len(urlParts) > 0 {
		name := urlParts[len(urlParts)-1]
		name = strings.TrimSuffix(name, ".git")
		return name
	}
	return key
}

// extractPackFromInvkfile extracts the pack field from invkpack content.
// This is a simplified implementation - full parsing uses CUE.
func extractPackFromInvkfile(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if value, found := strings.CutPrefix(line, "pack:"); found {
			value = strings.TrimSpace(value)
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

// extractRequiresFromInvkfile extracts requires from invkpack content.
// This is a simplified implementation - full parsing uses CUE.
func extractRequiresFromInvkfile(_ string) []PackRef {
	// Simplified: return empty for now
	// Full implementation would parse CUE and extract requires field
	return nil
}

// copyDir recursively copies a directory, skipping symlinks for security.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip symlinks to prevent directory traversal attacks
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, srcInfo.Mode())
}
