// SPDX-License-Identifier: MPL-2.0

// Package discovery handles finding and loading invkfiles from various locations.
package discovery

import (
	"errors"
	"invowk-cli/internal/config"
	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setHomeDirEnv sets the appropriate HOME environment variable based on platform
// and returns a cleanup function to restore the original value
func setHomeDirEnv(t *testing.T, dir string) func() {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		return testutil.MustSetenv(t, "USERPROFILE", dir)
	default: // Linux, macOS
		return testutil.MustSetenv(t, "HOME", dir)
	}
}

func TestSource_String(t *testing.T) {
	tests := []struct {
		source   Source
		expected string
	}{
		{SourceCurrentDir, "current directory"},
		{SourceUserDir, "user commands (~/.invowk/cmds)"},
		{SourceConfigPath, "configured search path"},
		{Source(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.source.String(); got != tt.expected {
				t.Errorf("Source(%d).String() = %s, want %s", tt.source, got, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()
	d := New(cfg)

	if d == nil {
		t.Fatal("New() returned nil")
	}

	if d.cfg != cfg {
		t.Error("New() did not set cfg correctly")
	}
}

func TestDiscoveredFile_Fields(t *testing.T) {
	df := &DiscoveredFile{
		Path:   "/path/to/invkfile.cue",
		Source: SourceCurrentDir,
	}

	if df.Path != "/path/to/invkfile.cue" {
		t.Errorf("Path = %s, want /path/to/invkfile.cue", df.Path)
	}

	if df.Source != SourceCurrentDir {
		t.Errorf("Source = %v, want SourceCurrentDir", df.Source)
	}

	if df.Invkfile != nil {
		t.Error("Invkfile should be nil by default")
	}

	if df.Error != nil {
		t.Error("Error should be nil by default")
	}
}

func TestCommandInfo_Fields(t *testing.T) {
	cmd := &invkfile.Command{
		Name:        "build",
		Description: "Build the project",
	}

	inv := &invkfile.Invkfile{
		Commands: []invkfile.Command{*cmd},
	}

	ci := &CommandInfo{
		Name:        "build",
		Description: "Build the project",
		Source:      SourceCurrentDir,
		FilePath:    "/path/to/invkfile.cue",
		Command:     cmd,
		Invkfile:    inv,
	}

	if ci.Name != "build" {
		t.Errorf("Name = %s, want build", ci.Name)
	}

	if ci.Description != "Build the project" {
		t.Errorf("Description = %s, want 'Build the project'", ci.Description)
	}

	if ci.Source != SourceCurrentDir {
		t.Errorf("Source = %v, want SourceCurrentDir", ci.Source)
	}
}

func TestDiscoverAll_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME to avoid finding real user commands
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Should return empty slice (no invkfiles found)
	if len(files) != 0 {
		t.Errorf("DiscoverAll() returned %d files, want 0", len(files))
	}
}

func TestDiscoverAll_FindsInvkfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invkfile.cue in the temp directory
	invkfileContent := `
cmds: [
	{
		name: "test"
		description: "Run tests"
		implementations: [{
			script: "echo test"
			runtimes: [{name: "native"}]
		}]
	}
]
`
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME to avoid finding real user commands
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("DiscoverAll() returned 0 files, want at least 1")
	}

	found := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find invkfile in current directory")
	}
}

func TestDiscoverAll_FindsInvkfileWithoutExtension(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invkfile (without .cue extension) in the temp directory
	invkfileContent := `
cmds: [
	{
		name: "test"
		description: "Run tests"
		implementations: [{
			script: "echo test"
			runtimes: [{name: "native"}]
		}]
	}
]
`
	invkfilePath := filepath.Join(tmpDir, "invkfile")
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME to avoid finding real user commands
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("DiscoverAll() returned 0 files, want at least 1")
	}
}

func TestDiscoverAll_PrefersInvkfileCue(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both invkfile and invkfile.cue
	content := `
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Should find invkfile.cue (preferred) in current dir
	found := false
	for _, f := range files {
		if f.Source == SourceCurrentDir && filepath.Base(f.Path) == "invkfile.cue" {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() should prefer invkfile.cue over invkfile")
	}
}

func TestDiscoverAll_FindsInUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user commands directory
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	if err := os.MkdirAll(userCmdsDir, 0o755); err != nil {
		t.Fatalf("failed to create user cmds dir: %v", err)
	}

	// Create an invkfile in user commands
	content := `
cmds: [{name: "user-cmd", implementations: [{script: "echo user", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory (which has no invkfile)
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourceUserDir {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find invkfile in user commands directory")
	}
}

func TestDiscoverAll_FindsInConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config search path directory
	searchPath := filepath.Join(tmpDir, "custom-commands")
	if err := os.MkdirAll(searchPath, 0o755); err != nil {
		t.Fatalf("failed to create search path dir: %v", err)
	}

	// Create an invkfile in search path
	content := `
cmds: [{name: "custom-cmd", implementations: [{script: "echo custom", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(searchPath, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory (which has no invkfile)
	emptyDir := filepath.Join(tmpDir, "empty")
	testutil.MustMkdirAll(t, emptyDir, 0o755)
	restoreWd := testutil.MustChdir(t, emptyDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	cfg.SearchPaths = []string{searchPath}
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourceConfigPath {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find invkfile in configured search path")
	}
}

func TestLoadFirst_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	_, err := d.LoadFirst()
	if err == nil {
		t.Error("LoadFirst() should return error when no files found")
	}
}

func TestLoadFirst_WithValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now only contains commands (module metadata is in invkmod.cue for modules)
	content := `
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	file, err := d.LoadFirst()
	if err != nil {
		t.Fatalf("LoadFirst() returned error: %v", err)
	}

	if file.Invkfile == nil {
		t.Error("LoadFirst() did not parse the invkfile")
	}

	if len(file.Invkfile.Commands) != 1 {
		t.Errorf("Invkfile should have 1 command, got %d", len(file.Invkfile.Commands))
	}
}

func TestLoadAll_WithMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current dir invkfile (no module metadata - it belongs in invkmod.cue)
	content := `
cmds: [{name: "current", implementations: [{script: "echo current", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile (no module metadata - it belongs in invkmod.cue)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	testutil.MustMkdirAll(t, userCmdsDir, 0o755)
	userContent := `
cmds: [{name: "user", implementations: [{script: "echo user", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(userContent), 0o644); err != nil {
		t.Fatalf("failed to write user invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() returned error: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("LoadAll() returned %d files, want at least 2", len(files))
	}

	// All files should be parsed
	for _, f := range files {
		if f.Invkfile == nil && f.Error == nil {
			t.Errorf("file %s was not parsed and has no error", f.Path)
		}
	}
}

func TestDiscoverCommands(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now contains only commands - module metadata is in invkmod.cue for modules
	content := `
cmds: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	commands, err := d.DiscoverCommands()
	if err != nil {
		t.Fatalf("DiscoverCommands() returned error: %v", err)
	}

	if len(commands) != 2 {
		t.Errorf("DiscoverCommands() returned %d commands, want 2", len(commands))
	}

	// Commands should be sorted by name (no module prefix for current-dir invkfiles)
	if len(commands) >= 2 {
		if commands[0].Name != "build" || commands[1].Name != "test" {
			t.Errorf("commands not sorted correctly: got %s, %s", commands[0].Name, commands[1].Name)
		}
	}
}

func TestGetCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now contains only commands - no module metadata
	content := `
cmds: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("ExistingCommand", func(t *testing.T) {
		// Current-dir invkfiles don't have module prefix
		cmd, err := d.GetCommand("build")
		if err != nil {
			t.Fatalf("GetCommand() returned error: %v", err)
		}

		if cmd.Name != "build" {
			t.Errorf("GetCommand().Name = %s, want 'build'", cmd.Name)
		}
	})

	t.Run("NonExistentCommand", func(t *testing.T) {
		_, err := d.GetCommand("nonexistent")
		if err == nil {
			t.Error("GetCommand() should return error for non-existent command")
		}
	})
}

func TestGetCommandsWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now contains only commands - no module metadata
	content := `
cmds: [
	{name: "build", implementations: [{script: "go build", runtimes: [{name: "native"}]}]},
	{name: "build-dev", implementations: [{script: "go build -tags dev", runtimes: [{name: "native"}]}]},
	{name: "test", implementations: [{script: "go test", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("EmptyPrefix", func(t *testing.T) {
		commands, err := d.GetCommandsWithPrefix("")
		if err != nil {
			t.Fatalf("GetCommandsWithPrefix() returned error: %v", err)
		}

		if len(commands) != 3 {
			t.Errorf("GetCommandsWithPrefix('') returned %d commands, want 3", len(commands))
		}
	})

	t.Run("BuildPrefix", func(t *testing.T) {
		// Current-dir invkfiles have no module prefix
		commands, err := d.GetCommandsWithPrefix("build")
		if err != nil {
			t.Fatalf("GetCommandsWithPrefix() returned error: %v", err)
		}

		if len(commands) != 2 {
			t.Errorf("GetCommandsWithPrefix('build') returned %d commands, want 2", len(commands))
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		commands, err := d.GetCommandsWithPrefix("xyz")
		if err != nil {
			t.Fatalf("GetCommandsWithPrefix() returned error: %v", err)
		}

		if len(commands) != 0 {
			t.Errorf("GetCommandsWithPrefix('xyz') returned %d commands, want 0", len(commands))
		}
	})
}

func TestDiscoverCommands_Precedence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current dir invkfile with "build" command
	// Without module field, commands are named directly (e.g., "build" not "project build")
	// When same command exists in multiple sources, current dir takes precedence
	currentContent := `
cmds: [{name: "build", description: "Current build", implementations: [{script: "echo current", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile with same "build" command
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	testutil.MustMkdirAll(t, userCmdsDir, 0o755)
	userContent := `
cmds: [{name: "build", description: "User build", implementations: [{script: "echo user", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(userContent), 0o644); err != nil {
		t.Fatalf("failed to write user invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	commands, err := d.DiscoverCommands()
	if err != nil {
		t.Fatalf("DiscoverCommands() returned error: %v", err)
	}

	// Should only have one "build" command (from current directory, higher precedence)
	// With no module field, command is just "build" not "project build"
	buildCount := 0
	var buildCmd *CommandInfo
	for _, cmd := range commands {
		if cmd.Name == "build" {
			buildCount++
			buildCmd = cmd
		}
	}

	if buildCount != 1 {
		t.Errorf("expected 1 'build' command, got %d", buildCount)
	}

	if buildCmd != nil && buildCmd.Source != SourceCurrentDir {
		t.Errorf("build command should be from current directory, got %v", buildCmd.Source)
	}
}

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
	cleanupHome := setHomeDirEnv(t, tmpDir)
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
	cleanupHome := setHomeDirEnv(t, tmpDir)
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
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	cfg.SearchPaths = []string{searchPath}
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
		t.Error("DiscoverAll() did not find module in configured search path")
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
version: "1.0"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invkmod.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [
	{name: "cmd1", description: "First command", implementations: [{script: "echo 1", runtimes: [{name: "native"}]}]},
	{name: "cmd2", description: "Second command", implementations: [{script: "echo 2", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := setHomeDirEnv(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	commands, err := d.DiscoverCommands()
	if err != nil {
		t.Fatalf("DiscoverCommands() returned error: %v", err)
	}

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
	cleanupHome := setHomeDirEnv(t, tmpDir)
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
version: "1.0"
description: "A test module"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invkmod.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := setHomeDirEnv(t, tmpDir)
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
	cleanupHome := setHomeDirEnv(t, tmpDir)
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

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createValidDiscoveryModule creates a module with the new two-file format (invkmod.cue + invkfile.cue)
func createValidDiscoveryModule(t *testing.T, moduleDir, moduleID, cmdName string) {
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
	invkfileContent := `cmds: [{name: "` + cmdName + `", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]`
	if err := os.WriteFile(filepath.Join(moduleDir, "invkfile.cue"), []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}
}

func TestDiscoverAll_CurrentDirInvkfileTakesPrecedenceOverModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular invkfile in current directory
	// Standalone invkfiles cannot have module/version - those fields only go in invkmod.cue
	currentContent := `
cmds: [{name: "cmd", implementations: [{script: "echo current", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0o644); err != nil {
		t.Fatalf("failed to write current invkfile: %v", err)
	}

	// Create a module in the same directory using new two-file format
	moduleDir := filepath.Join(tmpDir, "apack.invkmod")
	createValidDiscoveryModule(t, moduleDir, "apack", "cmd")

	// Change to temp directory
	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	// Override HOME
	cleanupHome := setHomeDirEnv(t, tmpDir)
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
