// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

type configDirTestCase struct {
	name           string
	goos           string
	getenv         func(string) string
	homeDir        func() (string, error)
	want           types.FilesystemPath
	wantErrorParts []string
}

func TestConfigDir(t *testing.T) {
	t.Parallel()

	fakeHomeDir := filepath.Join("fake", "home")
	fakeHome := func() (string, error) { return fakeHomeDir, nil }
	failHome := func() (string, error) { return "", errors.New("no home") }
	xdgPath := filepath.Join("custom", "xdg")
	appDataPath := filepath.Join("C", "Users", "test", "AppData", "Roaming")
	userProfile := filepath.Join("C", "Users", "test")
	tests := []configDirTestCase{
		{
			name: "linux with XDG_CONFIG_HOME set", goos: "linux",
			getenv: environmentLookup(map[string]string{"XDG_CONFIG_HOME": xdgPath}), homeDir: fakeHome,
			want: types.FilesystemPath(filepath.Join(xdgPath, AppName)),
		},
		{
			name: "linux without XDG_CONFIG_HOME", goos: "linux",
			getenv: environmentLookup(nil), homeDir: fakeHome,
			want: types.FilesystemPath(filepath.Join(fakeHomeDir, ".config", AppName)),
		},
		{
			name: "linux home dir error", goos: "linux",
			getenv: environmentLookup(nil), homeDir: failHome, wantErrorParts: []string{"no home"},
		},
		{
			name: "darwin", goos: "darwin",
			getenv: environmentLookup(nil), homeDir: fakeHome,
			want: types.FilesystemPath(filepath.Join(fakeHomeDir, "Library", "Application Support", AppName)),
		},
		{
			name: "darwin home dir error", goos: "darwin",
			getenv: environmentLookup(nil), homeDir: failHome, wantErrorParts: []string{"no home"},
		},
		{
			name: "windows with APPDATA", goos: "windows",
			getenv: environmentLookup(map[string]string{"APPDATA": appDataPath}), homeDir: fakeHome,
			want: types.FilesystemPath(filepath.Join(appDataPath, AppName)),
		},
		{
			name: "windows without APPDATA", goos: "windows",
			getenv: environmentLookup(map[string]string{"USERPROFILE": userProfile}), homeDir: fakeHome,
			want: types.FilesystemPath(filepath.Join(userProfile, "AppData", "Roaming", AppName)),
		},
		{
			name: "windows without APPDATA or USERPROFILE", goos: "windows",
			getenv: environmentLookup(nil), homeDir: fakeHome, wantErrorParts: []string{"APPDATA", "USERPROFILE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runConfigDirTestCase(t, tt)
		})
	}
}

func TestCommandsDir(t *testing.T) {
	t.Parallel()
	dir, err := CommandsDir()
	if err != nil {
		t.Fatalf("CommandsDir() returned error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := types.FilesystemPath(filepath.Join(home, ".invowk", "cmds"))
	if dir != expected {
		t.Errorf("CommandsDir() = %s, want %s", dir, expected)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	t.Parallel()
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, AppName)

	err := EnsureConfigDir(types.FilesystemPath(configDir))
	if err != nil {
		t.Fatalf("EnsureConfigDir() returned error: %v", err)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("EnsureConfigDir() did not create directory %s", configDir)
	}
}

func TestConfigDirWithOverride(t *testing.T) {
	t.Parallel()

	t.Run("explicit path returned as-is", func(t *testing.T) {
		t.Parallel()
		want := types.FilesystemPath(filepath.Join(t.TempDir(), "explicit-config"))
		dir, err := configDirWithOverride(want)
		if err != nil {
			t.Fatalf("configDirWithOverride() error: %v", err)
		}
		if dir != want {
			t.Errorf("configDirWithOverride() = %q, want %q", dir, want)
		}
	})

	t.Run("invalid explicit path rejected", func(t *testing.T) {
		t.Parallel()
		_, err := configDirWithOverride("   ")
		if !errors.Is(err, types.ErrInvalidFilesystemPath) {
			t.Fatalf("configDirWithOverride() error = %v, want %v", err, types.ErrInvalidFilesystemPath)
		}
	})

	t.Run("empty falls through to ConfigDir", func(t *testing.T) {
		t.Parallel()
		dir, err := configDirWithOverride("")
		if err != nil {
			t.Fatalf("configDirWithOverride() error: %v", err)
		}
		expected, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error: %v", err)
		}
		if dir != expected {
			t.Errorf("configDirWithOverride(\"\") = %q, want ConfigDir() = %q", dir, expected)
		}
	})
}

func TestCommandsDirWithOverride(t *testing.T) {
	t.Parallel()

	t.Run("explicit path returned as-is", func(t *testing.T) {
		t.Parallel()
		want := types.FilesystemPath(filepath.Join(t.TempDir(), "explicit-cmds"))
		dir, err := commandsDirWithOverride(want)
		if err != nil {
			t.Fatalf("commandsDirWithOverride() error: %v", err)
		}
		if dir != want {
			t.Errorf("commandsDirWithOverride() = %q, want %q", dir, want)
		}
	})

	t.Run("invalid explicit path rejected", func(t *testing.T) {
		t.Parallel()
		_, err := commandsDirWithOverride("   ")
		if !errors.Is(err, types.ErrInvalidFilesystemPath) {
			t.Fatalf("commandsDirWithOverride() error = %v, want %v", err, types.ErrInvalidFilesystemPath)
		}
	})

	t.Run("empty falls through to CommandsDir", func(t *testing.T) {
		t.Parallel()
		dir, err := commandsDirWithOverride("")
		if err != nil {
			t.Fatalf("commandsDirWithOverride() error: %v", err)
		}
		expected, err := CommandsDir()
		if err != nil {
			t.Fatalf("CommandsDir() error: %v", err)
		}
		if dir != expected {
			t.Errorf("commandsDirWithOverride(\"\") = %q, want CommandsDir() = %q", dir, expected)
		}
	})
}

// TestConfigDirFrom_UnknownGOOS verifies that an unrecognized GOOS value
// falls through to the default (Linux) case: $XDG_CONFIG_HOME if set,
// otherwise ~/.config.
func TestConfigDirFrom_UnknownGOOS(t *testing.T) {
	t.Parallel()

	fakeHomeDir := filepath.Join("fake", "home")
	fakeHome := func() (string, error) { return fakeHomeDir, nil }

	t.Run("unknown GOOS with XDG_CONFIG_HOME", func(t *testing.T) {
		t.Parallel()
		xdgPath := filepath.Join("custom", "xdg")
		getenv := func(key string) string {
			if key == "XDG_CONFIG_HOME" {
				return xdgPath
			}
			return ""
		}
		dir, err := configDirFrom("freebsd", getenv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(xdgPath, AppName))
		if dir != expected {
			t.Errorf("configDirFrom(freebsd) = %s, want %s", dir, expected)
		}
	})

	t.Run("unknown GOOS without XDG falls back to ~/.config", func(t *testing.T) {
		t.Parallel()
		noEnv := func(string) string { return "" }
		dir, err := configDirFrom("freebsd", noEnv, fakeHome)
		if err != nil {
			t.Fatalf("configDirFrom() error: %v", err)
		}
		expected := types.FilesystemPath(filepath.Join(fakeHomeDir, ".config", AppName))
		if dir != expected {
			t.Errorf("configDirFrom(freebsd) = %s, want %s", dir, expected)
		}
	})
}

func TestEnsureCommandsDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cmdsDir := filepath.Join(tmpDir, "cmds")

	err := EnsureCommandsDir(types.FilesystemPath(cmdsDir))
	if err != nil {
		t.Fatalf("EnsureCommandsDir() returned error: %v", err)
	}

	if _, err := os.Stat(cmdsDir); os.IsNotExist(err) {
		t.Errorf("EnsureCommandsDir() did not create directory %s", cmdsDir)
	}
}

func environmentLookup(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func runConfigDirTestCase(t *testing.T, tt configDirTestCase) {
	t.Helper()

	dir, err := configDirFrom(tt.goos, tt.getenv, tt.homeDir)
	if len(tt.wantErrorParts) > 0 {
		if err == nil {
			t.Fatalf("configDirFrom() error = nil, want error containing %v", tt.wantErrorParts)
		}
		for _, part := range tt.wantErrorParts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("configDirFrom() error = %v, want substring %q", err, part)
			}
		}
		return
	}
	if err != nil {
		t.Fatalf("configDirFrom() error: %v", err)
	}
	if dir != tt.want {
		t.Errorf("configDirFrom() = %s, want %s", dir, tt.want)
	}
}
