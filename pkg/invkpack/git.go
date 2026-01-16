// SPDX-License-Identifier: EPL-2.0

package invkpack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
)

// GitFetcher handles Git operations for pack fetching.
type GitFetcher struct {
	// CacheDir is the base directory for the Git source cache.
	CacheDir string

	// auth is the authentication method to use for Git operations.
	auth transport.AuthMethod
}

// NewGitFetcher creates a new Git fetcher.
func NewGitFetcher(cacheDir string) *GitFetcher {
	f := &GitFetcher{
		CacheDir: cacheDir,
	}
	f.setupAuth()
	return f
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

// ListVersions returns all version tags from a Git repository.
func (f *GitFetcher) ListVersions(ctx context.Context, gitURL string) ([]string, error) {
	// Use in-memory storage to list remote refs without cloning
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	// List all references from the remote
	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	// Filter for version tags
	var versions []string
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()
			// Accept both "v1.2.3" and "1.2.3" formats
			if IsValidVersion(tagName) {
				versions = append(versions, tagName)
			}
		}
	}

	// Sort versions (newest first)
	versions = SortVersions(versions)

	return versions, nil
}

// Fetch clones or fetches a Git repository and checks out the specified version.
// Returns the path to the repository and the commit SHA.
func (f *GitFetcher) Fetch(ctx context.Context, gitURL, version string) (string, string, error) {
	// Generate a cache path for this repository
	repoPath := f.getRepoCachePath(gitURL)

	// Check if we already have this repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		// Repository doesn't exist, clone it
		repo, err = f.clone(ctx, gitURL, repoPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		// Repository exists, fetch updates
		// Fetch might fail for various reasons (network, permissions), but continue
		// since the version might already be available locally
		_ = f.fetch(ctx, repo)
	}

	// Checkout the specified version
	commitHash, err := f.checkout(repo, version)
	if err != nil {
		return "", "", fmt.Errorf("failed to checkout version %s: %w", version, err)
	}

	return repoPath, commitHash, nil
}

// clone clones a repository to the specified path.
func (f *GitFetcher) clone(ctx context.Context, gitURL, destPath string) (*git.Repository, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone the repository
	repo, err := git.PlainCloneContext(ctx, destPath, false, &git.CloneOptions{
		URL:      gitURL,
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
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

// checkout checks out a specific version (tag) in the repository.
func (f *GitFetcher) checkout(repo *git.Repository, version string) (string, error) {
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

	return tagRef.String(), nil
}

// findTag finds a tag by name, trying both with and without "v" prefix.
func (f *GitFetcher) findTag(repo *git.Repository, version string) (plumbing.Hash, error) {
	// Try the version as-is first
	tagNames := []string{version}

	// Also try with/without "v" prefix
	if strings.HasPrefix(version, "v") {
		tagNames = append(tagNames, strings.TrimPrefix(version, "v"))
	} else {
		tagNames = append(tagNames, "v"+version)
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
func (f *GitFetcher) getRepoCachePath(gitURL string) string {
	// Convert git URL to path-safe format
	// e.g., "https://github.com/user/repo.git" -> "github.com/user/repo"
	path := strings.TrimPrefix(gitURL, "https://")
	path = strings.TrimPrefix(path, "git@")
	path = strings.TrimPrefix(path, "ssh://")
	path = strings.TrimSuffix(path, ".git")
	path = strings.ReplaceAll(path, ":", "/")

	return filepath.Join(f.CacheDir, "sources", path)
}

// GetCommitForTag returns the commit hash for a specific tag.
func (f *GitFetcher) GetCommitForTag(ctx context.Context, gitURL, tagName string) (string, error) {
	// Use in-memory storage to list remote refs without cloning
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list remote refs: %w", err)
	}

	// Try both with and without "v" prefix
	tagNames := []string{tagName}
	if strings.HasPrefix(tagName, "v") {
		tagNames = append(tagNames, strings.TrimPrefix(tagName, "v"))
	} else {
		tagNames = append(tagNames, "v"+tagName)
	}

	for _, ref := range refs {
		if ref.Name().IsTag() {
			for _, tn := range tagNames {
				if ref.Name().Short() == tn {
					return ref.Hash().String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("tag %q not found", tagName)
}

// CloneShallow performs a shallow clone of a repository at a specific tag.
// This is more efficient for modules where we only need a specific version.
func (f *GitFetcher) CloneShallow(ctx context.Context, gitURL, version, destPath string) (string, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Try with and without v prefix
	tagNames := []string{version}
	if strings.HasPrefix(version, "v") {
		tagNames = append(tagNames, strings.TrimPrefix(version, "v"))
	} else {
		tagNames = append(tagNames, "v"+version)
	}

	var lastErr error
	for _, tagName := range tagNames {
		repo, err := git.PlainCloneContext(ctx, destPath, false, &git.CloneOptions{
			URL:           gitURL,
			Auth:          f.auth,
			ReferenceName: plumbing.NewTagReferenceName(tagName),
			SingleBranch:  true,
			Depth:         1,
			Progress:      nil,
		})
		if err != nil {
			lastErr = err
			// Clean up failed attempt (best-effort)
			_ = os.RemoveAll(destPath)
			continue
		}

		// Get the HEAD commit
		head, err := repo.Head()
		if err != nil {
			return "", fmt.Errorf("failed to get HEAD: %w", err)
		}

		return head.Hash().String(), nil
	}

	return "", fmt.Errorf("failed to clone at version %s: %w", version, lastErr)
}

// IsPrivateRepo checks if a repository requires authentication.
func (f *GitFetcher) IsPrivateRepo(ctx context.Context, gitURL string) bool {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	// Try to list refs without auth
	_, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: nil,
	})

	return err != nil
}

// ValidateAuth checks if authentication is configured for a URL.
func (f *GitFetcher) ValidateAuth(gitURL string) error {
	if f.auth == nil {
		if strings.HasPrefix(gitURL, "git@") || strings.Contains(gitURL, "ssh://") {
			return fmt.Errorf("SSH URL detected but no SSH key found; please add an SSH key to ~/.ssh/")
		}
		// No auth configured, will work for public HTTPS repos
		return nil
	}
	return nil
}

// ListTags returns all tags from a repository sorted by version.
func (f *GitFetcher) ListTags(ctx context.Context, gitURL string) ([]string, error) {
	return f.ListVersions(ctx, gitURL)
}

// TagInfo contains information about a Git tag.
type TagInfo struct {
	Name   string
	Commit string
}

// ListTagsWithCommits returns all tags with their commit hashes.
func (f *GitFetcher) ListTagsWithCommits(ctx context.Context, gitURL string) ([]TagInfo, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
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
				Commit: ref.Hash().String(),
			})
		}
	}

	// Sort by version
	sort.Slice(tags, func(i, j int) bool {
		vi, _ := ParseVersion(tags[i].Name)
		vj, _ := ParseVersion(tags[j].Name)
		if vi == nil || vj == nil {
			return tags[i].Name < tags[j].Name
		}
		return vi.Compare(vj) > 0
	})

	return tags, nil
}
