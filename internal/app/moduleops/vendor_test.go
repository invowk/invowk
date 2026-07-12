// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type fakeVendorDependencyResolver struct {
	syncResult []*invowkmod.ResolvedModule
	lockResult []*invowkmod.ResolvedModule
	syncCalls  int
	lockCalls  int
}

func (r *fakeVendorDependencyResolver) Sync(context.Context, []invowkmod.ModuleRef) ([]*invowkmod.ResolvedModule, error) {
	r.syncCalls++
	return r.syncResult, nil
}

func (r *fakeVendorDependencyResolver) LoadDeclaredFromLock(context.Context, []invowkmod.ModuleRef) ([]*invowkmod.ResolvedModule, error) {
	r.lockCalls++
	return r.lockResult, nil
}

// ============================================================================
// Tests for Vendored Modules
// ============================================================================

func TestVendoredModulesDir(t *testing.T) {
	t.Parallel()

	if VendoredModulesDir != "invowk_modules" {
		t.Errorf("VendoredModulesDir = %q, want %q", VendoredModulesDir, "invowk_modules")
	}
}

func TestGetVendoredModulesDir(t *testing.T) {
	t.Parallel()

	modulePath := types.FilesystemPath("/path/to/mymodule.invowkmod")
	expected := types.FilesystemPath(filepath.Join(string(modulePath), "invowk_modules"))
	result := GetVendoredModulesDir(modulePath)
	if result != expected {
		t.Errorf("GetVendoredModulesDir() = %q, want %q", result, expected)
	}
}

func TestHasVendoredModules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		createDir    bool
		moduleNames  []string
		wantVendored bool
	}{
		{name: "no vendored modules directory"},
		{name: "empty vendored modules directory", createDir: true},
		{name: "with vendored modules", createDir: true, moduleNames: []string{"vendor"}, wantVendored: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			modulePath := createValidModuleForPackaging(t, t.TempDir(), "mymodule.invowkmod", "mymodule")
			if tt.createDir {
				vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
				if err := os.Mkdir(vendoredDir, 0o755); err != nil {
					t.Fatal(err)
				}
				for _, name := range tt.moduleNames {
					createValidModuleForPackaging(t, vendoredDir, name+".invowkmod", name)
				}
			}
			if got := HasVendoredModules(types.FilesystemPath(modulePath)); got != tt.wantVendored {
				t.Errorf("HasVendoredModules() = %v, want %v", got, tt.wantVendored)
			}
		})
	}
}

func TestListVendoredModules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		moduleNames  []string
		invalidNames []string
		wantNames    []invowkmod.ModuleID
	}{
		{name: "no vendored modules"},
		{name: "with vendored modules", moduleNames: []string{"vendor1", "vendor2"}, wantNames: []invowkmod.ModuleID{"vendor1", "vendor2"}},
		{name: "skips invalid modules", moduleNames: []string{"valid"}, invalidNames: []string{"invalid"}, wantNames: []invowkmod.ModuleID{"valid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			modulePath := filepath.Join(t.TempDir(), "mymodule.invowkmod")
			if err := os.Mkdir(modulePath, 0o755); err != nil {
				t.Fatal(err)
			}
			if len(tt.moduleNames) > 0 || len(tt.invalidNames) > 0 {
				vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
				if err := os.Mkdir(vendoredDir, 0o755); err != nil {
					t.Fatal(err)
				}
				for _, name := range tt.moduleNames {
					createValidModuleForPackaging(t, vendoredDir, name+".invowkmod", name)
				}
				for _, name := range tt.invalidNames {
					if err := os.Mkdir(filepath.Join(vendoredDir, name+".invowkmod"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
			}
			modules, err := ListVendoredModules(types.FilesystemPath(modulePath))
			if err != nil {
				t.Fatalf("ListVendoredModules() error: %v", err)
			}
			if len(modules) != len(tt.wantNames) {
				t.Fatalf("ListVendoredModules() returned %d modules, want %d", len(modules), len(tt.wantNames))
			}
			names := make(map[invowkmod.ModuleID]bool, len(modules))
			for _, module := range modules {
				names[module.Name()] = true
			}
			for _, want := range tt.wantNames {
				if !names[want] {
					t.Errorf("ListVendoredModules() missing %q, got %v", want, names)
				}
			}
		})
	}
}

// ============================================================================
// Tests for Validate with Nested/Vendored Modules
// ============================================================================

func TestValidate_AllowsNestedModulesInVendoredDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invowkmod", "mycommands")

	// Create invowk_modules directory with a nested module
	vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
	if err := os.Mkdir(vendoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createValidModuleForPackaging(t, vendoredDir, "vendored.invowkmod", "vendored")

	result, err := invowkmod.Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Validate() should return valid for module with nested modules in invowk_modules/. Issues: %v", result.Issues)
	}
}

func TestValidate_StillRejectsNestedModulesOutsideVendoredDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invowkmod", "mycommands")

	// Create a nested module NOT in invowk_modules
	nestedModule := filepath.Join(modulePath, "nested.invowkmod")
	if err := os.Mkdir(nestedModule, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := invowkmod.Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if result.Valid {
		t.Error("Validate() should reject nested modules outside of invowk_modules/")
	}

	// Check that the issue mentions nested module
	foundNestedIssue := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "nested") {
			foundNestedIssue = true
			break
		}
	}
	if !foundNestedIssue {
		t.Error("Validate() should report issue about nested module")
	}
}

func TestValidate_DetectsSymlinks(t *testing.T) {
	t.Parallel()

	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invowkmod", "mycommands")

	// Create a file outside the module
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the module pointing outside
	symlinkPath := filepath.Join(modulePath, "link_to_outside")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := invowkmod.Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should report a security issue about the symlink
	foundSymlinkIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "security" && strings.Contains(strings.ToLower(issue.Message), "symlink") {
			foundSymlinkIssue = true
			break
		}
	}
	if !foundSymlinkIssue {
		t.Error("Validate() should report security issue about symlink pointing outside module")
	}
}

func TestValidate_DetectsWindowsReservedFilenames(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invowkmod", "mycommands")

	// Create a file with a Windows reserved name
	reservedFile := filepath.Join(modulePath, "CON")
	if err := os.WriteFile(reservedFile, []byte("test"), 0o644); err != nil {
		// On Windows, this might fail - that's expected
		if runtime.GOOS == "windows" {
			t.Skip("Cannot create reserved filename on Windows")
		}
		t.Fatal(err)
	}

	result, err := invowkmod.Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should report a compatibility issue about the reserved filename
	foundReservedIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "compatibility" && strings.Contains(issue.Message, "reserved on Windows") {
			foundReservedIssue = true
			break
		}
	}
	if !foundReservedIssue {
		t.Error("Validate() should report compatibility issue about Windows reserved filename")
	}
}

func TestValidate_RejectsAllSymlinks(t *testing.T) {
	t.Parallel()

	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invowkmod", "mycommands")

	// Create scripts directory
	scriptsDir := filepath.Join(modulePath, "scripts")
	if err := os.Mkdir(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file inside the module
	internalFile := filepath.Join(scriptsDir, "original.sh")
	if err := os.WriteFile(internalFile, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the module pointing to another file inside the module
	symlinkPath := filepath.Join(modulePath, "link_to_internal")
	if err := os.Symlink(internalFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := invowkmod.Validate(types.FilesystemPath(modulePath))
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// ALL symlinks should be rejected as a security measure (even internal ones)
	// This is intentional to prevent zip slip attacks during archive extraction
	foundSecurityIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "security" && strings.Contains(strings.ToLower(issue.Message), "symlink") {
			foundSecurityIssue = true
			break
		}
	}
	if !foundSecurityIssue {
		t.Error("Validate() should report security issue for ALL symlinks (including internal ones)")
	}
}

// ============================================================================
// Tests for VendorModules
// ============================================================================

// createCacheModule sets up a synthetic cache directory containing a .invowkmod module,
// simulating what the Resolver produces after fetching and caching a Git dependency.
// Returns the cache directory path (to use as invowkmod.ResolvedModule.CachePath).
func createCacheModule(t *testing.T, parentDir, folderName, moduleID string) string {
	t.Helper()
	cacheDir := filepath.Join(parentDir, "cache-"+moduleID)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createValidModuleForPackaging(t, cacheDir, folderName, moduleID)
	return cacheDir
}

func TestVendorModules_CopiesFromCache(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	// Create two synthetic cache entries
	cache1 := createCacheModule(t, tmpDir, "dep1.invowkmod", "dep1")
	cache2 := createCacheModule(t, tmpDir, "dep2.invowkmod", "dep2")

	result, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules: []*invowkmod.ResolvedModule{
			{CachePath: types.FilesystemPath(cache1), Namespace: "dep1@1.0.0"},
			{CachePath: types.FilesystemPath(cache2), Namespace: "dep2@2.0.0"},
		},
	})
	if err != nil {
		t.Fatalf("VendorModules() error: %v", err)
	}

	if len(result.Vendored) != 2 {
		t.Fatalf("VendorModules() vendored %d modules, want 2", len(result.Vendored))
	}

	// Verify modules exist in invowk_modules/ with correct fields
	entryByNamespace := make(map[invowkmod.ModuleNamespace]VendoredEntry)
	for _, entry := range result.Vendored {
		entryByNamespace[entry.Namespace] = entry
		if _, err := os.Stat(string(entry.VendorPath)); err != nil {
			t.Errorf("vendored module not found at %s: %v", entry.VendorPath, err)
		}
		// Verify invowkmod.cue was copied
		invowkmodPath := filepath.Join(string(entry.VendorPath), "invowkmod.cue")
		if _, err := os.Stat(invowkmodPath); err != nil {
			t.Errorf("invowkmod.cue not found in vendored module: %v", err)
		}
	}

	// Verify Namespace fields match what was passed in invowkmod.ResolvedModule
	for _, ns := range []invowkmod.ModuleNamespace{"dep1@1.0.0", "dep2@2.0.0"} {
		if _, ok := entryByNamespace[ns]; !ok {
			t.Errorf("expected vendored entry with Namespace %q, not found", ns)
		}
	}

	// Verify SourcePath points to the actual .invowkmod directory inside the cache
	dep1Entry := entryByNamespace["dep1@1.0.0"]
	expectedDep1Source := types.FilesystemPath(filepath.Join(cache1, "dep1.invowkmod"))
	if dep1Entry.SourcePath != expectedDep1Source {
		t.Errorf("dep1 SourcePath = %q, want %q", dep1Entry.SourcePath, expectedDep1Source)
	}
	dep2Entry := entryByNamespace["dep2@2.0.0"]
	expectedDep2Source := types.FilesystemPath(filepath.Join(cache2, "dep2.invowkmod"))
	if dep2Entry.SourcePath != expectedDep2Source {
		t.Errorf("dep2 SourcePath = %q, want %q", dep2Entry.SourcePath, expectedDep2Source)
	}

	// Verify vendor dir is correct
	expectedVendorDir := types.FilesystemPath(filepath.Join(modulePath, VendoredModulesDir))
	if result.VendorDir != expectedVendorDir {
		t.Errorf("VendorDir = %q, want %q", result.VendorDir, expectedVendorDir)
	}
}

func TestVendorModules_UsesCanonicalDestinationFromModuleID(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cacheDir := filepath.Join(tmpDir, "cache", "github.com", "user", "tools", "1.2.3")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}
	sourcePath := createValidModuleForPackaging(t, cacheDir, "tools.invowkmod", "io.example.tools")
	hash, err := invowkmod.ComputeModuleHash(sourcePath)
	if err != nil {
		t.Fatalf("ComputeModuleHash() error = %v", err)
	}

	result, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules: []*invowkmod.ResolvedModule{{
			CachePath:   types.FilesystemPath(cacheDir),
			Namespace:   "io.example.tools@1.2.3",
			ModuleID:    "io.example.tools",
			ContentHash: hash,
			ModuleRef:   invowkmod.ModuleRef{GitURL: "https://github.com/user/tools.git", Version: "^1.0.0"},
		}},
	})
	if err != nil {
		t.Fatalf("VendorModules() error: %v", err)
	}
	if len(result.Vendored) != 1 {
		t.Fatalf("VendorModules() vendored %d modules, want 1", len(result.Vendored))
	}
	if got := filepath.Base(string(result.Vendored[0].VendorPath)); got != "io.example.tools.invowkmod" {
		t.Fatalf("vendored basename = %q, want io.example.tools.invowkmod", got)
	}
	if result.Vendored[0].SourcePath != types.FilesystemPath(sourcePath) {
		t.Fatalf("SourcePath = %q, want %q", result.Vendored[0].SourcePath, sourcePath)
	}
}

func TestVendorModules_PrunesRepositoryDerivedDestination(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cacheDir := filepath.Join(tmpDir, "cache", "github.com", "user", "tools", "1.2.3")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}
	createValidModuleForPackaging(t, cacheDir, "tools.invowkmod", "io.example.tools")
	vendorDir := filepath.Join(modulePath, VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(vendorDir) error = %v", err)
	}
	createValidModuleForPackaging(t, vendorDir, "tools.invowkmod", "io.example.tools")

	result, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules: []*invowkmod.ResolvedModule{{
			CachePath: types.FilesystemPath(cacheDir),
			Namespace: "io.example.tools@1.2.3",
			ModuleID:  "io.example.tools",
		}},
		Prune: true,
	})
	if err != nil {
		t.Fatalf("VendorModules() error: %v", err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0] != "tools.invowkmod" {
		t.Fatalf("Pruned = %v, want [tools.invowkmod]", result.Pruned)
	}
	if _, err := os.Stat(filepath.Join(vendorDir, "tools.invowkmod")); !os.IsNotExist(err) {
		t.Fatalf("repository-derived destination still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vendorDir, "io.example.tools.invowkmod")); err != nil {
		t.Fatalf("canonical destination missing: %v", err)
	}
}

func TestVendorDependenciesIgnoresStaleLockEntriesWhenPruning(t *testing.T) {
	tmpDir := t.TempDir()
	cacheRoot := filepath.Join(tmpDir, "module-cache")
	t.Setenv("INVOWK_MODULES_PATH", cacheRoot)

	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	invowkmodContent := `module: "parent"
version: "1.0.0"
requires: [
	{
		git_url: "https://github.com/example/dep.git"
		version: "^1.0.0"
	},
]
`
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue): %v", err)
	}

	depCacheDir := filepath.Join(cacheRoot, "github.com", "example", "dep", "1.0.0")
	if mkdirErr := os.MkdirAll(depCacheDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll(dep cache): %v", mkdirErr)
	}
	depModuleDir := createValidModuleForPackaging(t, depCacheDir, "dep.invowkmod", "dep")
	depHash, err := invowkmod.ComputeModuleHash(depModuleDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash(dep): %v", err)
	}

	staleCacheDir := filepath.Join(cacheRoot, "github.com", "example", "stale", "1.0.0")
	if mkdirErr := os.MkdirAll(staleCacheDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll(stale cache): %v", mkdirErr)
	}
	staleModuleDir := createValidModuleForPackaging(t, staleCacheDir, "stale.invowkmod", "stale")
	staleHash, err := invowkmod.ComputeModuleHash(staleModuleDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash(stale): %v", err)
	}

	lock := invowkmod.NewLockFile()
	lock.Modules["https://github.com/example/dep.git"] = invowkmod.LockedModule{
		GitURL:          "https://github.com/example/dep.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "abc123def456789012345678901234567890abcd",
		Namespace:       "dep@1.0.0",
		CommandSourceID: "dep",
		ModuleID:        "dep",
		ContentHash:     depHash,
	}
	lock.Modules["https://github.com/example/stale.git"] = invowkmod.LockedModule{
		GitURL:          "https://github.com/example/stale.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "def456789012345678901234567890abcdef1234",
		Namespace:       "stale@1.0.0",
		CommandSourceID: "stale",
		ModuleID:        "stale",
		ContentHash:     staleHash,
	}
	if saveErr := lock.Save(filepath.Join(modulePath, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("Save(lock): %v", saveErr)
	}

	vendorDir := filepath.Join(modulePath, VendoredModulesDir)
	if mkdirErr := os.MkdirAll(vendorDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll(vendor dir): %v", mkdirErr)
	}
	createValidModuleForPackaging(t, vendorDir, "stale.invowkmod", "stale")

	requirements, result, strategy, err := VendorDependencies(t.Context(), types.FilesystemPath(modulePath), false, true)
	if err != nil {
		t.Fatalf("VendorDependencies() error = %v", err)
	}
	if strategy != VendorResolutionLocked {
		t.Fatalf("strategy = %s, want %s", strategy, VendorResolutionLocked)
	}
	if len(requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(requirements))
	}
	if len(result.Vendored) != 1 {
		t.Fatalf("vendored = %d, want 1", len(result.Vendored))
	}
	if got := filepath.Base(string(result.Vendored[0].VendorPath)); got != "dep.invowkmod" {
		t.Fatalf("vendored directory = %q, want dep.invowkmod", got)
	}
	if len(result.Pruned) != 1 || filepath.Base(result.Pruned[0]) != "stale.invowkmod" {
		t.Fatalf("pruned = %v, want stale.invowkmod", result.Pruned)
	}
	if _, err := os.Stat(filepath.Join(vendorDir, "stale.invowkmod")); !os.IsNotExist(err) {
		t.Fatalf("stale vendored module still exists: %v", err)
	}
}

func TestResolveVendorDependenciesUsesUpdateStrategy(t *testing.T) {
	t.Parallel()

	modulePath := types.FilesystemPath(t.TempDir())
	requirements := []invowkmod.ModuleRef{{GitURL: "https://example.com/dep.git", Version: "^1.0.0"}}
	if writeErr := os.WriteFile(filepath.Join(string(modulePath), invowkmod.LockFileName), []byte("stale lock"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	resolver := &fakeVendorDependencyResolver{
		syncResult: []*invowkmod.ResolvedModule{{ModuleRef: requirements[0]}},
		lockResult: []*invowkmod.ResolvedModule{
			{ModuleRef: invowkmod.ModuleRef{GitURL: "https://example.com/locked.git", Version: "^1.0.0"}},
		},
	}

	resolved, strategy, err := resolveVendorDependencies(t.Context(), resolver, modulePath, requirements, true)
	if err != nil {
		t.Fatalf("resolveVendorDependencies(update) error = %v", err)
	}
	if strategy != VendorResolutionUpdated {
		t.Fatalf("strategy = %s, want %s", strategy, VendorResolutionUpdated)
	}
	if resolver.syncCalls != 1 || resolver.lockCalls != 0 {
		t.Fatalf("calls = sync:%d lock:%d, want sync:1 lock:0", resolver.syncCalls, resolver.lockCalls)
	}
	if len(resolved) != 1 || resolved[0].ModuleRef.GitURL != requirements[0].GitURL {
		t.Fatalf("resolved = %#v, want sync result", resolved)
	}
}

func TestResolveVendorDependenciesSyncsWhenLockMissing(t *testing.T) {
	t.Parallel()

	modulePath := types.FilesystemPath(t.TempDir())
	requirements := []invowkmod.ModuleRef{{GitURL: "https://example.com/dep.git", Version: "^1.0.0"}}
	resolver := &fakeVendorDependencyResolver{
		syncResult: []*invowkmod.ResolvedModule{{ModuleRef: requirements[0]}},
	}

	_, strategy, err := resolveVendorDependencies(t.Context(), resolver, modulePath, requirements, false)
	if err != nil {
		t.Fatalf("resolveVendorDependencies(no lock) error = %v", err)
	}
	if strategy != VendorResolutionSynced {
		t.Fatalf("strategy = %s, want %s", strategy, VendorResolutionSynced)
	}
	if resolver.syncCalls != 1 || resolver.lockCalls != 0 {
		t.Fatalf("calls = sync:%d lock:%d, want sync:1 lock:0", resolver.syncCalls, resolver.lockCalls)
	}
}

func TestResolveVendorDependenciesRejectsInvalidLock(t *testing.T) {
	t.Parallel()

	modulePath := types.FilesystemPath(t.TempDir())
	requirements := []invowkmod.ModuleRef{{GitURL: "https://example.com/dep.git", Version: "^1.0.0"}}
	if writeErr := os.WriteFile(filepath.Join(string(modulePath), invowkmod.LockFileName), []byte("not: valid: lock:"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	resolver := &fakeVendorDependencyResolver{
		syncResult: []*invowkmod.ResolvedModule{{ModuleRef: requirements[0]}},
	}

	_, strategy, err := resolveVendorDependencies(t.Context(), resolver, modulePath, requirements, false)
	if err == nil {
		t.Fatal("resolveVendorDependencies(invalid lock) error = nil, want error")
	}
	if strategy != "" {
		t.Fatalf("strategy = %s, want empty", strategy)
	}
	if resolver.syncCalls != 0 || resolver.lockCalls != 0 {
		t.Fatalf("calls = sync:%d lock:%d, want sync:0 lock:0", resolver.syncCalls, resolver.lockCalls)
	}
}

func TestVendorModules_OverwritesExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cache1 := createCacheModule(t, tmpDir, "dep1.invowkmod", "dep1")

	// Vendor once
	_, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules:    []*invowkmod.ResolvedModule{{CachePath: types.FilesystemPath(cache1), Namespace: "dep1@1.0.0"}},
	})
	if err != nil {
		t.Fatalf("first VendorModules() error: %v", err)
	}

	// Add a marker file to the vendored copy
	vendorDir := GetVendoredModulesDir(types.FilesystemPath(modulePath))
	markerPath := filepath.Join(string(vendorDir), "dep1.invowkmod", "stale-marker.txt")
	if writeErr := os.WriteFile(markerPath, []byte("stale"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	// Vendor again — should overwrite, removing the stale marker
	_, err = VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules:    []*invowkmod.ResolvedModule{{CachePath: types.FilesystemPath(cache1), Namespace: "dep1@1.0.0"}},
	})
	if err != nil {
		t.Fatalf("second VendorModules() error: %v", err)
	}

	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Error("VendorModules() should have overwritten existing vendored module, but stale marker still exists")
	}
}

func TestVendorModules_Prune(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cache1 := createCacheModule(t, tmpDir, "dep1.invowkmod", "dep1")
	cache2 := createCacheModule(t, tmpDir, "dep2.invowkmod", "dep2")

	// Vendor both modules initially
	_, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules: []*invowkmod.ResolvedModule{
			{CachePath: types.FilesystemPath(cache1), Namespace: "dep1@1.0.0"},
			{CachePath: types.FilesystemPath(cache2), Namespace: "dep2@2.0.0"},
		},
	})
	if err != nil {
		t.Fatalf("initial VendorModules() error: %v", err)
	}

	// Now vendor only dep1 with prune — dep2 should be removed
	result, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules:    []*invowkmod.ResolvedModule{{CachePath: types.FilesystemPath(cache1), Namespace: "dep1@1.0.0"}},
		Prune:      true,
	})
	if err != nil {
		t.Fatalf("prune VendorModules() error: %v", err)
	}

	if len(result.Pruned) != 1 {
		t.Fatalf("VendorModules() pruned %d, want 1", len(result.Pruned))
	}
	if result.Pruned[0] != "dep2.invowkmod" {
		t.Errorf("pruned %q, want %q", result.Pruned[0], "dep2.invowkmod")
	}

	// Verify dep2 is gone
	dep2Path := filepath.Join(string(GetVendoredModulesDir(types.FilesystemPath(modulePath))), "dep2.invowkmod")
	if _, err := os.Stat(dep2Path); !os.IsNotExist(err) {
		t.Error("dep2.invowkmod should have been pruned")
	}

	// Verify dep1 still exists
	dep1Path := filepath.Join(string(GetVendoredModulesDir(types.FilesystemPath(modulePath))), "dep1.invowkmod")
	if _, err := os.Stat(dep1Path); err != nil {
		t.Errorf("dep1.invowkmod should still exist: %v", err)
	}
}

func TestVendorModules_EmptyModulesList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	result, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules:    []*invowkmod.ResolvedModule{},
	})
	if err != nil {
		t.Fatalf("VendorModules() error: %v", err)
	}

	if len(result.Vendored) != 0 {
		t.Errorf("VendorModules() vendored %d modules, want 0", len(result.Vendored))
	}

	// Vendor dir should still be created
	if _, err := os.Stat(string(result.VendorDir)); err != nil {
		t.Errorf("vendor directory should exist: %v", err)
	}
}

func TestVendorModules_InvalidCachePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	_, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules: []*invowkmod.ResolvedModule{
			{CachePath: types.FilesystemPath(filepath.Join(tmpDir, "nonexistent-cache")), Namespace: "bad@1.0.0"},
		},
	})
	if err == nil {
		t.Fatal("VendorModules() should error with nonexistent cache path")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error should wrap os.ErrNotExist, got: %v", err)
	}
}

// ============================================================================
// Tests for pruneVendorDir (direct unit tests)
// ============================================================================
