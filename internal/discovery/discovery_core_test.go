// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/internal/testutil"
)

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

func TestNewDiscoveredCommandSet(t *testing.T) {
	// Test T004/T005: DiscoveredCommandSet constructor
	set := NewDiscoveredCommandSet()

	if set == nil {
		t.Fatal("NewDiscoveredCommandSet() returned nil")
	}
	if set.Commands == nil {
		t.Error("Commands should be initialized")
	}
	if set.BySimpleName == nil {
		t.Error("BySimpleName should be initialized")
	}
	if set.AmbiguousNames == nil {
		t.Error("AmbiguousNames should be initialized")
	}
	if set.BySource == nil {
		t.Error("BySource should be initialized")
	}
	if set.SourceOrder == nil {
		t.Error("SourceOrder should be initialized")
	}
}

func TestDiscoverAll_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

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
			platforms: [{name: "linux"}, {name: "macos"}]
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
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
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
			platforms: [{name: "linux"}, {name: "macos"}]
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
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
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
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
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
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
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
cmds: [{name: "user-cmd", implementations: [{script: "echo user", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Change to temp directory (which has no invkfile)
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
cmds: [{name: "custom-cmd", implementations: [{script: "echo custom", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
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
	cleanupHome := testutil.SetHomeDir(t, tmpDir)
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

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	_, err := d.LoadFirst()
	if err == nil {
		t.Error("LoadFirst() should return error when no files found")
	}
	if !errors.Is(err, ErrNoInvkfileFound) {
		t.Errorf("LoadFirst() error should be ErrNoInvkfileFound, got: %v", err)
	}
}

func TestErrNoInvkfileFound_Sentinel(t *testing.T) {
	if ErrNoInvkfileFound == nil {
		t.Fatal("ErrNoInvkfileFound should not be nil")
	}
	if ErrNoInvkfileFound.Error() != "no invkfile found" {
		t.Errorf("ErrNoInvkfileFound.Error() = %q, want %q", ErrNoInvkfileFound.Error(), "no invkfile found")
	}
}

func TestLoadFirst_WithValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now only contains commands (module metadata is in invkmod.cue for modules)
	content := `
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
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
cmds: [{name: "current", implementations: [{script: "echo current", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile (no module metadata - it belongs in invkmod.cue)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	testutil.MustMkdirAll(t, userCmdsDir, 0o755)
	userContent := `
cmds: [{name: "user", implementations: [{script: "echo user", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(userContent), 0o644); err != nil {
		t.Fatalf("failed to write user invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
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
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}
	commands := result.Set.Commands

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
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	t.Run("ExistingCommand", func(t *testing.T) {
		// Current-dir invkfiles don't have module prefix
		lookup, err := d.GetCommand(context.Background(), "build")
		if err != nil {
			t.Fatalf("GetCommand() returned error: %v", err)
		}
		cmd := lookup.Command
		if cmd == nil {
			t.Fatal("GetCommand() returned nil command")
		}

		if cmd.Name != "build" {
			t.Errorf("GetCommand().Name = %s, want 'build'", cmd.Name)
		}
	})

	t.Run("NonExistentCommand", func(t *testing.T) {
		lookup, err := d.GetCommand(context.Background(), "nonexistent")
		if err == nil {
			if lookup.Command != nil {
				t.Error("GetCommand() should return nil command for non-existent command")
			}
			if len(lookup.Diagnostics) == 0 {
				t.Error("GetCommand() should return diagnostics for non-existent command")
			}
		}
	})
}

func TestGetCommandsWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	// invkfile.cue now contains only commands - no module metadata
	content := `
cmds: [
	{name: "build", implementations: [{script: "go build", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "build-dev", implementations: [{script: "go build -tags dev", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "test", implementations: [{script: "go test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}

	filterPrefix := func(prefix string) []*CommandInfo {
		matching := make([]*CommandInfo, 0)
		for _, cmd := range result.Set.Commands {
			if prefix == "" || strings.HasPrefix(cmd.Name, prefix) {
				matching = append(matching, cmd)
			}
		}
		return matching
	}

	t.Run("EmptyPrefix", func(t *testing.T) {
		commands := filterPrefix("")
		if len(commands) != 3 {
			t.Errorf("prefix filter returned %d commands, want 3", len(commands))
		}
	})

	t.Run("BuildPrefix", func(t *testing.T) {
		commands := filterPrefix("build")
		if len(commands) != 2 {
			t.Errorf("prefix filter returned %d commands, want 2", len(commands))
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		commands := filterPrefix("xyz")
		if len(commands) != 0 {
			t.Errorf("prefix filter returned %d commands, want 0", len(commands))
		}
	})
}

func TestDiscoverCommands_Precedence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current dir invkfile with "build" command
	// Without module field, commands are named directly (e.g., "build" not "project build")
	// When same command exists in multiple sources, current dir takes precedence
	currentContent := `
cmds: [{name: "build", description: "Current build", implementations: [{script: "echo current", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte(currentContent), 0o644); err != nil {
		t.Fatalf("failed to write invkfile: %v", err)
	}

	// Create user commands invkfile with same "build" command
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	testutil.MustMkdirAll(t, userCmdsDir, 0o755)
	userContent := `
cmds: [{name: "build", description: "User build", implementations: [{script: "echo user", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invkfile.cue"), []byte(userContent), 0o644); err != nil {
		t.Fatalf("failed to write user invkfile: %v", err)
	}

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}
	commands := result.Set.Commands

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

func TestDiscoverAll_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based tests are unreliable on Windows")
	}

	tmpDir := t.TempDir()

	// Create a subdirectory and make it unreadable
	unreadableDir := filepath.Join(tmpDir, ".invowk", "cmds")
	testutil.MustMkdirAll(t, unreadableDir, 0o755)
	if err := os.Chmod(unreadableDir, 0o000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	// Restore permissions so t.TempDir() cleanup can remove the directory
	t.Cleanup(func() {
		_ = os.Chmod(unreadableDir, 0o755) //nolint:errcheck // best-effort cleanup
	})

	restoreWd := testutil.MustChdir(t, tmpDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	// DiscoverAll should not panic when encountering an unreadable directory.
	// It may return an error or an empty result; the key invariant is no panic.
	files, err := d.DiscoverAll()
	if err != nil {
		// Returning an error is acceptable behavior
		return
	}

	// If no error, we expect an empty or non-nil slice (no panic occurred)
	_ = files
}

func TestDiscoverAll_SymlinkToInvkfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}

	tmpDir := t.TempDir()

	// Create a real invkfile.cue in a separate directory
	sourceDir := filepath.Join(tmpDir, "source")
	testutil.MustMkdirAll(t, sourceDir, 0o755)

	invkfileContent := `
cmds: [{name: "symlinked", implementations: [{script: "echo symlinked", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	sourcePath := filepath.Join(sourceDir, "invkfile.cue")
	if err := os.WriteFile(sourcePath, []byte(invkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write source invkfile: %v", err)
	}

	// Create a working directory with a symlink pointing to the real invkfile
	workDir := filepath.Join(tmpDir, "work")
	testutil.MustMkdirAll(t, workDir, 0o755)
	symlinkPath := filepath.Join(workDir, "invkfile.cue")
	if err := os.Symlink(sourcePath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	restoreWd := testutil.MustChdir(t, workDir)
	defer restoreWd()

	cleanupHome := testutil.SetHomeDir(t, tmpDir)
	defer cleanupHome()

	cfg := config.DefaultConfig()
	d := New(cfg)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Discovery should follow the symlink and find the invkfile
	found := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find symlinked invkfile in current directory")
	}
}
