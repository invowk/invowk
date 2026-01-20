// SPDX-License-Identifier: EPL-2.0

package invkmod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// ModuleCachePathEnv is the environment variable for overriding the default module cache path.
	ModuleCachePathEnv = "INVOWK_MODULES_PATH"

	// DefaultModulesDir is the default subdirectory within ~/.invowk for module cache.
	DefaultModulesDir = "modules"

	// LockFileName is the name of the lock file.
	// The lock file pairs naturally with invkmod.cue (like go.sum pairs with go.mod).
	LockFileName = "invkmod.lock.cue"
)

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

// validateModuleRef validates a module requirement.
func (m *Resolver) validateModuleRef(req ModuleRef) error {
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
func (m *Resolver) resolveAll(ctx context.Context, requirements []ModuleRef) ([]*ResolvedModule, error) {
	var resolved []*ResolvedModule
	visited := make(map[string]bool)
	inProgress := make(map[string]bool) // For cycle detection

	var resolve func(req ModuleRef) error
	resolve = func(req ModuleRef) error {
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
func (m *Resolver) resolveOne(ctx context.Context, req ModuleRef, _ map[string]bool) (*ResolvedModule, error) {
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

	// Determine module path within the repository
	modulePath := repoPath
	if req.Path != "" {
		modulePath = filepath.Join(repoPath, req.Path)
	}

	// Find .invkmod directory
	moduleDir, moduleName, err := findModuleInDir(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find module in %s: %w", modulePath, err)
	}

	// Compute namespace
	namespace := computeNamespace(moduleName, resolvedVersion, req.Alias)

	// Cache the module in the versioned directory
	cachePath := m.getCachePath(req.GitURL, resolvedVersion, req.Path)
	if err = m.cacheModule(moduleDir, cachePath); err != nil {
		return nil, fmt.Errorf("failed to cache module: %w", err)
	}

	// Load transitive dependencies from the module's invkmod.cue
	transitiveDeps, moduleID, err := m.loadTransitiveDeps(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load transitive dependencies: %w", err)
	}

	return &ResolvedModule{
		ModuleRef:       req,
		ResolvedVersion: resolvedVersion,
		GitCommit:       commit,
		CachePath:       cachePath,
		Namespace:       namespace,
		ModuleName:      moduleName,
		ModuleID:        moduleID,
		TransitiveDeps:  transitiveDeps,
	}, nil
}

// getCachePath returns the cache path for a module.
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

// cacheModule copies a module to the cache directory.
func (m *Resolver) cacheModule(srcDir, dstDir string) error {
	// Check if already cached
	if _, err := os.Stat(dstDir); err == nil {
		return nil // Already cached
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Copy the module directory
	return copyDir(srcDir, dstDir)
}

// loadTransitiveDeps loads transitive dependencies from a cached module.
// Dependencies are declared in invkmod.cue (not invkfile.cue).
func (m *Resolver) loadTransitiveDeps(cachePath string) ([]ModuleRef, string, error) {
	// Find invkmod.cue in the module (contains module metadata and requires)
	invkmodPath := filepath.Join(cachePath, "invkmod.cue")
	if _, err := os.Stat(invkmodPath); err != nil {
		// Try finding .invkmod directory
		entries, err := os.ReadDir(cachePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, "", nil // Directory doesn't exist - no dependencies
			}
			return nil, "", fmt.Errorf("reading cache directory %s: %w", cachePath, err)
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invkmod") {
				invkmodPath = filepath.Join(cachePath, entry.Name(), "invkmod.cue")
				break
			}
		}
	}

	// Parse invkmod to extract module name and requires
	// This is a simplified implementation - in practice, we'd use the invkfile package
	data, err := os.ReadFile(invkmodPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil // No invkmod.cue - no dependencies
		}
		return nil, "", fmt.Errorf("reading invkmod %s: %w", invkmodPath, err)
	}

	// Extract module and requires from invkmod content
	// This is a basic parser - full implementation uses CUE
	moduleName := extractModuleFromInvkmod(string(data))
	reqs := extractRequiresFromInvkmod(string(data))

	return reqs, moduleName, nil
}

// computeNamespace generates the namespace for a module.
func computeNamespace(moduleName, version, alias string) string {
	if alias != "" {
		return alias
	}
	return fmt.Sprintf("%s@%s", moduleName, version)
}

// findModuleInDir finds a .invkmod directory or invkmod.cue in the given directory.
// A Git repo is considered a module if:
//   - Repo name ends with .invkmod suffix, OR
//   - Contains an invkmod.cue file at the root
func findModuleInDir(dir string) (moduleDir, moduleName string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read directory: %w", err)
	}

	// First, look for .invkmod directories
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invkmod") {
			moduleName = strings.TrimSuffix(entry.Name(), ".invkmod")
			return filepath.Join(dir, entry.Name()), moduleName, nil
		}
	}

	// Check if this directory IS a module (has invkmod.cue at root)
	// This supports Git repos with .invkmod suffix in their name
	invkmodPath := filepath.Join(dir, "invkmod.cue")
	if _, err := os.Stat(invkmodPath); err == nil {
		// Extract module name from directory (for .invkmod repos)
		dirName := filepath.Base(dir)
		if name, found := strings.CutSuffix(dirName, ".invkmod"); found {
			moduleName = name
		} else {
			// Fall back to parsing invkmod.cue to get the module name
			moduleName = dirName
		}
		return dir, moduleName, nil
	}

	return "", "", fmt.Errorf("no module found in %s (expected .invkmod directory or invkmod.cue)", dir)
}

// extractModuleName extracts the module name from a module key.
func extractModuleName(key string) string {
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

// extractModuleFromInvkmod extracts the module field from invkmod content.
// This is a simplified implementation - full parsing uses CUE.
func extractModuleFromInvkmod(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if value, found := strings.CutPrefix(line, "module:"); found {
			value = strings.TrimSpace(value)
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

// extractRequiresFromInvkmod extracts requires from invkmod content.
// This is a simplified implementation - full parsing uses CUE.
func extractRequiresFromInvkmod(_ string) []ModuleRef {
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

	if mkdirErr := os.MkdirAll(dst, srcInfo.Mode()); mkdirErr != nil {
		return mkdirErr
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
