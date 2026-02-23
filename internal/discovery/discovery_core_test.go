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

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	d := New(cfg)

	if d == nil {
		t.Fatal("New() returned nil")
	}

	if d.cfg != cfg {
		t.Error("New() did not set cfg correctly")
	}
}

func TestNew_WithOptions(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	d := New(cfg,
		WithBaseDir("/some/dir"),
		WithCommandsDir("/some/cmds"),
	)

	if d.baseDir != "/some/dir" {
		t.Errorf("WithBaseDir() did not set baseDir: got %q", d.baseDir)
	}
	if d.commandsDir != "/some/cmds" {
		t.Errorf("WithCommandsDir() did not set commandsDir: got %q", d.commandsDir)
	}
}

// TestNew_Defaults verifies that New() without options populates baseDir
// from os.Getwd() and commandsDir from config.CommandsDir(). This ensures
// backward compatibility for callers that don't use functional options.
func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	d := New(cfg)

	// baseDir should equal the current working directory (os.Getwd).
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error: %v", err)
	}
	if d.baseDir != types.FilesystemPath(cwd) {
		t.Errorf("New() baseDir = %q, want os.Getwd() = %q", d.baseDir, cwd)
	}

	// commandsDir should equal config.CommandsDir() when available.
	expectedCmdsDir, err := config.CommandsDir()
	if err != nil {
		t.Fatalf("config.CommandsDir() error: %v", err)
	}
	if d.commandsDir != types.FilesystemPath(expectedCmdsDir) {
		t.Errorf("New() commandsDir = %q, want config.CommandsDir() = %q", d.commandsDir, expectedCmdsDir)
	}

	// The "set" flags should be false since we didn't use options.
	if d.baseDirSet {
		t.Error("New() baseDirSet should be false when no WithBaseDir option is used")
	}
	if d.commandsDirSet {
		t.Error("New() commandsDirSet should be false when no WithCommandsDir option is used")
	}
}

// TestNew_WithBaseDir verifies that WithBaseDir sets the directory and the
// "set" flag, preventing the Getwd fallback from overwriting the value.
func TestNew_WithBaseDir(t *testing.T) {
	t.Parallel()

	customDir := types.FilesystemPath(t.TempDir())
	d := New(config.DefaultConfig(), WithBaseDir(customDir))

	if d.baseDir != customDir {
		t.Errorf("WithBaseDir() baseDir = %q, want %q", d.baseDir, customDir)
	}
	if !d.baseDirSet {
		t.Error("WithBaseDir() should set baseDirSet = true")
	}
}

// TestNew_WithCommandsDir verifies that WithCommandsDir sets the directory
// and the "set" flag, preventing the config.CommandsDir() fallback.
func TestNew_WithCommandsDir(t *testing.T) {
	t.Parallel()

	customDir := types.FilesystemPath(t.TempDir())
	d := New(config.DefaultConfig(), WithCommandsDir(customDir))

	if d.commandsDir != customDir {
		t.Errorf("WithCommandsDir() commandsDir = %q, want %q", d.commandsDir, customDir)
	}
	if !d.commandsDirSet {
		t.Error("WithCommandsDir() should set commandsDirSet = true")
	}
}

// TestNew_WithCommandsDir_Empty verifies that passing an empty string to
// WithCommandsDir disables user-dir discovery (the "set" flag is true,
// so the fallback to config.CommandsDir() is skipped).
func TestNew_WithCommandsDir_Empty(t *testing.T) {
	t.Parallel()

	d := New(config.DefaultConfig(), WithCommandsDir(""))

	if d.commandsDir != "" {
		t.Errorf("WithCommandsDir(\"\") commandsDir = %q, want empty", d.commandsDir)
	}
	if !d.commandsDirSet {
		t.Error("WithCommandsDir(\"\") should set commandsDirSet = true to skip fallback")
	}
}

func TestNewDiscoveredCommandSet(t *testing.T) {
	t.Parallel()

	// Test T004/T005: DiscoveredCommandSet constructor
	set := NewDiscoveredCommandSet()

	if set == nil {
		t.Fatal("NewDiscoveredCommandSet() returned nil")
	}
	// Maps must be non-nil (nil map panics on write); slices can be nil (append works).
	if set.BySimpleName == nil {
		t.Error("BySimpleName should be initialized")
	}
	if set.AmbiguousNames == nil {
		t.Error("AmbiguousNames should be initialized")
	}
	if set.BySource == nil {
		t.Error("BySource should be initialized")
	}
	if len(set.Commands) != 0 {
		t.Error("Commands should be empty")
	}
	if len(set.SourceOrder) != 0 {
		t.Error("SourceOrder should be empty")
	}
}

func TestDiscoverAll_EmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

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
	t.Parallel()

	tmpDir := t.TempDir()

	// Create an invowkfile.cue in the temp directory
	invowkfileContent := `
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
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

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
	t.Parallel()

	tmpDir := t.TempDir()

	// Create an invowkfile (without .cue extension) in the temp directory
	invowkfileContent := `
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
	invowkfilePath := filepath.Join(tmpDir, "invowkfile")
	if err := os.WriteFile(invowkfilePath, []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("DiscoverAll() returned 0 files, want at least 1")
	}
}

func TestDiscoverAll_PrefersInvowkfileCue(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create both invowkfile and invowkfile.cue
	content := `
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Should find invowkfile.cue (preferred) in current dir
	found := false
	for _, f := range files {
		if f.Source == SourceCurrentDir && filepath.Base(string(f.Path)) == "invowkfile.cue" {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() should prefer invowkfile.cue over invowkfile")
	}
}

func TestDiscoverAll_UserDirInvowkfileNotDiscovered(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create user commands directory with a loose invowkfile (not in a module)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	if err := os.MkdirAll(userCmdsDir, 0o755); err != nil {
		t.Fatalf("failed to create user cmds dir: %v", err)
	}

	content := `
cmds: [{name: "user-cmd", implementations: [{script: "echo user", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(userCmdsDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	testutil.MustMkdirAll(t, workDir, 0o755)

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithBaseDir(types.FilesystemPath(workDir)),
		WithCommandsDir(types.FilesystemPath(userCmdsDir)),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Loose invowkfiles in ~/.invowk/cmds/ should NOT be discovered
	if len(files) != 0 {
		t.Errorf("DiscoverAll() returned %d files, want 0 (user-dir invowkfiles should not be discovered)", len(files))
	}
}

func TestDiscoverAll_UserDirNonRecursive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a module in a subdirectory of ~/.invowk/cmds/ (nested)
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	nestedModuleDir := filepath.Join(userCmdsDir, "subdir", "nested.invowkmod")
	createTestModule(t, nestedModuleDir, "nested", "nested-cmd")

	// Create an empty working directory
	workDir := filepath.Join(tmpDir, "work")
	testutil.MustMkdirAll(t, workDir, 0o755)

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithBaseDir(types.FilesystemPath(workDir)),
		WithCommandsDir(types.FilesystemPath(userCmdsDir)),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Modules in subdirectories of ~/.invowk/cmds/ should NOT be discovered
	// (only immediate children are scanned)
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "nested" {
			t.Error("DiscoverAll() should not discover modules in subdirectories of user commands dir")
		}
	}
}

func TestDiscoverAll_FindsModuleFromIncludes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a module directory in a custom path
	modulePath := filepath.Join(tmpDir, "custom-modules", "custom.invowkmod")
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create invowkmod.cue (module metadata, matching folder prefix "custom")
	invowkmodContent := `module: "custom"
version: "1.0.0"
description: "Test module for config includes"
`
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}

	// Create invowkfile.cue with a command
	invowkfileContent := `
cmds: [{name: "custom-cmd", implementations: [{script: "echo custom", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}

	// Use an empty base directory (which has no invowkfile)
	emptyDir := filepath.Join(tmpDir, "empty")
	testutil.MustMkdirAll(t, emptyDir, 0o755)

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(modulePath)},
	}
	d := newTestDiscovery(t, cfg, tmpDir,
		WithBaseDir(types.FilesystemPath(emptyDir)),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Source == SourceModule && f.Module != nil && f.Module.Name() == "custom" {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find module from configured includes")
	}
}

func TestLoadFirst_NoFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	_, err := d.LoadFirst()
	if err == nil {
		t.Error("LoadFirst() should return error when no files found")
	}
	if !errors.Is(err, ErrNoInvowkfileFound) {
		t.Errorf("LoadFirst() error should be ErrNoInvowkfileFound, got: %v", err)
	}
}

func TestErrNoInvowkfileFound_Sentinel(t *testing.T) {
	t.Parallel()

	if ErrNoInvowkfileFound == nil {
		t.Fatal("ErrNoInvowkfileFound should not be nil")
	}
	if ErrNoInvowkfileFound.Error() != "no invowkfile found" {
		t.Errorf("ErrNoInvowkfileFound.Error() = %q, want %q", ErrNoInvowkfileFound.Error(), "no invowkfile found")
	}
}

func TestLoadFirst_WithValidFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// invowkfile.cue now only contains commands (module metadata is in invowkmod.cue for modules)
	content := `
cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	file, err := d.LoadFirst()
	if err != nil {
		t.Fatalf("LoadFirst() returned error: %v", err)
	}

	if file.Invowkfile == nil {
		t.Error("LoadFirst() did not parse the invowkfile")
	}

	if len(file.Invowkfile.Commands) != 1 {
		t.Errorf("Invowkfile should have 1 command, got %d", len(file.Invowkfile.Commands))
	}
}

func TestLoadAll_WithMultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create current dir invowkfile
	content := `
cmds: [{name: "current", implementations: [{script: "echo current", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create a local module (provides second file)
	moduleDir := filepath.Join(tmpDir, "extra.invowkmod")
	createTestModule(t, moduleDir, "extra", "extra-cmd")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

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
	t.Parallel()

	tmpDir := t.TempDir()

	// invowkfile.cue now contains only commands - module metadata is in invowkmod.cue for modules
	content := `
cmds: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}
	commands := result.Set.Commands

	if len(commands) != 2 {
		t.Errorf("DiscoverCommands() returned %d commands, want 2", len(commands))
	}

	// Commands should be sorted by name (no module prefix for current-dir invowkfiles)
	if len(commands) >= 2 {
		if commands[0].Name != "build" || commands[1].Name != "test" {
			t.Errorf("commands not sorted correctly: got %s, %s", commands[0].Name, commands[1].Name)
		}
	}
}

func TestGetCommand(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// invowkfile.cue now contains only commands - no module metadata
	content := `
cmds: [
	{name: "build", description: "Build the project", implementations: [{script: "go build", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "test", description: "Run tests", implementations: [{script: "go test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	t.Run("ExistingCommand", func(t *testing.T) {
		t.Parallel()

		// Current-dir invowkfiles don't have module prefix
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
		t.Parallel()

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
	t.Parallel()

	tmpDir := t.TempDir()

	// invowkfile.cue now contains only commands - no module metadata
	content := `
cmds: [
	{name: "build", implementations: [{script: "go build", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "build-dev", implementations: [{script: "go build -tags dev", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]},
	{name: "test", implementations: [{script: "go test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}

	filterPrefix := func(prefix string) []*CommandInfo {
		matching := make([]*CommandInfo, 0)
		for _, cmd := range result.Set.Commands {
			if prefix == "" || strings.HasPrefix(string(cmd.Name), prefix) {
				matching = append(matching, cmd)
			}
		}
		return matching
	}

	t.Run("EmptyPrefix", func(t *testing.T) {
		t.Parallel()

		commands := filterPrefix("")
		if len(commands) != 3 {
			t.Errorf("prefix filter returned %d commands, want 3", len(commands))
		}
	})

	t.Run("BuildPrefix", func(t *testing.T) {
		t.Parallel()

		commands := filterPrefix("build")
		if len(commands) != 2 {
			t.Errorf("prefix filter returned %d commands, want 2", len(commands))
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		t.Parallel()

		commands := filterPrefix("xyz")
		if len(commands) != 0 {
			t.Errorf("prefix filter returned %d commands, want 0", len(commands))
		}
	})
}

func TestDiscoverCommands_Precedence(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create current dir invowkfile with "build" command
	currentContent := `
cmds: [{name: "build", description: "Current build", implementations: [{script: "echo current", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(currentContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create a local module with a "build" command (different source)
	moduleDir := filepath.Join(tmpDir, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "build")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	result, err := d.DiscoverCommandSet(context.Background())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() returned error: %v", err)
	}

	// Both commands should exist: "build" from invowkfile and "tools build" from module.
	// The root invowkfile "build" and module "tools build" are separate commands.
	// The "build" SimpleName should be ambiguous (invowkfile vs tools).
	if !result.Set.AmbiguousNames["build"] {
		t.Error("'build' should be marked as ambiguous (invowkfile vs module)")
	}

	// Verify current-dir invowkfile command exists
	found := false
	for _, cmd := range result.Set.Commands {
		if cmd.Name == "build" && cmd.Source == SourceCurrentDir {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'build' command from current directory")
	}
}

func TestDiscoverAll_PermissionDenied(t *testing.T) {
	t.Parallel()

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

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithCommandsDir(types.FilesystemPath(unreadableDir)),
	)

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

func TestDiscoverAll_SymlinkToInvowkfile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}

	tmpDir := t.TempDir()

	// Create a real invowkfile.cue in a separate directory
	sourceDir := filepath.Join(tmpDir, "source")
	testutil.MustMkdirAll(t, sourceDir, 0o755)

	invowkfileContent := `
cmds: [{name: "symlinked", implementations: [{script: "echo symlinked", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]
`
	sourcePath := filepath.Join(sourceDir, "invowkfile.cue")
	if err := os.WriteFile(sourcePath, []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write source invowkfile: %v", err)
	}

	// Create a working directory with a symlink pointing to the real invowkfile
	workDir := filepath.Join(tmpDir, "work")
	testutil.MustMkdirAll(t, workDir, 0o755)
	symlinkPath := filepath.Join(workDir, "invowkfile.cue")
	if err := os.Symlink(sourcePath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir,
		WithBaseDir(types.FilesystemPath(workDir)),
	)

	files, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll() returned error: %v", err)
	}

	// Discovery should follow the symlink and find the invowkfile
	found := false
	for _, f := range files {
		if f.Source == SourceCurrentDir {
			found = true
			break
		}
	}

	if !found {
		t.Error("DiscoverAll() did not find symlinked invowkfile in current directory")
	}
}
