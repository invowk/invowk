// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
)

func TestDiscoveredCommandSet_Add(t *testing.T) {
	// Test T006: Add method
	set := NewDiscoveredCommandSet()

	cmd1 := &CommandInfo{
		Name:       "hello",
		SimpleName: "hello",
		SourceID:   "invkfile",
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
	if len(set.BySource["invkfile"]) != 1 {
		t.Errorf("len(BySource[invkfile]) = %d, want 1", len(set.BySource["invkfile"]))
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
	// Test T007: Analyze method with no conflicts
	set := NewDiscoveredCommandSet()

	set.Add(&CommandInfo{SimpleName: "hello", SourceID: "invkfile"})
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

	// SourceOrder should be sorted: invkfile first, then alphabetically
	if set.SourceOrder[0] != "invkfile" {
		t.Errorf("SourceOrder[0] = %s, want invkfile", set.SourceOrder[0])
	}
	if set.SourceOrder[1] != "bar" {
		t.Errorf("SourceOrder[1] = %s, want bar", set.SourceOrder[1])
	}
	if set.SourceOrder[2] != "foo" {
		t.Errorf("SourceOrder[2] = %s, want foo", set.SourceOrder[2])
	}
}

func TestDiscoveredCommandSet_Analyze_WithConflicts(t *testing.T) {
	// Test T007: Analyze method with conflicts
	set := NewDiscoveredCommandSet()

	// "deploy" exists in both invkfile and foo
	set.Add(&CommandInfo{SimpleName: "hello", SourceID: "invkfile"})
	set.Add(&CommandInfo{SimpleName: "deploy", SourceID: "invkfile"})
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
	// Test T011: Multi-source aggregation for User Story 1
	// Simulates commands from invkfile + two modules

	set := NewDiscoveredCommandSet()

	// Commands from invkfile
	set.Add(&CommandInfo{
		Name:       "hello",
		SimpleName: "hello",
		SourceID:   "invkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "invkfile",
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
	if len(set.BySource["invkfile"]) != 2 {
		t.Errorf("len(BySource[invkfile]) = %d, want 2", len(set.BySource["invkfile"]))
	}
	if len(set.BySource["foo"]) != 2 {
		t.Errorf("len(BySource[foo]) = %d, want 2", len(set.BySource["foo"]))
	}
	if len(set.BySource["bar"]) != 1 {
		t.Errorf("len(BySource[bar]) = %d, want 1", len(set.BySource["bar"]))
	}

	// Test source order (invkfile first, then alphabetically)
	if set.SourceOrder[0] != "invkfile" {
		t.Errorf("SourceOrder[0] = %s, want invkfile", set.SourceOrder[0])
	}
	if set.SourceOrder[1] != "bar" {
		t.Errorf("SourceOrder[1] = %s, want bar", set.SourceOrder[1])
	}
	if set.SourceOrder[2] != "foo" {
		t.Errorf("SourceOrder[2] = %s, want foo", set.SourceOrder[2])
	}

	// Test ambiguity detection - "deploy" is in both invkfile and foo
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
	// Test T035: Hierarchical command ambiguity detection (User Story 4)
	// Tests that subcommands like "deploy staging" are tracked separately from "deploy"
	// and ambiguity is detected at the correct hierarchical level

	set := NewDiscoveredCommandSet()

	// Commands from invkfile - parent "deploy" and subcommand "deploy staging"
	set.Add(&CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "invkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy staging",
		SimpleName: "deploy staging",
		SourceID:   "invkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy local",
		SimpleName: "deploy local",
		SourceID:   "invkfile",
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

	// Test that "deploy" is ambiguous (invkfile vs foo)
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous (exists in invkfile and foo)")
	}

	// Test that "deploy staging" is ambiguous (invkfile vs bar)
	if !set.AmbiguousNames["deploy staging"] {
		t.Error("'deploy staging' should be marked as ambiguous (exists in invkfile and bar)")
	}

	// Test that "deploy local" is NOT ambiguous (only in invkfile)
	if set.AmbiguousNames["deploy local"] {
		t.Error("'deploy local' should NOT be marked as ambiguous (only in invkfile)")
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
	err := &ModuleCollisionError{
		ModuleID:     "io.example.tools",
		FirstSource:  "/path/to/first",
		SecondSource: "/path/to/second",
	}

	errMsg := err.Error()
	if !containsString(errMsg, "io.example.tools") {
		t.Error("error message should contain module ID")
	}
	if !containsString(errMsg, "/path/to/first") {
		t.Error("error message should contain first source")
	}
	if !containsString(errMsg, "/path/to/second") {
		t.Error("error message should contain second source")
	}
	if !containsString(errMsg, "alias") {
		t.Error("error message should mention alias as a solution")
	}
}

func TestCheckModuleCollisions(t *testing.T) {
	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("NoCollision", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/module1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.module1"}},
			},
			{
				Path:     "/path/to/module2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.module2"}},
			},
		}

		err := d.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() returned unexpected error: %v", err)
		}
	})

	t.Run("WithCollision", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/module1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.same"}},
			},
			{
				Path:     "/path/to/module2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.same"}},
			},
		}

		err := d.CheckModuleCollisions(files)
		if err == nil {
			t.Error("CheckModuleCollisions() should return error for collision")
		}

		var collisionErr *ModuleCollisionError
		if !errors.As(err, &collisionErr) {
			t.Errorf("error should be ModuleCollisionError, got %T", err)
		}
		if collisionErr != nil && collisionErr.ModuleID != "io.example.same" {
			t.Errorf("ModuleID = %s, want io.example.same", collisionErr.ModuleID)
		}
	})

	t.Run("CollisionResolvedByAlias", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.ModuleAliases = map[string]string{
			"/path/to/module1": "io.example.alias1",
		}
		dAlias := New(cfg)

		files := []*DiscoveredFile{
			{
				Path:     "/path/to/module1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.same"}},
			},
			{
				Path:     "/path/to/module2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.same"}},
			},
		}

		err := dAlias.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should not return error when alias resolves collision: %v", err)
		}
	})

	t.Run("SkipsFilesWithErrors", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/module1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.same"}},
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
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/module1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.module1"}},
			},
			{
				Path:     "/path/to/module2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: ""}}, // Empty module ID
			},
		}

		err := d.CheckModuleCollisions(files)
		if err != nil {
			t.Errorf("CheckModuleCollisions() should skip files without module ID: %v", err)
		}
	})
}

func TestGetEffectiveModuleID(t *testing.T) {
	t.Run("WithoutAlias", func(t *testing.T) {
		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:     "/path/to/module",
			Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.original"}},
		}

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID != "io.example.original" {
			t.Errorf("GetEffectiveModuleID() = %s, want io.example.original", moduleID)
		}
	})

	t.Run("WithAlias", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.ModuleAliases = map[string]string{
			"/path/to/module": "io.example.aliased",
		}
		d := New(cfg)

		file := &DiscoveredFile{
			Path:     "/path/to/module",
			Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkmod{Module: "io.example.original"}},
		}

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID != "io.example.aliased" {
			t.Errorf("GetEffectiveModuleID() = %s, want io.example.aliased", moduleID)
		}
	})

	t.Run("WithNilInvkfile", func(t *testing.T) {
		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:     "/path/to/module",
			Invkfile: nil,
		}

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID != "" {
			t.Errorf("GetEffectiveModuleID() = %s, want empty string", moduleID)
		}
	})
}

func TestDiscoverAll_CurrentDirInvkfileTakesPrecedenceOverModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular invkfile in current directory
	// Standalone invkfiles cannot have module/version - those fields only go in invkmod.cue
	currentContent := `
cmds: [{name: "cmd", implementations: [{script: "echo current", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0o644); err != nil {
		t.Fatalf("failed to write current invkfile: %v", err)
	}

	// Create a module in the same directory using new two-file format
	moduleDir := filepath.Join(tmpDir, "apack.invkmod")
	createValidCollisionTestModule(t, moduleDir, "apack", "cmd")

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

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
		t.Error("DiscoverAll() did not find invkfile in current directory")
	}
	if !foundModule {
		t.Error("DiscoverAll() did not find module")
	}
}

func TestDiscoverAll_SkipsReservedModuleName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invkfile.cue in tmpDir
	invkfileContent := `cmds: [{
		name: "root-cmd"
		description: "Root command"
		implementations: [{script: "echo root", runtimes: [{name: "virtual"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	}]`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to create invkfile: %v", err)
	}

	// Create a valid module
	validModDir := filepath.Join(tmpDir, "valid.invkmod")
	if err := os.MkdirAll(validModDir, 0o755); err != nil {
		t.Fatalf("failed to create valid module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validModDir, "invkmod.cue"), []byte(`module: "valid"
version: "1.0"
`), 0o644); err != nil {
		t.Fatalf("failed to create invkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validModDir, "invkfile.cue"), []byte(`cmds: [{
		name: "valid-cmd"
		description: "Valid command"
		implementations: [{script: "echo valid", runtimes: [{name: "virtual"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	}]`), 0o644); err != nil {
		t.Fatalf("failed to create invkfile.cue: %v", err)
	}

	// Create a module with reserved name "invkfile" (FR-015)
	reservedModDir := filepath.Join(tmpDir, "invkfile.invkmod")
	if err := os.MkdirAll(reservedModDir, 0o755); err != nil {
		t.Fatalf("failed to create reserved module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reservedModDir, "invkmod.cue"), []byte(`module: "invkfile"
version: "1.0"
`), 0o644); err != nil {
		t.Fatalf("failed to create invkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reservedModDir, "invkfile.cue"), []byte(`cmds: [{
		name: "reserved-cmd"
		description: "Reserved command"
		implementations: [{script: "echo reserved", runtimes: [{name: "virtual"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	}]`), 0o644); err != nil {
		t.Fatalf("failed to create invkfile.cue: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Set HOME to isolated directory
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	restoreHome := testutil.SetHomeDir(t, homeDir)
	defer restoreHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}

	// Should find invkfile.cue and valid module, but NOT the reserved module
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
			if f.Module.Name() == "invkfile" {
				foundReservedModule = true
			}
		}
	}

	if !foundCurrentDir {
		t.Error("DiscoverAll() did not find invkfile in current directory")
	}
	if !foundValidModule {
		t.Error("DiscoverAll() did not find valid module")
	}
	if foundReservedModule {
		t.Error("DiscoverAll() should skip module with reserved name 'invkfile' (FR-015)")
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createValidCollisionTestModule creates a module with the new two-file format (invkmod.cue + invkfile.cue)
func createValidCollisionTestModule(t *testing.T, moduleDir, moduleID, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	// Create invkmod.cue with metadata
	invkmodContent := `module: "` + moduleID + `"
version: "1.0"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invkmod.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [{name: "` + cmdName + `", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}
}
