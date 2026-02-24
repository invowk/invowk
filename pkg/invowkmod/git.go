// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/invowk/invowk/pkg/types"
)

type (
	// GitFetcher handles Git operations for module fetching.
	GitFetcher struct {
		// CacheDir is the base directory for the Git source cache.
		CacheDir types.FilesystemPath

		// auth is the authentication method to use for Git operations.
		auth transport.AuthMethod
	}

	// TagInfo contains information about a Git tag.
	TagInfo struct {
		// Name is the git tag name; intentionally untyped (pass-through from go-git).
		Name   string
		Commit GitCommit
	}
)

// NewGitFetcher creates a new Git fetcher.
func NewGitFetcher(cacheDir types.FilesystemPath) *GitFetcher {
	f := &GitFetcher{
		CacheDir: cacheDir,
	}
	f.setupAuth()
	return f
}

// ListVersions returns all version tags from a Git repository.
func (f *GitFetcher) ListVersions(ctx context.Context, gitURL GitURL) ([]SemVer, error) {
	// Use in-memory storage to list remote refs without cloning
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{string(gitURL)},
	})

	// List all references from the remote
	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	// Filter for version tags
	var versions []SemVer
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()
			// Accept both "v1.2.3" and "1.2.3" formats
			if isValidVersionString(tagName) {
				versions = append(versions, SemVer(tagName))
			}
		}
	}

	// Sort versions (newest first)
	versions = SortVersions(versions)

	return versions, nil
}

// Fetch clones or fetches a Git repository and checks out the specified version.
// Returns the path to the repository and the commit SHA.
func (f *GitFetcher) Fetch(ctx context.Context, gitURL GitURL, version SemVer) (repoPath types.FilesystemPath, commitSHA GitCommit, err error) {
	// Generate a cache path for this repository
	cachePath := f.getRepoCachePath(gitURL)

	// Check if we already have this repository
	repo, err := git.PlainOpen(string(cachePath))
	if err != nil {
		// Repository doesn't exist, clone it
		repo, err = f.clone(ctx, gitURL, cachePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		// Repository exists, fetch updates
		// Fetch might fail for various reasons (network, permissions), but continue
		// since the version might already be available locally
		_ = f.fetch(ctx, repo) //nolint:errcheck // Best-effort fetch; local version may suffice
	}

	// Checkout the specified version
	commitHash, err := f.checkout(repo, version)
	if err != nil {
		return "", "", fmt.Errorf("failed to checkout version %s: %w", version, err)
	}

	return cachePath, commitHash, nil
}

// GetCommitForTag returns the commit hash for a specific tag.
func (f *GitFetcher) GetCommitForTag(ctx context.Context, gitURL GitURL, tagName SemVer) (GitCommit, error) {
	// Use in-memory storage to list remote refs without cloning
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{string(gitURL)},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list remote refs: %w", err)
	}

	// Try both with and without "v" prefix
	tagStr := string(tagName)
	tagNames := []string{tagStr}
	if noV, found := strings.CutPrefix(tagStr, "v"); found {
		tagNames = append(tagNames, noV)
	} else {
		tagNames = append(tagNames, "v"+tagStr)
	}

	for _, ref := range refs {
		if ref.Name().IsTag() && slices.Contains(tagNames, ref.Name().Short()) {
			return GitCommit(ref.Hash().String()), nil
		}
	}

	return "", fmt.Errorf("tag %q not found", tagName)
}

// CloneShallow performs a shallow clone of a repository at a specific tag.
// This is more efficient for modules where we only need a specific version.
func (f *GitFetcher) CloneShallow(ctx context.Context, gitURL GitURL, version SemVer, destPath types.FilesystemPath) (GitCommit, error) {
	dest := string(destPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Try with and without v prefix
	versionStr := string(version)
	tagNames := []string{versionStr}
	if noV, found := strings.CutPrefix(versionStr, "v"); found {
		tagNames = append(tagNames, noV)
	} else {
		tagNames = append(tagNames, "v"+versionStr)
	}

	var lastErr error
	for _, tagName := range tagNames {
		repo, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
			URL:           string(gitURL),
			Auth:          f.auth,
			ReferenceName: plumbing.NewTagReferenceName(tagName),
			SingleBranch:  true,
			Depth:         1,
			Progress:      nil,
		})
		if err != nil {
			lastErr = err
			// Clean up failed attempt (best-effort)
			_ = os.RemoveAll(dest)
			continue
		}

		// Get the HEAD commit
		head, err := repo.Head()
		if err != nil {
			return "", fmt.Errorf("failed to get HEAD: %w", err)
		}

		return GitCommit(head.Hash().String()), nil
	}

	return "", fmt.Errorf("failed to clone at version %s: %w", version, lastErr)
}

// IsPrivateRepo checks if a repository requires authentication.
func (f *GitFetcher) IsPrivateRepo(ctx context.Context, gitURL GitURL) bool {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{string(gitURL)},
	})

	// Try to list refs without auth
	_, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: nil,
	})

	return err != nil
}

// ValidateAuth checks if authentication is configured for a URL.
func (f *GitFetcher) ValidateAuth(gitURL GitURL) error {
	gitURLStr := string(gitURL)
	if f.auth == nil {
		if strings.HasPrefix(gitURLStr, "git@") || strings.Contains(gitURLStr, "ssh://") {
			return fmt.Errorf("SSH URL detected but no SSH key found; please add an SSH key to ~/.ssh/")
		}
		// No auth configured, will work for public HTTPS repos
		return nil
	}
	return nil
}

// ListTags returns all tags from a repository sorted by version.
func (f *GitFetcher) ListTags(ctx context.Context, gitURL GitURL) ([]SemVer, error) {
	return f.ListVersions(ctx, gitURL)
}

// ListTagsWithCommits returns all tags with their commit hashes.
func (f *GitFetcher) ListTagsWithCommits(ctx context.Context, gitURL GitURL) ([]TagInfo, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{string(gitURL)},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	var tags []TagInfo
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tags = append(tags, TagInfo{
				Name:   ref.Name().Short(),
				Commit: GitCommit(ref.Hash().String()),
			})
		}
	}

	// Sort by version
	sort.Slice(tags, func(i, j int) bool {
		vi, _ := ParseVersion(tags[i].Name) //nolint:errcheck // Non-semver tags sort lexically
		vj, _ := ParseVersion(tags[j].Name) //nolint:errcheck // Non-semver tags sort lexically
		if vi == nil || vj == nil {
			return tags[i].Name < tags[j].Name
		}
		return vi.Compare(vj) > 0
	})

	return tags, nil
}

// setupAuth configures authentication based on available credentials.
func (f *GitFetcher) setupAuth() {
	// Try SSH authentication first
	if sshAuth := f.trySSHAuth(); sshAuth != nil {
		f.auth = sshAuth
		return
	}

	// Try HTTPS authentication via environment variables
	if httpAuth := f.tryHTTPAuth(); httpAuth != nil {
		f.auth = httpAuth
		return
	}

	// No authentication configured - will work for public repos
}

// trySSHAuth attempts to configure SSH authentication.
func (f *GitFetcher) trySSHAuth() transport.AuthMethod {
	// Check common SSH key locations
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	keyPaths := []string{
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
		filepath.Join(homeDir, ".ssh", "id_rsa"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa"),
	}

	for _, keyPath := range keyPaths {
		if _, err := os.Stat(keyPath); err == nil {
			auth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
			if err == nil {
				return auth
			}
		}
	}

	return nil
}

// tryHTTPAuth attempts to configure HTTP authentication.
func (f *GitFetcher) tryHTTPAuth() transport.AuthMethod {
	// Check for GitHub token
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	// Check for GitLab token
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "gitlab-ci-token",
			Password: token,
		}
	}

	// Check for generic Git token
	if token := os.Getenv("GIT_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "git",
			Password: token,
		}
	}

	return nil
}

// clone clones a repository to the specified path.
func (f *GitFetcher) clone(ctx context.Context, gitURL GitURL, destPath types.FilesystemPath) (*git.Repository, error) {
	dest := string(destPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone the repository
	repo, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:      string(gitURL),
		Auth:     f.auth,
		Progress: nil, // Could add progress reporting here
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// fetch fetches updates from the remote repository.
func (f *GitFetcher) fetch(ctx context.Context, repo *git.Repository) error {
	err := repo.FetchContext(ctx, &git.FetchOptions{
		Auth:  f.auth,
		Tags:  git.AllTags,
		Force: true,
	})

	// ErrAlreadyUpToDate is not a real error
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	return nil
}

// checkout checks out a specific version (tag) in the repository.
func (f *GitFetcher) checkout(repo *git.Repository, version SemVer) (GitCommit, error) {
	// Get worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Try to find the tag
	tagRef, err := f.findTag(repo, version)
	if err != nil {
		return "", fmt.Errorf("tag not found: %w", err)
	}

	// Checkout the tag
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:  tagRef,
		Force: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to checkout: %w", err)
	}

	return GitCommit(tagRef.String()), nil
}

// findTag finds a tag by name, trying both with and without "v" prefix.
func (f *GitFetcher) findTag(repo *git.Repository, version SemVer) (plumbing.Hash, error) {
	// Try the version as-is first
	versionStr := string(version)
	tagNames := []string{versionStr}

	// Also try with/without "v" prefix
	if noV, found := strings.CutPrefix(versionStr, "v"); found {
		tagNames = append(tagNames, noV)
	} else {
		tagNames = append(tagNames, "v"+versionStr)
	}

	for _, tagName := range tagNames {
		// Try full reference name
		ref, err := repo.Reference(plumbing.NewTagReferenceName(tagName), true)
		if err == nil {
			// If it's an annotated tag, we need to dereference it
			tagObj, err := repo.TagObject(ref.Hash())
			if err == nil {
				// Annotated tag - return the commit it points to
				return tagObj.Target, nil
			}
			// Lightweight tag - return the hash directly
			return ref.Hash(), nil
		}
	}

	return plumbing.ZeroHash, fmt.Errorf("tag %q not found", version)
}

// getRepoCachePath generates a cache path for a repository.
func (f *GitFetcher) getRepoCachePath(gitURL GitURL) types.FilesystemPath {
	// Convert git URL to path-safe format
	// e.g., "https://github.com/user/repo.git" -> "github.com/user/repo"
	urlStr := string(gitURL)
	path := strings.TrimPrefix(urlStr, "https://")
	path = strings.TrimPrefix(path, "git@")
	path = strings.TrimPrefix(path, "ssh://")
	path = strings.TrimSuffix(path, ".git")
	path = strings.ReplaceAll(path, ":", "/")

	return types.FilesystemPath(filepath.Join(string(f.CacheDir), "sources", path))
}
