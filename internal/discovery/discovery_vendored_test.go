// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
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
	declareVendoredModule(t, parentModulePath, moduleDir, moduleID)
	return moduleDir
}

func declareVendoredModule(t *testing.T, parentModulePath, moduleDir, moduleID string) {
	t.Helper()

	gitURL := invowkmod.GitURL("https://example.com/" + moduleID + ".git")
	invowkmodPath := filepath.Join(parentModulePath, "invowkmod.cue")
	content, err := os.ReadFile(invowkmodPath)
	if err != nil {
		t.Fatalf("failed to read parent invowkmod.cue: %v", err)
	}
	updated := string(content) + `requires: [
	{git_url: "` + string(gitURL) + `", version: "^1.0.0"},
]
`
	if writeErr := os.WriteFile(invowkmodPath, []byte(updated), 0o644); writeErr != nil {
		t.Fatalf("failed to update parent invowkmod.cue: %v", writeErr)
	}

	hash, err := invowkmod.ComputeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	lock := invowkmod.NewLockFile()
	lock.Modules[invowkmod.ModuleRefKey(gitURL)] = invowkmod.LockedModule{
		GitURL:          gitURL,
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       invowkmod.ModuleNamespace(moduleID),
		CommandSourceID: invowkmod.ModuleSourceID(moduleID),
		ModuleID:        invowkmod.ModuleID(moduleID),
		ContentHash:     hash,
	}
	if saveErr := lock.Save(filepath.Join(parentModulePath, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}
}

func refreshVendoredModuleHash(t *testing.T, parentModulePath, moduleDir, moduleID string) {
	t.Helper()

	lockPath := filepath.Join(parentModulePath, invowkmod.LockFileName)
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("LoadLockFile() = %v", err)
	}
	hash, err := invowkmod.ComputeModuleHash(moduleDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	key := invowkmod.ModuleRefKey("https://example.com/" + moduleID + ".git")
	entry := lock.Modules[key]
	entry.ContentHash = hash
	lock.Modules[key] = entry
	if saveErr := lock.Save(lockPath); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}
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

func TestDiscoverAll_SkipsUndeclaredVendoredModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}
	orphanDir := filepath.Join(vendorDir, "orphan.invowkmod")
	createTestModule(t, orphanDir, "orphan", "orphan-cmd")

	d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)
	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "orphan" {
			t.Fatal("undeclared vendored module should not be discovered")
		}
	}

	var foundDiagnostic bool
	for _, diag := range diagnostics {
		if diag.code == CodeVendoredUndeclaredSkipped {
			foundDiagnostic = true
			break
		}
	}
	if !foundDiagnostic {
		t.Fatalf("missing %s diagnostic: %v", CodeVendoredUndeclaredSkipped, diagnostics)
	}
}

func TestDiscoverAll_VerifiesOnlyDeclaredVendoredModules(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")
	createVendoredModule(t, parentDir, "declared.invowkmod", "declared", "declared-cmd")

	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	staleDir := filepath.Join(vendorDir, "stale.invowkmod")
	createTestModule(t, staleDir, "stale", "stale-cmd")

	d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)
	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	var foundDeclared bool
	for _, f := range files {
		if f.Module == nil {
			continue
		}
		switch f.Module.Name() {
		case "declared":
			foundDeclared = true
		case "stale":
			t.Fatal("stale undeclared vendored module should not be discovered")
		}
	}
	if !foundDeclared {
		t.Fatal("declared vendored module was not discovered")
	}

	var foundDiagnostic bool
	for _, diag := range diagnostics {
		if diag.code == CodeVendoredUndeclaredSkipped {
			foundDiagnostic = true
			break
		}
	}
	if !foundDiagnostic {
		t.Fatalf("missing %s diagnostic: %v", CodeVendoredUndeclaredSkipped, diagnostics)
	}
}

func TestDiscoverAll_SkipsVendoredModuleWithUndeclaredTransitiveRequirement(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")
	childDir := createVendoredModule(t, parentDir, "child.invowkmod", "child", "child-cmd")

	childModPath := filepath.Join(childDir, "invowkmod.cue")
	content, err := os.ReadFile(childModPath)
	if err != nil {
		t.Fatalf("ReadFile(child invowkmod.cue) = %v", err)
	}
	updated := string(content) + `requires: [
	{git_url: "https://example.com/grandchild.git", version: "^1.0.0"},
]
`
	if writeErr := os.WriteFile(childModPath, []byte(updated), 0o644); writeErr != nil {
		t.Fatalf("WriteFile(child invowkmod.cue) = %v", writeErr)
	}
	refreshVendoredModuleHash(t, parentDir, childDir, "child")

	d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)
	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "child" {
			t.Fatal("vendored module with undeclared transitive dependency should not be discovered")
		}
	}

	var foundDiagnostic bool
	for _, diag := range diagnostics {
		if diag.code == CodeVendoredTransitiveSkipped {
			foundDiagnostic = true
			break
		}
	}
	if !foundDiagnostic {
		t.Fatalf("missing %s diagnostic: %v", CodeVendoredTransitiveSkipped, diagnostics)
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

func TestDiscoverCommandSet_UsesVendoredAliasNamespaceFromLockFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "io.example.parent.invowkmod")
	createTestModule(t, parentDir, "io.example.parent", "parent-cmd")
	vendoredDir := createVendoredModule(t, parentDir, "io.example.vendored.invowkmod", "io.example.vendored", "run")
	hash, err := invowkmod.ComputeModuleHash(vendoredDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	lock := invowkmod.NewLockFile()
	lock.Modules["https://example.com/io.example.vendored.git"] = invowkmod.LockedModule{
		GitURL:          "https://example.com/io.example.vendored.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Alias:           "tools",
		Namespace:       "tools",
		CommandSourceID: "tools",
		ModuleID:        "io.example.vendored",
		ContentHash:     hash,
	}
	if saveErr := lock.Save(filepath.Join(parentDir, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}

	d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)
	result, err := d.DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() = %v", err)
	}

	cmd := result.Set.ByName["tools run"]
	if cmd == nil {
		t.Fatalf("ByName missing aliased vendored command; sources: %v", result.Set.SourceOrder)
	}
	if cmd.SourceID != "tools" {
		t.Fatalf("SourceID = %q, want tools", cmd.SourceID)
	}
	if cmd.ModuleID == nil || *cmd.ModuleID != "io.example.vendored" {
		t.Fatalf("ModuleID = %v, want io.example.vendored", cmd.ModuleID)
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

	d := newTestDiscovery(t, cfg, tmpDir, WithBaseDir(types.FilesystemPath(workDir)))

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
		WithBaseDir(types.FilesystemPath(workDir)),
		WithCommandsDir(types.FilesystemPath(userCmdsDir)),
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
	parentMod, err := invowkmod.Load(types.FilesystemPath(parentDir))
	if err != nil {
		t.Fatal(err)
	}
	dup1Mod, err := invowkmod.Load(types.FilesystemPath(dup1Dir))
	if err != nil {
		t.Fatal(err)
	}
	dup2Mod, err := invowkmod.Load(types.FilesystemPath(dup2Dir))
	if err != nil {
		t.Fatal(err)
	}

	// Build DiscoveredFiles with parsed Invowkfiles (module metadata attached)
	// so CheckModuleCollisions can read module IDs.
	dup1Inv, err := invowkfile.ParseLoadedModuleInvowkfile(dup1Mod)
	if err != nil {
		t.Fatal(err)
	}
	dup2Inv, err := invowkfile.ParseLoadedModuleInvowkfile(dup2Mod)
	if err != nil {
		t.Fatal(err)
	}

	files := []*DiscoveredFile{
		{Path: dup1Mod.InvowkfilePath(), Source: SourceModule, Module: dup1Mod, Invowkfile: dup1Inv},
		{Path: dup2Mod.InvowkfilePath(), Source: SourceModule, Module: dup2Mod, Invowkfile: dup2Inv, ParentModule: parentMod},
	}

	collisionErr := d.CheckModuleCollisions(files)
	if collisionErr == nil {
		t.Fatal("CheckModuleCollisions() should return error for duplicate module IDs")
	}

	var typedErr *ModuleCollisionError
	if !errors.As(collisionErr, &typedErr) {
		t.Fatalf("collision error = %T, want ModuleCollisionError", collisionErr)
	}
	if typedErr.SecondKind != ModuleCollisionSourceVendored {
		t.Errorf("SecondKind = %q, want vendored", typedErr.SecondKind)
	}
}

func TestDiscoverAll_VendoredInGlobalModuleInheritsIsGlobalModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a global module in user-dir with a vendored dependency.
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	parentDir := filepath.Join(userCmdsDir, "globalparent.invowkmod")
	createTestModule(t, parentDir, "globalparent", "parent-cmd")
	createVendoredModule(t, parentDir, "vendored.invowkmod", "vendored", "vendored-cmd")

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithBaseDir(types.FilesystemPath(workDir)),
		WithCommandsDir(types.FilesystemPath(userCmdsDir)),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	var foundParent, foundVendored bool
	for _, f := range files {
		if f.Module == nil {
			continue
		}
		if f.Module.Name() == "globalparent" {
			foundParent = true
			if !f.IsGlobalModule {
				t.Error("global parent module should have IsGlobalModule=true")
			}
		}
		if f.Module.Name() == "vendored" {
			foundVendored = true
			if !f.IsGlobalModule {
				t.Error("vendored child of global module should inherit IsGlobalModule=true")
			}
		}
	}

	if !foundParent {
		t.Error("did not find global parent module")
	}
	if !foundVendored {
		t.Error("did not find vendored module inside global parent")
	}
}

func TestDiscoverAll_VendoredInLocalModuleNotGlobal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a local module (not in user-dir) with a vendored dependency.
	parentDir := filepath.Join(tmpDir, "localparent.invowkmod")
	createTestModule(t, parentDir, "localparent", "parent-cmd")
	createVendoredModule(t, parentDir, "localchild.invowkmod", "localchild", "child-cmd")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Module == nil {
			continue
		}
		// Vendored module name comes from the folder name, not the module: field in invowkmod.cue.
		if f.Module.Name() == "localchild" && f.ParentModule != nil {
			found = true
			if f.IsGlobalModule {
				t.Error("vendored child of local module should NOT have IsGlobalModule=true")
			}
		}
	}
	if !found {
		t.Error("did not find vendored module inside local parent")
	}
}
