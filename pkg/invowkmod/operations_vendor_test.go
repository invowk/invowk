// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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

	modulePath := "/path/to/mymodule.invowkmod"
	expected := filepath.Join(modulePath, "invowk_modules")
	result := GetVendoredModulesDir(modulePath)
	if result != expected {
		t.Errorf("GetVendoredModulesDir() = %q, want %q", result, expected)
	}
}

func TestHasVendoredModules(t *testing.T) {
	t.Parallel()

	t.Run("no vendored modules directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		modulePath := createValidModuleForPackaging(t, tmpDir, "mymodule.invowkmod", "mymodule")

		if HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return false when invowk_modules/ doesn't exist")
		}
	})

	t.Run("empty vendored modules directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		modulePath := createValidModuleForPackaging(t, tmpDir, "mymodule.invowkmod", "mymodule")
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return false when invowk_modules/ is empty")
		}
	})

	t.Run("with vendored modules", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		modulePath := createValidModuleForPackaging(t, tmpDir, "mymodule.invowkmod", "mymodule")
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Create a vendored module using new format
		createValidModuleForPackaging(t, vendoredDir, "vendor.invowkmod", "vendor")

		if !HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return true when invowk_modules/ has modules")
		}
	})
}

func TestListVendoredModules(t *testing.T) {
	t.Parallel()

	t.Run("no vendored modules", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invowkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 0 {
			t.Errorf("ListVendoredModules() returned %d modules, want 0", len(modules))
		}
	})

	t.Run("with vendored modules", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invowkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create two vendored modules using new format
		createValidModuleForPackaging(t, vendoredDir, "vendor1.invowkmod", "vendor1")
		createValidModuleForPackaging(t, vendoredDir, "vendor2.invowkmod", "vendor2")

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 2 {
			t.Errorf("ListVendoredModules() returned %d modules, want 2", len(modules))
		}

		// Check module names
		names := make(map[ModuleID]bool)
		for _, p := range modules {
			names[p.Name()] = true
		}
		if !names[ModuleID("vendor1")] || !names[ModuleID("vendor2")] {
			t.Errorf("ListVendoredModules() missing expected modules, got: %v", names)
		}
	})

	t.Run("skips invalid modules", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invowkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a valid module using new format
		createValidModuleForPackaging(t, vendoredDir, "valid.invowkmod", "valid")

		// Create an invalid module (no invowkmod.cue)
		invalidModule := filepath.Join(vendoredDir, "invalid.invowkmod")
		if err := os.Mkdir(invalidModule, 0o755); err != nil {
			t.Fatal(err)
		}

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 1 {
			t.Errorf("ListVendoredModules() returned %d modules, want 1 (should skip invalid)", len(modules))
		}
		if len(modules) > 0 && modules[0].Name() != "valid" {
			t.Errorf("ListVendoredModules() returned wrong module: %s", modules[0].Name())
		}
	})
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

	result, err := Validate(modulePath)
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

	result, err := Validate(modulePath)
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

	result, err := Validate(modulePath)
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

	result, err := Validate(modulePath)
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

	result, err := Validate(modulePath)
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
// Returns the cache directory path (to use as ResolvedModule.CachePath).
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
		ModulePath: modulePath,
		Modules: []*ResolvedModule{
			{CachePath: cache1, Namespace: "dep1@1.0.0"},
			{CachePath: cache2, Namespace: "dep2@2.0.0"},
		},
	})
	if err != nil {
		t.Fatalf("VendorModules() error: %v", err)
	}

	if len(result.Vendored) != 2 {
		t.Fatalf("VendorModules() vendored %d modules, want 2", len(result.Vendored))
	}

	// Verify modules exist in invowk_modules/ with correct fields
	entryByNamespace := make(map[string]VendoredEntry)
	for _, entry := range result.Vendored {
		entryByNamespace[entry.Namespace] = entry
		if _, err := os.Stat(entry.VendorPath); err != nil {
			t.Errorf("vendored module not found at %s: %v", entry.VendorPath, err)
		}
		// Verify invowkmod.cue was copied
		invowkmodPath := filepath.Join(entry.VendorPath, "invowkmod.cue")
		if _, err := os.Stat(invowkmodPath); err != nil {
			t.Errorf("invowkmod.cue not found in vendored module: %v", err)
		}
	}

	// Verify Namespace fields match what was passed in ResolvedModule
	for _, ns := range []string{"dep1@1.0.0", "dep2@2.0.0"} {
		if _, ok := entryByNamespace[ns]; !ok {
			t.Errorf("expected vendored entry with Namespace %q, not found", ns)
		}
	}

	// Verify SourcePath points to the actual .invowkmod directory inside the cache
	dep1Entry := entryByNamespace["dep1@1.0.0"]
	expectedDep1Source := filepath.Join(cache1, "dep1.invowkmod")
	if dep1Entry.SourcePath != expectedDep1Source {
		t.Errorf("dep1 SourcePath = %q, want %q", dep1Entry.SourcePath, expectedDep1Source)
	}
	dep2Entry := entryByNamespace["dep2@2.0.0"]
	expectedDep2Source := filepath.Join(cache2, "dep2.invowkmod")
	if dep2Entry.SourcePath != expectedDep2Source {
		t.Errorf("dep2 SourcePath = %q, want %q", dep2Entry.SourcePath, expectedDep2Source)
	}

	// Verify vendor dir is correct
	expectedVendorDir := filepath.Join(modulePath, VendoredModulesDir)
	if result.VendorDir != expectedVendorDir {
		t.Errorf("VendorDir = %q, want %q", result.VendorDir, expectedVendorDir)
	}
}

func TestVendorModules_OverwritesExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cache1 := createCacheModule(t, tmpDir, "dep1.invowkmod", "dep1")

	// Vendor once
	_, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    []*ResolvedModule{{CachePath: cache1, Namespace: "dep1@1.0.0"}},
	})
	if err != nil {
		t.Fatalf("first VendorModules() error: %v", err)
	}

	// Add a marker file to the vendored copy
	vendorDir := GetVendoredModulesDir(modulePath)
	markerPath := filepath.Join(vendorDir, "dep1.invowkmod", "stale-marker.txt")
	if writeErr := os.WriteFile(markerPath, []byte("stale"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	// Vendor again — should overwrite, removing the stale marker
	_, err = VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    []*ResolvedModule{{CachePath: cache1, Namespace: "dep1@1.0.0"}},
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
		ModulePath: modulePath,
		Modules: []*ResolvedModule{
			{CachePath: cache1, Namespace: "dep1@1.0.0"},
			{CachePath: cache2, Namespace: "dep2@2.0.0"},
		},
	})
	if err != nil {
		t.Fatalf("initial VendorModules() error: %v", err)
	}

	// Now vendor only dep1 with prune — dep2 should be removed
	result, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    []*ResolvedModule{{CachePath: cache1, Namespace: "dep1@1.0.0"}},
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
	dep2Path := filepath.Join(GetVendoredModulesDir(modulePath), "dep2.invowkmod")
	if _, err := os.Stat(dep2Path); !os.IsNotExist(err) {
		t.Error("dep2.invowkmod should have been pruned")
	}

	// Verify dep1 still exists
	dep1Path := filepath.Join(GetVendoredModulesDir(modulePath), "dep1.invowkmod")
	if _, err := os.Stat(dep1Path); err != nil {
		t.Errorf("dep1.invowkmod should still exist: %v", err)
	}
}

func TestVendorModules_EmptyModulesList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	result, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    []*ResolvedModule{},
	})
	if err != nil {
		t.Fatalf("VendorModules() error: %v", err)
	}

	if len(result.Vendored) != 0 {
		t.Errorf("VendorModules() vendored %d modules, want 0", len(result.Vendored))
	}

	// Vendor dir should still be created
	if _, err := os.Stat(result.VendorDir); err != nil {
		t.Errorf("vendor directory should exist: %v", err)
	}
}

func TestVendorModules_InvalidCachePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	_, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules: []*ResolvedModule{
			{CachePath: filepath.Join(tmpDir, "nonexistent-cache"), Namespace: "bad@1.0.0"},
		},
	})
	if err == nil {
		t.Fatal("VendorModules() should error with nonexistent cache path")
	}
	if !strings.Contains(err.Error(), "failed to locate module") {
		t.Errorf("error should mention module location failure, got: %v", err)
	}
}

// ============================================================================
// Tests for pruneVendorDir (direct unit tests)
// ============================================================================

func TestPruneVendorDir(t *testing.T) {
	t.Parallel()

	t.Run("preserves non-invowkmod entries", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		vendorDir := filepath.Join(tmpDir, VendoredModulesDir)
		if err := os.MkdirAll(vendorDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a stale .invowkmod directory (should be pruned).
		staleDir := filepath.Join(vendorDir, "stale.invowkmod")
		if err := os.Mkdir(staleDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a regular directory (not .invowkmod — should be preserved).
		toolsDir := filepath.Join(vendorDir, "tools")
		if err := os.Mkdir(toolsDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a regular file (should be preserved).
		readmePath := filepath.Join(vendorDir, "README.md")
		if err := os.WriteFile(readmePath, []byte("vendor readme"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Prune with empty expected set — only .invowkmod entries are candidates.
		pruned, err := pruneVendorDir(vendorDir, map[string]bool{})
		if err != nil {
			t.Fatalf("pruneVendorDir() error: %v", err)
		}

		if len(pruned) != 1 || pruned[0] != "stale.invowkmod" {
			t.Errorf("pruned = %v, want [stale.invowkmod]", pruned)
		}

		// Verify non-module entries survive.
		if _, err := os.Stat(toolsDir); err != nil {
			t.Errorf("tools/ directory should survive pruning: %v", err)
		}
		if _, err := os.Stat(readmePath); err != nil {
			t.Errorf("README.md should survive pruning: %v", err)
		}
	})

	t.Run("empty expected prunes all invowkmod entries", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		vendorDir := filepath.Join(tmpDir, VendoredModulesDir)
		if err := os.MkdirAll(vendorDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create two .invowkmod directories.
		for _, name := range []string{"dep1.invowkmod", "dep2.invowkmod"} {
			if err := os.Mkdir(filepath.Join(vendorDir, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		pruned, err := pruneVendorDir(vendorDir, map[string]bool{})
		if err != nil {
			t.Fatalf("pruneVendorDir() error: %v", err)
		}

		if len(pruned) != 2 {
			t.Errorf("pruned %d entries, want 2", len(pruned))
		}

		// Verify both are gone.
		for _, name := range []string{"dep1.invowkmod", "dep2.invowkmod"} {
			if _, err := os.Stat(filepath.Join(vendorDir, name)); !os.IsNotExist(err) {
				t.Errorf("%s should have been pruned", name)
			}
		}
	})

	t.Run("empty vendor dir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		vendorDir := filepath.Join(tmpDir, VendoredModulesDir)
		if err := os.MkdirAll(vendorDir, 0o755); err != nil {
			t.Fatal(err)
		}

		pruned, err := pruneVendorDir(vendorDir, map[string]bool{})
		if err != nil {
			t.Fatalf("pruneVendorDir() error: %v", err)
		}

		if len(pruned) != 0 {
			t.Errorf("pruned %d entries, want 0", len(pruned))
		}
	})
}

func TestVendorModules_SameBasenameFails(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	// Create two cache entries that both contain a module named "dep.invowkmod"
	// but with different module IDs (simulating two Git repos each shipping dep.invowkmod).
	cache1 := createCacheModule(t, tmpDir, "dep.invowkmod", "dep-alpha")
	cache2 := createCacheModule(t, tmpDir, "dep.invowkmod", "dep-beta")

	_, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules: []*ResolvedModule{
			{CachePath: cache1, Namespace: "dep-alpha@1.0.0"},
			{CachePath: cache2, Namespace: "dep-beta@2.0.0"},
		},
	})
	if err == nil {
		t.Fatal("VendorModules() should fail when two modules resolve to the same directory name")
	}
	if !strings.Contains(err.Error(), "vendor conflict") {
		t.Errorf("error should mention 'vendor conflict', got: %v", err)
	}
	if !strings.Contains(err.Error(), "dep.invowkmod") {
		t.Errorf("error should mention the conflicting directory name, got: %v", err)
	}
}

func TestVendorModules_PruneNoOp(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cache1 := createCacheModule(t, tmpDir, "dep1.invowkmod", "dep1")
	cache2 := createCacheModule(t, tmpDir, "dep2.invowkmod", "dep2")

	modules := []*ResolvedModule{
		{CachePath: cache1, Namespace: "dep1@1.0.0"},
		{CachePath: cache2, Namespace: "dep2@2.0.0"},
	}

	// Vendor both modules initially
	_, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    modules,
	})
	if err != nil {
		t.Fatalf("initial VendorModules() error: %v", err)
	}

	// Re-vendor the same set with prune — nothing should be pruned
	result, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    modules,
		Prune:      true,
	})
	if err != nil {
		t.Fatalf("prune VendorModules() error: %v", err)
	}

	if len(result.Pruned) != 0 {
		t.Errorf("VendorModules() pruned %d entries, want 0 (nothing stale)", len(result.Pruned))
	}

	// Verify both modules still exist on disk
	for _, name := range []string{"dep1.invowkmod", "dep2.invowkmod"} {
		modPath := filepath.Join(GetVendoredModulesDir(modulePath), name)
		if _, statErr := os.Stat(modPath); statErr != nil {
			t.Errorf("%s should still exist after prune no-op: %v", name, statErr)
		}
	}
}

func TestVendorModules_PruneNoVendorDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	// Prune with empty modules list on a fresh module (no vendor dir yet)
	result, err := VendorModules(VendorOptions{
		ModulePath: modulePath,
		Modules:    []*ResolvedModule{},
		Prune:      true,
	})
	if err != nil {
		t.Fatalf("VendorModules() with prune and no vendor dir should not error: %v", err)
	}

	if len(result.Pruned) != 0 {
		t.Errorf("VendorModules() pruned %d, want 0", len(result.Pruned))
	}
}
