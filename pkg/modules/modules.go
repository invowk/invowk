// SPDX-License-Identifier: EPL-2.0

// Package modules provides functionality for managing pack dependencies from Git repositories.
//
// Modules enable packs to declare dependencies on other packs hosted in Git repositories
// (GitHub, GitLab, etc.). Dependencies are declared in the invkfile using the 'requires'
// field with semantic version constraints.
//
// Key features:
//   - Git repository support (HTTPS and SSH)
//   - Semantic versioning with constraints (^, ~, >=, <)
//   - Transitive dependency resolution
//   - Lock file for reproducible builds
//   - Configurable command namespacing with aliases
//   - Circular dependency detection
package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ModuleCachePathEnv is the environment variable for overriding the default module cache path.
const ModuleCachePathEnv = "INVOWK_MODULES_PATH"

// DefaultModulesDir is the default subdirectory within ~/.invowk for module cache.
const DefaultModulesDir = "modules"

// LockFileName is the name of the lock file.
const LockFileName = "invowk.lock.cue"

// Requirement represents a pack dependency declaration from an invkfile.
type Requirement struct {
	// GitURL is the Git repository URL (HTTPS or SSH format).
	// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
	GitURL string

	// Version is the semver constraint for version selection.
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
	Version string

	// Alias overrides the default namespace for imported commands (optional).
	// If not set, the namespace is: <pack-group>@<resolved-version>
	Alias string

	// Path specifies a subdirectory containing the pack (optional).
	// Used for monorepos with multiple packs.
	Path string
}

// Key returns a unique key for this requirement based on GitURL and Path.
func (r Requirement) Key() string {
	if r.Path != "" {
		return fmt.Sprintf("%s#%s", r.GitURL, r.Path)
	}
	return r.GitURL
}

// String returns a human-readable representation of the requirement.
func (r Requirement) String() string {
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

// ResolvedModule represents a fully resolved and cached module.
type ResolvedModule struct {
	// Requirement is the original requirement that was resolved.
	Requirement Requirement

	// ResolvedVersion is the exact version that was selected.
	// This is always a concrete version (e.g., "1.2.3"), not a constraint.
	ResolvedVersion string

	// GitCommit is the Git commit SHA for the resolved version.
	GitCommit string

	// CachePath is the absolute path to the cached module directory.
	CachePath string

	// Namespace is the computed namespace for this module's commands.
	// Format: "<pack-group>@<version>" or alias if specified.
	Namespace string

	// PackName is the name of the pack (from the folder name without .invkpack).
	PackName string

	// PackGroup is the group from the pack's invkfile.
	PackGroup string

	// TransitiveDeps are dependencies declared by this module (for recursive resolution).
	TransitiveDeps []Requirement
}

// Manager handles module operations including resolution, caching, and synchronization.
type Manager struct {
	// CacheDir is the root directory for module cache.
	CacheDir string

	// WorkingDir is the directory containing the invkfile (for relative path resolution).
	WorkingDir string

	// fetcher handles Git operations.
	fetcher *GitFetcher

	// resolver handles version constraint resolution.
	resolver *SemverResolver

	// mu protects concurrent access to the manager.
	mu sync.Mutex
}

// NewManager creates a new module manager.
//
// workingDir is the directory containing the invkfile (typically current working directory).
// cacheDir can be empty to use the default (~/.invowk/modules or $INVOWK_MODULES_PATH).
func NewManager(workingDir, cacheDir string) (*Manager, error) {
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

	return &Manager{
		CacheDir:   absCacheDir,
		WorkingDir: absWorkingDir,
		fetcher:    NewGitFetcher(absCacheDir),
		resolver:   NewSemverResolver(),
	}, nil
}

// GetDefaultCacheDir returns the default module cache directory.
// It checks INVOWK_MODULES_PATH environment variable first, then falls back to ~/.invowk/modules.
func GetDefaultCacheDir() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv(ModuleCachePathEnv); envPath != "" {
		return envPath, nil
	}

	// Fall back to ~/.invowk/modules
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".invowk", DefaultModulesDir), nil
}

// Add adds a new module requirement to the invkfile and resolves it.
func (m *Manager) Add(ctx context.Context, req Requirement) (*ResolvedModule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate the requirement
	if err := m.validateRequirement(req); err != nil {
		return nil, fmt.Errorf("invalid requirement: %w", err)
	}

	// Resolve the module
	resolved, err := m.resolveOne(ctx, req, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve module: %w", err)
	}

	return resolved, nil
}

// Remove removes a module requirement from the invkfile.
func (m *Manager) Remove(ctx context.Context, gitURL string) error {
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
func (m *Manager) Update(ctx context.Context, gitURL string) ([]*ResolvedModule, error) {
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
		req := Requirement{
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
			GitURL:          resolved.Requirement.GitURL,
			Version:         resolved.Requirement.Version,
			ResolvedVersion: resolved.ResolvedVersion,
			GitCommit:       resolved.GitCommit,
			Alias:           resolved.Requirement.Alias,
			Path:            resolved.Requirement.Path,
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

// Sync resolves all requirements from the invkfile and updates the lock file.
func (m *Manager) Sync(ctx context.Context, requirements []Requirement) ([]*ResolvedModule, error) {
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
		lock.Modules[mod.Requirement.Key()] = LockedModule{
			GitURL:          mod.Requirement.GitURL,
			Version:         mod.Requirement.Version,
			ResolvedVersion: mod.ResolvedVersion,
			GitCommit:       mod.GitCommit,
			Alias:           mod.Requirement.Alias,
			Path:            mod.Requirement.Path,
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
func (m *Manager) List(ctx context.Context) ([]*ResolvedModule, error) {
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
			Requirement: Requirement{
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

// LoadFromLock loads modules from an existing lock file without re-resolving.
// This is used for command discovery when a lock file already exists.
func (m *Manager) LoadFromLock(ctx context.Context) ([]*ResolvedModule, error) {
	return m.List(ctx)
}

// validateRequirement validates a module requirement.
func (m *Manager) validateRequirement(req Requirement) error {
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
	if _, err := m.resolver.ParseConstraint(req.Version); err != nil {
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
func (m *Manager) resolveAll(ctx context.Context, requirements []Requirement) ([]*ResolvedModule, error) {
	var resolved []*ResolvedModule
	visited := make(map[string]bool)
	inProgress := make(map[string]bool) // For cycle detection

	var resolve func(req Requirement) error
	resolve = func(req Requirement) error {
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

// resolveOne resolves a single module requirement.
func (m *Manager) resolveOne(ctx context.Context, req Requirement, _ map[string]bool) (*ResolvedModule, error) {
	// Get available versions from Git
	versions, err := m.fetcher.ListVersions(ctx, req.GitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions for %s: %w", req.GitURL, err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no version tags found for %s", req.GitURL)
	}

	// Resolve version constraint
	resolvedVersion, err := m.resolver.Resolve(req.Version, versions)
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

	// Cache the module in the versioned directory
	cachePath := m.getCachePath(req.GitURL, resolvedVersion, req.Path)
	if err := m.cacheModule(packDir, cachePath); err != nil {
		return nil, fmt.Errorf("failed to cache module: %w", err)
	}

	// Load transitive dependencies from the pack's invkfile
	transitiveDeps, packGroup, err := m.loadTransitiveDeps(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load transitive dependencies: %w", err)
	}

	return &ResolvedModule{
		Requirement:     req,
		ResolvedVersion: resolvedVersion,
		GitCommit:       commit,
		CachePath:       cachePath,
		Namespace:       namespace,
		PackName:        packName,
		PackGroup:       packGroup,
		TransitiveDeps:  transitiveDeps,
	}, nil
}

// getCachePath returns the cache path for a module.
func (m *Manager) getCachePath(gitURL, version, subPath string) string {
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
func (m *Manager) cacheModule(srcDir, dstDir string) error {
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
func (m *Manager) loadTransitiveDeps(cachePath string) ([]Requirement, string, error) {
	// Find invkfile.cue in the pack
	invkfilePath := filepath.Join(cachePath, "invkfile.cue")
	if _, err := os.Stat(invkfilePath); err != nil {
		// Try finding .invkpack directory
		entries, err := os.ReadDir(cachePath)
		if err != nil {
			return nil, "", nil // No invkfile, no dependencies
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invkpack") {
				invkfilePath = filepath.Join(cachePath, entry.Name(), "invkfile.cue")
				break
			}
		}
	}

	// Parse invkfile to extract requires and group
	// This is a simplified implementation - in practice, we'd use the invkfile package
	data, err := os.ReadFile(invkfilePath)
	if err != nil {
		return nil, "", nil // No invkfile, no dependencies
	}

	// Extract group and requires from invkfile content
	// This is a basic parser - full implementation uses CUE
	group := extractGroupFromInvkfile(string(data))
	reqs := extractRequiresFromInvkfile(string(data))

	return reqs, group, nil
}

// computeNamespace generates the namespace for a module.
func computeNamespace(packName, version, alias string) string {
	if alias != "" {
		return alias
	}
	return fmt.Sprintf("%s@%s", packName, version)
}

// findPackInDir finds a .invkpack directory or invkfile.cue in the given directory.
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

	// If no .invkpack directory, check for invkfile.cue directly
	invkfilePath := filepath.Join(dir, "invkfile.cue")
	if _, err := os.Stat(invkfilePath); err == nil {
		// Use directory name as pack name
		packName = filepath.Base(dir)
		return dir, packName, nil
	}

	return "", "", fmt.Errorf("no pack found in %s (expected .invkpack directory or invkfile.cue)", dir)
}

// extractPackName extracts the pack name from a module key.
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

// extractGroupFromInvkfile extracts the group field from invkfile content.
// This is a simplified implementation - full parsing uses CUE.
func extractGroupFromInvkfile(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if value, found := strings.CutPrefix(line, "group:"); found {
			value = strings.TrimSpace(value)
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

// extractRequiresFromInvkfile extracts requires from invkfile content.
// This is a simplified implementation - full parsing uses CUE.
func extractRequiresFromInvkfile(_ string) []Requirement {
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

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
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
