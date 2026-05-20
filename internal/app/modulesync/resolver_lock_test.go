// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestLoadFromLockUsesCanonicalCachePath(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cacheDir := t.TempDir()
	lock := invowkmod.NewLockFile()
	lock.Modules["https://github.com/user/tools.git"] = LockedModule{
		GitURL:          "https://github.com/user/tools.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.2.3",
		GitCommit:       "abc123def456789012345678901234567890abcd",
		Namespace:       "io.example.tools@1.2.3",
		CommandSourceID: "io.example.tools",
		ModuleID:        "io.example.tools",
		ContentHash:     ContentHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
	}
	if err := lock.Save(filepath.Join(workDir, LockFileName)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	resolver, err := NewResolver(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir))
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	modules, err := resolver.LoadFromLock(t.Context())
	if err != nil {
		t.Fatalf("LoadFromLock() error = %v", err)
	}
	if len(modules) != 1 {
		t.Fatalf("LoadFromLock() returned %d modules, want 1", len(modules))
	}
	wantSuffix := filepath.Join("github.com", "user", "tools", "1.2.3", "io.example.tools.invowkmod")
	if !strings.HasSuffix(string(modules[0].CachePath), wantSuffix) {
		t.Fatalf("CachePath = %q, want suffix %q", modules[0].CachePath, wantSuffix)
	}
	if modules[0].CommandSourceID != "io.example.tools" {
		t.Fatalf("CommandSourceID = %q, want io.example.tools", modules[0].CommandSourceID)
	}
}

func TestSyncDeduplicatesSameSourceRequirement(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cacheDir := t.TempDir()
	repoDir := t.TempDir()
	writeSourceModule(t, repoDir, "io.example.tools")
	fetcher := &fakeModuleFetcher{
		repoPath:     types.FilesystemPath(repoDir),
		listVersions: []SemVer{"1.2.3"},
	}
	resolver, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir), fetcher)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	req := ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"}
	resolved, err := resolver.Sync(t.Context(), []ModuleRef{req, req})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("Sync() resolved %d modules, want 1", len(resolved))
	}
	if fetcher.listCalls != 1 || fetcher.fetchCalls != 1 {
		t.Fatalf("fetcher calls = list:%d fetch:%d, want 1 each", fetcher.listCalls, fetcher.fetchCalls)
	}
}

func TestSyncRejectsCanonicalModuleCollision(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cacheDir := t.TempDir()
	repoDir := t.TempDir()
	writeSourceModule(t, repoDir, "io.example.tools")
	fetcher := &fakeModuleFetcher{
		repoPath:     types.FilesystemPath(repoDir),
		listVersions: []SemVer{"1.2.3"},
	}
	resolver, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir), fetcher)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	_, err = resolver.Sync(t.Context(), []ModuleRef{
		{GitURL: "https://github.com/org-a/tools.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/org-b/tools.git", Version: "^1.0.0"},
	})
	if err == nil {
		t.Fatal("Sync() error = nil, want canonical collision")
	}
	if !errors.Is(err, ErrCanonicalModuleCollision) {
		t.Fatalf("Sync() error = %v, want ErrCanonicalModuleCollision", err)
	}
	var collision *CanonicalModuleCollisionError
	if !errors.As(err, &collision) {
		t.Fatalf("Sync() error = %T, want CanonicalModuleCollisionError", err)
	}
	if collision.DirectoryName.String() != "io.example.tools.invowkmod" {
		t.Fatalf("DirectoryName = %q, want io.example.tools.invowkmod", collision.DirectoryName)
	}
}

func TestAddWritesFreshLockFile(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), LockFileName)
	resolved := testResolvedModule("https://github.com/user/tools.git", "tools@1.2.3")

	saveResolvedModuleToLock(t, lockPath, resolved)
	reloaded := loadLockFileForTest(t, lockPath)

	if len(reloaded.Modules) != 1 {
		t.Fatalf("lock file has %d modules, want 1", len(reloaded.Modules))
	}

	key := ModuleRefKey("https://github.com/user/tools.git")
	entry, ok := reloaded.Modules[key]
	if !ok {
		t.Fatalf("lock file missing key %q", key)
	}
	if entry.GitURL != "https://github.com/user/tools.git" {
		t.Errorf("GitURL = %q, want %q", entry.GitURL, "https://github.com/user/tools.git")
	}
	if entry.ResolvedVersion != "1.2.3" {
		t.Errorf("ResolvedVersion = %q, want %q", entry.ResolvedVersion, "1.2.3")
	}
	if entry.GitCommit != "abc123def456789012345678901234567890abcd" {
		t.Errorf("GitCommit = %q, want %q", entry.GitCommit, "abc123def456789012345678901234567890abcd")
	}
	if entry.Namespace != "tools@1.2.3" {
		t.Errorf("Namespace = %q, want %q", entry.Namespace, "tools@1.2.3")
	}
}

func TestAddAppendsToExistingLockFile(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), LockFileName)
	existing := invowkmod.NewLockFile()
	existing.Modules["https://github.com/user/utils.git"] = LockedModule{
		GitURL:          "https://github.com/user/utils.git",
		Version:         "^2.0.0",
		ResolvedVersion: "2.1.0",
		GitCommit:       "eee78901234567890123456789abcdef12345678",
		Namespace:       "utils@2.1.0",
		CommandSourceID: "utils",
		ModuleID:        "io.example.utils",
		ContentHash:     testContentHash(),
	}
	if err := existing.Save(lockPath); err != nil {
		t.Fatalf("failed to save existing lock file: %v", err)
	}

	resolved := testResolvedModule("https://github.com/user/tools.git", "mytools")
	resolved.ModuleRef.Version = "^1.0.0"
	resolved.ModuleRef.Alias = "mytools"
	resolved.ModuleRef.Path = "packages/core"
	resolved.ResolvedVersion = "1.5.0"
	resolved.GitCommit = "aaa456bbb789ccc012ddd345eee678fff901abc2"

	saveResolvedModuleToLock(t, lockPath, resolved)
	reloaded := loadLockFileForTest(t, lockPath)

	if len(reloaded.Modules) != 2 {
		t.Fatalf("lock file has %d modules, want 2", len(reloaded.Modules))
	}

	utilsKey := ModuleRefKey("https://github.com/user/utils.git")
	utils, ok := reloaded.Modules[utilsKey]
	if !ok {
		t.Fatal("existing utils entry should be preserved")
	}
	if utils.ResolvedVersion != "2.1.0" {
		t.Errorf("utils ResolvedVersion = %q, want %q", utils.ResolvedVersion, "2.1.0")
	}

	toolsKey := ModuleRefKey("https://github.com/user/tools.git#packages/core")
	tools, ok := reloaded.Modules[toolsKey]
	if !ok {
		t.Fatalf("new tools entry missing, expected key %q", toolsKey)
	}
	if tools.Alias != "mytools" {
		t.Errorf("Alias = %q, want %q", tools.Alias, "mytools")
	}
	if tools.Path != "packages/core" {
		t.Errorf("Path = %q, want %q", tools.Path, "packages/core")
	}
	if tools.Namespace != "mytools" {
		t.Errorf("Namespace = %q, want %q", tools.Namespace, "mytools")
	}
}

func testContentHash() ContentHash {
	return ContentHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
}

func testResolvedModule(gitURL GitURL, namespace ModuleNamespace) *ResolvedModule {
	commandSourceID, _, _ := strings.Cut(string(namespace), "@")
	return &ResolvedModule{
		ModuleRef: ModuleRef{
			GitURL:  gitURL,
			Version: "^1.0.0",
		},
		ResolvedVersion: "1.2.3",
		GitCommit:       "abc123def456789012345678901234567890abcd",
		Namespace:       namespace,
		CommandSourceID: invowkmod.ModuleSourceID(commandSourceID),
		ModuleName:      "tools",
		ModuleID:        "io.example.tools",
		ContentHash:     testContentHash(),
	}
}

func saveResolvedModuleToLock(t *testing.T, lockPath string, resolved *ResolvedModule) {
	t.Helper()

	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("invowkmod.LoadLockFile() error = %v", err)
	}
	lock.AddModule(resolved)
	if saveErr := lock.Save(lockPath); saveErr != nil {
		t.Fatalf("Save() error = %v", saveErr)
	}
}

func loadLockFileForTest(t *testing.T, lockPath string) *invowkmod.LockFile {
	t.Helper()

	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("invowkmod.LoadLockFile() after save error = %v", err)
	}
	return lock
}
