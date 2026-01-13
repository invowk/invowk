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
group: "test"
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
group: "usercmds"
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
group: "customcmds"
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
group: "test"
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
group: "current"
version: "1.0"
commands: [{name: "current", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create user commands invowkfile
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
group: "usercmds"
version: "1.0"
commands: [{name: "user", implementations: [{script: "echo user", target: {runtimes: [{name: "native"}]}}]}]
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
group: "project"
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

	// Create current dir invowkfile with "build" command
	// Note: With group field, the command names will be different: "current build" vs "usercmds build"
	// So they won't conflict anymore. We need to test with the same group to test precedence.
	currentContent := `
group: "project"
version: "1.0"
commands: [{name: "build", description: "Current build", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create user commands invowkfile with same "build" command and same group
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	os.MkdirAll(userCmdsDir, 0755)
	userContent := `
group: "project"
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

func TestSourceBundle_String(t *testing.T) {
	if got := SourceBundle.String(); got != "bundle" {
		t.Errorf("SourceBundle.String() = %s, want bundle", got)
	}
}

func TestDiscoverAll_FindsBundlesInCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid bundle in the temp directory
	bundleDir := filepath.Join(tmpDir, "mycommands.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	// Create invowkfile.cue inside the bundle
	bundleContent := `
group: "mycommands"
version: "1.0"
commands: [{name: "bundled-cmd", implementations: [{script: "echo bundled", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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
		if f.Source == SourceBundle && f.Bundle != nil {
			if f.Bundle.Name == "mycommands" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find bundle in current directory")
	}
}

func TestDiscoverAll_FindsBundlesInUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user commands directory with a bundle
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	bundleDir := filepath.Join(userCmdsDir, "userbundle.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	// Create invowkfile.cue inside the bundle
	bundleContent := `
group: "userbundle"
version: "1.0"
commands: [{name: "user-bundled-cmd", implementations: [{script: "echo user bundled", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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
		if f.Source == SourceBundle && f.Bundle != nil {
			if f.Bundle.Name == "userbundle" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find bundle in user commands directory")
	}
}

func TestDiscoverAll_FindsBundlesInConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config search path with a bundle
	searchPath := filepath.Join(tmpDir, "custom-commands")
	bundleDir := filepath.Join(searchPath, "configbundle.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	// Create invowkfile.cue inside the bundle
	bundleContent := `
group: "configbundle"
version: "1.0"
commands: [{name: "config-bundled-cmd", implementations: [{script: "echo config bundled", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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
		if f.Source == SourceBundle && f.Bundle != nil {
			if f.Bundle.Name == "configbundle" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find bundle in configured search path")
	}
}

func TestDiscoveredFile_BundleField(t *testing.T) {
	df := &DiscoveredFile{
		Path:   "/path/to/bundle/invowkfile.cue",
		Source: SourceBundle,
	}

	if df.Bundle != nil {
		t.Error("Bundle should be nil by default")
	}

	if df.Source != SourceBundle {
		t.Errorf("Source = %v, want SourceBundle", df.Source)
	}
}

func TestDiscoverCommands_FromBundle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid bundle
	bundleDir := filepath.Join(tmpDir, "testbundle.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	// Create invowkfile.cue inside the bundle
	bundleContent := `
group: "testbundle"
version: "1.0"
commands: [
	{name: "cmd1", description: "First command", implementations: [{script: "echo 1", target: {runtimes: [{name: "native"}]}}]},
	{name: "cmd2", description: "Second command", implementations: [{script: "echo 2", target: {runtimes: [{name: "native"}]}}]}
]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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

	// Should find both commands from the bundle
	foundCmd1 := false
	foundCmd2 := false
	for _, cmd := range commands {
		if cmd.Name == "testbundle cmd1" && cmd.Source == SourceBundle {
			foundCmd1 = true
		}
		if cmd.Name == "testbundle cmd2" && cmd.Source == SourceBundle {
			foundCmd2 = true
		}
	}

	if !foundCmd1 {
		t.Error("DiscoverCommands() did not find 'testbundle cmd1' from bundle")
	}
	if !foundCmd2 {
		t.Error("DiscoverCommands() did not find 'testbundle cmd2' from bundle")
	}
}

func TestDiscoverAll_SkipsInvalidBundles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid bundle (missing invowkfile.cue)
	invalidBundleDir := filepath.Join(tmpDir, "invalid.invowkbundle")
	if err := os.MkdirAll(invalidBundleDir, 0755); err != nil {
		t.Fatalf("failed to create invalid bundle dir: %v", err)
	}

	// Create a valid bundle
	validBundleDir := filepath.Join(tmpDir, "valid.invowkbundle")
	if err := os.MkdirAll(validBundleDir, 0755); err != nil {
		t.Fatalf("failed to create valid bundle dir: %v", err)
	}
	bundleContent := `
group: "valid"
version: "1.0"
commands: [{name: "cmd", implementations: [{script: "echo", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(validBundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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

	// Should only find the valid bundle
	bundleCount := 0
	for _, f := range files {
		if f.Source == SourceBundle {
			bundleCount++
			if f.Bundle != nil && f.Bundle.Name != "valid" {
				t.Errorf("unexpected bundle found: %s", f.Bundle.Name)
			}
		}
	}

	if bundleCount != 1 {
		t.Errorf("expected 1 bundle, found %d", bundleCount)
	}
}

func TestLoadAll_ParsesBundles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid bundle
	bundleDir := filepath.Join(tmpDir, "parsebundle.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	// Create invowkfile.cue inside the bundle
	bundleContent := `
group: "parsebundle"
version: "1.0"
description: "A test bundle"
commands: [{name: "test", implementations: [{script: "echo test", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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

	// Find the bundle file
	var bundleFile *DiscoveredFile
	for _, f := range files {
		if f.Source == SourceBundle {
			bundleFile = f
			break
		}
	}

	if bundleFile == nil {
		t.Fatal("LoadAll() did not find bundle")
	}

	if bundleFile.Invowkfile == nil {
		t.Fatal("LoadAll() did not parse bundle invowkfile")
	}

	if bundleFile.Invowkfile.Description != "A test bundle" {
		t.Errorf("Invowkfile.Description = %s, want 'A test bundle'", bundleFile.Invowkfile.Description)
	}

	// Verify that BundlePath is set on the parsed invowkfile
	if !bundleFile.Invowkfile.IsFromBundle() {
		t.Error("Invowkfile.IsFromBundle() should return true for bundle-parsed file")
	}
}

func TestLoadFirst_LoadsBundle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid bundle (but no regular invowkfile)
	bundleDir := filepath.Join(tmpDir, "firstbundle.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	bundleContent := `
group: "firstbundle"
version: "1.0"
commands: [{name: "first", implementations: [{script: "echo first", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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

	if file.Source != SourceBundle {
		t.Errorf("LoadFirst().Source = %v, want SourceBundle", file.Source)
	}

	if file.Invowkfile == nil {
		t.Fatal("LoadFirst() did not parse bundle invowkfile")
	}

	if file.Bundle == nil {
		t.Fatal("LoadFirst().Bundle should not be nil for bundle source")
	}

	if file.Bundle.Name != "firstbundle" {
		t.Errorf("Bundle.Name = %s, want 'firstbundle'", file.Bundle.Name)
	}
}

func TestDiscoverAll_CurrentDirInvowkfileTakesPrecedenceOverBundle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular invowkfile in current directory
	currentContent := `
group: "current"
version: "1.0"
commands: [{name: "cmd", implementations: [{script: "echo current", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(currentContent), 0644); err != nil {
		t.Fatalf("failed to write current invowkfile: %v", err)
	}

	// Create a bundle in the same directory
	bundleDir := filepath.Join(tmpDir, "abundle.invowkbundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}
	bundleContent := `
group: "abundle"
version: "1.0"
commands: [{name: "cmd", implementations: [{script: "echo bundle", target: {runtimes: [{name: "native"}]}}]}]
`
	if err := os.WriteFile(filepath.Join(bundleDir, "invowkfile.cue"), []byte(bundleContent), 0644); err != nil {
		t.Fatalf("failed to write bundle invowkfile: %v", err)
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

	// First file should be from current directory, not bundle
	if len(files) == 0 {
		t.Fatal("DiscoverAll() returned no files")
	}

	if files[0].Source != SourceCurrentDir {
		t.Errorf("first file source = %v, want SourceCurrentDir", files[0].Source)
	}

	// Both should be found
	foundCurrentDir := false
	foundBundle := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			foundCurrentDir = true
		}
		if f.Source == SourceBundle {
			foundBundle = true
		}
	}

	if !foundCurrentDir {
		t.Error("DiscoverAll() did not find invowkfile in current directory")
	}
	if !foundBundle {
		t.Error("DiscoverAll() did not find bundle")
	}
}
