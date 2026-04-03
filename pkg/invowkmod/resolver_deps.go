// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrGitURLRequired is returned when a module ref is missing the git_url field.
	ErrGitURLRequired = errors.New("git_url is required")

	// ErrUnsupportedGitURLScheme is returned when a git_url uses an unsupported scheme.
	ErrUnsupportedGitURLScheme = errors.New("git_url must start with https://, git@, or ssh://")

	// ErrVersionRequired is returned when a module ref is missing the version field.
	ErrVersionRequired = errors.New("version is required")
)

// isSupportedGitURLPrefix returns true when the URL uses a supported Git scheme.
func isSupportedGitURLPrefix(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

// validateModuleRef validates a module requirement.
func (m *Resolver) validateModuleRef(req ModuleRef) error {
	if req.GitURL == "" {
		return ErrGitURLRequired
	}

	if !isSupportedGitURLPrefix(string(req.GitURL)) {
		return ErrUnsupportedGitURLScheme
	}

	if req.Version == "" {
		return ErrVersionRequired
	}

	// Validate version constraint format
	if _, err := m.semver.ParseConstraint(string(req.Version)); err != nil {
		return fmt.Errorf("invalid version constraint: %w", err)
	}

	// Validate path to prevent directory traversal attacks
	if req.Path != "" {
		if err := req.Path.Validate(); err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
	}

	return nil
}

// resolveAll resolves all explicitly declared requirements without transitive recursion.
//
// This implements the Go-style explicit-only dependency model: only modules listed
// in the root invowkmod.cue are resolved. Transitive dependencies declared by resolved
// modules are loaded (via loadTransitiveDeps) for validation purposes but are NOT
// recursively resolved. The caller (Sync) validates that all transitive deps are
// explicitly declared in the root requirements.
//
// The visited map deduplicates diamond dependencies — when two direct requirements
// point to the same module (by Key()), it is resolved only once.
//
// knownHashes provides content hashes from the existing lock file for cache tamper
// detection. When a module is already cached, its hash is verified against the
// known hash before reuse. Pass nil when no prior lock file exists.
func (m *Resolver) resolveAll(ctx context.Context, requirements []ModuleRef, knownHashes map[ModuleRefKey]ContentHash) ([]*ResolvedModule, error) {
	var resolved []*ResolvedModule
	visited := make(map[ModuleRefKey]bool)

	for _, req := range requirements {
		// Check for context cancellation between modules.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("resolving module dependencies: %w", ctx.Err())
		default:
		}

		key := req.Key()

		// Skip duplicate requirements (diamond deps in direct requirements).
		if visited[key] {
			continue
		}

		mod, err := m.resolveOne(ctx, req, knownHashes)
		if err != nil {
			return nil, err
		}

		visited[key] = true
		resolved = append(resolved, mod)
	}

	return resolved, nil
}

// resolveOne resolves a single module requirement.
//
// knownHashes provides content hashes from the existing lock file for cache
// tamper detection. When a cached module exists, its hash is verified against
// the known hash before reuse. This prevents an attacker with write access to
// the module cache from silently replacing module content. Pass nil when no
// prior lock file exists.
func (m *Resolver) resolveOne(ctx context.Context, req ModuleRef, knownHashes map[ModuleRefKey]ContentHash) (*ResolvedModule, error) {
	// Get available versions from Git
	versions, err := m.fetcher.ListVersions(ctx, req.GitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions for %s: %w", req.GitURL, err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no version tags found for %s", req.GitURL)
	}

	// Resolve version constraint
	resolvedVersion, err := m.semver.Resolve(string(req.Version), versions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve version for %s: %w", req.GitURL, err)
	}

	// Clone/fetch the repository at the resolved version
	repoPath, commit, err := m.fetcher.Fetch(ctx, req.GitURL, resolvedVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s@%s: %w", req.GitURL, resolvedVersion, err)
	}

	// Determine module path within the repository
	modulePath := string(repoPath)
	if req.Path != "" {
		modulePath = filepath.Join(string(repoPath), string(req.Path))
	}

	// Find .invowkmod directory
	moduleDir, moduleName, err := findModuleInDir(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find module in %s: %w", modulePath, err)
	}

	// Compute namespace
	namespace := computeNamespace(moduleName, string(resolvedVersion), req.Alias)

	// Look up known hash from the prior lock file for cache tamper detection.
	// If the module is already cached, cacheModule verifies the cached content
	// matches this hash before reuse.
	var expectedHash ContentHash
	if knownHashes != nil {
		expectedHash = knownHashes[req.Key()]
	}

	// Cache the module in the versioned directory and compute content hash.
	cachePath := m.getCachePath(string(req.GitURL), string(resolvedVersion), string(req.Path))
	contentHash, err := m.cacheModule(moduleDir, cachePath, expectedHash)
	if err != nil {
		return nil, fmt.Errorf("failed to cache module: %w", err)
	}

	// Load transitive dependencies from the module's invowkmod.cue.
	// These are NOT resolved recursively — they are used for validation only
	// (checkMissingTransitiveDeps verifies they are declared in the root invowkmod.cue).
	transitiveDeps, moduleID, err := m.loadTransitiveDeps(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load transitive dependencies: %w", err)
	}

	return &ResolvedModule{
		ModuleRef:       req,
		ResolvedVersion: resolvedVersion,
		GitCommit:       commit,
		CachePath:       types.FilesystemPath(cachePath),
		Namespace:       namespace,
		ModuleName:      moduleName,
		ModuleID:        moduleID,
		TransitiveDeps:  transitiveDeps,
		ContentHash:     contentHash,
	}, nil
}

// loadTransitiveDeps loads transitive dependencies from a cached module.
// Dependencies are declared in invowkmod.cue (not invowkfile.cue).
func (m *Resolver) loadTransitiveDeps(cachePath string) ([]ModuleRef, ModuleID, error) {
	// Find invowkmod.cue in the module (contains module metadata and requires)
	invowkmodPath := filepath.Join(cachePath, "invowkmod.cue")
	if _, statErr := os.Stat(invowkmodPath); statErr != nil {
		// Only fall through to directory scan for missing files. Permission errors
		// and other I/O failures are returned as hard errors to avoid silently
		// skipping modules that exist but are unreadable.
		if !os.IsNotExist(statErr) {
			return nil, "", fmt.Errorf("checking invowkmod path %s: %w", invowkmodPath, statErr)
		}
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

	meta, err := ParseInvowkmodBytes(data, types.FilesystemPath(invowkmodPath))
	if err != nil {
		return nil, "", fmt.Errorf("parsing invowkmod %s: %w", invowkmodPath, err)
	}

	reqs := extractRequiresFromInvowkmod(meta.Requires)

	return reqs, meta.Module, nil
}

// computeNamespace generates the namespace for a module.
func computeNamespace(moduleName ModuleShortName, version string, alias ModuleAlias) ModuleNamespace {
	if alias != "" {
		return ModuleNamespace(alias)
	}
	return ModuleNamespace(fmt.Sprintf("%s@%s", moduleName, version))
}

// extractModuleName extracts the module name from a module key.
func extractModuleName(key ModuleRefKey) ModuleShortName {
	// key format: "github.com/user/repo" or "github.com/user/repo#subpath"
	parts := strings.Split(string(key), "#")
	url := parts[0]

	// Extract repo name
	urlParts := strings.Split(url, "/")
	if len(urlParts) > 0 {
		name := urlParts[len(urlParts)-1]
		name = strings.TrimSuffix(name, ".git")
		return ModuleShortName(name) //goplint:ignore -- best-effort name extraction from URL
	}
	return ModuleShortName(key) //goplint:ignore -- fallback from require key
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
