// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/internal/testutil"
)

func TestSourceModule_String(t *testing.T) {
	if got := SourceModule.String(); got != "module" {
		t.Errorf("SourceModule.String() = %s, want module", got)
	}
}

func TestDiscoverAll_FindsModulesInCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid module in the temp directory (using new two-file format)
	moduleDir := filepath.Join(tmpDir, "mycommands.invkmod")
	createValidDiscoveryModule(t, moduleDir, "mycommands", "packed-cmd")

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME to avoid finding real user commands
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourceModule && f.Module != nil {
			if f.Module.Name() == "mycommands" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find module in current directory")
	}
}

func TestDiscoverAll_FindsModulesInUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user commands directory with a module (using new two-file format)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	moduleDir := filepath.Join(userCmdsDir, "userpack.invkmod")
	createValidDiscoveryModule(t, moduleDir, "userpack", "user-packed-cmd")

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	// Change to work directory
	restoreWd := testutil.MustChdir(t, workDir)
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

	found := false
	for _, f := range files {
		if f.Source == SourceModule && f.Module != nil {
			if f.Module.Name() == "userpack" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find module in user commands directory")
	}
}

func TestDiscoverAll_FindsModulesInConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config search path with a module (using new two-file format)
	searchPath := filepath.Join(tmpDir, "custom-commands")
	moduleDir := filepath.Join(searchPath, "configpack.invkmod")
	createValidDiscoveryModule(t, moduleDir, "configpack", "config-packed-cmd")

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	// Change to work directory
	restoreWd := testutil.MustChdir(t, workDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: moduleDir},
	}
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourceModule && f.Module != nil {
			if f.Module.Name() == "configpack" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find module in configured includes")
	}
}

func TestDiscoveredFile_ModuleField(t *testing.T) {
	df := &DiscoveredFile{
		Path:   "/path/to/module/invkfile.cue",
		Source: SourceModule,
	}

	if df.Module != nil {
		t.Error("Module should be nil by default")
	}

	if df.Source != SourceModule {
		t.Errorf("Source = %v, want SourceModule", df.Source)
	}
}

func TestDiscoverCommands_FromModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid module with the new two-file format
	moduleDir := filepath.Join(tmpDir, "testpack.invkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	// Create invkmod.cue with metadata
	invkmodContent := `module: "testpack"
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invkmod.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [
	{name: "cmd1", description: "First command", implementations: [{script: "echo 1", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "cmd2", description: "Second command", implementations: [{script: "echo 2", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}
	commands := result.Set.Commands

	// Should find both commands from the module
	foundCmd1 := false
	foundCmd2 := false
	for _, cmd := range commands {
		if cmd.Name == "testpack cmd1" && cmd.Source == SourceModule {
			foundCmd1 = true
		}
		if cmd.Name == "testpack cmd2" && cmd.Source == SourceModule {
			foundCmd2 = true
		}
	}

	if !foundCmd1 {
		t.Error("DiscoverCommands() did not find 'testpack cmd1' from module")
	}
	if !foundCmd2 {
		t.Error("DiscoverCommands() did not find 'testpack cmd2' from module")
	}
}

func TestDiscoverAll_SkipsInvalidModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid module (missing invkmod.cue - required now)
	invalidModuleDir := filepath.Join(tmpDir, "invalid.invkmod")
	if err := os.MkdirAll(invalidModuleDir, 0o755); err != nil {
		t.Fatalf("failed to create invalid module dir: %v", err)
	}

	// Create a valid module using new two-file format
	validModuleDir := filepath.Join(tmpDir, "valid.invkmod")
	createValidDiscoveryModule(t, validModuleDir, "valid", "cmd")

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

	// Should only find the valid module
	moduleCount := 0
	for _, f := range files {
		if f.Source == SourceModule {
			moduleCount++
			if f.Module != nil && f.Module.Name() != "valid" {
				t.Errorf("unexpected module found: %s", f.Module.Name())
			}
		}
	}

	if moduleCount != 1 {
		t.Errorf("expected 1 module, found %d", moduleCount)
	}
}

func TestLoadAll_ParsesModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid module with new two-file format
	moduleDir := filepath.Join(tmpDir, "parsepack.invkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	// Create invkmod.cue with metadata
	invkmodContent := `module: "parsepack"
version: "1.0.0"
description: "A test module"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invkmod.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() returned error: %v", err)
	}

	// Find the module file
	var moduleFile *DiscoveredFile
	for _, f := range files {
		if f.Source == SourceModule {
			moduleFile = f
			break
		}
	}

	if moduleFile == nil {
		t.Fatal("LoadAll() did not find module")
	}

	if moduleFile.Invkfile == nil {
		t.Fatal("LoadAll() did not parse module invkfile")
	}

	// In the new format, description is in Metadata, not Invkfile
	if moduleFile.Invkfile.Metadata == nil {
		t.Fatal("Invkfile.Metadata should not be nil for module-parsed file")
	}

	if moduleFile.Invkfile.Metadata.Description != "A test module" {
		t.Errorf("Invkfile.Metadata.Description = %s, want 'A test module'", moduleFile.Invkfile.Metadata.Description)
	}

	// Verify that ModulePath is set on the parsed invkfile
	if !moduleFile.Invkfile.IsFromModule() {
		t.Error("Invkfile.IsFromModule() should return true for module-parsed file")
	}
}

func TestLoadFirst_LoadsModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid module (but no regular invkfile in root)
	moduleDir := filepath.Join(tmpDir, "firstpack.invkmod")
	createValidDiscoveryModule(t, moduleDir, "firstpack", "first")

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	file, err := d.LoadFirst()
	if err != nil {
		t.Fatalf("LoadFirst() returned error: %v", err)
	}

	if file.Source != SourceModule {
		t.Errorf("LoadFirst().Source = %v, want SourceModule", file.Source)
	}

	if file.Invkfile == nil {
		t.Fatal("LoadFirst() did not parse module invkfile")
	}

	if file.Module == nil {
		t.Fatal("LoadFirst().Module should not be nil for module source")
	}

	if file.Module.Name() != "firstpack" {
		t.Errorf("Module.Name = %s, want 'firstpack'", file.Module.Name())
	}
}

func TestDiscoverAll_ConfigIncludesPrecedeUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config-include module
	configModuleDir := filepath.Join(tmpDir, "custom-path", "configmod.invkmod")
	createValidDiscoveryModule(t, configModuleDir, "configmod", "config-cmd")

	// Create a user-dir module
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	userModuleDir := filepath.Join(userCmdsDir, "usermod.invkmod")
	createValidDiscoveryModule(t, userModuleDir, "usermod", "user-cmd")

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	restoreWd := testutil.MustChdir(t, workDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: configModuleDir},
	}
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Config-include module should appear before user-dir module
	configIdx := -1
	userIdx := -1
	for i, f := range files {
		if f.Module == nil {
			continue
		}
		if f.Module.Name() == "configmod" {
			configIdx = i
		}
		if f.Module.Name() == "usermod" {
			userIdx = i
		}
	}

	if configIdx == -1 {
		t.Fatal("config-include module not found in discovered files")
	}
	if userIdx == -1 {
		t.Fatal("user-dir module not found in discovered files")
	}
	if configIdx >= userIdx {
		t.Errorf("config-include module (index %d) should appear before user-dir module (index %d)", configIdx, userIdx)
	}
}

// createValidDiscoveryModule creates a module with the new two-file format (invkmod.cue + invkfile.cue)
func createValidDiscoveryModule(t *testing.T, moduleDir, moduleID, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	// Create invkmod.cue with metadata
	invkmodContent := `module: "` + moduleID + `"
version: "1.0.0"
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
