// Package discovery handles finding and loading invowkfiles from various locations.
package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invowkfile"
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
		Path:   "/path/to/invowkfile.cue",
		Source: SourceCurrentDir,
	}

	if df.Path != "/path/to/invowkfile.cue" {
		t.Errorf("Path = %s, want /path/to/invowkfile.cue", df.Path)
	}

	if df.Source != SourceCurrentDir {
		t.Errorf("Source = %v, want SourceCurrentDir", df.Source)
	}

	if df.Invowkfile != nil {
		t.Error("Invowkfile should be nil by default")
	}

	if df.Error != nil {
		t.Error("Error should be nil by default")
	}
}

func TestCommandInfo_Fields(t *testing.T) {
	cmd := &invowkfile.Command{
		Name:        "build",
		Description: "Build the project",
	}

	inv := &invowkfile.Invowkfile{
		Version: "1.0",
	}

	ci := &CommandInfo{
		Name:        "build",
		Description: "Build the project",
		Source:      SourceCurrentDir,
		FilePath:    "/path/to/invowkfile.cue",
		Command:     cmd,
		Invowkfile:  inv,
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

	// Should return empty slice (no invowkfiles found)
	if len(files) != 0 {
		t.Errorf("DiscoverAll() returned %d files, want 0", len(files))
	}
}

func TestDiscoverAll_FindsInvowkfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invowkfile.cue in the temp directory
	invowkfileContent := `
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
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(invowkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
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
		t.Error("DiscoverAll() did not find invowkfile in current directory")
	}
}

func TestDiscoverAll_FindsInvowkfileWithoutExtension(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invowkfile (without .cue extension) in the temp directory
	invowkfileContent := `
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
	invowkfilePath := filepath.Join(tmpDir, "invowkfile")
	if err := os.WriteFile(invowkfilePath, []byte(invowkfileContent), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
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

func TestDiscoverAll_PrefersInvowkfileCue(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both invowkfile and invowkfile.cue
	content := `
version: "1.0"
commands: [{name: "test", implementations: [{script: "echo test", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
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

	// Should find invowkfile.cue (preferred) in current dir
	found := false
	for _, f := range files {
		if f.Source == SourceCurrentDir && filepath.Base(f.Path) == "invowkfile.cue" {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() should prefer invowkfile.cue over invowkfile")
	}
}

func TestDiscoverAll_FindsInUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user commands directory
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	if err := os.MkdirAll(userCmdsDir, 0755); err != nil {
		t.Fatalf("failed to create user cmds dir: %v", err)
	}

	// Create an invowkfile in user commands
	content := `
version: "1.0"
commands: [{name: "user-cmd", implementations: [{script: "echo user", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Change to temp directory (which has no invowkfile)
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
		t.Error("DiscoverAll() did not find invowkfile in user commands directory")
	}
}

func TestDiscoverAll_FindsInConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config search path directory
	searchPath := filepath.Join(tmpDir, "custom-commands")
	if err := os.MkdirAll(searchPath, 0755); err != nil {
		t.Fatalf("failed to create search path dir: %v", err)
	}

	// Create an invowkfile in search path
	content := `
version: "1.0"
commands: [{name: "custom-cmd", implementations: [{script: "echo custom", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(searchPath, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Change to temp directory (which has no invowkfile)
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
		t.Error("DiscoverAll() did not find invowkfile in configured search path")
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
version: "1.0"
description: "Test"
commands: [{name: "test", implementations: [{script: "echo test", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
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

	if file.Invowkfile == nil {
		t.Error("LoadFirst() did not parse the invowkfile")
	}

	if file.Invowkfile.Version != "1.0" {
		t.Errorf("Invowkfile.Version = %s, want 1.0", file.Invowkfile.Version)
	}
}

func TestLoadAll_WithMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current dir invowkfile
	content := `
version: "1.0"
commands: [{name: "current", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create user commands invowkfile
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write user invowkfile: %v", err)
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
		if f.Invowkfile == nil && f.Error == nil {
			t.Errorf("file %s was not parsed and has no error", f.Path)
		}
	}
}

func TestDiscoverCommands(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
version: "1.0"
commands: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", target: {runtimes: [{name: "native"}]}}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", target: {runtimes: [{name: "native"}]}}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
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

	// Commands should be sorted by name
	if len(commands) >= 2 {
		if commands[0].Name != "build" || commands[1].Name != "test" {
			t.Errorf("commands not sorted correctly: got %s, %s", commands[0].Name, commands[1].Name)
		}
	}
}

func TestGetCommand(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
version: "1.0"
commands: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", target: {runtimes: [{name: "native"}]}}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", target: {runtimes: [{name: "native"}]}}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
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
		cmd, err := d.GetCommand("build")
		if err != nil {
			t.Fatalf("GetCommand() returned error: %v", err)
		}

		if cmd.Name != "build" {
			t.Errorf("GetCommand().Name = %s, want build", cmd.Name)
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

	content := `
version: "1.0"
commands: [
	{name: "build", implementations: [{script: "go build", target: {runtimes: [{name: "native"}]}}]},
	{name: "build-dev", implementations: [{script: "go build -tags dev", target: {runtimes: [{name: "native"}]}}]},
	{name: "test", implementations: [{script: "go test", target: {runtimes: [{name: "native"}]}}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
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

	// Create current dir invowkfile with "build" command
	currentContent := `
version: "1.0"
commands: [{name: "build", description: "Current build", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create user commands invowkfile with same "build" command
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
version: "1.0"
commands: [{name: "build", description: "User build", implementations: [{script: "echo user", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invowkfile.cue"), []byte(userContent), 0644); err != nil {
		t.Fatalf("failed to write user invowkfile: %v", err)
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
	buildCount := 0
	var buildCmd *CommandInfo
	for _, cmd := range commands {
		if cmd.Name == "build" {
			buildCount++
			buildCmd = cmd
		}
	}

	if buildCount != 1 {
		t.Errorf("expected 1 build command, got %d", buildCount)
	}

	if buildCmd != nil && buildCmd.Source != SourceCurrentDir {
		t.Errorf("build command should be from current directory, got %v", buildCmd.Source)
	}
}
