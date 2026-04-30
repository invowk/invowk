// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestNewResolverForInvowkmodPathUsesMetadataDirectory(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	moduleDir := filepath.Join(rootDir, "nested", "module")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(moduleDir) error = %v", err)
	}

	resolver, err := newResolverForInvowkmodPath(types.FilesystemPath(filepath.Join(moduleDir, "invowkmod.cue")))
	if err != nil {
		t.Fatalf("newResolverForInvowkmodPath() error = %v", err)
	}

	want, err := filepath.Abs(moduleDir)
	if err != nil {
		t.Fatalf("Abs(moduleDir) error = %v", err)
	}
	if resolver.workingDir != types.FilesystemPath(want) {
		t.Fatalf("workingDir = %q, want %q", resolver.workingDir, want)
	}
}

func TestAddModuleDependencyReportsDeclarationFailureAfterLockUpdate(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cacheDir := t.TempDir()
	repoDir := t.TempDir()
	writeTestModule(t, repoDir, "tools.invowkmod", `module: "io.example.tools"
version: "1.2.3"`)

	resolver, err := newResolverWithFetcher(
		types.FilesystemPath(workDir),
		types.FilesystemPath(cacheDir),
		&fakeModuleFetcher{repoPath: types.FilesystemPath(repoDir), listVersions: []SemVer{"1.2.3"}},
	)
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}

	result, err := resolver.AddModuleDependency(t.Context(), types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")), ModuleRef{
		GitURL:  "https://github.com/user/tools.git",
		Version: "^1.0.0",
	})
	if err != nil {
		t.Fatalf("AddModuleDependency() error = %v", err)
	}
	if result.Resolved() == nil {
		t.Fatal("Resolved = nil, want resolved module")
	}
	if result.Declaration().Updated() {
		t.Fatal("Declaration.Updated = true, want false")
	}
	if result.Declaration().Err() == nil {
		t.Fatal("Declaration.Err = nil, want declaration edit error")
	}

	lock, err := invowkmod.LoadLockFile(filepath.Join(workDir, LockFileName))
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	if len(lock.Modules) != 1 {
		t.Fatalf("lock modules = %d, want 1", len(lock.Modules))
	}
}

func TestRemoveModuleDependencyReportsDeclarationFailureAfterLockUpdate(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cacheDir := t.TempDir()
	lock := invowkmod.NewLockFile()
	lock.Modules["https://github.com/user/tools.git"] = LockedModule{
		GitURL:          "https://github.com/user/tools.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.2.3",
		GitCommit:       "abc123def456789012345678901234567890abcd",
		Namespace:       "tools@1.2.3",
		ContentHash:     ContentHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
	}
	if err := lock.Save(filepath.Join(workDir, LockFileName)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	resolver, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir), nil)
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	if mkdirErr := os.MkdirAll(invowkmodPath, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll(invowkmod path) error = %v", mkdirErr)
	}
	result, err := resolver.RemoveModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), "tools")
	if err != nil {
		t.Fatalf("RemoveModuleDependency() error = %v", err)
	}
	removed := result.Removed()
	declarations := result.Declarations()
	if len(removed) != 1 {
		t.Fatalf("Removed = %d, want 1", len(removed))
	}
	if len(declarations) != 1 {
		t.Fatalf("Declarations = %d, want 1", len(declarations))
	}
	if declarations[0].Updated() {
		t.Fatal("Declaration.Updated = true, want false")
	}
	if declarations[0].Err() == nil {
		t.Fatal("Declaration.Err = nil, want declaration edit error")
	}

	reloaded, err := invowkmod.LoadLockFile(filepath.Join(workDir, LockFileName))
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	if len(reloaded.Modules) != 0 {
		t.Fatalf("lock modules = %d, want 0", len(reloaded.Modules))
	}
}

func writeTestModule(t *testing.T, repoDir, name, invowkmodContent string) {
	t.Helper()

	moduleDir := filepath.Join(repoDir, name)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
}
