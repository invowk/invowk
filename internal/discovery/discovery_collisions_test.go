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
)

func TestDiscoveredCommandSet_Add(t *testing.T) {
	t.Parallel()

	// Test T006: Add method
	set := NewDiscoveredCommandSet()

	cmd1 := &CommandInfo{
		Name:       "hello",
		SimpleName: "hello",
		SourceID:   "invowkfile",
	}
	cmd2 := &CommandInfo{
		Name:       "build",
		SimpleName: "build",
		SourceID:   "foo",
	}
	cmd3 := &CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "foo",
	}

	set.Add(cmd1)
	set.Add(cmd2)
	set.Add(cmd3)

	// Check Commands slice
	if len(set.Commands) != 3 {
		t.Errorf("len(Commands) = %d, want 3", len(set.Commands))
	}

	// Check BySimpleName index
	if len(set.BySimpleName["hello"]) != 1 {
		t.Errorf("len(BySimpleName[hello]) = %d, want 1", len(set.BySimpleName["hello"]))
	}
	if len(set.BySimpleName["build"]) != 1 {
		t.Errorf("len(BySimpleName[build]) = %d, want 1", len(set.BySimpleName["build"]))
	}

	// Check BySource index
	if len(set.BySource["invowkfile"]) != 1 {
		t.Errorf("len(BySource[invowkfile]) = %d, want 1", len(set.BySource["invowkfile"]))
	}
	if len(set.BySource["foo"]) != 2 {
		t.Errorf("len(BySource[foo]) = %d, want 2", len(set.BySource["foo"]))
	}

	// Check SourceOrder
	if len(set.SourceOrder) != 2 {
		t.Errorf("len(SourceOrder) = %d, want 2", len(set.SourceOrder))
	}
}

func TestDiscoveredCommandSet_Analyze_NoConflicts(t *testing.T) {
	t.Parallel()

	// Test T007: Analyze method with no conflicts
	set := NewDiscoveredCommandSet()

	set.Add(&CommandInfo{SimpleName: "hello", SourceID: "invowkfile"})
	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"})
	set.Add(&CommandInfo{SimpleName: "test", SourceID: "bar"})

	set.Analyze()

	// No ambiguous names
	if len(set.AmbiguousNames) != 0 {
		t.Errorf("len(AmbiguousNames) = %d, want 0", len(set.AmbiguousNames))
	}

	// All commands should not be ambiguous
	for _, cmd := range set.Commands {
		if cmd.IsAmbiguous {
			t.Errorf("Command %s should not be ambiguous", cmd.SimpleName)
		}
	}

	// SourceOrder should be sorted: invowkfile first, then alphabetically
	if set.SourceOrder[0] != "invowkfile" {
		t.Errorf("SourceOrder[0] = %s, want invowkfile", set.SourceOrder[0])
	}
	if set.SourceOrder[1] != "bar" {
		t.Errorf("SourceOrder[1] = %s, want bar", set.SourceOrder[1])
	}
	if set.SourceOrder[2] != "foo" {
		t.Errorf("SourceOrder[2] = %s, want foo", set.SourceOrder[2])
	}
}

func TestDiscoveredCommandSet_Analyze_WithConflicts(t *testing.T) {
	t.Parallel()

	// Test T007: Analyze method with conflicts
	set := NewDiscoveredCommandSet()

	// "deploy" exists in both invowkfile and foo
	set.Add(&CommandInfo{SimpleName: "hello", SourceID: "invowkfile"})
	set.Add(&CommandInfo{SimpleName: "deploy", SourceID: "invowkfile"})
	set.Add(&CommandInfo{SimpleName: "deploy", SourceID: "foo"})
	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"})

	set.Analyze()

	// "deploy" should be ambiguous
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous")
	}

	// "hello" and "build" should not be ambiguous
	if set.AmbiguousNames["hello"] {
		t.Error("'hello' should not be marked as ambiguous")
	}
	if set.AmbiguousNames["build"] {
		t.Error("'build' should not be marked as ambiguous")
	}

	// Check IsAmbiguous flag on commands
	for _, cmd := range set.Commands {
		if cmd.SimpleName == "deploy" {
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy' from %s should be marked as ambiguous", cmd.SourceID)
			}
		} else {
			if cmd.IsAmbiguous {
				t.Errorf("Command '%s' should not be marked as ambiguous", cmd.SimpleName)
			}
		}
	}
}

func TestDiscoveredCommandSet_Analyze_SameNameSameSource(t *testing.T) {
	t.Parallel()

	// Test: Multiple commands with same name from same source are NOT ambiguous
	// (This could happen with command overloading or errors)
	set := NewDiscoveredCommandSet()

	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"})
	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"}) // Same name, same source

	set.Analyze()

	// Should NOT be marked as ambiguous since they're from the same source
	if set.AmbiguousNames["build"] {
		t.Error("Commands with same name from same source should not be marked as ambiguous")
	}
}

func TestDiscoveredCommandSet_MultiSourceAggregation(t *testing.T) {
	t.Parallel()

	// Test T011: Multi-source aggregation for User Story 1
	// Simulates commands from invowkfile + two modules

	set := NewDiscoveredCommandSet()

	// Commands from invowkfile
	set.Add(&CommandInfo{
		Name:       "hello",
		SimpleName: "hello",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})

	// Commands from foo module
	set.Add(&CommandInfo{
		Name:       "foo build",
		SimpleName: "build",
		SourceID:   "foo",
		ModuleID:   "io.invowk.foo",
		Source:     SourceModule,
	})
	set.Add(&CommandInfo{
		Name:       "foo deploy",
		SimpleName: "deploy",
		SourceID:   "foo",
		ModuleID:   "io.invowk.foo",
		Source:     SourceModule,
	})

	// Commands from bar module
	set.Add(&CommandInfo{
		Name:       "bar test",
		SimpleName: "test",
		SourceID:   "bar",
		ModuleID:   "io.invowk.bar",
		Source:     SourceModule,
	})

	set.Analyze()

	// Test aggregation counts
	if len(set.Commands) != 5 {
		t.Errorf("len(Commands) = %d, want 5", len(set.Commands))
	}

	// Test source grouping
	if len(set.BySource["invowkfile"]) != 2 {
		t.Errorf("len(BySource[invowkfile]) = %d, want 2", len(set.BySource["invowkfile"]))
	}
	if len(set.BySource["foo"]) != 2 {
		t.Errorf("len(BySource[foo]) = %d, want 2", len(set.BySource["foo"]))
	}
	if len(set.BySource["bar"]) != 1 {
		t.Errorf("len(BySource[bar]) = %d, want 1", len(set.BySource["bar"]))
	}

	// Test source order (invowkfile first, then alphabetically)
	if set.SourceOrder[0] != "invowkfile" {
		t.Errorf("SourceOrder[0] = %s, want invowkfile", set.SourceOrder[0])
	}
	if set.SourceOrder[1] != "bar" {
		t.Errorf("SourceOrder[1] = %s, want bar", set.SourceOrder[1])
	}
	if set.SourceOrder[2] != "foo" {
		t.Errorf("SourceOrder[2] = %s, want foo", set.SourceOrder[2])
	}

	// Test ambiguity detection - "deploy" is in both invowkfile and foo
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous")
	}
	// "hello", "build", "test" are unique
	if set.AmbiguousNames["hello"] {
		t.Error("'hello' should not be marked as ambiguous")
	}
	if set.AmbiguousNames["build"] {
		t.Error("'build' should not be marked as ambiguous")
	}
	if set.AmbiguousNames["test"] {
		t.Error("'test' should not be marked as ambiguous")
	}

	// Test IsAmbiguous flag on individual commands
	for _, cmd := range set.Commands {
		if cmd.SimpleName == "deploy" {
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy' from %s should be marked as ambiguous", cmd.SourceID)
			}
		} else {
			if cmd.IsAmbiguous {
				t.Errorf("Command '%s' from %s should not be marked as ambiguous", cmd.SimpleName, cmd.SourceID)
			}
		}
	}
}

func TestDiscoveredCommandSet_HierarchicalAmbiguity(t *testing.T) {
	t.Parallel()

	// Test T035: Hierarchical command ambiguity detection (User Story 4)
	// Tests that subcommands like "deploy staging" are tracked separately from "deploy"
	// and ambiguity is detected at the correct hierarchical level

	set := NewDiscoveredCommandSet()

	// Commands from invowkfile - parent "deploy" and subcommand "deploy staging"
	set.Add(&CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy staging",
		SimpleName: "deploy staging",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy local",
		SimpleName: "deploy local",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})

	// Commands from foo module - conflicting parent "deploy"
	set.Add(&CommandInfo{
		Name:       "foo deploy",
		SimpleName: "deploy",
		SourceID:   "foo",
		ModuleID:   "io.invowk.foo",
		Source:     SourceModule,
	})

	// Commands from bar module - conflicting "deploy staging" but unique "deploy production"
	set.Add(&CommandInfo{
		Name:       "bar deploy staging",
		SimpleName: "deploy staging",
		SourceID:   "bar",
		ModuleID:   "io.invowk.bar",
		Source:     SourceModule,
	})
	set.Add(&CommandInfo{
		Name:       "bar deploy production",
		SimpleName: "deploy production",
		SourceID:   "bar",
		ModuleID:   "io.invowk.bar",
		Source:     SourceModule,
	})

	set.Analyze()

	// Test that "deploy" is ambiguous (invowkfile vs foo)
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous (exists in invowkfile and foo)")
	}

	// Test that "deploy staging" is ambiguous (invowkfile vs bar)
	if !set.AmbiguousNames["deploy staging"] {
		t.Error("'deploy staging' should be marked as ambiguous (exists in invowkfile and bar)")
	}

	// Test that "deploy local" is NOT ambiguous (only in invowkfile)
	if set.AmbiguousNames["deploy local"] {
		t.Error("'deploy local' should NOT be marked as ambiguous (only in invowkfile)")
	}

	// Test that "deploy production" is NOT ambiguous (only in bar)
	if set.AmbiguousNames["deploy production"] {
		t.Error("'deploy production' should NOT be marked as ambiguous (only in bar)")
	}

	// Verify IsAmbiguous flag on individual commands
	for _, cmd := range set.Commands {
		switch cmd.SimpleName {
		case "deploy":
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy' from %s should be marked as ambiguous", cmd.SourceID)
			}
		case "deploy staging":
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy staging' from %s should be marked as ambiguous", cmd.SourceID)
			}
		case "deploy local", "deploy production":
			if cmd.IsAmbiguous {
				t.Errorf("Command '%s' from %s should NOT be marked as ambiguous", cmd.SimpleName, cmd.SourceID)
			}
		}
	}

	// Verify correct command counts per SimpleName
	if len(set.BySimpleName["deploy"]) != 2 {
		t.Errorf("Expected 2 commands with SimpleName 'deploy', got %d", len(set.BySimpleName["deploy"]))
	}
	if len(set.BySimpleName["deploy staging"]) != 2 {
		t.Errorf("Expected 2 commands with SimpleName 'deploy staging', got %d", len(set.BySimpleName["deploy staging"]))
	}
	if len(set.BySimpleName["deploy local"]) != 1 {
		t.Errorf("Expected 1 command with SimpleName 'deploy local', got %d", len(set.BySimpleName["deploy local"]))
	}
	if len(set.BySimpleName["deploy production"]) != 1 {
		t.Errorf("Expected 1 command with SimpleName 'deploy production', got %d", len(set.BySimpleName["deploy production"]))
	}
}

func TestModuleCollisionError(t *testing.T) {
	t.Parallel()

	err := &ModuleCollisionError{
		ModuleID:     "io.example.tools",
		FirstSource:  "/path/to/first",
		SecondSource: "/path/to/second",
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "io.example.tools") {
		t.Error("error message should contain module ID")
	}
	if !strings.Contains(errMsg, "/path/to/first") {
		t.Error("error message should contain first source")
	}
	if !strings.Contains(errMsg, "/path/to/second") {
		t.Error("error message should contain second source")
	}
	if !strings.Contains(errMsg, "alias") {
		t.Error("error message should mention alias as a solution")
	}

	if err.Unwrap() != ErrModuleCollision {
		t.Errorf("Unwrap() should return ErrModuleCollision, got %v", err.Unwrap())
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
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.module1"}},
			},
			{
				Path:       "/path/to/module2",
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.module2"}},
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
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.same"}},
			},
			{
				Path:       "/path/to/module2",
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.same"}},
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
		if collisionErr != nil && collisionErr.ModuleID != "io.example.same" {
			t.Errorf("ModuleID = %s, want io.example.same", collisionErr.ModuleID)
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
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.same"}},
				Module:     &invowkmod.Module{Path: "/path/to/module1.invowkmod"},
			},
			{
				Path:       "/path/to/module2.invowkmod/invowkfile.cue",
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.same"}},
				Module:     &invowkmod.Module{Path: "/path/to/module2.invowkmod"},
			},
		}

		err := dAlias.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should not return error when alias resolves collision: %v", err)
		}
	})

	t.Run("SkipsFilesWithErrors", func(t *testing.T) {
		t.Parallel()

		files := []*DiscoveredFile{
			{
				Path:       "/path/to/module1",
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.same"}},
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
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.module1"}},
			},
			{
				Path:       "/path/to/module2",
				Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: ""}}, // Empty module ID
			},
		}

		err := d.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should skip files without module ID: %v", err)
		}
	})
}

func TestGetEffectiveModuleID(t *testing.T) {
	t.Parallel()

	t.Run("WithoutAlias", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:       "/path/to/module",
			Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.original"}},
		}

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID != "io.example.original" {
			t.Errorf("GetEffectiveModuleID() = %s, want io.example.original", moduleID)
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
			Invowkfile: &invowkfile.Invowkfile{Metadata: &invowkfile.ModuleMetadata{Module: "io.example.original"}},
			Module:     &invowkmod.Module{Path: "/path/to/module.invowkmod"},
		}

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID != "io.example.aliased" {
			t.Errorf("GetEffectiveModuleID() = %s, want io.example.aliased", moduleID)
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

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID != "" {
			t.Errorf("GetEffectiveModuleID() = %s, want empty string", moduleID)
		}
	})
}

func TestDiscoverAll_CurrentDirInvowkfileTakesPrecedenceOverModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a regular invowkfile in current directory
	currentContent := `
cmds: [{name: "cmd", implementations: [{script: "echo current", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
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
		implementations: [{script: "echo root", runtimes: [{name: "virtual"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
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
		implementations: [{script: "echo valid", runtimes: [{name: "virtual"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
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
		implementations: [{script: "echo reserved", runtimes: [{name: "virtual"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
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
		WithCommandsDir(filepath.Join(homeDir, ".invowk", "cmds")),
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
