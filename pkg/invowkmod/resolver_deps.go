// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isSupportedGitURLPrefix returns true when the URL uses a supported Git scheme.
func isSupportedGitURLPrefix(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

// validateModuleRef validates a module requirement.
func (m *Resolver) validateModuleRef(req ModuleRef) error {
	if req.GitURL == "" {
		return fmt.Errorf("git_url is required")
	}

	if !isSupportedGitURLPrefix(req.GitURL) {
		return fmt.Errorf("git_url must start with https://, git@, or ssh://")
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
//
// It uses a dual-map pattern for traversal control:
//   - visited: marks modules that have been fully resolved, preventing reprocessing.
//   - inProgress: marks modules currently on the resolution call stack, detecting
//     cycles within the current dependency path. An entry is added when resolution
//     begins and removed (via defer) when it completes, so only ancestors in the
//     current chain are flagged.
func (m *Resolver) resolveAll(ctx context.Context, requirements []ModuleRef) ([]*ResolvedModule, error) {
	var resolved []*ResolvedModule
	visited := make(map[string]bool)
	inProgress := make(map[string]bool) // cycle detection within current resolution path

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

	// Find .invowkmod directory
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

	// Load transitive dependencies from the module's invowkmod.cue
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

// loadTransitiveDeps loads transitive dependencies from a cached module.
// Dependencies are declared in invowkmod.cue (not invowkfile.cue).
func (m *Resolver) loadTransitiveDeps(cachePath string) ([]ModuleRef, string, error) {
	// Find invowkmod.cue in the module (contains module metadata and requires)
	invowkmodPath := filepath.Join(cachePath, "invowkmod.cue")
	if _, err := os.Stat(invowkmodPath); err != nil {
		// Try finding .invowkmod directory
		entries, err := os.ReadDir(cachePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, "", nil // Directory doesn't exist - no dependencies
			}
			return nil, "", fmt.Errorf("reading cache directory %s: %w", cachePath, err)
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invowkmod") {
				invowkmodPath = filepath.Join(cachePath, entry.Name(), "invowkmod.cue")
				break
			}
		}
	}

	// Parse invowkmod to extract module name and requires.
	data, err := os.ReadFile(invowkmodPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil // No invowkmod.cue - no dependencies
		}
		return nil, "", fmt.Errorf("reading invowkmod %s: %w", invowkmodPath, err)
	}

	meta, err := ParseInvowkmodBytes(data, invowkmodPath)
	if err != nil {
		return nil, "", fmt.Errorf("parsing invowkmod %s: %w", invowkmodPath, err)
	}

	reqs := extractRequiresFromInvowkmod(meta.Requires)

	return reqs, meta.Module, nil
}

// computeNamespace generates the namespace for a module.
func computeNamespace(moduleName, version, alias string) string {
	if alias != "" {
		return alias
	}
	return fmt.Sprintf("%s@%s", moduleName, version)
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

// extractModuleFromInvowkmod extracts the module field from invowkmod content
// using lightweight string matching. This avoids a full CUE evaluation dependency
// and is sufficient for the "module:" top-level field.
func extractModuleFromInvowkmod(content string) string {
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

// extractRequiresFromInvowkmod converts invowkmod requirements into resolver refs.
func extractRequiresFromInvowkmod(reqs []ModuleRequirement) []ModuleRef {
	refs := make([]ModuleRef, 0, len(reqs))
	for _, req := range reqs {
		refs = append(refs, ModuleRef(req))
	}
	return refs
}
