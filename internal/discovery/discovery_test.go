// SPDX-License-Identifier: EPL-2.0

// Package discovery handles finding and loading invkfiles from various locations.
package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invkfile"
)

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
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME to avoid finding real user commands
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME to avoid finding real user commands
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME to avoid finding real user commands
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
	if err := os.MkdirAll(userCmdsDir, 0755); err != nil {
		t.Fatalf("failed to create user cmds dir: %v", err)
	}

	// Create an invkfile in user commands
	content := `
cmds: [{name: "user-cmd", implementations: [{script: "echo user", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory (which has no invkfile)
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
	if err := os.MkdirAll(searchPath, 0755); err != nil {
		t.Fatalf("failed to create search path dir: %v", err)
	}

	// Create an invkfile in search path
	content := `
cmds: [{name: "custom-cmd", implementations: [{script: "echo custom", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(searchPath, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory (which has no invkfile)
	emptyDir := filepath.Join(tmpDir, "empty")
	os.MkdirAll(emptyDir, 0755)
	originalWd, _ := os.Getwd()
	os.Chdir(emptyDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	_, err := d.LoadFirst()
	if err == nil {
		t.Error("LoadFirst() should return error when no files found")
	}
}

func TestLoadFirst_WithValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now only contains commands (pack metadata is in invkpack.cue for packs)
	content := `
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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

	// Create current dir invkfile (no pack metadata - it belongs in invkpack.cue)
	content := `
cmds: [{name: "current", implementations: [{script: "echo current", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile (no pack metadata - it belongs in invkpack.cue)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
cmds: [{name: "user", implementations: [{script: "echo user", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(userContent), 0644); err != nil {
		t.Fatalf("failed to write user invkfile: %v", err)
	}

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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

	// invkfile.cue now contains only commands - pack metadata is in invkpack.cue for packs
	content := `
cmds: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	commands, err := d.DiscoverCommands()
	if err != nil {
		t.Fatalf("DiscoverCommands() returned error: %v", err)
	}

	if len(commands) != 2 {
		t.Errorf("DiscoverCommands() returned %d commands, want 2", len(commands))
	}

	// Commands should be sorted by name (no pack prefix for current-dir invkfiles)
	if len(commands) >= 2 {
		if commands[0].Name != "build" || commands[1].Name != "test" {
			t.Errorf("commands not sorted correctly: got %s, %s", commands[0].Name, commands[1].Name)
		}
	}
}

func TestGetCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now contains only commands - no pack metadata
	content := `
cmds: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("ExistingCommand", func(t *testing.T) {
		// Current-dir invkfiles don't have pack prefix
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

	// invkfile.cue now contains only commands - no pack metadata
	content := `
cmds: [
	{name: "build", implementations: [{script: "go build", runtimes: [{name: "native"}]}]},
	{name: "build-dev", implementations: [{script: "go build -tags dev", runtimes: [{name: "native"}]}]},
	{name: "test", implementations: [{script: "go test", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

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
		// Current-dir invkfiles have no pack prefix
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
	// Without pack field, commands are named directly (e.g., "build" not "project build")
	// When same command exists in multiple sources, current dir takes precedence
	currentContent := `
cmds: [{name: "build", description: "Current build", implementations: [{script: "echo current", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile with same "build" command
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
cmds: [{name: "build", description: "User build", implementations: [{script: "echo user", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(userContent), 0644); err != nil {
		t.Fatalf("failed to write user invkfile: %v", err)
	}

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	commands, err := d.DiscoverCommands()
	if err != nil {
		t.Fatalf("DiscoverCommands() returned error: %v", err)
	}

	// Should only have one "build" command (from current directory, higher precedence)
	// With no pack field, command is just "build" not "project build"
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

func TestSourcePack_String(t *testing.T) {
	if got := SourcePack.String(); got != "pack" {
		t.Errorf("SourcePack.String() = %s, want pack", got)
	}
}

func TestDiscoverAll_FindsPacksInCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid pack in the temp directory (using new two-file format)
	packDir := filepath.Join(tmpDir, "mycommands.invkpack")
	createValidDiscoveryPack(t, packDir, "mycommands", "packed-cmd")

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME to avoid finding real user commands
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourcePack && f.Pack != nil {
			if f.Pack.Name == "mycommands" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find pack in current directory")
	}
}

func TestDiscoverAll_FindsPacksInUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user commands directory with a pack (using new two-file format)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	packDir := filepath.Join(userCmdsDir, "userpack.invkpack")
	createValidDiscoveryPack(t, packDir, "userpack", "user-packed-cmd")

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	// Change to work directory
	originalWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourcePack && f.Pack != nil {
			if f.Pack.Name == "userpack" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find pack in user commands directory")
	}
}

func TestDiscoverAll_FindsPacksInConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config search path with a pack (using new two-file format)
	searchPath := filepath.Join(tmpDir, "custom-commands")
	packDir := filepath.Join(searchPath, "configpack.invkpack")
	createValidDiscoveryPack(t, packDir, "configpack", "config-packed-cmd")

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	// Change to work directory
	originalWd, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	cfg.SearchPaths = []string{searchPath}
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourcePack && f.Pack != nil {
			if f.Pack.Name == "configpack" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find pack in configured search path")
	}
}

func TestDiscoveredFile_PackField(t *testing.T) {
	df := &DiscoveredFile{
		Path:   "/path/to/pack/invkfile.cue",
		Source: SourcePack,
	}

	if df.Pack != nil {
		t.Error("Pack should be nil by default")
	}

	if df.Source != SourcePack {
		t.Errorf("Source = %v, want SourcePack", df.Source)
	}
}

func TestDiscoverCommands_FromPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid pack with the new two-file format
	packDir := filepath.Join(tmpDir, "testpack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}
	// Create invkpack.cue with metadata
	invkpackContent := `pack: "testpack"
version: "1.0"
`
	if err := os.WriteFile(filepath.Join(packDir, "invkpack.cue"), []byte(invkpackContent), 0644); err != nil {
		t.Fatalf("failed to write invkpack.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [
	{name: "cmd1", description: "First command", implementations: [{script: "echo 1", runtimes: [{name: "native"}]}]},
	{name: "cmd2", description: "Second command", implementations: [{script: "echo 2", runtimes: [{name: "native"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(invkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	commands, err := d.DiscoverCommands()
	if err != nil {
		t.Fatalf("DiscoverCommands() returned error: %v", err)
	}

	// Should find both commands from the pack
	foundCmd1 := false
	foundCmd2 := false
	for _, cmd := range commands {
		if cmd.Name == "testpack cmd1" && cmd.Source == SourcePack {
			foundCmd1 = true
		}
		if cmd.Name == "testpack cmd2" && cmd.Source == SourcePack {
			foundCmd2 = true
		}
	}

	if !foundCmd1 {
		t.Error("DiscoverCommands() did not find 'testpack cmd1' from pack")
	}
	if !foundCmd2 {
		t.Error("DiscoverCommands() did not find 'testpack cmd2' from pack")
	}
}

func TestDiscoverAll_SkipsInvalidPacks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid pack (missing invkpack.cue - required now)
	invalidPackDir := filepath.Join(tmpDir, "invalid.invkpack")
	if err := os.MkdirAll(invalidPackDir, 0755); err != nil {
		t.Fatalf("failed to create invalid pack dir: %v", err)
	}

	// Create a valid pack using new two-file format
	validPackDir := filepath.Join(tmpDir, "valid.invkpack")
	createValidDiscoveryPack(t, validPackDir, "valid", "cmd")

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Should only find the valid pack
	packCount := 0
	for _, f := range files {
		if f.Source == SourcePack {
			packCount++
			if f.Pack != nil && f.Pack.Name != "valid" {
				t.Errorf("unexpected pack found: %s", f.Pack.Name)
			}
		}
	}

	if packCount != 1 {
		t.Errorf("expected 1 pack, found %d", packCount)
	}
}

func TestLoadAll_ParsesPacks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid pack with new two-file format
	packDir := filepath.Join(tmpDir, "parsepack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}
	// Create invkpack.cue with metadata
	invkpackContent := `pack: "parsepack"
version: "1.0"
description: "A test pack"
`
	if err := os.WriteFile(filepath.Join(packDir, "invkpack.cue"), []byte(invkpackContent), 0644); err != nil {
		t.Fatalf("failed to write invkpack.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(invkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() returned error: %v", err)
	}

	// Find the pack file
	var packFile *DiscoveredFile
	for _, f := range files {
		if f.Source == SourcePack {
			packFile = f
			break
		}
	}

	if packFile == nil {
		t.Fatal("LoadAll() did not find pack")
	}

	if packFile.Invkfile == nil {
		t.Fatal("LoadAll() did not parse pack invkfile")
	}

	// In the new format, description is in Metadata, not Invkfile
	if packFile.Invkfile.Metadata == nil {
		t.Fatal("Invkfile.Metadata should not be nil for pack-parsed file")
	}

	if packFile.Invkfile.Metadata.Description != "A test pack" {
		t.Errorf("Invkfile.Metadata.Description = %s, want 'A test pack'", packFile.Invkfile.Metadata.Description)
	}

	// Verify that PackPath is set on the parsed invkfile
	if !packFile.Invkfile.IsFromPack() {
		t.Error("Invkfile.IsFromPack() should return true for pack-parsed file")
	}
}

func TestLoadFirst_LoadsPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid pack (but no regular invkfile in root)
	packDir := filepath.Join(tmpDir, "firstpack.invkpack")
	createValidDiscoveryPack(t, packDir, "firstpack", "first")

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	file, err := d.LoadFirst()
	if err != nil {
		t.Fatalf("LoadFirst() returned error: %v", err)
	}

	if file.Source != SourcePack {
		t.Errorf("LoadFirst().Source = %v, want SourcePack", file.Source)
	}

	if file.Invkfile == nil {
		t.Fatal("LoadFirst() did not parse pack invkfile")
	}

	if file.Pack == nil {
		t.Fatal("LoadFirst().Pack should not be nil for pack source")
	}

	if file.Pack.Name != "firstpack" {
		t.Errorf("Pack.Name = %s, want 'firstpack'", file.Pack.Name)
	}
}

func TestPackCollisionError(t *testing.T) {
	err := &PackCollisionError{
		PackID:       "io.example.tools",
		FirstSource:  "/path/to/first",
		SecondSource: "/path/to/second",
	}

	errMsg := err.Error()
	if !containsString(errMsg, "io.example.tools") {
		t.Error("error message should contain pack ID")
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

func TestCheckPackCollisions(t *testing.T) {
	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("NoCollision", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/pack1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.pack1"}},
			},
			{
				Path:     "/path/to/pack2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.pack2"}},
			},
		}

		err := d.CheckPackCollisions(files)
		if err != nil {
			t.Errorf("CheckPackCollisions() returned unexpected error: %v", err)
		}
	})

	t.Run("WithCollision", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/pack1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.same"}},
			},
			{
				Path:     "/path/to/pack2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.same"}},
			},
		}

		err := d.CheckPackCollisions(files)
		if err == nil {
			t.Error("CheckPackCollisions() should return error for collision")
		}

		collisionErr, ok := err.(*PackCollisionError)
		if !ok {
			t.Errorf("error should be PackCollisionError, got %T", err)
		}
		if collisionErr.PackID != "io.example.same" {
			t.Errorf("PackID = %s, want io.example.same", collisionErr.PackID)
		}
	})

	t.Run("CollisionResolvedByAlias", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.PackAliases = map[string]string{
			"/path/to/pack1": "io.example.alias1",
		}
		d := New(cfg)

		files := []*DiscoveredFile{
			{
				Path:     "/path/to/pack1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.same"}},
			},
			{
				Path:     "/path/to/pack2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.same"}},
			},
		}

		err := d.CheckPackCollisions(files)
		if err != nil {
			t.Errorf("CheckPackCollisions() should not return error when alias resolves collision: %v", err)
		}
	})

	t.Run("SkipsFilesWithErrors", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/pack1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.same"}},
			},
			{
				Path:  "/path/to/pack2",
				Error: os.ErrNotExist, // This file has an error
			},
		}

		err := d.CheckPackCollisions(files)
		if err != nil {
			t.Errorf("CheckPackCollisions() should skip files with errors: %v", err)
		}
	})

	t.Run("SkipsFilesWithoutPackID", func(t *testing.T) {
		files := []*DiscoveredFile{
			{
				Path:     "/path/to/pack1",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.pack1"}},
			},
			{
				Path:     "/path/to/pack2",
				Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: ""}}, // Empty pack ID
			},
		}

		err := d.CheckPackCollisions(files)
		if err != nil {
			t.Errorf("CheckPackCollisions() should skip files without pack ID: %v", err)
		}
	})
}

func TestGetEffectivePackID(t *testing.T) {
	t.Run("WithoutAlias", func(t *testing.T) {
		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:     "/path/to/pack",
			Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.original"}},
		}

		packID := d.GetEffectivePackID(file)
		if packID != "io.example.original" {
			t.Errorf("GetEffectivePackID() = %s, want io.example.original", packID)
		}
	})

	t.Run("WithAlias", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.PackAliases = map[string]string{
			"/path/to/pack": "io.example.aliased",
		}
		d := New(cfg)

		file := &DiscoveredFile{
			Path:     "/path/to/pack",
			Invkfile: &invkfile.Invkfile{Metadata: &invkfile.Invkpack{Pack: "io.example.original"}},
		}

		packID := d.GetEffectivePackID(file)
		if packID != "io.example.aliased" {
			t.Errorf("GetEffectivePackID() = %s, want io.example.aliased", packID)
		}
	})

	t.Run("WithNilInvkfile", func(t *testing.T) {
		cfg := config.DefaultConfig()
		d := New(cfg)

		file := &DiscoveredFile{
			Path:     "/path/to/pack",
			Invkfile: nil,
		}

		packID := d.GetEffectivePackID(file)
		if packID != "" {
			t.Errorf("GetEffectivePackID() = %s, want empty string", packID)
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

// createValidDiscoveryPack creates a pack with the new two-file format (invkpack.cue + invkfile.cue)
func createValidDiscoveryPack(t *testing.T, packDir, packID string, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}
	// Create invkpack.cue with metadata
	invkpackContent := `pack: "` + packID + `"
version: "1.0"
`
	if err := os.WriteFile(filepath.Join(packDir, "invkpack.cue"), []byte(invkpackContent), 0644); err != nil {
		t.Fatalf("failed to write invkpack.cue: %v", err)
	}
	// Create invkfile.cue with commands
	invkfileContent := `cmds: [{name: "` + cmdName + `", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(invkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile.cue: %v", err)
	}
}

func TestDiscoverAll_CurrentDirInvkfileTakesPrecedenceOverPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular invkfile in current directory
	// Standalone invkfiles cannot have pack/version - those fields only go in invkpack.cue
	currentContent := `
cmds: [{name: "cmd", implementations: [{script: "echo current", runtimes: [{name: "native"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write current invkfile: %v", err)
	}

	// Create a pack in the same directory using new two-file format
	packDir := filepath.Join(tmpDir, "apack.invkpack")
	createValidDiscoveryPack(t, packDir, "apack", "cmd")

	// Change to temp directory
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Override HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// First file should be from current directory, not pack
	if len(files) == 0 {
		t.Fatal("DiscoverAll() returned no files")
	}

	if files[0].Source != SourceCurrentDir {
		t.Errorf("first file source = %v, want SourceCurrentDir", files[0].Source)
	}

	// Both should be found
	foundCurrentDir := false
	foundPack := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			foundCurrentDir = true
		}
		if f.Source == SourcePack {
			foundPack = true
		}
	}

	if !foundCurrentDir {
		t.Error("DiscoverAll() did not find invkfile in current directory")
	}
	if !foundPack {
		t.Error("DiscoverAll() did not find pack")
	}
}
