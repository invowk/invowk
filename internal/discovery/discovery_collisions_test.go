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

type collisionFileSpec struct {
	path             string
	moduleID         invowkmod.ModuleID
	modulePath       types.FilesystemPath
	parentModuleID   invowkmod.ModuleID
	commandNamespace invowkmod.ModuleNamespace
	libraryOnly      bool
	emptyInvowkfile  bool
	err              error
}

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

func discoveredFileFromCollisionSpec(t *testing.T, spec collisionFileSpec) *DiscoveredFile {
	t.Helper()
	file := &DiscoveredFile{Path: types.FilesystemPath(spec.path), Error: spec.err, CommandNamespace: spec.commandNamespace}
	if spec.emptyInvowkfile {
		file.Invowkfile = &invowkfile.Invowkfile{}
	} else if spec.moduleID != "" && !spec.libraryOnly {
		file.Invowkfile = &invowkfile.Invowkfile{Metadata: testModuleMetadata(t, spec.moduleID)}
	}
	if spec.modulePath != "" || spec.libraryOnly {
		file.Module = &invowkmod.Module{Path: spec.modulePath, IsLibraryOnly: spec.libraryOnly}
		if spec.libraryOnly {
			file.Module.Metadata = &invowkmod.Invowkmod{Module: spec.moduleID}
		}
	}
	if spec.parentModuleID != "" {
		file.ParentModule = &invowkmod.Module{Metadata: &invowkmod.Invowkmod{Module: spec.parentModuleID}}
	}
	return file
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
	tests := []struct {
		name          string
		includes      []config.IncludeEntry
		files         []collisionFileSpec
		wantErr       bool
		wantNamespace SourceID
	}{
		{name: "NoCollision", files: []collisionFileSpec{{path: "/path/to/module1", moduleID: "io.example.module1"}, {path: "/path/to/module2", moduleID: "io.example.module2"}}},
		{name: "WithCollision", files: []collisionFileSpec{{path: "/path/to/module1", moduleID: "io.example.same"}, {path: "/path/to/module2", moduleID: "io.example.same"}}, wantErr: true, wantNamespace: "io.example.same"},
		{name: "CollisionResolvedByAlias", includes: []config.IncludeEntry{{Path: "/path/to/module1.invowkmod", Alias: "io.example.alias1"}}, files: []collisionFileSpec{{path: "/path/to/module1.invowkmod/invowkfile.cue", moduleID: "io.example.same", modulePath: "/path/to/module1.invowkmod"}, {path: "/path/to/module2.invowkmod/invowkfile.cue", moduleID: "io.example.same", modulePath: "/path/to/module2.invowkmod"}}},
		{name: "HyphenatedAliasCollisionReportsNamespace", includes: []config.IncludeEntry{{Path: "/path/to/module1.invowkmod", Alias: "ci-tools"}, {Path: "/path/to/module2.invowkmod", Alias: "ci-tools"}}, files: []collisionFileSpec{{path: "/path/to/module1.invowkmod/invowkfile.cue", moduleID: "io.example.one", modulePath: "/path/to/module1.invowkmod"}, {path: "/path/to/module2.invowkmod/invowkfile.cue", moduleID: "io.example.two", modulePath: "/path/to/module2.invowkmod"}}, wantErr: true, wantNamespace: "ci-tools"},
		{name: "CommandSourceCollisionWithDifferentModuleIDs", files: []collisionFileSpec{{path: "/first/tools.invowkmod/invowkfile.cue", moduleID: "io.example.first", modulePath: "/first/tools.invowkmod"}, {path: "/second/tools.invowkmod/invowkfile.cue", moduleID: "io.example.second", modulePath: "/second/tools.invowkmod"}}, wantErr: true, wantNamespace: "tools"},
		{name: "DuplicateModuleIDWithoutCommandSourceCollision", files: []collisionFileSpec{{path: "/first/one.invowkmod/invowkfile.cue", moduleID: "io.example.same", modulePath: "/first/one.invowkmod"}, {path: "/second/two.invowkmod/invowkfile.cue", moduleID: "io.example.same", modulePath: "/second/two.invowkmod"}}, wantErr: true, wantNamespace: "io.example.same"},
		{name: "LibraryOnlyDuplicateModuleID", files: []collisionFileSpec{{moduleID: "io.example.lib", modulePath: "/first/lib.invowkmod", libraryOnly: true}, {moduleID: "io.example.lib", modulePath: "/second/lib.invowkmod", libraryOnly: true}}, wantErr: true, wantNamespace: "io.example.lib"},
		{name: "VendoredAliasCollision", files: []collisionFileSpec{{path: "/parent1.invowkmod/invowk_modules/child1.invowkmod/invowkfile.cue", moduleID: "io.example.child1", modulePath: "/parent1.invowkmod/invowk_modules/child1.invowkmod", parentModuleID: "io.example.parent1", commandNamespace: "tools"}, {path: "/parent2.invowkmod/invowk_modules/child2.invowkmod/invowkfile.cue", moduleID: "io.example.child2", modulePath: "/parent2.invowkmod/invowk_modules/child2.invowkmod", parentModuleID: "io.example.parent2", commandNamespace: "tools"}}, wantErr: true, wantNamespace: "tools"},
		{name: "SkipsFilesWithErrors", files: []collisionFileSpec{{path: "/path/to/module1", moduleID: "io.example.same"}, {path: "/path/to/module2", err: os.ErrNotExist}}},
		{name: "SkipsFilesWithoutModuleID", files: []collisionFileSpec{{path: "/path/to/module1", moduleID: "io.example.module1"}, {path: "/path/to/module2", emptyInvowkfile: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Includes = tt.includes
			d := New(cfg)
			files := make([]*DiscoveredFile, 0, len(tt.files))
			for _, spec := range tt.files {
				files = append(files, discoveredFileFromCollisionSpec(t, spec))
			}
			err := d.CheckModuleCollisions(files)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckModuleCollisions() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantErr {
				collisionErr, ok := errors.AsType[*ModuleCollisionError](err)
				if !ok {
					t.Fatalf("CheckModuleCollisions() error = %T %v, want ModuleCollisionError", err, err)
				}
				if collisionErr.Namespace != tt.wantNamespace {
					t.Errorf("Namespace = %q, want %q", collisionErr.Namespace, tt.wantNamespace)
				}
			}
		})
	}
}

func TestGetEffectiveCommandNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		includes []config.IncludeEntry
		file     collisionFileSpec
		want     SourceID
	}{
		{name: "WithoutAlias", file: collisionFileSpec{path: "/path/to/module", moduleID: "io.example.original"}, want: "io.example.original"},
		{name: "WithAlias", includes: []config.IncludeEntry{{Path: "/path/to/module.invowkmod", Alias: "io.example.aliased"}}, file: collisionFileSpec{path: "/path/to/module.invowkmod/invowkfile.cue", moduleID: "io.example.original", modulePath: "/path/to/module.invowkmod"}, want: "io.example.aliased"},
		{name: "WithModuleDefaultSourceID", file: collisionFileSpec{path: "/path/to/tools.invowkmod/invowkfile.cue", moduleID: "io.example.tools", modulePath: "/path/to/tools.invowkmod"}, want: "tools"},
		{name: "WithVendoredCommandNamespace", file: collisionFileSpec{path: "/path/to/parent.invowkmod/invowk_modules/tools.invowkmod/invowkfile.cue", moduleID: "io.example.tools", modulePath: "/path/to/parent.invowkmod/invowk_modules/tools.invowkmod", commandNamespace: "tools"}, want: "tools"},
		{name: "WithNilInvowkfile", file: collisionFileSpec{path: "/path/to/module"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Includes = tt.includes
			d := New(cfg)
			file := discoveredFileFromCollisionSpec(t, tt.file)
			if got := d.GetEffectiveCommandNamespace(file); got != tt.want {
				t.Errorf("GetEffectiveCommandNamespace() = %q, want %q", got, tt.want)
			}
		})
	}
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
