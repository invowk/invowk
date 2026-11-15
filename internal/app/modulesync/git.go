// SPDX-License-Identifier: MPL-2.0

package modulesync

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

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	errMsgListRemoteRefs = "failed to list remote refs: %w"

	// sshKnownHostsEnv is go-git's known_hosts override. It is not a base
	// directory variable, so it is read here and not through basedir.
	sshKnownHostsEnv = "SSH_KNOWN_HOSTS"
)

type (
	// GitFetcher handles Git operations for module fetching.
	// Fields are unexported for immutability; use CacheDir() accessor.
	GitFetcher struct {
		cacheDir types.FilesystemPath
		auth     transport.AuthMethod
		// dirs resolves the home directory that SSH keys and known_hosts
		// come from; the zero value is the real process.
		dirs basedir.Dirs
		// getenv reads SSH_KNOWN_HOSTS and the HTTPS token variables; nil
		// means os.Getenv.
		getenv func(string) string
	}

	// SSHTrustFilesError reports an SSH-transport fetch refused before
	// connecting, because the home directory or an SSH_KNOWN_HOSTS entry is
	// relative or unavailable (base-directory-resolution).
	SSHTrustFilesError struct {
		// Source names the refused value: "home directory" or
		// "SSH_KNOWN_HOSTS".
		Source string //goplint:ignore -- fixed label for the diagnostic text.
		// Value is the refused SSH_KNOWN_HOSTS entry; empty for the home.
		Value string //goplint:ignore -- raw environment value preserved for the diagnostic.
		Err   error
	}

	// tagInfo contains information about a Git tag.
	tagInfo struct {
		// Name is the git tag name; intentionally untyped (pass-through from go-git).
		Name   string
		Commit GitCommit
	}
)

// NewGitFetcher creates a new Git fetcher.
func NewGitFetcher(cacheDir types.FilesystemPath) *GitFetcher {
	return newGitFetcherWith(cacheDir, basedir.OS(), os.Getenv)
}

// newGitFetcherWith is NewGitFetcher with an injected base-directory
// resolver and environment lookup.
func newGitFetcherWith(cacheDir types.FilesystemPath, dirs basedir.Dirs, getenv func(string) string) *GitFetcher {
	f := &GitFetcher{
		cacheDir: cacheDir,
		dirs:     dirs,
		getenv:   getenv,
	}
	f.setupAuth()
	return f
}

// Error implements error.
func (e *SSHTrustFilesError) Error() string {
	subject := e.Source
	if e.Value != "" {
		subject = fmt.Sprintf("%s entry %q", e.Source, e.Value)
	}
	return fmt.Sprintf("%v: %s: %v; use an absolute home directory and SSH_KNOWN_HOSTS, "+
		"or an https:// URL with GITHUB_TOKEN, GITLAB_TOKEN or GIT_TOKEN",
		ErrSSHTrustFilesNotAbsolute, subject, e.Err)
}

// Unwrap returns the sentinel and the cause, so errors.Is matches
// ErrSSHTrustFilesNotAbsolute and basedir.ErrNotAbsolute (or the home lookup
// error).
func (e *SSHTrustFilesError) Unwrap() []error {
	return []error{ErrSSHTrustFilesNotAbsolute, e.Err}
}

// CacheDir returns the base directory for the Git source cache.
func (f *GitFetcher) CacheDir() types.FilesystemPath { return f.cacheDir }

// ListVersions returns all version tags from a Git repository.
func (f *GitFetcher) ListVersions(ctx context.Context, gitURL GitURL) ([]SemVer, error) {
	if err := f.requireTrustedSSHFiles(gitURL); err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf(errMsgListRemoteRefs, err)
	}

	// Filter for version tags
	var versions []SemVer
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()
			// Accept both "v1.2.3" and "1.2.3" formats
			if isValidVersionString(tagName) {
				versions = append(versions, SemVer(tagName)) //goplint:ignore -- isValidVersionString pre-validates format
			}
		}
	}

	// Sort versions (newest first)
	versions = invowkmod.SortVersions(versions)

	return versions, nil
}

// Fetch clones or fetches a Git repository and checks out the specified version.
// Returns the path to the repository and the commit SHA.
func (f *GitFetcher) Fetch(ctx context.Context, gitURL GitURL, version SemVer) (repoPath types.FilesystemPath, commitSHA GitCommit, err error) {
	if gateErr := f.requireTrustedSSHFiles(gitURL); gateErr != nil {
		return "", "", gateErr
	}
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
	if err := f.requireTrustedSSHFiles(gitURL); err != nil {
		return "", err
	}
	// Use in-memory storage to list remote refs without cloning
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{string(gitURL)},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return "", fmt.Errorf(errMsgListRemoteRefs, err)
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
			commit := GitCommit(ref.Hash().String())
			if err := commit.Validate(); err != nil {
				return "", fmt.Errorf("git commit hash for tag %q: %w", tagName, err)
			}
			return commit, nil
		}
	}

	return "", fmt.Errorf("%w: %q", ErrTagNotFound, tagName)
}

// CloneShallow performs a shallow clone of a repository at a specific tag.
// This is more efficient for modules where we only need a specific version.
func (f *GitFetcher) CloneShallow(ctx context.Context, gitURL GitURL, version SemVer, destPath types.FilesystemPath) (GitCommit, error) {
	if err := f.requireTrustedSSHFiles(gitURL); err != nil {
		return "", err
	}
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

		commit := GitCommit(head.Hash().String())
		if err := commit.Validate(); err != nil {
			return "", fmt.Errorf("git HEAD commit: %w", err)
		}
		return commit, nil
	}

	return "", fmt.Errorf("%w at version %s: %w", ErrCloneFailed, version, lastErr)
}

// IsPrivateRepo checks if a repository requires authentication. An SSH URL
// refused by requireTrustedSSHFiles reports true without connecting.
func (f *GitFetcher) IsPrivateRepo(ctx context.Context, gitURL GitURL) bool {
	if f.requireTrustedSSHFiles(gitURL) != nil {
		return true
	}
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
			return fmt.Errorf("%w: SSH URL detected; please add an SSH key to ~/.ssh/", ErrSSHKeyNotFound)
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
func (f *GitFetcher) ListTagsWithCommits(ctx context.Context, gitURL GitURL) ([]tagInfo, error) {
	if err := f.requireTrustedSSHFiles(gitURL); err != nil {
		return nil, err
	}
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{string(gitURL)},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{
		Auth: f.auth,
	})
	if err != nil {
		return nil, fmt.Errorf(errMsgListRemoteRefs, err)
	}

	var tags []tagInfo
	for _, ref := range refs {
		if ref.Name().IsTag() {
			commit := GitCommit(ref.Hash().String())
			if err := commit.Validate(); err != nil {
				return nil, fmt.Errorf("git commit hash for tag %q: %w", ref.Name().Short(), err)
			}
			tags = append(tags, tagInfo{
				Name:   ref.Name().Short(),
				Commit: commit,
			})
		}
	}

	// Sort by version
	sort.Slice(tags, func(i, j int) bool {
		vi, _ := invowkmod.ParseVersion(tags[i].Name) //nolint:errcheck // Non-semver tags sort lexically
		vj, _ := invowkmod.ParseVersion(tags[j].Name) //nolint:errcheck // Non-semver tags sort lexically
		if vi == nil || vj == nil {
			return tags[i].Name < tags[j].Name
		}
		return vi.Compare(vj) > 0
	})

	return tags, nil
}

// requireTrustedSSHFiles refuses an SSH-transport URL (anything that is not
// https://) unless the home directory resolves to an absolute path and every
// non-empty SSH_KNOWN_HOSTS entry is absolute. invowk reads SSH keys from the
// home; go-git reads known_hosts from it and uses SSH_KNOWN_HOSTS entries as
// given; ssh_config reads .ssh/config from it. A relative or unavailable value
// would let a repository supply those files from the current directory.
//
// The test is "not https://" rather than a list of SSH prefixes, so a scheme
// added to GitURL.Validate later fails closed.
func (f *GitFetcher) requireTrustedSSHFiles(gitURL GitURL) error {
	if strings.HasPrefix(string(gitURL), "https://") {
		return nil
	}
	if _, err := f.dirs.Home(); err != nil {
		return &SSHTrustFilesError{Source: "home directory", Err: err}
	}
	for _, entry := range filepath.SplitList(f.env(sshKnownHostsEnv)) {
		if entry != "" && !filepath.IsAbs(entry) {
			return &SSHTrustFilesError{Source: sshKnownHostsEnv, Value: entry, Err: basedir.ErrNotAbsolute}
		}
	}
	return nil
}

// env reads an environment variable through the injected lookup.
//
//goplint:ignore -- raw environment variable name and value at the OS boundary.
func (f *GitFetcher) env(name string) string {
	if f.getenv == nil {
		return os.Getenv(name)
	}
	return f.getenv(name)
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
	// Check common SSH key locations, only under an absolute home: a
	// relative one would load keys from the current directory.
	home, err := f.dirs.Home()
	if err != nil {
		return nil
	}
	homeDir := string(home)

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
	if token := f.env("GITHUB_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	// Check for GitLab token
	if token := f.env("GITLAB_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "gitlab-ci-token",
			Password: token,
		}
	}

	// Check for generic Git token
	if token := f.env("GIT_TOKEN"); token != "" {
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
		return "", err
	}

	// Checkout the tag
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:  tagRef,
		Force: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to checkout: %w", err)
	}

	commit := GitCommit(tagRef.String())
	if err := commit.Validate(); err != nil {
		return "", fmt.Errorf("git tag commit: %w", err)
	}
	return commit, nil
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

	return plumbing.ZeroHash, fmt.Errorf("%w: %q", ErrTagNotFound, version)
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

	return fspath.JoinStr(f.cacheDir, "sources", path)
}
