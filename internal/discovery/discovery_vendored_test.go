// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// createVendoredModule creates a vendored module inside a parent module's invowk_modules/ dir.
func createVendoredModule(t *testing.T, parentModulePath, moduleFolder, moduleID, cmdName string) string {
	t.Helper()
	vendorDir := filepath.Join(parentModulePath, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}
	moduleDir := filepath.Join(vendorDir, moduleFolder)
	createTestModule(t, moduleDir, moduleID, cmdName)
	return moduleDir
}

func TestDiscoverAll_FindsVendoredModulesInLocalModules(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a parent module in the current directory
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create a vendored module inside the parent
	createVendoredModule(t, parentDir, "vendored.invowkmod", "vendored", "vendored-cmd")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	// Should find both parent and vendored
	var foundParent, foundVendored bool
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "parent" {
			foundParent = true
			if f.ParentModule != nil {
				t.Error("parent module should not have ParentModule set")
			}
		}
		if f.Module != nil && f.Module.Name() == "vendored" {
			foundVendored = true
			if f.ParentModule == nil {
				t.Error("vendored module should have ParentModule set")
			} else if f.ParentModule.Name() != "parent" {
				t.Errorf("vendored ParentModule = %q, want %q", f.ParentModule.Name(), "parent")
			}
			if f.Source != SourceModule {
				t.Errorf("vendored module source = %v, want SourceModule", f.Source)
			}
		}
	}

	if !foundParent {
		t.Error("DiscoverAll() did not find parent module")
	}
	if !foundVendored {
		t.Error("DiscoverAll() did not find vendored module")
	}
}

func TestDiscoverAll_VendoredModulesOrderedAfterParent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")
	createVendoredModule(t, parentDir, "child.invowkmod", "child", "child-cmd")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	parentIdx := -1
	childIdx := -1
	for i, f := range files {
		if f.Module != nil && f.Module.Name() == "parent" {
			parentIdx = i
		}
		if f.Module != nil && f.Module.Name() == "child" {
			childIdx = i
		}
	}

	if parentIdx == -1 || childIdx == -1 {
		t.Fatalf("did not find both modules: parentIdx=%d, childIdx=%d", parentIdx, childIdx)
	}
	if childIdx <= parentIdx {
		t.Errorf("vendored module (idx %d) should come after parent (idx %d)", childIdx, parentIdx)
	}
}

func TestDiscoverAll_FindsVendoredModulesInIncludes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create an included module with a vendored dependency
	includeDir := filepath.Join(tmpDir, "includes")
	includedModule := filepath.Join(includeDir, "included.invowkmod")
	createTestModule(t, includedModule, "included", "included-cmd")
	createVendoredModule(t, includedModule, "vendep.invowkmod", "vendep", "vendep-cmd")

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(includedModule)}}

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := newTestDiscovery(t, cfg, tmpDir, WithBaseDir(workDir))

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	var foundVendep bool
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "vendep" {
			foundVendep = true
			if f.ParentModule == nil || f.ParentModule.Name() != "included" {
				t.Error("vendored module from include should have correct ParentModule")
			}
		}
	}
	if !foundVendep {
		t.Error("DiscoverAll() did not find vendored module from include")
	}
}

func TestDiscoverAll_FindsVendoredModulesInUserDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a user-dir module with a vendored dependency
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	userModule := filepath.Join(userCmdsDir, "userm.invowkmod")
	createTestModule(t, userModule, "userm", "userm-cmd")
	createVendoredModule(t, userModule, "uvendor.invowkmod", "uvendor", "uvendor-cmd")

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithBaseDir(workDir),
		WithCommandsDir(userCmdsDir),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	var foundUvendor bool
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "uvendor" {
			foundUvendor = true
			if f.ParentModule == nil || f.ParentModule.Name() != "userm" {
				t.Error("vendored module from user-dir should have correct ParentModule")
			}
		}
	}
	if !foundUvendor {
		t.Error("DiscoverAll() did not find vendored module from user-dir")
	}
}

func TestDiscoverAll_NestedVendoredNotRecursed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create vendored module with its OWN invowk_modules/
	vendoredDir := createVendoredModule(t, parentDir, "mid.invowkmod", "mid", "mid-cmd")
	createVendoredModule(t, vendoredDir, "deep.invowkmod", "deep", "deep-cmd")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, _, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	// "mid" should be found, "deep" should NOT
	var foundMid, foundDeep bool
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "mid" {
			foundMid = true
		}
		if f.Module != nil && f.Module.Name() == "deep" {
			foundDeep = true
		}
	}

	if !foundMid {
		t.Error("first-level vendored module 'mid' should be discovered")
	}
	if foundDeep {
		t.Error("nested vendored module 'deep' should NOT be discovered (no recursion)")
	}
}

func TestDiscoverAll_NestedVendoredEmitsDiagnostic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	vendoredDir := createVendoredModule(t, parentDir, "mid.invowkmod", "mid", "mid-cmd")

	// Create invowk_modules/ inside the vendored module (triggering the warning)
	nestedVendorDir := filepath.Join(vendoredDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(nestedVendorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	_, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	var foundNestedWarning bool
	for _, diag := range diagnostics {
		if diag.Code == "vendored_nested_ignored" {
			foundNestedWarning = true
			break
		}
	}
	if !foundNestedWarning {
		t.Error("should emit 'vendored_nested_ignored' diagnostic when vendored module has its own invowk_modules/")
	}
}

func TestDiscoverAll_InvalidVendoredModuleSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create an invalid vendored module (dir with .invowkmod suffix but no invowkmod.cue)
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidDir := filepath.Join(vendorDir, "broken.invowkmod")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	_, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	var foundSkipDiag bool
	for _, diag := range diagnostics {
		if diag.Code == "vendored_module_load_skipped" {
			foundSkipDiag = true
			break
		}
	}
	if !foundSkipDiag {
		t.Error("should emit 'vendored_module_load_skipped' diagnostic for invalid vendored module")
	}
}

func TestDiscoverAll_EmptyVendorDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create empty invowk_modules/
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	// Only parent should be found, no vendored
	for _, f := range files {
		if f.ParentModule != nil {
			t.Error("should not find vendored modules in empty vendor dir")
		}
	}

	// No vendor-related diagnostics
	for _, diag := range diagnostics {
		if strings.HasPrefix(string(diag.Code), "vendored_") {
			t.Errorf("unexpected vendor diagnostic: %s", diag.Code)
		}
	}
}

func TestDiscoverAll_VendoredReservedModuleSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a parent module
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create a vendored directory with the reserved "invowkfile" name.
	// The reserved name check fires before Load(), so no invowkmod.cue is needed inside.
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reservedDir := filepath.Join(vendorDir, "invowkfile.invowkmod")
	if err := os.MkdirAll(reservedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	// Verify no DiscoveredFile has module name "invowkfile"
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "invowkfile" {
			t.Error("should not discover a module with reserved name 'invowkfile'")
		}
	}

	// Verify diagnostic was emitted
	var foundDiag bool
	for _, diag := range diagnostics {
		if diag.Code == "vendored_reserved_module_skipped" {
			foundDiag = true
			break
		}
	}
	if !foundDiag {
		t.Error("should emit 'vendored_reserved_module_skipped' diagnostic for reserved module name in vendor dir")
	}
}

func TestDiscoverAll_VendoredScanFailed(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping: Windows does not use POSIX file permissions")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping: root can read any directory regardless of permissions")
	}

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create invowk_modules/ and make it unreadable
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(vendorDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Restore permissions so t.TempDir() cleanup can remove it
		_ = os.Chmod(vendorDir, 0o755)
	})

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	// discoverAllWithDiagnostics should NOT return an error (non-fatal)
	_, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() should not return error for unreadable vendor dir: %v", err)
	}

	var foundDiag bool
	for _, diag := range diagnostics {
		if diag.Code == "vendored_scan_failed" {
			foundDiag = true
			break
		}
	}
	if !foundDiag {
		t.Error("should emit 'vendored_scan_failed' diagnostic when vendor directory is unreadable")
	}
}

func TestCheckModuleCollisions_AnnotatesVendored(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create two modules with the same module ID — one vendored, one direct
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	dup1Dir := filepath.Join(tmpDir, "dup.invowkmod")
	createTestModule(t, dup1Dir, "dup", "dup-cmd")

	dup2Dir := createVendoredModule(t, parentDir, "dup.invowkmod", "dup", "dup-cmd2")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	// Load modules
	parentMod, err := invowkmod.Load(parentDir)
	if err != nil {
		t.Fatal(err)
	}
	dup1Mod, err := invowkmod.Load(dup1Dir)
	if err != nil {
		t.Fatal(err)
	}
	dup2Mod, err := invowkmod.Load(dup2Dir)
	if err != nil {
		t.Fatal(err)
	}

	// Build DiscoveredFiles with parsed Invowkfiles (module metadata attached).
	// Parse the invowkfile.cue for each so CheckModuleCollisions can read module IDs.
	dup1Inv, err := invowkfile.Parse(dup1Mod.InvowkfilePath())
	if err != nil {
		t.Fatal(err)
	}
	dup1Inv.Metadata = invowkfile.NewModuleMetadataFromInvowkmod(dup1Mod.Metadata)

	dup2Inv, err := invowkfile.Parse(dup2Mod.InvowkfilePath())
	if err != nil {
		t.Fatal(err)
	}
	dup2Inv.Metadata = invowkfile.NewModuleMetadataFromInvowkmod(dup2Mod.Metadata)

	files := []*DiscoveredFile{
		{Path: types.FilesystemPath(dup1Mod.InvowkfilePath()), Source: SourceModule, Module: dup1Mod, Invowkfile: dup1Inv},
		{Path: types.FilesystemPath(dup2Mod.InvowkfilePath()), Source: SourceModule, Module: dup2Mod, Invowkfile: dup2Inv, ParentModule: parentMod},
	}

	collisionErr := d.CheckModuleCollisions(files)
	if collisionErr == nil {
		t.Fatal("CheckModuleCollisions() should return error for duplicate module IDs")
	}

	errMsg := collisionErr.Error()
	if !strings.Contains(errMsg, "vendored in parent") {
		t.Errorf("collision error should mention 'vendored in parent', got: %s", errMsg)
	}
}
