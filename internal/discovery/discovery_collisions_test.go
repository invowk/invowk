// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// moduleIDPtr is a test helper that creates a *invowkmod.ModuleID from a string.
func moduleIDPtr(s string) *invowkmod.ModuleID {
	id := invowkmod.ModuleID(s)
	return &id
}

// testModuleMetadata creates a minimal validated ModuleMetadata for test fixtures.
func testModuleMetadata(t *testing.T, moduleID invowkmod.ModuleID) *invowkfile.ModuleMetadata {
	t.Helper()
	metadata, err := invowkfile.NewModuleMetadata(moduleID, "0.0.0", "", nil)
	if err != nil {
		t.Fatalf("NewModuleMetadata() error = %v", err)
	}
	return metadata
}

func TestModuleCollisionError(t *testing.T) {
	t.Parallel()

	err := &ModuleCollisionError{
		Namespace:    "io.example.tools",
		FirstSource:  "/path/to/first",
		SecondSource: "/path/to/second",
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "io.example.tools") {
		t.Error("error message should contain module ID")
	}
	if strings.Contains(errMsg, "includes:") || strings.Contains(errMsg, "alias") {
		t.Error("domain error should not render CLI remediation")
	}

	if !errors.Is(err, ErrModuleCollision) {
		t.Error("errors.Is(err, ErrModuleCollision) should be true")
	}
}

func TestCheckModuleCollisions(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("NoCollision", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.module1")},
			},
			{
				Path:       "/path/to/module2",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.module2")},
			},
		}

		err := d.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() returned unexpected error: %v", err)
		}
	})

	t.Run("WithCollision", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
			},
			{
				Path:       "/path/to/module2",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
			},
		}

		err := d.CheckModuleCollisions(files)
		if err == nil {
			t.Error("CheckModuleCollisions() should return error for collision")
		}

		collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
		if !ok {
			t.Errorf("error should be ModuleCollisionError, got %T", err)
		}
		if collisionErr != nil && collisionErr.Namespace != "io.example.same" {
			t.Errorf("Namespace = %s, want io.example.same", collisionErr.Namespace)
		}
	})

	t.Run("CollisionResolvedByAlias", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		cfg.Includes = []config.IncludeEntry{
			{Path: "/path/to/module1.invowkmod", Alias: "io.example.alias1"},
		}
		dAlias := New(cfg)

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
				Module:     &invowkmod.Module{Path: "/path/to/module1.invowkmod"},
			},
			{
				Path:       "/path/to/module2.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
				Module:     &invowkmod.Module{Path: "/path/to/module2.invowkmod"},
			},
		}

		err := dAlias.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should not return error when alias resolves collision: %v", err)
		}
	})

	t.Run("HyphenatedAliasCollisionReportsNamespace", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		cfg.Includes = []config.IncludeEntry{
			{Path: "/path/to/module1.invowkmod", Alias: "ci-tools"},
			{Path: "/path/to/module2.invowkmod", Alias: "ci-tools"},
		}
		dAlias := New(cfg)

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.one")},
				Module:     &invowkmod.Module{Path: "/path/to/module1.invowkmod"},
			},
			{
				Path:       "/path/to/module2.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.two")},
				Module:     &invowkmod.Module{Path: "/path/to/module2.invowkmod"},
			},
		}

		err := dAlias.CheckModuleCollisions(files)
		collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
		if !ok {
			t.Fatalf("CheckModuleCollisions() error = %T %v, want ModuleCollisionError", err, err)
		}
		if collisionErr.Namespace != "ci-tools" {
			t.Fatalf("Namespace = %q, want ci-tools", collisionErr.Namespace)
		}
	})

	t.Run("CommandSourceCollisionWithDifferentModuleIDs", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/first/tools.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.first")},
				Module:     &invowkmod.Module{Path: "/first/tools.invowkmod"},
			},
			{
				Path:       "/second/tools.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.second")},
				Module:     &invowkmod.Module{Path: "/second/tools.invowkmod"},
			},
		}

		err := d.CheckModuleCollisions(files)
		collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
		if !ok {
			t.Fatalf("CheckModuleCollisions() error = %T %v, want ModuleCollisionError", err, err)
		}
		if collisionErr.Namespace != "tools" {
			t.Fatalf("Namespace = %q, want tools", collisionErr.Namespace)
		}
	})

	t.Run("DuplicateModuleIDWithoutCommandSourceCollision", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/first/one.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
				Module:     &invowkmod.Module{Path: "/first/one.invowkmod"},
			},
			{
				Path:       "/second/two.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
				Module:     &invowkmod.Module{Path: "/second/two.invowkmod"},
			},
		}

		err := d.CheckModuleCollisions(files)
		collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
		if !ok {
			t.Fatalf("CheckModuleCollisions() error = %T %v, want ModuleCollisionError", err, err)
		}
		if collisionErr.Namespace != "io.example.same" {
			t.Fatalf("Namespace = %q, want io.example.same", collisionErr.Namespace)
		}
	})

	t.Run("LibraryOnlyDuplicateModuleID", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Module: &invowkmod.Module{
					Path:          "/first/lib.invowkmod",
					IsLibraryOnly: true,
					Metadata:      &invowkmod.Invowkmod{Module: "io.example.lib"},
				},
			},
			{
				Module: &invowkmod.Module{
					Path:          "/second/lib.invowkmod",
					IsLibraryOnly: true,
					Metadata:      &invowkmod.Invowkmod{Module: "io.example.lib"},
				},
			},
		}

		err := d.CheckModuleCollisions(files)
		collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
		if !ok {
			t.Fatalf("CheckModuleCollisions() error = %T %v, want ModuleCollisionError", err, err)
		}
		if collisionErr.Namespace != "io.example.lib" {
			t.Fatalf("Namespace = %q, want io.example.lib", collisionErr.Namespace)
		}
	})

	t.Run("VendoredAliasCollision", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:             "/parent1.invowkmod/invowk_modules/child1.invowkmod/invowkfile.cue",
				Invowkfile:       &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.child1")},
				Module:           &invowkmod.Module{Path: "/parent1.invowkmod/invowk_modules/child1.invowkmod"},
				ParentModule:     &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "io.example.parent1"}},
				CommandNamespace: "tools",
			},
			{
				Path:             "/parent2.invowkmod/invowk_modules/child2.invowkmod/invowkfile.cue",
				Invowkfile:       &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.child2")},
				Module:           &invowkmod.Module{Path: "/parent2.invowkmod/invowk_modules/child2.invowkmod"},
				ParentModule:     &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: "io.example.parent2"}},
				CommandNamespace: "tools",
			},
		}

		err := d.CheckModuleCollisions(files)
		collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
		if !ok {
			t.Fatalf("CheckModuleCollisions() error = %T %v, want ModuleCollisionError", err, err)
		}
		if collisionErr.Namespace != "tools" {
			t.Fatalf("Namespace = %q, want tools", collisionErr.Namespace)
		}
	})

	t.Run("SkipsFilesWithErrors", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.same")},
			},
			{
				Path:  "/path/to/module2",
				Error: os.ErrNotExist, // This file has an error
			},
		}

		err := d.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should skip files with errors: %v", err)
		}
	})

	t.Run("SkipsFilesWithoutModuleID", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1",
				Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.module1")},
			},
			{
				Path:       "/path/to/module2",
				Invowkfile: &invowkfile.Invowkfile{}, // Missing module metadata
			},
		}

		err := d.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should skip files without module ID: %v", err)
		}
	})
}

func TestGetEffectiveCommandNamespace(t *testing.T) {
	t.Parallel()

	t.Run("WithoutAlias", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:       "/path/to/module",
			Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.original")},
		}

		namespace := d.GetEffectiveCommandNamespace(file)
		if namespace != "io.example.original" {
			t.Errorf("GetEffectiveCommandNamespace() = %s, want io.example.original", namespace)
		}
	})

	t.Run("WithAlias", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		cfg.Includes = []config.IncludeEntry{
			{Path: "/path/to/module.invowkmod", Alias: "io.example.aliased"},
		}
		d := New(cfg)

		file := &DiscoveredFile{
			Path:       "/path/to/module.invowkmod/invowkfile.cue",
			Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.original")},
			Module:     &invowkmod.Module{Path: "/path/to/module.invowkmod"},
		}

		namespace := d.GetEffectiveCommandNamespace(file)
		if namespace != "io.example.aliased" {
			t.Errorf("GetEffectiveCommandNamespace() = %s, want io.example.aliased", namespace)
		}
	})

	t.Run("WithModuleDefaultSourceID", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:       "/path/to/tools.invowkmod/invowkfile.cue",
			Invowkfile: &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.tools")},
			Module:     &invowkmod.Module{Path: "/path/to/tools.invowkmod"},
		}

		namespace := d.GetEffectiveCommandNamespace(file)
		if namespace != "tools" {
			t.Errorf("GetEffectiveCommandNamespace() = %s, want tools", namespace)
		}
	})

	t.Run("WithVendoredCommandNamespace", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:             "/path/to/parent.invowkmod/invowk_modules/tools.invowkmod/invowkfile.cue",
			Invowkfile:       &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, "io.example.tools")},
			Module:           &invowkmod.Module{Path: "/path/to/parent.invowkmod/invowk_modules/tools.invowkmod"},
			CommandNamespace: "tools",
		}

		namespace := d.GetEffectiveCommandNamespace(file)
		if namespace != "tools" {
			t.Errorf("GetEffectiveCommandNamespace() = %s, want tools", namespace)
		}
	})

	t.Run("WithNilInvowkfile", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:       "/path/to/module",
			Invowkfile: nil,
		}

		namespace := d.GetEffectiveCommandNamespace(file)
		if namespace != "" {
			t.Errorf("GetEffectiveCommandNamespace() = %s, want empty string", namespace)
		}
	})
}

func TestDiscoverCommandSet_UsesAliasNamespaceForIncludedModule(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	firstModule := filepath.Join(firstRoot, "io.example.same.invowkmod")
	secondModule := filepath.Join(secondRoot, "io.example.same.invowkmod")
	createTestModule(t, firstModule, "io.example.same", "run")
	createTestModule(t, secondModule, "io.example.same", "run")

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(firstModule)},
		{Path: config.ModuleIncludePath(secondModule), Alias: "aliased"},
	}
	d := New(cfg, WithBaseDir(types.FilesystemPath(baseDir)), WithCommandsDir(""))

	result, err := d.DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}

	aliased := result.Set.ByName["aliased run"]
	if aliased == nil {
		t.Fatalf("ByName missing aliased command; sources: %v", result.Set.SourceOrder)
	}
	if aliased.SourceID != "aliased" {
		t.Errorf("SourceID = %q, want aliased", aliased.SourceID)
	}
	if aliased.ModuleID == nil {
		t.Fatal("ModuleID = nil, want aliased")
	}
	if *aliased.ModuleID != "io.example.same" {
		t.Errorf("ModuleID = %q, want io.example.same", *aliased.ModuleID)
	}
	if len(result.Set.BySource["aliased"]) != 1 {
		t.Errorf("BySource[aliased] has %d commands, want 1", len(result.Set.BySource["aliased"]))
	}
}

func TestDiscoverAll_CurrentDirInvowkfileTakesPrecedenceOverModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a regular invowkfile in current directory
	currentContent := `
cmds: [{name: "cmd", implementations: [{script: {content: "echo current"}, runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(currentContent), 0o644); err != nil {
		t.Fatalf("failed to write current invowkfile: %v", err)
	}

	// Create a module in the same directory using new two-file format
	moduleDir := filepath.Join(tmpDir, "apack.invowkmod")
	createTestModule(t, moduleDir, "apack", "cmd")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// First file should be from current directory, not module
	if len(files) == 0 {
		t.Fatal("DiscoverAll() returned no files")
	}

	if files[0].Source != SourceCurrentDir {
		t.Errorf("first file source = %v, want SourceCurrentDir", files[0].Source)
	}

	// Both should be found
	foundCurrentDir := false
	foundModule := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			foundCurrentDir = true
		}
		if f.Source == SourceModule {
			foundModule = true
		}
	}

	if !foundCurrentDir {
		t.Error("DiscoverAll() did not find invowkfile in current directory")
	}
	if !foundModule {
		t.Error("DiscoverAll() did not find module")
	}
}

func TestDiscoverAll_SkipsReservedModuleName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create invowkfile.cue in tmpDir
	invowkfileContent := `cmds: [{
		name: "root-cmd"
		description: "Root command"
		implementations: [{script: {content: "echo root"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	}]`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to create invowkfile: %v", err)
	}

	// Create a valid module
	validModDir := filepath.Join(tmpDir, "valid.invowkmod")
	if err := os.MkdirAll(validModDir, 0o755); err != nil {
		t.Fatalf("failed to create valid module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validModDir, "invowkmod.cue"), []byte(`module: "valid"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("failed to create invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validModDir, "invowkfile.cue"), []byte(`cmds: [{
		name: "valid-cmd"
		description: "Valid command"
		implementations: [{script: {content: "echo valid"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	}]`), 0o644); err != nil {
		t.Fatalf("failed to create invowkfile.cue: %v", err)
	}

	// Create a module with reserved name "invowkfile" (FR-015)
	reservedModDir := filepath.Join(tmpDir, "invowkfile.invowkmod")
	if err := os.MkdirAll(reservedModDir, 0o755); err != nil {
		t.Fatalf("failed to create reserved module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reservedModDir, "invowkmod.cue"), []byte(`module: "invowkfile"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("failed to create invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reservedModDir, "invowkfile.cue"), []byte(`cmds: [{
		name: "reserved-cmd"
		description: "Reserved command"
		implementations: [{script: {content: "echo reserved"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	}]`), 0o644); err != nil {
		t.Fatalf("failed to create invowkfile.cue: %v", err)
	}

	// Set HOME to isolated directory to avoid user-dir interference
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithCommandsDir(types.FilesystemPath(filepath.Join(homeDir, ".invowk", "cmds"))),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	// Should find invowkfile.cue and valid module, but NOT the reserved module
	foundCurrentDir := false
	foundValidModule := false
	foundReservedModule := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			foundCurrentDir = true
		}
		if f.Source == SourceModule && f.Module != nil {
			if f.Module.Name() == "valid" {
				foundValidModule = true
			}
			if f.Module.Name() == "invowkfile" {
				foundReservedModule = true
			}
		}
	}

	if !foundCurrentDir {
		t.Error("DiscoverAll() did not find invowkfile in current directory")
	}
	if !foundValidModule {
		t.Error("DiscoverAll() did not find valid module")
	}
	if foundReservedModule {
		t.Error("DiscoverAll() should skip module with reserved name 'invowkfile' (FR-015)")
	}
}
