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
		Version: "1.0",
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
group: "test"
version: "1.0"
description: "Test commands"

commands: [
	{
		name: "test"
		description: "Run tests"
		implementations: [{
			script: "echo test"
			target: {runtimes: [{name: "native"}]}
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
group: "test"
version: "1.0"
description: "Test commands"

commands: [
	{
		name: "test"
		description: "Run tests"
		implementations: [{
			script: "echo test"
			target: {runtimes: [{name: "native"}]}
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
group: "test"
version: "1.0"
commands: [{name: "test", implementations: [{script: "echo test", target: {runtimes: [{name: "native"}]}}]}]
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
group: "usercmds"
version: "1.0"
commands: [{name: "user-cmd", implementations: [{script: "echo user", target: {runtimes: [{name: "native"}]}}]}]
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
group: "customcmds"
version: "1.0"
commands: [{name: "custom-cmd", implementations: [{script: "echo custom", target: {runtimes: [{name: "native"}]}}]}]
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

	content := `
group: "test"
version: "1.0"
description: "Test"
commands: [{name: "test", implementations: [{script: "echo test", target: {runtimes: [{name: "native"}]}}]}]
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

	if file.Invkfile.Version != "1.0" {
		t.Errorf("Invkfile.Version = %s, want 1.0", file.Invkfile.Version)
	}
}

func TestLoadAll_WithMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current dir invkfile
	content := `
group: "current"
version: "1.0"
commands: [{name: "current", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
group: "usercmds"
version: "1.0"
commands: [{name: "user", implementations: [{script: "echo user", target: {runtimes: [{name: "native"}]}}]}]
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

	content := `
group: "project"
version: "1.0"
commands: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", target: {runtimes: [{name: "native"}]}}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", target: {runtimes: [{name: "native"}]}}]}
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

	// Commands should be sorted by name (with group prefix)
	if len(commands) >= 2 {
		if commands[0].Name != "project build" || commands[1].Name != "project test" {
			t.Errorf("commands not sorted correctly: got %s, %s", commands[0].Name, commands[1].Name)
		}
	}
}

func TestGetCommand(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
group: "project"
version: "1.0"
commands: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", target: {runtimes: [{name: "native"}]}}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", target: {runtimes: [{name: "native"}]}}]}
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
		cmd, err := d.GetCommand("project build")
		if err != nil {
			t.Fatalf("GetCommand() returned error: %v", err)
		}

		if cmd.Name != "project build" {
			t.Errorf("GetCommand().Name = %s, want 'project build'", cmd.Name)
		}
	})

	t.Run("NonExistentCommand", func(t *testing.T) {
		_, err := d.GetCommand("nonexistent")
		if err == nil {
			t.Error("GetCommand() should return error for non-existent command")
		}
	})

	t.Run("CommandWithoutGroup", func(t *testing.T) {
		_, err := d.GetCommand("build")
		if err == nil {
			t.Error("GetCommand() should return error when searching without group prefix")
		}
	})
}

func TestGetCommandsWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
group: "project"
version: "1.0"
commands: [
	{name: "build", implementations: [{script: "go build", target: {runtimes: [{name: "native"}]}}]},
	{name: "build-dev", implementations: [{script: "go build -tags dev", target: {runtimes: [{name: "native"}]}}]},
	{name: "test", implementations: [{script: "go test", target: {runtimes: [{name: "native"}]}}]}
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

	t.Run("GroupPrefix", func(t *testing.T) {
		commands, err := d.GetCommandsWithPrefix("project")
		if err != nil {
			t.Fatalf("GetCommandsWithPrefix() returned error: %v", err)
		}

		if len(commands) != 3 {
			t.Errorf("GetCommandsWithPrefix('project') returned %d commands, want 3", len(commands))
		}
	})

	t.Run("GroupAndBuildPrefix", func(t *testing.T) {
		commands, err := d.GetCommandsWithPrefix("project build")
		if err != nil {
			t.Fatalf("GetCommandsWithPrefix() returned error: %v", err)
		}

		if len(commands) != 2 {
			t.Errorf("GetCommandsWithPrefix('project build') returned %d commands, want 2", len(commands))
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
	// Note: With group field, the command names will be different: "current build" vs "usercmds build"
	// So they won't conflict anymore. We need to test with the same group to test precedence.
	currentContent := `
group: "project"
version: "1.0"
commands: [{name: "build", description: "Current build", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile with same "build" command and same group
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
group: "project"
version: "1.0"
commands: [{name: "build", description: "User build", implementations: [{script: "echo user", target: {runtimes: [{name: "native"}]}}]}]
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

	// Should only have one "project build" command (from current directory, higher precedence)
	buildCount := 0
	var buildCmd *CommandInfo
	for _, cmd := range commands {
		if cmd.Name == "project build" {
			buildCount++
			buildCmd = cmd
		}
	}

	if buildCount != 1 {
		t.Errorf("expected 1 'project build' command, got %d", buildCount)
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

	// Create a valid pack in the temp directory
	packDir := filepath.Join(tmpDir, "mycommands.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	// Create invkfile.cue inside the pack
	packContent := `
group: "mycommands"
version: "1.0"
commands: [{name: "packed-cmd", implementations: [{script: "echo packed", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
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

	// Create user commands directory with a pack
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	packDir := filepath.Join(userCmdsDir, "userpack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	// Create invkfile.cue inside the pack
	packContent := `
group: "userpack"
version: "1.0"
commands: [{name: "user-packed-cmd", implementations: [{script: "echo user packed", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
	}

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

	// Create a config search path with a pack
	searchPath := filepath.Join(tmpDir, "custom-commands")
	packDir := filepath.Join(searchPath, "configpack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	// Create invkfile.cue inside the pack
	packContent := `
group: "configpack"
version: "1.0"
commands: [{name: "config-packed-cmd", implementations: [{script: "echo config packed", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
	}

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

	// Create a valid pack
	packDir := filepath.Join(tmpDir, "testpack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	// Create invkfile.cue inside the pack
	packContent := `
group: "testpack"
version: "1.0"
commands: [
	{name: "cmd1", description: "First command", implementations: [{script: "echo 1", target: {runtimes: [{name: "native"}]}}]},
	{name: "cmd2", description: "Second command", implementations: [{script: "echo 2", target: {runtimes: [{name: "native"}]}}]}
]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
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

	// Create an invalid pack (missing invkfile.cue)
	invalidPackDir := filepath.Join(tmpDir, "invalid.invkpack")
	if err := os.MkdirAll(invalidPackDir, 0755); err != nil {
		t.Fatalf("failed to create invalid pack dir: %v", err)
	}

	// Create a valid pack
	validPackDir := filepath.Join(tmpDir, "valid.invkpack")
	if err := os.MkdirAll(validPackDir, 0755); err != nil {
		t.Fatalf("failed to create valid pack dir: %v", err)
	}
	packContent := `
group: "valid"
version: "1.0"
commands: [{name: "cmd", implementations: [{script: "echo", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(validPackDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
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

	// Create a valid pack
	packDir := filepath.Join(tmpDir, "parsepack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	// Create invkfile.cue inside the pack
	packContent := `
group: "parsepack"
version: "1.0"
description: "A test pack"
commands: [{name: "test", implementations: [{script: "echo test", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
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

	if packFile.Invkfile.Description != "A test pack" {
		t.Errorf("Invkfile.Description = %s, want 'A test pack'", packFile.Invkfile.Description)
	}

	// Verify that PackPath is set on the parsed invkfile
	if !packFile.Invkfile.IsFromPack() {
		t.Error("Invkfile.IsFromPack() should return true for pack-parsed file")
	}
}

func TestLoadFirst_LoadsPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid pack (but no regular invkfile)
	packDir := filepath.Join(tmpDir, "firstpack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	packContent := `
group: "firstpack"
version: "1.0"
commands: [{name: "first", implementations: [{script: "echo first", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
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

func TestDiscoverAll_CurrentDirInvkfileTakesPrecedenceOverPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular invkfile in current directory
	currentContent := `
group: "current"
version: "1.0"
commands: [{name: "cmd", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write current invkfile: %v", err)
	}

	// Create a pack in the same directory
	packDir := filepath.Join(tmpDir, "apack.invkpack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}
	packContent := `
group: "apack"
version: "1.0"
commands: [{name: "cmd", implementations: [{script: "echo pack", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(packContent), 0644); err != nil {
		t.Fatalf("failed to write pack invkfile: %v", err)
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
