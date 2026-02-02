// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
