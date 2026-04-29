// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/invowk/invowk/pkg/types"
)

// newTestGitRepo creates a local git repo with tagged versions in t.TempDir().
// Returns the file:// URL as a GitURL. Each version string becomes a lightweight
// git tag pointing to a unique commit. Additional non-version tags can be added
// via extraTags.
func newTestGitRepo(t *testing.T, versions, extraTags []string) GitURL {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	// Create initial file and commit so the repo is non-empty
	filePath := filepath.Join(dir, "README.md")
	err = os.WriteFile(filePath, []byte("# Test Module\n"), 0o644)
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err = w.Add("README.md")
	if err != nil {
		t.Fatalf("git add: %v", err)
	}

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err = w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  baseTime,
		},
	})
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// Create version tags — each gets a unique commit so tags point to different SHAs
	allTags := make([]string, 0, len(versions)+len(extraTags))
	allTags = append(allTags, versions...)
	allTags = append(allTags, extraTags...)
	for i, tag := range allTags {
		vFile := filepath.Join(dir, tag+".txt")
		if err := os.WriteFile(vFile, []byte(tag), 0o644); err != nil {
			t.Fatalf("write version file: %v", err)
		}
		if _, err := w.Add(tag + ".txt"); err != nil {
			t.Fatalf("git add version: %v", err)
		}

		commitTime := baseTime.Add(time.Duration(i+1) * time.Hour)
		commitHash, commitErr := w.Commit("release "+tag, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@example.com",
				When:  commitTime,
			},
		})
		if commitErr != nil {
			t.Fatalf("git commit version: %v", commitErr)
		}

		// Lightweight tag
		if _, tagErr := repo.CreateTag(tag, commitHash, nil); tagErr != nil {
			t.Fatalf("git tag %s: %v", tag, tagErr)
		}
	}

	// Normalize to a valid file:// URL on all platforms.
	// On Windows t.TempDir() returns "C:\Users\...", which must become
	// "file:///C:/Users/..." (three slashes, forward slashes per RFC 8089).
	urlPath := filepath.ToSlash(dir)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	return GitURL("file://" + urlPath)
}

func newTestFetcher(t *testing.T) *GitFetcher {
	t.Helper()
	return NewGitFetcher(types.FilesystemPath(t.TempDir()))
}

func TestGitFetcher_CacheDir(t *testing.T) {
	t.Parallel()

	cacheDir := types.FilesystemPath(t.TempDir())
	fetcher := NewGitFetcher(cacheDir)

	if fetcher.CacheDir() != cacheDir {
		t.Errorf("CacheDir() = %q, want %q", fetcher.CacheDir(), cacheDir)
	}
}

func TestGitFetcher_ListVersions(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t,
		[]string{"v1.0.0", "v2.0.0", "v1.1.0"},
		[]string{"latest", "nightly-2024-01-01"},
	)

	fetcher := newTestFetcher(t)

	versions, err := fetcher.ListVersions(t.Context(), repoURL)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}

	// Should only contain valid semver tags, not "latest" or "nightly-2024-01-01"
	if len(versions) != 3 {
		t.Fatalf("ListVersions() returned %d versions, want 3: %v", len(versions), versions)
	}

	// Versions should be sorted newest-first
	want := []SemVer{"v2.0.0", "v1.1.0", "v1.0.0"}
	for i, v := range versions {
		if v != want[i] {
			t.Errorf("versions[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestGitFetcher_ListVersions_EmptyRepo(t *testing.T) {
	t.Parallel()

	// Create a repo with no tags at all
	repoURL := newTestGitRepo(t, nil, nil)

	fetcher := newTestFetcher(t)

	versions, err := fetcher.ListVersions(t.Context(), repoURL)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("ListVersions() returned %d versions for empty repo, want 0: %v", len(versions), versions)
	}
}

func TestGitFetcher_GetCommitForTag(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0", "v2.0.0"}, nil)

	fetcher := newTestFetcher(t)

	commit, err := fetcher.GetCommitForTag(t.Context(), repoURL, "v1.0.0")
	if err != nil {
		t.Fatalf("GetCommitForTag() error: %v", err)
	}

	// Validate that the returned commit is a proper 40-char hex SHA
	if validateErr := commit.Validate(); validateErr != nil {
		t.Errorf("returned commit %q failed validation: %v", commit, validateErr)
	}

	// Fetch the same tag again — should return the same commit
	commit2, err := fetcher.GetCommitForTag(t.Context(), repoURL, "v1.0.0")
	if err != nil {
		t.Fatalf("GetCommitForTag() second call error: %v", err)
	}

	if commit != commit2 {
		t.Errorf("same tag returned different commits: %q vs %q", commit, commit2)
	}

	// Different tag should return a different commit
	commitV2, err := fetcher.GetCommitForTag(t.Context(), repoURL, "v2.0.0")
	if err != nil {
		t.Fatalf("GetCommitForTag(v2.0.0) error: %v", err)
	}

	if commit == commitV2 {
		t.Errorf("different tags returned same commit %q", commit)
	}
}

func TestGitFetcher_GetCommitForTag_NotFound(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0"}, nil)

	fetcher := newTestFetcher(t)

	_, err := fetcher.GetCommitForTag(t.Context(), repoURL, "v9.9.9")
	if err == nil {
		t.Fatal("GetCommitForTag() expected error for nonexistent tag, got nil")
	}

	if !errors.Is(err, ErrTagNotFound) {
		t.Errorf("error should wrap ErrTagNotFound, got: %v", err)
	}
}

func TestGitFetcher_ListTagsWithCommits(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0", "v2.0.0"}, []string{"docs-update"})

	fetcher := newTestFetcher(t)

	tags, err := fetcher.ListTagsWithCommits(t.Context(), repoURL)
	if err != nil {
		t.Fatalf("ListTagsWithCommits() error: %v", err)
	}

	// Should include ALL tags (version and non-version)
	if len(tags) != 3 {
		t.Fatalf("ListTagsWithCommits() returned %d tags, want 3: %v", len(tags), tags)
	}

	// Verify each tag has a valid commit SHA
	for _, tag := range tags {
		if tag.Name == "" {
			t.Error("TagInfo.Name is empty")
		}
		if validateErr := tag.Commit.Validate(); validateErr != nil {
			t.Errorf("tag %q commit %q failed validation: %v", tag.Name, tag.Commit, validateErr)
		}
	}

	// Check that version tags appear and are sorted (newest-first for semver tags)
	foundVersionTags := make(map[string]bool)
	for _, tag := range tags {
		foundVersionTags[tag.Name] = true
	}
	for _, expected := range []string{"v1.0.0", "v2.0.0", "docs-update"} {
		if !foundVersionTags[expected] {
			t.Errorf("expected tag %q not found in results", expected)
		}
	}
}

func TestGitFetcher_Fetch(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0", "v2.0.0"}, nil)

	cacheDir := types.FilesystemPath(t.TempDir())
	fetcher := NewGitFetcher(cacheDir)

	// Fetch v1.0.0
	repoPath, commit, err := fetcher.Fetch(t.Context(), repoURL, "v1.0.0")
	if err != nil {
		t.Fatalf("Fetch(v1.0.0) error: %v", err)
	}

	// Verify returned path exists
	if _, statErr := os.Stat(string(repoPath)); statErr != nil {
		t.Errorf("Fetch() returned path %q that does not exist: %v", repoPath, statErr)
	}

	// Verify returned path is under the cache directory
	if !strings.HasPrefix(string(repoPath), string(cacheDir)) {
		t.Errorf("Fetch() returned path %q is not under cache dir %q", repoPath, cacheDir)
	}

	// Verify commit SHA is valid
	if validateErr := commit.Validate(); validateErr != nil {
		t.Errorf("Fetch() commit %q failed validation: %v", commit, validateErr)
	}

	// Verify the v1.0.0.txt file exists in the checked-out repo (tag-specific content)
	versionFile := filepath.Join(string(repoPath), "v1.0.0.txt")
	if _, statErr := os.Stat(versionFile); statErr != nil {
		t.Errorf("expected file %q not found after checkout: %v", versionFile, statErr)
	}

	// Fetch again (tests the "repo already exists, fetch updates" path)
	repoPath2, commit2, err := fetcher.Fetch(t.Context(), repoURL, "v1.0.0")
	if err != nil {
		t.Fatalf("Fetch(v1.0.0) second call error: %v", err)
	}

	// Same version should return the same commit
	if commit != commit2 {
		t.Errorf("repeated Fetch returned different commits: %q vs %q", commit, commit2)
	}

	// Same repo URL should use the same cache path
	if repoPath != repoPath2 {
		t.Errorf("repeated Fetch returned different paths: %q vs %q", repoPath, repoPath2)
	}

	// Fetch a different version
	_, commitV2, err := fetcher.Fetch(t.Context(), repoURL, "v2.0.0")
	if err != nil {
		t.Fatalf("Fetch(v2.0.0) error: %v", err)
	}

	if commit == commitV2 {
		t.Errorf("different versions returned same commit %q", commit)
	}
}

func TestGitFetcher_Fetch_NonexistentTag(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0"}, nil)

	fetcher := newTestFetcher(t)

	_, _, err := fetcher.Fetch(t.Context(), repoURL, "v9.9.9")
	if err == nil {
		t.Fatal("Fetch() expected error for nonexistent tag, got nil")
	}

	if !errors.Is(err, ErrTagNotFound) {
		t.Errorf("error should wrap ErrTagNotFound, got: %v", err)
	}
}

func TestGitFetcher_CloneShallow(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0"}, nil)

	fetcher := newTestFetcher(t)

	destDir := filepath.Join(t.TempDir(), "shallow-clone")
	destPath := types.FilesystemPath(destDir)

	commit, err := fetcher.CloneShallow(t.Context(), repoURL, "v1.0.0", destPath)
	if err != nil {
		t.Fatalf("CloneShallow() error: %v", err)
	}

	// Verify commit SHA is valid
	if validateErr := commit.Validate(); validateErr != nil {
		t.Errorf("CloneShallow() commit %q failed validation: %v", commit, validateErr)
	}

	// Verify destination exists and contains repo content
	readmePath := filepath.Join(destDir, "README.md")
	if _, statErr := os.Stat(readmePath); statErr != nil {
		t.Errorf("expected README.md at %q after shallow clone: %v", readmePath, statErr)
	}

	// Verify the tag-specific file exists
	versionFile := filepath.Join(destDir, "v1.0.0.txt")
	if _, statErr := os.Stat(versionFile); statErr != nil {
		t.Errorf("expected v1.0.0.txt at %q after shallow clone: %v", versionFile, statErr)
	}
}

func TestGitFetcher_CloneShallow_NonexistentTag(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0"}, nil)

	fetcher := newTestFetcher(t)

	destDir := filepath.Join(t.TempDir(), "shallow-missing")
	destPath := types.FilesystemPath(destDir)

	_, err := fetcher.CloneShallow(t.Context(), repoURL, "v9.9.9", destPath)
	if err == nil {
		t.Fatal("CloneShallow() expected error for nonexistent tag, got nil")
	}

	if !errors.Is(err, ErrCloneFailed) {
		t.Errorf("error should wrap ErrCloneFailed, got: %v", err)
	}
}

func TestGitFetcher_IsPrivateRepo(t *testing.T) {
	t.Parallel()

	// Local file:// repos are always accessible without auth
	repoURL := newTestGitRepo(t, []string{"v1.0.0"}, nil)

	fetcher := newTestFetcher(t)

	isPrivate := fetcher.IsPrivateRepo(t.Context(), repoURL)
	if isPrivate {
		t.Error("IsPrivateRepo() = true for local file:// repo, want false")
	}
}

func TestGitFetcher_ValidateAuth(t *testing.T) {
	t.Parallel()

	fetcher := newTestFetcher(t)

	// Clear auth so we test the no-auth code path explicitly
	fetcher.auth = nil

	t.Run("https_no_auth", func(t *testing.T) {
		t.Parallel()
		err := fetcher.ValidateAuth("https://github.com/user/repo.git")
		if err != nil {
			t.Errorf("ValidateAuth() for HTTPS with no auth should return nil, got: %v", err)
		}
	})

	t.Run("ssh_no_auth", func(t *testing.T) {
		t.Parallel()
		err := fetcher.ValidateAuth("git@github.com:user/repo.git")
		if err == nil {
			t.Fatal("ValidateAuth() for SSH URL with no auth should return error, got nil")
		}
		if !errors.Is(err, ErrSSHKeyNotFound) {
			t.Errorf("error should wrap ErrSSHKeyNotFound, got: %v", err)
		}
	})

	t.Run("ssh_protocol_no_auth", func(t *testing.T) {
		t.Parallel()
		err := fetcher.ValidateAuth("ssh://git@github.com/user/repo.git")
		if err == nil {
			t.Fatal("ValidateAuth() for ssh:// URL with no auth should return error, got nil")
		}
		if !errors.Is(err, ErrSSHKeyNotFound) {
			t.Errorf("error should wrap ErrSSHKeyNotFound, got: %v", err)
		}
	})
}

func TestGitFetcher_ListTags(t *testing.T) {
	t.Parallel()

	repoURL := newTestGitRepo(t, []string{"v1.0.0", "v3.0.0", "v2.0.0"}, nil)

	fetcher := newTestFetcher(t)

	// ListTags delegates to ListVersions; verify it returns the same result
	tags, err := fetcher.ListTags(t.Context(), repoURL)
	if err != nil {
		t.Fatalf("ListTags() error: %v", err)
	}

	versions, err := fetcher.ListVersions(t.Context(), repoURL)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}

	if len(tags) != len(versions) {
		t.Fatalf("ListTags() returned %d, ListVersions() returned %d", len(tags), len(versions))
	}

	for i := range tags {
		if tags[i] != versions[i] {
			t.Errorf("ListTags()[%d] = %q, ListVersions()[%d] = %q", i, tags[i], i, versions[i])
		}
	}
}

func TestGitFetcher_ListVersions_OnlySemverTags(t *testing.T) {
	t.Parallel()

	// Mix of valid semver and non-semver tags to verify filtering
	repoURL := newTestGitRepo(t,
		[]string{"v1.0.0"},
		[]string{"release-candidate", "beta", "stable"},
	)

	fetcher := newTestFetcher(t)

	versions, err := fetcher.ListVersions(t.Context(), repoURL)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}

	// Only "v1.0.0" is valid semver
	if len(versions) != 1 {
		t.Fatalf("ListVersions() returned %d versions, want 1: %v", len(versions), versions)
	}

	if versions[0] != "v1.0.0" {
		t.Errorf("versions[0] = %q, want %q", versions[0], "v1.0.0")
	}
}

func TestGitFetcher_GetCommitForTag_VPrefixFallback(t *testing.T) {
	t.Parallel()

	// Create repo with a tag that has no "v" prefix
	repoURL := newTestGitRepo(t, nil, []string{"1.0.0"})

	fetcher := newTestFetcher(t)

	// Request with "v" prefix — should fall back to finding "1.0.0"
	commit, err := fetcher.GetCommitForTag(t.Context(), repoURL, "v1.0.0")
	if err != nil {
		t.Fatalf("GetCommitForTag(v1.0.0) with tag '1.0.0' error: %v", err)
	}

	if validateErr := commit.Validate(); validateErr != nil {
		t.Errorf("commit %q failed validation: %v", commit, validateErr)
	}

	// Request without "v" prefix directly
	commit2, err := fetcher.GetCommitForTag(t.Context(), repoURL, "1.0.0")
	if err != nil {
		t.Fatalf("GetCommitForTag(1.0.0) error: %v", err)
	}

	// Both lookups should resolve to the same commit
	if commit != commit2 {
		t.Errorf("v-prefix fallback returned different commits: %q vs %q", commit, commit2)
	}
}
