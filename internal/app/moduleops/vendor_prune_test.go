// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestPruneVendorDirPreservesNonModuleEntries(t *testing.T) {
	t.Parallel()

	vendorDir := makeVendorDir(t)
	staleDir := filepath.Join(vendorDir, "stale.invowkmod")
	if err := os.Mkdir(staleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toolsDir := filepath.Join(vendorDir, "tools")
	if err := os.Mkdir(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	readmePath := filepath.Join(vendorDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("vendor readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	pruned, err := pruneVendorDir(vendorDir, map[string]bool{})
	if err != nil {
		t.Fatalf("pruneVendorDir() error: %v", err)
	}

	if len(pruned) != 1 || pruned[0] != "stale.invowkmod" {
		t.Errorf("pruned = %v, want [stale.invowkmod]", pruned)
	}
	if _, err := os.Stat(toolsDir); err != nil {
		t.Errorf("tools/ directory should survive pruning: %v", err)
	}
	if _, err := os.Stat(readmePath); err != nil {
		t.Errorf("README.md should survive pruning: %v", err)
	}
}

func TestPruneVendorDirEmptyExpectedPrunesModules(t *testing.T) {
	t.Parallel()

	vendorDir := makeVendorDir(t)
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
	for _, name := range []string{"dep1.invowkmod", "dep2.invowkmod"} {
		if _, err := os.Stat(filepath.Join(vendorDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should have been pruned", name)
		}
	}
}

func TestPruneVendorDirEmptyVendorDir(t *testing.T) {
	t.Parallel()

	pruned, err := pruneVendorDir(makeVendorDir(t), map[string]bool{})
	if err != nil {
		t.Fatalf("pruneVendorDir() error: %v", err)
	}

	if len(pruned) != 0 {
		t.Errorf("pruned %d entries, want 0", len(pruned))
	}
}

func makeVendorDir(t *testing.T) string {
	t.Helper()

	vendorDir := filepath.Join(t.TempDir(), VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return vendorDir
}

func TestVendorModules_CanonicalCollisionFails(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")

	cache1 := createCacheModule(t, tmpDir, "depone.invowkmod", "io.example.dep")
	cache2 := createCacheModule(t, tmpDir, "deptwo.invowkmod", "io.example.dep")

	_, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules: []*invowkmod.ResolvedModule{
			{CachePath: types.FilesystemPath(cache1), Namespace: "dep@1.0.0", ModuleID: "io.example.dep"},
			{CachePath: types.FilesystemPath(cache2), Namespace: "dep@2.0.0", ModuleID: "io.example.dep"},
		},
	})
	if err == nil {
		t.Fatal("VendorModules() should fail when two modules resolve to the same directory name")
	}
	if !errors.Is(err, ErrVendorConflict) {
		t.Errorf("error should wrap ErrVendorConflict, got: %v", err)
	}
	if !strings.Contains(err.Error(), "io.example.dep.invowkmod") {
		t.Errorf("error should mention the conflicting directory name, got: %v", err)
	}
}

func TestVendorModules_PruneNoOp(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "parent.invowkmod", "parent")
	cache1 := createCacheModule(t, tmpDir, "dep1.invowkmod", "dep1")
	cache2 := createCacheModule(t, tmpDir, "dep2.invowkmod", "dep2")

	modules := []*invowkmod.ResolvedModule{
		{CachePath: types.FilesystemPath(cache1), Namespace: "dep1@1.0.0"},
		{CachePath: types.FilesystemPath(cache2), Namespace: "dep2@2.0.0"},
	}

	// Vendor both modules initially
	_, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
		Modules:    modules,
	})
	if err != nil {
		t.Fatalf("initial VendorModules() error: %v", err)
	}

	// Re-vendor the same set with prune — nothing should be pruned
	result, err := VendorModules(VendorOptions{
		ModulePath: types.FilesystemPath(modulePath),
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
		modPath := filepath.Join(string(GetVendoredModulesDir(types.FilesystemPath(modulePath))), name)
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
		ModulePath: types.FilesystemPath(modulePath),
		Modules:    []*invowkmod.ResolvedModule{},
		Prune:      true,
	})
	if err != nil {
		t.Fatalf("VendorModules() with prune and no vendor dir should not error: %v", err)
	}

	if len(result.Pruned) != 0 {
		t.Errorf("VendorModules() pruned %d, want 0", len(result.Pruned))
	}
}
